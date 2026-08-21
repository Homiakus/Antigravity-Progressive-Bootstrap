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

func TestCancelWorkflowCancelsReadyDAGAndFinalizesProgress(t *testing.T) {
	ctx := context.Background()
	eng, db, _, _ := newTestEngine(t)
	run, err := eng.StartWorkflow(ctx, dagDefinition())
	if err != nil {
		t.Fatal(err)
	}
	result, err := eng.CancelWorkflow(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowRun.State != harnessmodel.WorkflowCancelled || result.Stats.Nodes != 4 {
		t.Fatalf("unexpected cancel result: %+v", result)
	}
	if p := workflowProgress(t, db, run.ID); p.TerminalNodes != p.TotalNodes || p.FailedNodes != 0 {
		t.Fatalf("cancelled workflow progress mismatch: %+v", p)
	}
	var ready int
	if err := db.SQLDB().QueryRow(`SELECT COUNT(*) FROM ready_queue WHERE workflow_run_id=?`, string(run.ID)).Scan(&ready); err != nil {
		t.Fatal(err)
	}
	if ready != 0 {
		t.Fatalf("cancelled workflow retained %d ready rows", ready)
	}
	duplicate, err := eng.CancelWorkflow(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate.Idempotent || duplicate.WorkflowRun.State != harnessmodel.WorkflowCancelled {
		t.Fatalf("duplicate cancellation not idempotent: %+v", duplicate)
	}
}

func TestCancelWorkflowFencesActiveLeasedAttempt(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-cancel", clock.current)
	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("cancel-active"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-cancel", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, "worker-cancel", claim.Lease.Epoch)
	if err != nil {
		t.Fatal(err)
	}
	result, err := eng.CancelWorkflow(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.Attempts != 1 || result.Stats.Leases != 1 || result.Stats.Nodes != 1 {
		t.Fatalf("active cancellation stats mismatch: %+v", result.Stats)
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		got, err := r.GetAttempt(ctx, attempt.ID)
		if err != nil {
			return err
		}
		if got.State != harnessmodel.AttemptCancelled {
			t.Fatalf("attempt state=%s want CANCELLED", got.State)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var leaseState string
	if err := db.SQLDB().QueryRow(`SELECT state FROM leases WHERE id=?`, string(claim.Lease.ID)).Scan(&leaseState); err != nil {
		t.Fatal(err)
	}
	if leaseState != string(harnessmodel.LeaseReleased) {
		t.Fatalf("lease state=%s want RELEASED", leaseState)
	}
	if _, err := eng.CompleteAttemptSuccessFenced(ctx, attempt.ID, "worker-cancel", claim.Lease.Epoch); !errors.Is(err, harnesslease.ErrStaleFence) {
		t.Fatalf("late completion after cancel error=%v want ErrStaleFence", err)
	}
}

func TestCancelWorkflowCleansTimersSignalsApprovalsAndRetries(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	def := harnessmodel.WorkflowDefinition{
		ID: "wfd_cancel_waits", Version: 1, Name: "cancel-waits", CreatedAt: time.Unix(1900, 0).UTC(), CompilerVersion: "test",
		RetryPolicies: map[string]harnessmodel.RetryPolicySpec{"transient": transientPolicy()},
		Nodes: []harnessmodel.NodeSpec{
			{ID: "retry", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, RetryPolicyRef: "transient", CachePolicy: harnessmodel.CacheDisabled},
			{ID: "timer", Kind: harnessmodel.NodeKindWait, CachePolicy: harnessmodel.CacheDisabled},
			{ID: "signal", Kind: harnessmodel.NodeKindWait, CachePolicy: harnessmodel.CacheDisabled},
			{ID: "approval", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, CachePolicy: harnessmodel.CacheDisabled},
		},
	}
	run, err := eng.StartWorkflow(ctx, def)
	if err != nil {
		t.Fatal(err)
	}

	retryNode := nodeRunFor(t, db, run.ID, "retry")
	attempt, err := eng.StartAttempt(ctx, retryNode.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.CompleteAttemptFailureWithRetry(ctx, attempt.ID, harnessmodel.Failure{Class: harnessmodel.ErrorInfraTransient, Message: "retry me"}); err != nil {
		t.Fatal(err)
	}
	timerNode := nodeRunFor(t, db, run.ID, "timer")
	if _, err := eng.WaitUntil(ctx, timerNode.ID, clock.current.Add(time.Hour), nil); err != nil {
		t.Fatal(err)
	}
	signalNode := nodeRunFor(t, db, run.ID, "signal")
	if _, err := eng.WaitForSignal(ctx, signalNode.ID, "operator.ready"); err != nil {
		t.Fatal(err)
	}
	approvalNode := nodeRunFor(t, db, run.ID, "approval")
	if _, err := eng.RequestApproval(ctx, approvalNode.ID, "deploy.production", "HIGH", "deploy release", time.Hour); err != nil {
		t.Fatal(err)
	}

	result, err := eng.CancelWorkflow(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.Nodes != 4 || result.Stats.Timers != 2 || result.Stats.SignalWaits != 1 || result.Stats.Approvals != 1 || result.Stats.Retries != 1 {
		t.Fatalf("durable runtime cancellation stats mismatch: %+v", result.Stats)
	}
	queries := []struct {
		name  string
		query string
		args  []any
	}{
		{"pending timers", `SELECT COUNT(*) FROM timers WHERE workflow_run_id=? AND state=?`, []any{string(run.ID), string(harnessmodel.TimerPending)}},
		{"waiting signals", `SELECT COUNT(*) FROM signal_waits WHERE workflow_run_id=? AND state=?`, []any{string(run.ID), string(harnessmodel.SignalWaitWaiting)}},
		{"pending approvals", `SELECT COUNT(*) FROM approvals WHERE workflow_run_id=? AND state=?`, []any{string(run.ID), string(harnessmodel.ApprovalPending)}},
		{"active retries", `SELECT COUNT(*) FROM retry_schedule WHERE workflow_run_id=?`, []any{string(run.ID)}},
		{"ready rows", `SELECT COUNT(*) FROM ready_queue WHERE workflow_run_id=?`, []any{string(run.ID)}},
	}
	for _, q := range queries {
		var count int
		if err := db.SQLDB().QueryRow(q.query, q.args...).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s survived cancellation: %d rows", q.name, count)
		}
	}
	if p := workflowProgress(t, db, run.ID); p.TerminalNodes != 4 || p.FailedNodes != 0 {
		t.Fatalf("cancel cleanup progress mismatch: %+v", p)
	}
}
