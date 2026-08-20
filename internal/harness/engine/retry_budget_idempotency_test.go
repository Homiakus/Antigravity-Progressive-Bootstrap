package engine

import (
	"context"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestDuplicateRetryDoesNotConsumeBudgetAgain(t *testing.T) {
	ctx := context.Background()
	eng, db, _, _ := newTestEngine(t)
	policy := transientPolicy()
	policy.WorkflowBudgetLimit = 1
	policy.WorkflowBudgetWindow = time.Hour
	run, err := eng.StartWorkflow(ctx, retryDefinition("wfd_retry_budget_idempotent", policy))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	attempt, err := eng.StartAttempt(ctx, node.ID)
	if err != nil {
		t.Fatal(err)
	}
	failure := harnessmodel.Failure{Class: harnessmodel.ErrorInfraTransient, Message: "temporary"}
	if _, err := eng.CompleteAttemptFailureWithRetry(ctx, attempt.ID, failure); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.CompleteAttemptFailureWithRetry(ctx, attempt.ID, failure); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.CompleteAttemptFailureWithRetry(ctx, attempt.ID, failure); err != nil {
		t.Fatal(err)
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		budget, err := reader.GetRetryBudget(ctx, harnessmodel.RetryBudgetWorkflow, string(run.ID))
		if err != nil {
			return err
		}
		if budget.Used != 1 {
			t.Fatalf("idempotent duplicate spent retry budget more than once: %+v", budget)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
