package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func waitDefinition(id string) harnessmodel.WorkflowDefinition {
	def := dagDefinition()
	def.ID = harnessmodel.WorkflowDefinitionID(id)
	def.Name = id
	def.Nodes = []harnessmodel.NodeSpec{{ID: "wait", Kind: harnessmodel.NodeKindWait, CachePolicy: harnessmodel.CacheDisabled}}
	return def
}

func TestDurableNodeTimerRemovesReadyProjectionAndWakesAtDeadline(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	run, err := eng.StartWorkflow(ctx, waitDefinition("timer-wakeup"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "wait")
	due := clock.current.Add(24 * time.Hour)
	timer, err := eng.WaitUntil(ctx, node.ID, due, []byte(`{"purpose":"overnight"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := nodeRunFor(t, db, run.ID, "wait"); got.State != harnessmodel.NodeWaiting {
		t.Fatalf("wait node state=%s want WAITING", got.State)
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		_, err := r.GetReadyNode(ctx, node.ID)
		return err
	}); !errors.Is(err, harnessstore.ErrNotFound) {
		t.Fatalf("WAITING node remained in ready queue: %v", err)
	}

	clock.current = due.Add(-2 * time.Second)
	if released, err := eng.ReleaseDueTimers(ctx, 10); err != nil {
		t.Fatal(err)
	} else if len(released) != 0 {
		t.Fatalf("timer fired early: %+v", released)
	}
	clock.current = due
	released, err := eng.ReleaseDueTimers(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(released) != 1 || released[0] != node.ID {
		t.Fatalf("unexpected timer release: %+v", released)
	}
	if got := nodeRunFor(t, db, run.ID, "wait"); got.State != harnessmodel.NodeReady {
		t.Fatalf("timer did not restore READY: %+v", got)
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		stored, err := r.GetTimer(ctx, timer.ID)
		if err != nil {
			return err
		}
		if stored.State != harnessmodel.TimerFired || stored.ResolvedAt.IsZero() {
			t.Fatalf("timer not durably FIRED: %+v", stored)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTimerCanWakeWhilePausedButCannotDispatchUntilResume(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-timer-pause", clock.current)
	run, err := eng.StartWorkflow(ctx, waitDefinition("timer-paused"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "wait")
	due := clock.current.Add(time.Hour)
	if _, err := eng.WaitUntil(ctx, node.ID, due, nil); err != nil {
		t.Fatal(err)
	}
	pause, err := eng.PauseWorkflow(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pause.WorkflowRun.State != harnessmodel.WorkflowPaused {
		t.Fatalf("wait-only workflow did not pause immediately: %+v", pause)
	}
	clock.current = due
	released, err := eng.ReleaseDueTimers(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(released) != 1 {
		t.Fatalf("paused timer did not wake durable node: %+v", released)
	}
	node = nodeRunFor(t, db, run.ID, "wait")
	if node.State != harnessmodel.NodeReady {
		t.Fatalf("paused timer wake state=%s want READY", node.State)
	}
	if _, err := eng.ClaimNode(ctx, node.ID, "worker-timer-pause", 30*time.Second); err == nil {
		t.Fatal("timer-woken node dispatched while workflow PAUSED")
	}
	if _, err := eng.ResumeWorkflow(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	// WAIT nodes are control-flow nodes and normally do not create Attempts;
	// this assertion verifies the durable workflow gate instead through the
	// state itself rather than forcing an ACTION claim.
	if got := workflowRun(t, db, run.ID); got.State != harnessmodel.WorkflowRunning {
		t.Fatalf("workflow did not resume: %+v", got)
	}
}
