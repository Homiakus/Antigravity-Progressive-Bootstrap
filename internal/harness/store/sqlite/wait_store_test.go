package sqlite

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func seedWaitNode(t *testing.T, db *DB, now time.Time) harnessmodel.NodeRun {
	t.Helper()
	seedRun(t, db, now)
	node := harnessmodel.NodeRun{
		ID: "nr_wait", WorkflowRunID: "wfr_test",
		NodeID: "a", GraphRevision: 1, Generation: 1, State: harnessmodel.NodeWaiting,
		RemainingDependencies: 0, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Update(context.Background(), func(tx harnessstore.Tx) error {
		if err := tx.CreateGraphRevision(context.Background(), harnessmodel.GraphRevision{WorkflowRunID: "wfr_test", Number: 1, CreatedAt: now, Reason: "wait fixture"}); err != nil {
			return err
		}
		if err := tx.CreateWorkflowProgress(context.Background(), harnessmodel.WorkflowProgress{WorkflowRunID: "wfr_test", TotalNodes: 1, UpdatedAt: now}); err != nil {
			return err
		}
		return tx.CreateNodeRun(context.Background(), node)
	}); err != nil {
		t.Fatal(err)
	}
	return node
}

func TestDurableTimerDueOrderingAndCAS(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(20_000, 123).UTC()
	node := seedWaitNode(t, db, now)
	timer := harnessmodel.Timer{
		ID: "tmr_test", WorkflowRunID: node.WorkflowRunID, NodeRunID: node.ID,
		Kind: harnessmodel.TimerNodeWait, State: harnessmodel.TimerPending,
		DueAt: now.Add(time.Hour), CreatedAt: now, Payload: []byte(`{"reason":"delay"}`),
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateTimer(ctx, timer) }); err != nil {
		t.Fatal(err)
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		due, err := r.ListDueTimers(ctx, now.Add(59*time.Minute), 10)
		if err != nil {
			return err
		}
		if len(due) != 0 {
			t.Fatalf("timer fired early: %+v", due)
		}
		due, err = r.ListDueTimers(ctx, now.Add(time.Hour), 10)
		if err != nil {
			return err
		}
		if len(due) != 1 || due[0].ID != timer.ID || !bytes.Equal(due[0].Payload, timer.Payload) {
			t.Fatalf("unexpected due timers: %+v", due)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	timer.State = harnessmodel.TimerFired
	timer.ResolvedAt = now.Add(time.Hour)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CompareAndSwapTimer(ctx, harnessmodel.TimerPending, timer); err != nil {
			return err
		}
		if err := tx.CompareAndSwapTimer(ctx, harnessmodel.TimerPending, timer); !errors.Is(err, harnessstore.ErrConflict) {
			t.Fatalf("stale timer CAS=%v want ErrConflict", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSignalArrivesBeforeWaiterAndDeduplicates(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(30_000, 0).UTC()
	node := seedWaitNode(t, db, now)
	signal := harnessmodel.Signal{
		ID: "sig_first", WorkflowRunID: node.WorkflowRunID, Name: "operator.ready", MessageID: "msg-1",
		Payload: []byte(`{"ready":true}`), State: harnessmodel.SignalPending, ReceivedAt: now.Add(time.Minute),
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		stored, created, err := tx.PutSignal(ctx, signal)
		if err != nil {
			return err
		}
		if !created || stored.ID != signal.ID {
			t.Fatalf("first signal was not created: created=%v stored=%+v", created, stored)
		}
		replay := signal
		replay.ID = "sig_duplicate"
		stored, created, err = tx.PutSignal(ctx, replay)
		if err != nil {
			return err
		}
		if created || stored.ID != signal.ID {
			t.Fatalf("dedupe did not recover original signal: created=%v stored=%+v", created, stored)
		}
		replay.Payload = []byte(`{"ready":false}`)
		if _, _, err := tx.PutSignal(ctx, replay); !errors.Is(err, harnessstore.ErrConflict) {
			t.Fatalf("different-payload replay=%v want ErrConflict", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	wait := harnessmodel.SignalWait{NodeRunID: node.ID, WorkflowRunID: node.WorkflowRunID, SignalName: signal.Name, State: harnessmodel.SignalWaitWaiting, CreatedAt: now.Add(2 * time.Minute)}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CreateSignalWait(ctx, wait); err != nil {
			return err
		}
		pending, err := tx.ListPendingSignals(ctx, node.WorkflowRunID, signal.Name, 10)
		if err != nil {
			return err
		}
		if len(pending) != 1 || pending[0].ID != signal.ID {
			t.Fatalf("pre-wait signal disappeared: %+v", pending)
		}
		return tx.DeliverSignal(ctx, node.ID, signal.ID, now.Add(3*time.Minute))
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		gotSignal, err := r.GetSignal(ctx, signal.ID)
		if err != nil {
			return err
		}
		gotWait, err := r.GetSignalWait(ctx, node.ID)
		if err != nil {
			return err
		}
		if gotSignal.State != harnessmodel.SignalConsumed || gotSignal.ConsumedByNodeRunID != node.ID {
			t.Fatalf("signal not consumed atomically: %+v", gotSignal)
		}
		if gotWait.State != harnessmodel.SignalWaitDelivered || gotWait.DeliveredSignalID != signal.ID {
			t.Fatalf("wait not delivered atomically: %+v", gotWait)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestApprovalLifecycleAndPendingQuery(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(40_000, 0).UTC()
	node := seedWaitNode(t, db, now)
	approval := harnessmodel.Approval{
		ID: "apr_test", WorkflowRunID: node.WorkflowRunID, NodeRunID: node.ID,
		RequestedCapability: "filesystem.write", Risk: "HIGH", Reason: "publish generated artifact",
		RequestedAt: now, ExpiresAt: now.Add(time.Hour), State: harnessmodel.ApprovalPending,
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateApproval(ctx, approval) }); err != nil {
		t.Fatal(err)
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		pending, err := r.ListPendingApprovals(ctx, node.WorkflowRunID, 10)
		if err != nil {
			return err
		}
		if len(pending) != 1 || pending[0].ID != approval.ID {
			t.Fatalf("unexpected pending approvals: %+v", pending)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	approval.State = harnessmodel.ApprovalApproved
	approval.Actor = "operator@example"
	approval.ResolvedAt = now.Add(30 * time.Minute)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CompareAndSwapApproval(ctx, harnessmodel.ApprovalPending, approval); err != nil {
			return err
		}
		if err := tx.CompareAndSwapApproval(ctx, harnessmodel.ApprovalPending, approval); !errors.Is(err, harnessstore.ErrConflict) {
			t.Fatalf("stale approval CAS=%v want ErrConflict", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestDurableWaitsSurviveDatabaseReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	now := time.Unix(50_000, 0).UTC()
	first, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	node := seedWaitNode(t, first, now)
	timer := harnessmodel.Timer{ID: "tmr_reopen", WorkflowRunID: node.WorkflowRunID, NodeRunID: node.ID, Kind: harnessmodel.TimerNodeWait, State: harnessmodel.TimerPending, DueAt: now.Add(24 * time.Hour), CreatedAt: now}
	signal := harnessmodel.Signal{ID: "sig_reopen", WorkflowRunID: node.WorkflowRunID, Name: "resume", MessageID: "before-restart", State: harnessmodel.SignalPending, ReceivedAt: now.Add(time.Minute)}
	if err := first.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CreateTimer(ctx, timer); err != nil {
			return err
		}
		_, _, err := tx.PutSignal(ctx, signal)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if err := second.View(ctx, func(r harnessstore.Reader) error {
		gotTimer, err := r.GetTimer(ctx, timer.ID)
		if err != nil {
			return err
		}
		gotSignal, err := r.GetSignal(ctx, signal.ID)
		if err != nil {
			return err
		}
		if gotTimer.State != harnessmodel.TimerPending || !gotTimer.DueAt.Equal(timer.DueAt) || gotSignal.State != harnessmodel.SignalPending {
			t.Fatalf("durable waits did not survive reopen: timer=%+v signal=%+v", gotTimer, gotSignal)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
