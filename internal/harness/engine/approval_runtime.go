package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type ApprovalResult struct {
	Approval    harnessmodel.Approval    `json:"approval"`
	WorkflowRun harnessmodel.WorkflowRun `json:"workflowRun"`
	NodeRun     harnessmodel.NodeRun     `json:"nodeRun"`
	Idempotent  bool                     `json:"idempotent,omitempty"`
}

// RequestApproval removes a schedulable node from the ready projection and
// persists a human-approval wait. An optional TTL is represented by a durable
// APPROVAL_EXPIRY timer, so process restarts cannot lose the deadline.
func (e *Engine) RequestApproval(ctx context.Context, nodeRunID harnessmodel.NodeRunID, capability, risk, reason string, ttl time.Duration) (harnessmodel.Approval, error) {
	if nodeRunID == "" || strings.TrimSpace(capability) == "" || strings.TrimSpace(reason) == "" {
		return harnessmodel.Approval{}, fmt.Errorf("node run id, requested capability and reason are required")
	}
	if ttl < 0 {
		return harnessmodel.Approval{}, fmt.Errorf("approval ttl cannot be negative")
	}
	now := e.now().UTC()
	rawApprovalID, err := e.nextID(harnessmodel.IDApproval)
	if err != nil {
		return harnessmodel.Approval{}, err
	}
	var rawTimerID string
	if ttl > 0 {
		rawTimerID, err = e.nextID(harnessmodel.IDTimer)
		if err != nil {
			return harnessmodel.Approval{}, err
		}
	}

	var approval harnessmodel.Approval
	err = e.store.Update(ctx, func(tx harnessstore.Tx) error {
		nr, err := tx.GetNodeRun(ctx, nodeRunID)
		if err != nil {
			return err
		}
		if nr.State != harnessmodel.NodeReady {
			return fmt.Errorf("cannot request approval for node %s from state %s", nr.ID, nr.State)
		}
		run, err := tx.GetWorkflowRun(ctx, nr.WorkflowRunID)
		if err != nil {
			return err
		}
		if run.State != harnessmodel.WorkflowRunning {
			return fmt.Errorf("cannot request approval while workflow %s is %s", run.ID, run.State)
		}
		if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeWaiting, now); err != nil {
			return err
		}
		if err := tx.RemoveReadyNode(ctx, nr.ID); err != nil {
			return err
		}
		approval = harnessmodel.Approval{
			ID: harnessmodel.ApprovalID(rawApprovalID), WorkflowRunID: run.ID, NodeRunID: nr.ID,
			RequestedCapability: strings.TrimSpace(capability), Risk: strings.TrimSpace(risk), Reason: strings.TrimSpace(reason),
			RequestedAt: now, State: harnessmodel.ApprovalPending,
		}
		if ttl > 0 {
			approval.ExpiresAt = now.Add(ttl)
		}
		if err := tx.CreateApproval(ctx, approval); err != nil {
			return err
		}
		if ttl > 0 {
			timer := harnessmodel.Timer{
				ID: harnessmodel.TimerID(rawTimerID), WorkflowRunID: run.ID, NodeRunID: nr.ID,
				Kind: harnessmodel.TimerApprovalExpiry, Payload: []byte(approval.ID), State: harnessmodel.TimerPending,
				DueAt: approval.ExpiresAt, CreatedAt: now,
			}
			if err := tx.CreateTimer(ctx, timer); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "TimerScheduled", "timer", string(timer.ID), map[string]any{"nodeRunId": nr.ID, "kind": timer.Kind, "dueAt": timer.DueAt, "approvalId": approval.ID}); err != nil {
				return err
			}
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "ApprovalRequested", "approval", string(approval.ID), map[string]any{
			"nodeRunId": nr.ID, "capability": approval.RequestedCapability, "risk": approval.Risk,
			"reason": approval.Reason, "expiresAt": approval.ExpiresAt,
		}); err != nil {
			return err
		}
		return nil
	})
	return approval, err
}

func (e *Engine) Approve(ctx context.Context, approvalID harnessmodel.ApprovalID, actor string) (ApprovalResult, error) {
	return e.resolveApproval(ctx, approvalID, actor, true)
}

func (e *Engine) Reject(ctx context.Context, approvalID harnessmodel.ApprovalID, actor string) (ApprovalResult, error) {
	return e.resolveApproval(ctx, approvalID, actor, false)
}

func (e *Engine) resolveApproval(ctx context.Context, approvalID harnessmodel.ApprovalID, actor string, approved bool) (ApprovalResult, error) {
	if approvalID == "" || strings.TrimSpace(actor) == "" {
		return ApprovalResult{}, fmt.Errorf("approval id and actor are required")
	}
	now := e.now().UTC()
	var result ApprovalResult
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		approval, err := tx.GetApproval(ctx, approvalID)
		if err != nil {
			return err
		}
		if approval.State != harnessmodel.ApprovalPending {
			result.Approval = approval
			result.Idempotent = true
			result.NodeRun, _ = tx.GetNodeRun(ctx, approval.NodeRunID)
			result.WorkflowRun, _ = tx.GetWorkflowRun(ctx, approval.WorkflowRunID)
			return nil
		}
		if !approval.ExpiresAt.IsZero() && !now.Before(approval.ExpiresAt) {
			return fmt.Errorf("approval %s expired at %s", approval.ID, approval.ExpiresAt.Format(time.RFC3339Nano))
		}
		nr, err := tx.GetNodeRun(ctx, approval.NodeRunID)
		if err != nil {
			return err
		}
		if nr.State != harnessmodel.NodeWaiting {
			return fmt.Errorf("approval %s has node %s in state %s: %w", approval.ID, nr.ID, nr.State, harnessstore.ErrConflict)
		}
		run, err := tx.GetWorkflowRun(ctx, approval.WorkflowRunID)
		if err != nil {
			return err
		}

		approval.Actor = strings.TrimSpace(actor)
		approval.ResolvedAt = now
		if approved {
			approval.State = harnessmodel.ApprovalApproved
			if err := tx.CompareAndSwapApproval(ctx, harnessmodel.ApprovalPending, approval); err != nil {
				return err
			}
			if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeReady, now); err != nil {
				return err
			}
			if err := tx.EnqueueReadyNode(ctx, nr.ID, now, time.Time{}, ""); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "ApprovalApproved", "approval", string(approval.ID), map[string]any{"nodeRunId": nr.ID, "actor": approval.Actor}); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "NodeReady", "node_run", string(nr.ID), map[string]any{"nodeId": nr.NodeID, "wakeReason": "approval", "approvalId": approval.ID}); err != nil {
				return err
			}
		} else {
			approval.State = harnessmodel.ApprovalRejected
			if err := tx.CompareAndSwapApproval(ctx, harnessmodel.ApprovalPending, approval); err != nil {
				return err
			}
			if err := e.failApprovalWait(ctx, tx, &run, &nr, now, harnessmodel.NodeFailed, "ApprovalRejected", approval); err != nil {
				return err
			}
		}
		result = ApprovalResult{Approval: approval, WorkflowRun: run, NodeRun: nr}
		return nil
	})
	return result, err
}

// ReleaseExpiredApprovals resolves due APPROVAL_EXPIRY timers. Expiry and the
// fail-closed node/workflow transition share one SQLite transaction, preventing
// a crash from recording EXPIRED without applying its control-flow effect.
func (e *Engine) ReleaseExpiredApprovals(ctx context.Context, limit int) ([]harnessmodel.ApprovalID, error) {
	now := e.now().UTC()
	var due []harnessmodel.Timer
	if err := e.store.View(ctx, func(r harnessstore.Reader) error {
		var err error
		due, err = r.ListDueTimers(ctx, now, limit)
		return err
	}); err != nil {
		return nil, err
	}
	expired := make([]harnessmodel.ApprovalID, 0)
	for _, candidate := range due {
		if candidate.Kind != harnessmodel.TimerApprovalExpiry {
			continue
		}
		approvalID := harnessmodel.ApprovalID(string(candidate.Payload))
		if approvalID == "" {
			continue
		}
		var didExpire bool
		err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
			timer, err := tx.GetTimer(ctx, candidate.ID)
			if err != nil {
				return err
			}
			if timer.State != harnessmodel.TimerPending || timer.DueAt.After(now) || timer.Kind != harnessmodel.TimerApprovalExpiry {
				return nil
			}
			approval, err := tx.GetApproval(ctx, approvalID)
			if err != nil {
				return err
			}
			if approval.State != harnessmodel.ApprovalPending {
				timer.State = harnessmodel.TimerCancelled
				timer.ResolvedAt = now
				return tx.CompareAndSwapTimer(ctx, harnessmodel.TimerPending, timer)
			}
			nr, err := tx.GetNodeRun(ctx, approval.NodeRunID)
			if err != nil {
				return err
			}
			if nr.State != harnessmodel.NodeWaiting {
				return fmt.Errorf("approval expiry %s has node %s in state %s: %w", approval.ID, nr.ID, nr.State, harnessstore.ErrConflict)
			}
			run, err := tx.GetWorkflowRun(ctx, approval.WorkflowRunID)
			if err != nil {
				return err
			}
			approval.State = harnessmodel.ApprovalExpired
			approval.ResolvedAt = now
			if err := tx.CompareAndSwapApproval(ctx, harnessmodel.ApprovalPending, approval); err != nil {
				return err
			}
			timer.State = harnessmodel.TimerFired
			timer.ResolvedAt = now
			if err := tx.CompareAndSwapTimer(ctx, harnessmodel.TimerPending, timer); err != nil {
				return err
			}
			if err := e.failApprovalWait(ctx, tx, &run, &nr, now, harnessmodel.NodeTimedOut, "ApprovalExpired", approval); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "TimerFired", "timer", string(timer.ID), map[string]any{"nodeRunId": nr.ID, "kind": timer.Kind, "approvalId": approval.ID}); err != nil {
				return err
			}
			didExpire = true
			return nil
		})
		if err != nil {
			if errors.Is(err, harnessstore.ErrNotFound) || errors.Is(err, harnessstore.ErrConflict) {
				continue
			}
			return expired, err
		}
		if didExpire {
			expired = append(expired, approvalID)
		}
	}
	return expired, nil
}

func (e *Engine) failApprovalWait(ctx context.Context, tx harnessstore.Tx, run *harnessmodel.WorkflowRun, nr *harnessmodel.NodeRun, now time.Time, nodeState harnessmodel.NodeState, eventType string, approval harnessmodel.Approval) error {
	if err := transitionNode(ctx, tx, nr, nodeState, now); err != nil {
		return err
	}
	progress, err := tx.IncrementWorkflowProgress(ctx, run.ID, true, now)
	if err != nil {
		return err
	}
	if _, err := e.appendEvent(ctx, tx, run.ID, now, eventType, "approval", string(approval.ID), map[string]any{"nodeRunId": nr.ID, "actor": approval.Actor}); err != nil {
		return err
	}
	if !run.State.Terminal() && run.State != harnessmodel.WorkflowCancelling {
		if err := transitionWorkflow(ctx, tx, run, harnessmodel.WorkflowFailed, now); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "WorkflowFailed", "workflow_run", string(run.ID), map[string]any{"failedNodeId": nr.NodeID, "failedNodes": progress.FailedNodes, "approvalId": approval.ID}); err != nil {
			return err
		}
	}
	return nil
}
