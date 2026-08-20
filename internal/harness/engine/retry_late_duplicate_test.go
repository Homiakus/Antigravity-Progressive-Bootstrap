package engine

import (
	"context"
	"testing"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestLateDuplicateAfterRetryReleasePreservesOriginalRetryDecision(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	run, err := eng.StartWorkflow(ctx, retryDefinition("wfd_retry_late_duplicate", transientPolicy()))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	attempt, err := eng.StartAttempt(ctx, node.ID)
	if err != nil {
		t.Fatal(err)
	}
	failure := harnessmodel.Failure{Class: harnessmodel.ErrorInfraTransient, Message: "temporary"}
	first, err := eng.CompleteAttemptFailureWithRetry(ctx, attempt.ID, failure)
	if err != nil {
		t.Fatal(err)
	}
	if first.RetrySchedule == nil {
		t.Fatal("retry schedule missing")
	}
	clock.current = first.RetrySchedule.NotBefore
	if _, err := eng.ReleaseDueRetries(ctx, 10); err != nil {
		t.Fatal(err)
	}
	late, err := eng.CompleteAttemptFailureWithRetry(ctx, attempt.ID, failure)
	if err != nil {
		t.Fatal(err)
	}
	if late.Terminal || !late.Decision.Retry || !late.Completion.Idempotent {
		t.Fatalf("late duplicate lost original retry semantics: %+v", late)
	}
	if late.RetrySchedule != nil {
		t.Fatalf("released retry unexpectedly recreated active schedule: %+v", late.RetrySchedule)
	}
	if late.Completion.NodeRun.State != harnessmodel.NodeReady || late.Completion.WorkflowRun.State != harnessmodel.WorkflowRunning {
		t.Fatalf("late duplicate changed runtime state: %+v", late.Completion)
	}
}
