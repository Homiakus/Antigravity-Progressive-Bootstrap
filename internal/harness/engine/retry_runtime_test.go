package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func retryDefinition(id string, policy harnessmodel.RetryPolicySpec) harnessmodel.WorkflowDefinition {
	def := dagDefinition()
	def.ID = harnessmodel.WorkflowDefinitionID(id)
	def.Name = id
	def.Nodes = def.Nodes[:1]
	def.Nodes[0].RetryPolicyRef = "transient"
	def.RetryPolicies = map[string]harnessmodel.RetryPolicySpec{"transient": policy}
	return def
}

func transientPolicy() harnessmodel.RetryPolicySpec {
	return harnessmodel.RetryPolicySpec{
		MaxAttempts: 3, InitialDelay: 5 * time.Second, BackoffFactor: 2, MaxDelay: time.Minute,
		RetryableClasses: []harnessmodel.ErrorClass{harnessmodel.ErrorInfraTransient, harnessmodel.ErrorRateLimited, harnessmodel.ErrorTimeout},
	}
}

func TestRetryFailureKeepsWorkflowRunningAndCreatesNewAttemptLater(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	run, err := eng.StartWorkflow(ctx, retryDefinition("wfd_retry_lifecycle", transientPolicy()))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	attempt1, err := eng.StartAttempt(ctx, node.ID)
	if err != nil {
		t.Fatal(err)
	}
	failure := harnessmodel.Failure{Class: harnessmodel.ErrorInfraTransient, Message: "temporary network failure", ServiceKey: "github-api"}
	failed, err := eng.CompleteAttemptFailureWithRetry(ctx, attempt1.ID, failure)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Terminal || failed.RetrySchedule == nil || !failed.Decision.Retry {
		t.Fatalf("retry was not scheduled: %+v", failed)
	}
	if failed.Completion.Attempt.State != harnessmodel.AttemptFailed || failed.Completion.NodeRun.State != harnessmodel.NodeRetryWait || failed.Completion.WorkflowRun.State != harnessmodel.WorkflowRunning {
		t.Fatalf("wrong states after retry scheduling: %+v", failed)
	}
	if p := workflowProgress(t, db, run.ID); p.TerminalNodes != 0 || p.FailedNodes != 0 {
		t.Fatalf("retryable failure incorrectly counted terminal progress: %+v", p)
	}

	duplicate, err := eng.CompleteAttemptFailureWithRetry(ctx, attempt1.ID, failure)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.RetrySchedule == nil || !duplicate.Completion.Idempotent || duplicate.RetrySchedule.FailedAttemptID != attempt1.ID {
		t.Fatalf("duplicate retry report was not idempotent: %+v", duplicate)
	}

	clock.current = failed.RetrySchedule.NotBefore
	released, err := eng.ReleaseDueRetries(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(released) != 1 || released[0] != node.ID {
		t.Fatalf("unexpected released retries: %+v", released)
	}
	node = nodeRunFor(t, db, run.ID, "a")
	if node.State != harnessmodel.NodeReady {
		t.Fatalf("retry node not READY: %+v", node)
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		_, err := reader.GetRetrySchedule(ctx, node.ID)
		return err
	}); !errors.Is(err, harnessstore.ErrNotFound) {
		t.Fatalf("retry schedule survived release: %v", err)
	}

	attempt2, err := eng.StartAttempt(ctx, node.ID)
	if err != nil {
		t.Fatal(err)
	}
	if attempt2.Number != 2 || attempt2.ID == attempt1.ID || attempt2.State != harnessmodel.AttemptRunning {
		t.Fatalf("retry reused old attempt or wrong number: old=%+v new=%+v", attempt1, attempt2)
	}
	if _, err := eng.CompleteAttemptSuccess(ctx, attempt2.ID); err != nil {
		t.Fatal(err)
	}
	if got := workflowRun(t, db, run.ID); got.State != harnessmodel.WorkflowSucceeded {
		t.Fatalf("workflow did not succeed after retry: %+v", got)
	}
	if p := workflowProgress(t, db, run.ID); p.TerminalNodes != 1 || p.FailedNodes != 0 {
		t.Fatalf("final progress double-counted failed attempt: %+v", p)
	}
}

func TestRetryBudgetExhaustionBecomesTerminalWithoutLeakingBudget(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	policy := transientPolicy()
	policy.WorkflowBudgetLimit = 1
	policy.WorkflowBudgetWindow = time.Hour
	run, err := eng.StartWorkflow(ctx, retryDefinition("wfd_retry_budget", policy))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	attempt1, err := eng.StartAttempt(ctx, node.ID)
	if err != nil {
		t.Fatal(err)
	}
	failure := harnessmodel.Failure{Class: harnessmodel.ErrorInfraTransient, Message: "outage"}
	first, err := eng.CompleteAttemptFailureWithRetry(ctx, attempt1.ID, failure)
	if err != nil {
		t.Fatal(err)
	}
	if first.RetrySchedule == nil {
		t.Fatal("first retry not scheduled")
	}
	clock.current = first.RetrySchedule.NotBefore
	if _, err := eng.ReleaseDueRetries(ctx, 10); err != nil {
		t.Fatal(err)
	}
	node = nodeRunFor(t, db, run.ID, "a")
	attempt2, err := eng.StartAttempt(ctx, node.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := eng.CompleteAttemptFailureWithRetry(ctx, attempt2.ID, failure)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Terminal || !second.BudgetDenied || second.RetrySchedule != nil {
		t.Fatalf("budget exhaustion did not terminate retry loop: %+v", second)
	}
	if second.Completion.Attempt.State != harnessmodel.AttemptFailed || second.Completion.NodeRun.State != harnessmodel.NodeFailed || second.Completion.WorkflowRun.State != harnessmodel.WorkflowFailed {
		t.Fatalf("wrong terminal states after budget exhaustion: %+v", second)
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		budget, err := reader.GetRetryBudget(ctx, harnessmodel.RetryBudgetWorkflow, string(run.ID))
		if err != nil {
			return err
		}
		if budget.Used != 1 {
			t.Fatalf("denied retry consumed budget token: %+v", budget)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownEffectNeverAutomaticallyRetries(t *testing.T) {
	ctx := context.Background()
	eng, db, _, _ := newTestEngine(t)
	policy := transientPolicy()
	run, err := eng.StartWorkflow(ctx, retryDefinition("wfd_unknown_effect", policy))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	attempt, err := eng.StartAttempt(ctx, node.ID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := eng.CompleteAttemptFailureWithRetry(ctx, attempt.ID, harnessmodel.Failure{Class: harnessmodel.ErrorUnknownEffect, Message: "remote side effect may have completed"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Terminal || result.Decision.Retry || result.RetrySchedule != nil {
		t.Fatalf("UNKNOWN_EFFECT was automatically retried: %+v", result)
	}
	if result.Completion.WorkflowRun.State != harnessmodel.WorkflowFailed {
		t.Fatalf("unexpected workflow state: %+v", result.Completion.WorkflowRun)
	}
}
