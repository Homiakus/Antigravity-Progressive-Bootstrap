package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	harnesslease "github.com/homiakus/agctl/internal/harness/lease"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestReclaimedClaimedAttemptRejectsOldFenceAcrossCompletionAPIs(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-old-fence", clock.current)
	addActiveWorker(t, ctx, db, "worker-new-fence", clock.current)

	run, err := eng.StartWorkflow(ctx, retryDefinition("fence-order-all-completions", transientPolicy()))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-old-fence", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if claim.Attempt.State != harnessmodel.AttemptClaimed {
		t.Fatalf("claimed attempt state=%s want CLAIMED", claim.Attempt.State)
	}

	clock.current = claim.Lease.ExpiresAt.Add(time.Second)
	newLease, err := eng.ReclaimExpiredAttempt(ctx, claim.Attempt.ID, "worker-new-fence", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if newLease.Epoch != claim.Lease.Epoch+1 {
		t.Fatalf("reclaim epoch=%d want=%d", newLease.Epoch, claim.Lease.Epoch+1)
	}

	if _, err := eng.CompleteAttemptSuccessFenced(ctx, claim.Attempt.ID, "worker-old-fence", claim.Lease.Epoch); !errors.Is(err, harnesslease.ErrStaleFence) {
		t.Fatalf("stale success completion error=%v want ErrStaleFence", err)
	}
	if _, err := eng.CompleteAttemptFailureFenced(ctx, claim.Attempt.ID, "worker-old-fence", claim.Lease.Epoch, string(harnessmodel.ErrorInfraTransient), "old worker"); !errors.Is(err, harnesslease.ErrStaleFence) {
		t.Fatalf("stale failure completion error=%v want ErrStaleFence", err)
	}
	if _, err := eng.CompleteAttemptFailureWithRetryFenced(ctx, claim.Attempt.ID, "worker-old-fence", claim.Lease.Epoch, harnessmodel.Failure{Class: harnessmodel.ErrorInfraTransient, Message: "old worker retry"}); !errors.Is(err, harnesslease.ErrStaleFence) {
		t.Fatalf("stale retry completion error=%v want ErrStaleFence", err)
	}

	persisted := attemptRunFor(t, db, claim.Attempt.ID)
	if persisted.State != harnessmodel.AttemptClaimed || persisted.WorkerID != "worker-new-fence" || persisted.LeaseEpoch != newLease.Epoch {
		t.Fatalf("stale completion mutated reclaimed attempt: %+v", persisted)
	}
}
