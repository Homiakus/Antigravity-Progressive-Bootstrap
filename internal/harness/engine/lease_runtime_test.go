package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	harnesslease "github.com/homiakus/agctl/internal/harness/lease"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func addActiveWorker(t *testing.T, ctx context.Context, db harnessstore.Store, id harnessmodel.WorkerID, at time.Time) {
	t.Helper()
	err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.UpsertWorker(ctx, harnessmodel.Worker{
			ID: id, Name: string(id), State: harnessmodel.WorkerActive, Trust: harnessmodel.WorkerTrustedLocal,
			CreatedAt: at, LastSeenAt: at,
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func oneNodeDefinition(id string) harnessmodel.WorkflowDefinition {
	def := dagDefinition()
	def.ID = harnessmodel.WorkflowDefinitionID(id)
	def.Name = id
	def.Nodes = def.Nodes[:1]
	return def
}

func TestClaimStartHeartbeatAndFencedCompletion(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-a", clock.current)
	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("leased-happy"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-a", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if claim.NodeRun.State != harnessmodel.NodeQueued || claim.Attempt.State != harnessmodel.AttemptClaimed || claim.Lease.Epoch != 1 || claim.Lease.State != harnessmodel.LeaseActive {
		t.Fatalf("unexpected claim: %+v", claim)
	}
	attempt, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, "worker-a", 1)
	if err != nil {
		t.Fatal(err)
	}
	if attempt.State != harnessmodel.AttemptRunning || attempt.WorkerID != "worker-a" || attempt.LeaseEpoch != 1 {
		t.Fatalf("unexpected started attempt: %+v", attempt)
	}
	if _, err := eng.CompleteAttemptSuccess(ctx, attempt.ID); err == nil {
		t.Fatal("unfenced completion unexpectedly accepted for leased attempt")
	}
	before := claim.Lease.ExpiresAt
	heartbeat, err := eng.HeartbeatLease(ctx, attempt.ID, "worker-a", 1, 20*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !heartbeat.ExpiresAt.After(before) {
		t.Fatalf("heartbeat did not extend lease: before=%s after=%s", before, heartbeat.ExpiresAt)
	}
	completed, err := eng.CompleteAttemptSuccessFenced(ctx, attempt.ID, "worker-a", 1)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Attempt.State != harnessmodel.AttemptSucceeded || completed.WorkflowRun.State != harnessmodel.WorkflowSucceeded {
		t.Fatalf("unexpected fenced completion: %+v", completed)
	}
	duplicate, err := eng.CompleteAttemptSuccessFenced(ctx, attempt.ID, "worker-a", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate.Idempotent {
		t.Fatal("duplicate fenced success was not idempotent")
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		_, err := reader.GetCurrentLease(ctx, attempt.ID)
		return err
	}); !errors.Is(err, harnessstore.ErrNotFound) {
		t.Fatalf("released lease still active: %v", err)
	}
}

func TestExpiredLeaseReclaimFencesOldWorker(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-old", clock.current)
	addActiveWorker(t, ctx, db, "worker-new", clock.current)
	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("lease-reclaim"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-old", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, "worker-old", claim.Lease.Epoch); err != nil {
		t.Fatal(err)
	}
	clock.current = claim.Lease.ExpiresAt.Add(time.Second)
	newLease, err := eng.ReclaimExpiredAttempt(ctx, claim.Attempt.ID, "worker-new", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if newLease.Epoch != 2 || newLease.WorkerID != "worker-new" {
		t.Fatalf("unexpected reclaimed lease: %+v", newLease)
	}
	if _, err := eng.CompleteAttemptSuccessFenced(ctx, claim.Attempt.ID, "worker-old", 1); !errors.Is(err, harnesslease.ErrStaleFence) {
		t.Fatalf("stale worker completion error=%v want ErrStaleFence", err)
	}
	if _, err := eng.HeartbeatLease(ctx, claim.Attempt.ID, "worker-old", 1, 30*time.Second); !errors.Is(err, harnesslease.ErrStaleFence) {
		t.Fatalf("stale worker heartbeat error=%v want ErrStaleFence", err)
	}
	if _, err := eng.CompleteAttemptSuccessFenced(ctx, claim.Attempt.ID, "worker-new", 2); err != nil {
		t.Fatalf("new owner could not complete: %v", err)
	}
}

func TestWorkerDiesBeforeExecutionCanBeReclaimed(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-first", clock.current)
	addActiveWorker(t, ctx, db, "worker-second", clock.current)
	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("claim-before-start"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-first", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	clock.current = claim.Lease.ExpiresAt.Add(time.Second)
	lease2, err := eng.ReclaimExpiredAttempt(ctx, claim.Attempt.ID, "worker-second", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, "worker-first", 1); !errors.Is(err, harnesslease.ErrStaleFence) {
		t.Fatalf("old owner started reclaimed attempt: %v", err)
	}
	started, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, "worker-second", lease2.Epoch)
	if err != nil {
		t.Fatal(err)
	}
	if started.State != harnessmodel.AttemptRunning {
		t.Fatalf("reclaimed claimed attempt did not start: %+v", started)
	}
}

func TestHeartbeatAfterExpiryIsRejected(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-expired", clock.current)
	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("heartbeat-expired"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-expired", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	clock.current = claim.Lease.ExpiresAt.Add(time.Second)
	if _, err := eng.HeartbeatLease(ctx, claim.Attempt.ID, "worker-expired", 1, 30*time.Second); !errors.Is(err, harnesslease.ErrLeaseExpired) {
		t.Fatalf("expired heartbeat error=%v want ErrLeaseExpired", err)
	}
}
