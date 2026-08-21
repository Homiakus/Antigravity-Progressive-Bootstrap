package engine

import (
	"context"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestRetryFailureDuringPauseDrainStaysDurableUntilResume(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-retry-pause", clock.current)
	run, err := eng.StartWorkflow(ctx, retryDefinition("retry-pause", transientPolicy()))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-retry-pause", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, "worker-retry-pause", claim.Lease.Epoch); err != nil {
		t.Fatal(err)
	}
	pause, err := eng.PauseWorkflow(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pause.WorkflowRun.State != harnessmodel.WorkflowPausing {
		t.Fatalf("workflow did not enter PAUSING: %+v", pause)
	}

	failure := harnessmodel.Failure{Class: harnessmodel.ErrorInfraTransient, Message: "provider transient", ServiceKey: "provider-a"}
	failed, err := eng.CompleteAttemptFailureWithRetryFenced(ctx, claim.Attempt.ID, "worker-retry-pause", claim.Lease.Epoch, failure)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Terminal || failed.RetrySchedule == nil || failed.Completion.NodeRun.State != harnessmodel.NodeRetryWait {
		t.Fatalf("retry not durably scheduled during PAUSING: %+v", failed)
	}
	if failed.Completion.WorkflowRun.State != harnessmodel.WorkflowPaused {
		t.Fatalf("last draining retry did not finalize PAUSED: %+v", failed.Completion.WorkflowRun)
	}

	clock.current = failed.RetrySchedule.NotBefore
	released, err := eng.ReleaseDueRetries(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(released) != 0 {
		t.Fatalf("retry was released while workflow PAUSED: %+v", released)
	}
	if got := nodeRunFor(t, db, run.ID, "a"); got.State != harnessmodel.NodeRetryWait {
		t.Fatalf("paused retry node changed state: %+v", got)
	}

	if _, err := eng.ResumeWorkflow(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	released, err = eng.ReleaseDueRetries(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(released) != 1 || released[0] != node.ID {
		t.Fatalf("retry did not release after resume: %+v", released)
	}
	if got := nodeRunFor(t, db, run.ID, "a"); got.State != harnessmodel.NodeReady {
		t.Fatalf("retry node not READY after resume: %+v", got)
	}
}
