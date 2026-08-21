package engine

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

func openRestartEngine(t *testing.T, path string, now *time.Time) (*sqlitestore.DB, *Engine) {
	t.Helper()
	db, err := sqlitestore.Open(context.Background(), path, sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	eng, err := New(db, Options{Now: func() time.Time { return *now }})
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	return db, eng
}

func nodeRunByID(t *testing.T, db *sqlitestore.DB, id harnessmodel.NodeRunID) harnessmodel.NodeRun {
	t.Helper()
	var out harnessmodel.NodeRun
	if err := db.View(context.Background(), func(r harnessstore.Reader) error {
		var err error
		out, err = r.GetNodeRun(context.Background(), id)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSignalInboxSurvivesEngineRestartBeforeWaiter(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	now := time.Unix(100_000, 0).UTC()
	firstDB, first := openRestartEngine(t, path, &now)
	run, err := first.StartWorkflow(ctx, waitDefinition("restart-signal"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, firstDB, run.ID, "wait")
	if _, err := first.SendSignal(ctx, run.ID, "external.ready", "message-before-restart", []byte(`{"ready":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := firstDB.Close(); err != nil {
		t.Fatal(err)
	}

	now = now.Add(10 * time.Minute)
	secondDB, second := openRestartEngine(t, path, &now)
	defer secondDB.Close()
	wait, err := second.WaitForSignal(ctx, node.ID, "external.ready")
	if err != nil {
		t.Fatal(err)
	}
	if wait.State != harnessmodel.SignalWaitDelivered || wait.DeliveredSignalID == "" {
		t.Fatalf("pre-restart signal was not delivered after restart: %+v", wait)
	}
	if got := nodeRunByID(t, secondDB, node.ID); got.State != harnessmodel.NodeReady {
		t.Fatalf("signal waiter state after restart=%s want READY", got.State)
	}
}

func TestDurableTimerFiresAfterEngineRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	now := time.Unix(110_000, 0).UTC()
	firstDB, first := openRestartEngine(t, path, &now)
	run, err := first.StartWorkflow(ctx, waitDefinition("restart-timer"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, firstDB, run.ID, "wait")
	due := now.Add(6 * time.Hour)
	timer, err := first.WaitUntil(ctx, node.ID, due, []byte("restart-proof"))
	if err != nil {
		t.Fatal(err)
	}
	if err := firstDB.Close(); err != nil {
		t.Fatal(err)
	}

	now = due.Add(time.Second)
	secondDB, second := openRestartEngine(t, path, &now)
	defer secondDB.Close()
	released, err := second.ReleaseDueTimers(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(released) != 1 || released[0] != node.ID {
		t.Fatalf("timer did not recover after restart: %+v", released)
	}
	if err := secondDB.View(ctx, func(r harnessstore.Reader) error {
		stored, err := r.GetTimer(ctx, timer.ID)
		if err != nil {
			return err
		}
		if stored.State != harnessmodel.TimerFired {
			t.Fatalf("timer state after restart=%s want FIRED", stored.State)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestApprovalExpirySurvivesEngineRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	now := time.Unix(120_000, 0).UTC()
	firstDB, first := openRestartEngine(t, path, &now)
	run, err := first.StartWorkflow(ctx, oneNodeDefinition("restart-approval"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, firstDB, run.ID, "a")
	approval, err := first.RequestApproval(ctx, node.ID, "deploy.production", "HIGH", "restart expiry", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := firstDB.Close(); err != nil {
		t.Fatal(err)
	}

	now = approval.ExpiresAt.Add(time.Second)
	secondDB, second := openRestartEngine(t, path, &now)
	defer secondDB.Close()
	expired, err := second.ReleaseExpiredApprovals(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 1 || expired[0] != approval.ID {
		t.Fatalf("approval expiry did not recover after restart: %+v", expired)
	}
	if got := nodeRunByID(t, secondDB, node.ID); got.State != harnessmodel.NodeTimedOut {
		t.Fatalf("approval node after restart=%s want TIMED_OUT", got.State)
	}
	if got := workflowRun(t, secondDB, run.ID); got.State != harnessmodel.WorkflowFailed {
		t.Fatalf("approval workflow after restart=%s want FAILED", got.State)
	}
}
