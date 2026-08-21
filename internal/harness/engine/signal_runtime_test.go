package engine

import (
	"context"
	"testing"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestSignalBeforeWaitIsPersistedThenConsumedOnActivation(t *testing.T) {
	ctx := context.Background()
	eng, db, _, _ := newTestEngine(t)
	run, err := eng.StartWorkflow(ctx, waitDefinition("signal-before-wait"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "wait")
	first, err := eng.SendSignal(ctx, run.ID, "external.ready", "message-1", []byte(`{"ready":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || first.DeliveredToNodeRun != "" || first.Signal.State != harnessmodel.SignalPending {
		t.Fatalf("signal was not durably buffered before waiter: %+v", first)
	}
	wait, err := eng.WaitForSignal(ctx, node.ID, "external.ready")
	if err != nil {
		t.Fatal(err)
	}
	if wait.State != harnessmodel.SignalWaitDelivered || wait.DeliveredSignalID != first.Signal.ID {
		t.Fatalf("buffered signal not delivered to waiter: %+v", wait)
	}
	if got := nodeRunFor(t, db, run.ID, "wait"); got.State != harnessmodel.NodeReady {
		t.Fatalf("node not READY after buffered signal delivery: %+v", got)
	}

	replay, err := eng.SendSignal(ctx, run.ID, "external.ready", "message-1", []byte(`{"ready":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if replay.Created || replay.Signal.ID != first.Signal.ID || replay.DeliveredToNodeRun != node.ID {
		t.Fatalf("signal replay was not idempotent: %+v", replay)
	}
	var count int
	if err := db.SQLDB().QueryRowContext(ctx, `SELECT COUNT(*) FROM signals WHERE workflow_run_id=?`, string(run.ID)).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("signal replay created %d durable rows, want 1", count)
	}
}

func TestWaitBeforeSignalWakesNodeAtomically(t *testing.T) {
	ctx := context.Background()
	eng, db, _, _ := newTestEngine(t)
	run, err := eng.StartWorkflow(ctx, waitDefinition("wait-before-signal"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "wait")
	wait, err := eng.WaitForSignal(ctx, node.ID, "external.done")
	if err != nil {
		t.Fatal(err)
	}
	if wait.State != harnessmodel.SignalWaitWaiting || nodeRunFor(t, db, run.ID, "wait").State != harnessmodel.NodeWaiting {
		t.Fatalf("waiter was not persisted: %+v", wait)
	}
	result, err := eng.SendSignal(ctx, run.ID, "external.done", "message-2", []byte("ok"))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.DeliveredToNodeRun != node.ID || result.Signal.State != harnessmodel.SignalConsumed {
		t.Fatalf("signal did not match active waiter: %+v", result)
	}
	if got := nodeRunFor(t, db, run.ID, "wait"); got.State != harnessmodel.NodeReady {
		t.Fatalf("delivered signal did not restore READY: %+v", got)
	}
}

func TestSignalDeliveryWhilePausedWakesDurableNodeButWorkflowGateStaysClosed(t *testing.T) {
	ctx := context.Background()
	eng, db, _, _ := newTestEngine(t)
	run, err := eng.StartWorkflow(ctx, waitDefinition("signal-paused"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "wait")
	if _, err := eng.WaitForSignal(ctx, node.ID, "resume.input"); err != nil {
		t.Fatal(err)
	}
	pause, err := eng.PauseWorkflow(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pause.WorkflowRun.State != harnessmodel.WorkflowPaused {
		t.Fatalf("workflow not PAUSED: %+v", pause)
	}
	result, err := eng.SendSignal(ctx, run.ID, "resume.input", "message-paused", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.DeliveredToNodeRun != node.ID || nodeRunFor(t, db, run.ID, "wait").State != harnessmodel.NodeReady {
		t.Fatalf("paused signal did not durably wake node: %+v", result)
	}
	if got := workflowRun(t, db, run.ID); got.State != harnessmodel.WorkflowPaused {
		t.Fatalf("signal delivery reopened workflow gate: %+v", got)
	}
}
