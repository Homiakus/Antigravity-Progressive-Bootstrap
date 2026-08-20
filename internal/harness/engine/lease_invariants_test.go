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

func TestLeaseHeartbeatDoesNotAmplifyEventHistory(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-events-a", clock.current)
	addActiveWorker(t, ctx, db, "worker-events-b", clock.current)
	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("lease-event-history"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-events-a", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, "worker-events-a", claim.Lease.Epoch); err != nil {
		t.Fatal(err)
	}

	var beforeHeartbeat int
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		events, err := reader.ListEvents(ctx, run.ID, 0, 1000)
		beforeHeartbeat = len(events)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := eng.HeartbeatLease(ctx, claim.Attempt.ID, "worker-events-a", claim.Lease.Epoch, 5*time.Second); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		events, err := reader.ListEvents(ctx, run.ID, 0, 1000)
		if err != nil {
			return err
		}
		if len(events) != beforeHeartbeat {
			t.Fatalf("heartbeat amplified history: before=%d after=%d", beforeHeartbeat, len(events))
		}
		for _, event := range events {
			if event.Type == "LeaseHeartbeat" || event.Type == "WorkerHeartbeat" {
				t.Fatalf("heartbeat event unexpectedly persisted: %+v", event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	current, err := currentLease(ctx, db, claim.Attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	clock.current = current.ExpiresAt.Add(time.Second)
	if _, err := eng.ReclaimExpiredAttempt(ctx, claim.Attempt.ID, "worker-events-b", 30*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		events, err := reader.ListEvents(ctx, run.ID, 0, 1000)
		if err != nil {
			return err
		}
		counts := map[string]int{}
		for _, event := range events {
			counts[event.Type]++
		}
		if counts["LeaseClaimed"] != 1 || counts["LeaseLost"] != 1 || counts["LeaseReclaimed"] != 1 {
			t.Fatalf("unexpected lease event counts: %+v", counts)
		}
		for i, event := range events {
			if event.WorkflowSeq != int64(i+1) {
				t.Fatalf("event sequence gap at %d: %+v", i, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func currentLease(ctx context.Context, db harnessstore.Store, attemptID harnessmodel.AttemptID) (harnessmodel.Lease, error) {
	var lease harnessmodel.Lease
	err := db.View(ctx, func(reader harnessstore.Reader) error {
		var err error
		lease, err = reader.GetCurrentLease(ctx, attemptID)
		return err
	})
	return lease, err
}

func TestOnlyOneReclaimCanAdvanceFenceEpoch(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-owner", clock.current)
	addActiveWorker(t, ctx, db, "worker-reclaimer-a", clock.current)
	addActiveWorker(t, ctx, db, "worker-reclaimer-b", clock.current)
	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("single-reclaim"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-owner", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	clock.current = claim.Lease.ExpiresAt.Add(time.Second)
	winner, err := eng.ReclaimExpiredAttempt(ctx, claim.Attempt.ID, "worker-reclaimer-a", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if winner.Epoch != 2 {
		t.Fatalf("winner epoch=%d want=2", winner.Epoch)
	}
	if _, err := eng.ReclaimExpiredAttempt(ctx, claim.Attempt.ID, "worker-reclaimer-b", 30*time.Second); err == nil {
		t.Fatal("second reclaimer unexpectedly advanced an active lease")
	}
	current, err := currentLease(ctx, db, claim.Attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Epoch != 2 || current.WorkerID != "worker-reclaimer-a" {
		t.Fatalf("current ownership changed after losing reclaim: %+v", current)
	}
	if _, err := eng.CompleteAttemptSuccessFenced(ctx, claim.Attempt.ID, "worker-owner", 1); !errors.Is(err, harnesslease.ErrStaleFence) {
		t.Fatalf("old fence completion error=%v want ErrStaleFence", err)
	}
}

func TestExpiredLeaseCannotCommitBeforeReclaim(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-expired-result", clock.current)
	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("expired-result"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-expired-result", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, "worker-expired-result", claim.Lease.Epoch); err != nil {
		t.Fatal(err)
	}
	clock.current = claim.Lease.ExpiresAt.Add(time.Second)
	if _, err := eng.CompleteAttemptSuccessFenced(ctx, claim.Attempt.ID, "worker-expired-result", claim.Lease.Epoch); !errors.Is(err, harnesslease.ErrLeaseExpired) {
		t.Fatalf("expired completion error=%v want ErrLeaseExpired", err)
	}
	attemptState := harnessmodel.AttemptState("")
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		attempt, err := reader.GetAttempt(ctx, claim.Attempt.ID)
		attemptState = attempt.State
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if attemptState != harnessmodel.AttemptRunning {
		t.Fatalf("expired result mutated attempt state: %s", attemptState)
	}
}
