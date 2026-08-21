package engine

import (
	"context"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestApprovalApproveRestoresReadyAndWorkflowCanComplete(t *testing.T) {
	ctx := context.Background()
	eng, db, _, _ := newTestEngine(t)
	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("approval-happy"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	approval, err := eng.RequestApproval(ctx, node.ID, "filesystem.write", "HIGH", "publish generated artifact", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if approval.State != harnessmodel.ApprovalPending || approval.ExpiresAt.IsZero() {
		t.Fatalf("unexpected approval: %+v", approval)
	}
	if got := nodeRunFor(t, db, run.ID, "a"); got.State != harnessmodel.NodeWaiting {
		t.Fatalf("approval did not suspend node: %+v", got)
	}
	result, err := eng.Approve(ctx, approval.ID, "operator@example")
	if err != nil {
		t.Fatal(err)
	}
	if result.Approval.State != harnessmodel.ApprovalApproved || result.NodeRun.State != harnessmodel.NodeReady {
		t.Fatalf("approval did not restore node: %+v", result)
	}
	attempt, err := eng.StartAttempt(ctx, node.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.CompleteAttemptSuccess(ctx, attempt.ID); err != nil {
		t.Fatal(err)
	}
	if got := workflowRun(t, db, run.ID); got.State != harnessmodel.WorkflowSucceeded {
		t.Fatalf("approved workflow did not complete: %+v", got)
	}
}

func TestApprovalRejectFailsClosedAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	eng, db, _, _ := newTestEngine(t)
	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("approval-reject"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	approval, err := eng.RequestApproval(ctx, node.ID, "network.publish", "HIGH", "publish release", 0)
	if err != nil {
		t.Fatal(err)
	}
	result, err := eng.Reject(ctx, approval.ID, "reviewer@example")
	if err != nil {
		t.Fatal(err)
	}
	if result.Approval.State != harnessmodel.ApprovalRejected || result.NodeRun.State != harnessmodel.NodeFailed || result.WorkflowRun.State != harnessmodel.WorkflowFailed {
		t.Fatalf("rejection did not fail closed: %+v", result)
	}
	if p := workflowProgress(t, db, run.ID); p.TerminalNodes != 1 || p.FailedNodes != 1 {
		t.Fatalf("rejection progress mismatch: %+v", p)
	}
	duplicate, err := eng.Reject(ctx, approval.ID, "reviewer@example")
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate.Idempotent || duplicate.Approval.State != harnessmodel.ApprovalRejected {
		t.Fatalf("duplicate rejection not idempotent: %+v", duplicate)
	}
}

func TestApprovalRejectWhilePausedTransitionsWorkflowToFailed(t *testing.T) {
	ctx := context.Background()
	eng, db, _, _ := newTestEngine(t)
	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("approval-paused-reject"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	approval, err := eng.RequestApproval(ctx, node.ID, "deploy.production", "CRITICAL", "production deploy", 0)
	if err != nil {
		t.Fatal(err)
	}
	paused, err := eng.PauseWorkflow(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if paused.WorkflowRun.State != harnessmodel.WorkflowPaused {
		t.Fatalf("workflow did not pause around approval wait: %+v", paused)
	}
	result, err := eng.Reject(ctx, approval.ID, "security@example")
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowRun.State != harnessmodel.WorkflowFailed || result.NodeRun.State != harnessmodel.NodeFailed {
		t.Fatalf("paused rejection did not fail workflow: %+v", result)
	}
}

func TestApprovalExpirySurvivesAsDurableTimerAndFailsClosed(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("approval-expiry"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	approval, err := eng.RequestApproval(ctx, node.ID, "filesystem.write", "HIGH", "time bounded write", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	clock.current = approval.ExpiresAt
	expired, err := eng.ReleaseExpiredApprovals(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 1 || expired[0] != approval.ID {
		t.Fatalf("unexpected expired approvals: %+v", expired)
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		got, err := r.GetApproval(ctx, approval.ID)
		if err != nil {
			return err
		}
		if got.State != harnessmodel.ApprovalExpired {
			t.Fatalf("approval state=%s want EXPIRED", got.State)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := nodeRunFor(t, db, run.ID, "a"); got.State != harnessmodel.NodeTimedOut {
		t.Fatalf("expired approval node=%s want TIMED_OUT", got.State)
	}
	if got := workflowRun(t, db, run.ID); got.State != harnessmodel.WorkflowFailed {
		t.Fatalf("expired approval workflow=%s want FAILED", got.State)
	}
	var fired int
	if err := db.SQLDB().QueryRow(`SELECT COUNT(*) FROM timers WHERE kind=? AND state=? AND payload=?`, string(harnessmodel.TimerApprovalExpiry), string(harnessmodel.TimerFired), string(approval.ID)).Scan(&fired); err != nil {
		t.Fatal(err)
	}
	if fired != 1 {
		t.Fatalf("approval expiry fired timers=%d want=1", fired)
	}
}
