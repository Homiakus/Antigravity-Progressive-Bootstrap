package engine

import (
	"context"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestPauseIdleWorkflowStopsClaimsAndResumesSameRun(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-pause-idle", clock.current)
	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("pause-idle"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")

	paused, err := eng.PauseWorkflow(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if paused.WorkflowRun.State != harnessmodel.WorkflowPaused || paused.ActiveAttempts != 0 {
		t.Fatalf("unexpected idle pause result: %+v", paused)
	}
	if _, err := eng.ClaimNode(ctx, node.ID, "worker-pause-idle", 10*time.Second); err == nil {
		t.Fatal("claim unexpectedly succeeded while workflow is PAUSED")
	}

	resumed, err := eng.ResumeWorkflow(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.ID != run.ID || resumed.State != harnessmodel.WorkflowRunning {
		t.Fatalf("resume created/returned wrong run: got=%+v original=%s", resumed, run.ID)
	}
	if _, err := eng.ClaimNode(ctx, node.ID, "worker-pause-idle", 10*time.Second); err != nil {
		t.Fatalf("claim after resume: %v", err)
	}
}

func TestPauseDrainsOwnedAttemptReleasesDependenciesThenPauses(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-drain", clock.current)
	run, err := eng.StartWorkflow(ctx, dagDefinition())
	if err != nil {
		t.Fatal(err)
	}
	root := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, root.ID, "worker-drain", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, "worker-drain", claim.Lease.Epoch); err != nil {
		t.Fatal(err)
	}

	pause, err := eng.PauseWorkflow(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pause.WorkflowRun.State != harnessmodel.WorkflowPausing || pause.ActiveAttempts != 1 {
		t.Fatalf("pause did not enter drain state: %+v", pause)
	}
	if _, err := eng.ResumeWorkflow(ctx, run.ID); err == nil {
		t.Fatal("resume unexpectedly succeeded before active attempt drained")
	}

	completion, err := eng.CompleteAttemptSuccessFenced(ctx, claim.Attempt.ID, "worker-drain", claim.Lease.Epoch)
	if err != nil {
		t.Fatal(err)
	}
	if completion.WorkflowRun.State != harnessmodel.WorkflowPaused {
		t.Fatalf("last draining attempt did not finalize PAUSED: %+v", completion.WorkflowRun)
	}
	for _, childID := range []harnessmodel.NodeID{"b", "c"} {
		child := nodeRunFor(t, db, run.ID, childID)
		if child.State != harnessmodel.NodeReady || child.RemainingDependencies != 0 {
			t.Fatalf("dependency %s was not released during PAUSING: %+v", childID, child)
		}
		if _, err := eng.ClaimNode(ctx, child.ID, "worker-drain", 30*time.Second); err == nil {
			t.Fatalf("child %s was claimable before resume", childID)
		}
	}

	resumed, err := eng.ResumeWorkflow(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.State != harnessmodel.WorkflowRunning || resumed.ID != run.ID {
		t.Fatalf("unexpected resumed workflow: %+v", resumed)
	}
	child := nodeRunFor(t, db, run.ID, "b")
	if _, err := eng.ClaimNode(ctx, child.ID, "worker-drain", 30*time.Second); err != nil {
		t.Fatalf("released child not claimable after resume: %v", err)
	}
}
