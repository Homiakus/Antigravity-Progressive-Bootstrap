package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessretry "github.com/homiakus/agctl/internal/harness/retry"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

var (
	errRetryTerminal     = errors.New("retry policy selected terminal failure")
	errRetryBudgetDenied = errors.New("retry budget denied")
)

type RetryFailureResult struct {
	Completion    CompletionResult            `json:"completion"`
	Decision      harnessretry.Decision       `json:"decision"`
	RetrySchedule *harnessmodel.RetrySchedule `json:"retrySchedule,omitempty"`
	Terminal      bool                        `json:"terminal"`
	BudgetDenied  bool                        `json:"budgetDenied,omitempty"`
}

func (e *Engine) CompleteAttemptFailureWithRetry(ctx context.Context, attemptID harnessmodel.AttemptID, failure harnessmodel.Failure) (RetryFailureResult, error) {
	return e.completeAttemptFailureWithRetry(ctx, attemptID, failure, nil)
}

func (e *Engine) CompleteAttemptFailureWithRetryFenced(ctx context.Context, attemptID harnessmodel.AttemptID, workerID harnessmodel.WorkerID, epoch uint64, failure harnessmodel.Failure) (RetryFailureResult, error) {
	if workerID == "" || epoch == 0 {
		return RetryFailureResult{}, fmt.Errorf("worker id and lease epoch are required")
	}
	return e.completeAttemptFailureWithRetry(ctx, attemptID, failure, &completionFence{WorkerID: workerID, Epoch: epoch})
}

func (e *Engine) completeAttemptFailureWithRetry(ctx context.Context, attemptID harnessmodel.AttemptID, failure harnessmodel.Failure, fence *completionFence) (RetryFailureResult, error) {
	if attemptID == "" {
		return RetryFailureResult{}, fmt.Errorf("attempt id is required")
	}
	if !failure.Class.Valid() {
		return RetryFailureResult{}, fmt.Errorf("invalid failure class %q", failure.Class)
	}

	result, err := e.tryScheduleRetry(ctx, attemptID, failure, fence)
	if err == nil {
		return result, nil
	}
	budgetDenied := errors.Is(err, errRetryBudgetDenied)
	if !errors.Is(err, errRetryTerminal) && !budgetDenied {
		return RetryFailureResult{}, err
	}

	// The retry scheduling transaction intentionally rolled back. Commit the
	// terminal path separately so a denied second budget cannot consume the
	// first budget without a corresponding RetryScheduled fact.
	var terminal CompletionResult
	if fence == nil {
		terminal, err = e.CompleteAttemptFailure(ctx, attemptID, string(failure.Class), failure.Message)
	} else {
		terminal, err = e.CompleteAttemptFailureFenced(ctx, attemptID, fence.WorkerID, fence.Epoch, string(failure.Class), failure.Message)
	}
	if err != nil {
		return RetryFailureResult{}, err
	}
	reason := "retry policy selected terminal failure"
	if budgetDenied {
		reason = "retry budget exhausted"
	}
	return RetryFailureResult{
		Completion: terminal,
		Decision: harnessretry.Decision{Retry: false, Reason: reason},
		Terminal: true, BudgetDenied: budgetDenied,
	}, nil
}

func (e *Engine) tryScheduleRetry(ctx context.Context, attemptID harnessmodel.AttemptID, failure harnessmodel.Failure, fence *completionFence) (RetryFailureResult, error) {
	now := e.now().UTC()
	var result RetryFailureResult
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		attempt, err := tx.GetAttempt(ctx, attemptID)
		if err != nil {
			return err
		}
		nr, err := tx.GetNodeRun(ctx, attempt.NodeRunID)
		if err != nil {
			return err
		}
		if attempt.State == harnessmodel.AttemptFailed {
			if err := authorizeTerminalDuplicate(attempt, fence); err != nil {
				return err
			}
			// The immutable history row is written by a SQLite trigger in the
			// same transaction as the active retry schedule. Unlike the active
			// row it survives RetryReady, later attempts and later terminal node
			// outcomes, so a duplicate report can always recover the original
			// decision for this exact failed Attempt.
			schedule, err := tx.GetRetryScheduleByAttempt(ctx, attempt.ID)
			if err != nil {
				if errors.Is(err, harnessstore.ErrNotFound) {
					return errRetryTerminal
				}
				return err
			}
			if schedule.NodeRunID != nr.ID {
				return fmt.Errorf("retry history for attempt %s belongs to node %s, not %s: %w", attempt.ID, schedule.NodeRunID, nr.ID, harnessstore.ErrConflict)
			}
			run, err := tx.GetWorkflowRun(ctx, nr.WorkflowRunID)
			if err != nil {
				return err
			}
			result.Completion = CompletionResult{Attempt: attempt, NodeRun: nr, WorkflowRun: run, Idempotent: true}
			result.Decision = harnessretry.Decision{
				Retry: true, NotBefore: schedule.NotBefore,
				Delay: schedule.NotBefore.Sub(schedule.ScheduledAt),
				Reason: "retry decision recovered from durable history",
			}
			result.RetrySchedule = &schedule
			return nil
		}
		// The fence is checked before the Attempt lifecycle state for the same
		// reason as success/failure completion: after reclaim, the old owner must
		// receive ErrStaleFence even if the new owner has not started execution
		// yet and the Attempt is still CLAIMED.
		if err := authorizeCompletion(ctx, tx, attempt, fence, now); err != nil {
			return err
		}
		if attempt.State != harnessmodel.AttemptRunning {
			return fmt.Errorf("cannot retry failure attempt %s from state %s", attempt.ID, attempt.State)
		}
		if nr.State != harnessmodel.NodeRunning {
			return fmt.Errorf("attempt %s is RUNNING but node %s is %s", attempt.ID, nr.ID, nr.State)
		}
		run, err := tx.GetWorkflowRun(ctx, nr.WorkflowRunID)
		if err != nil {
			return err
		}
		if run.State != harnessmodel.WorkflowRunning {
			return errRetryTerminal
		}
		def, err := tx.GetWorkflowDefinition(ctx, run.DefinitionID, run.DefinitionVersion)
		if err != nil {
			return err
		}
		node, ok := findNodeSpec(def, nr.NodeID)
		if !ok || node.RetryPolicyRef == "" {
			return errRetryTerminal
		}
		policy, ok := def.RetryPolicies[node.RetryPolicyRef]
		if !ok {
			return fmt.Errorf("durable node %s references missing retry policy %q", nr.NodeID, node.RetryPolicyRef)
		}
		firstAttemptAt, err := tx.GetFirstAttemptCreatedAt(ctx, nr.ID)
		if err != nil {
			return err
		}
		decision, err := harnessretry.Decide(harnessretry.DecisionInput{
			Policy: policy, Failure: failure, AttemptNumber: attempt.Number,
			FirstAttemptAt: firstAttemptAt, Now: now,
			Random: harnessretry.DeterministicRandom(string(attempt.ID)),
		})
		if err != nil {
			return err
		}
		if !decision.Retry {
			return errRetryTerminal
		}
		if policy.WorkflowBudgetLimit > 0 {
			_, allowed, err := tx.ReserveRetryBudget(ctx, harnessmodel.RetryBudgetWorkflow, string(run.ID), policy.WorkflowBudgetWindow, policy.WorkflowBudgetLimit, now)
			if err != nil {
				return err
			}
			if !allowed {
				return errRetryBudgetDenied
			}
		}
		if policy.ServiceBudgetLimit > 0 {
			if failure.ServiceKey == "" {
				return fmt.Errorf("retry policy %q requires service budget but failure has no service key", node.RetryPolicyRef)
			}
			_, allowed, err := tx.ReserveRetryBudget(ctx, harnessmodel.RetryBudgetService, failure.ServiceKey, policy.ServiceBudgetWindow, policy.ServiceBudgetLimit, now)
			if err != nil {
				return err
			}
			if !allowed {
				return errRetryBudgetDenied
			}
		}

		attempt.ErrorClass = string(failure.Class)
		attempt.ErrorMessage = failure.Message
		attempt.FinishedAt = now
		if err := transitionAttempt(ctx, tx, &attempt, harnessmodel.AttemptFailed); err != nil {
			return err
		}
		if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeRetryWait, now); err != nil {
			return err
		}
		if fence != nil {
			if err := tx.CloseLease(ctx, attempt.ID, fence.WorkerID, fence.Epoch, harnessmodel.LeaseReleased, now); err != nil {
				return err
			}
		}
		schedule := harnessmodel.RetrySchedule{
			NodeRunID: nr.ID, WorkflowRunID: run.ID, FailedAttemptID: attempt.ID, AttemptNumber: attempt.Number,
			FailureClass: failure.Class, PolicyRef: node.RetryPolicyRef, ServiceKey: failure.ServiceKey,
			ScheduledAt: now, NotBefore: decision.NotBefore,
		}
		if err := tx.CreateRetrySchedule(ctx, schedule); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "AttemptFailed", "attempt", string(attempt.ID), map[string]any{
			"nodeRunId": nr.ID, "attemptNumber": attempt.Number, "errorClass": failure.Class,
			"error": failure.Message, "workerId": attempt.WorkerID, "leaseEpoch": attempt.LeaseEpoch,
			"retryScheduled": true,
		}); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "RetryScheduled", "node_run", string(nr.ID), map[string]any{
			"failedAttemptId": attempt.ID, "attemptNumber": attempt.Number, "policyRef": node.RetryPolicyRef,
			"failureClass": failure.Class, "serviceKey": failure.ServiceKey, "notBefore": decision.NotBefore,
		}); err != nil {
			return err
		}
		result.Completion = CompletionResult{Attempt: attempt, NodeRun: nr, WorkflowRun: run}
		result.Decision = decision
		result.RetrySchedule = &schedule
		return nil
	})
	return result, err
}

func findNodeSpec(def harnessmodel.WorkflowDefinition, nodeID harnessmodel.NodeID) (harnessmodel.NodeSpec, bool) {
	for _, node := range def.Nodes {
		if node.ID == nodeID {
			return node, true
		}
	}
	return harnessmodel.NodeSpec{}, false
}

// ReleaseDueRetries makes durable RETRY_WAIT nodes schedulable again. A new
// Attempt is intentionally created later by ClaimNode, preserving the existing
// invariant that an Attempt represents actual physical ownership, not a timer.
func (e *Engine) ReleaseDueRetries(ctx context.Context, limit int) ([]harnessmodel.NodeRunID, error) {
	now := e.now().UTC()
	var due []harnessmodel.RetrySchedule
	if err := e.store.View(ctx, func(reader harnessstore.Reader) error {
		var err error
		due, err = reader.ListDueRetries(ctx, now, limit)
		return err
	}); err != nil {
		return nil, err
	}
	ready := make([]harnessmodel.NodeRunID, 0, len(due))
	for _, candidate := range due {
		var released bool
		err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
			schedule, err := tx.GetRetrySchedule(ctx, candidate.NodeRunID)
			if err != nil {
				return err
			}
			if schedule.NotBefore.After(now) {
				return nil
			}
			nr, err := tx.GetNodeRun(ctx, schedule.NodeRunID)
			if err != nil {
				return err
			}
			if nr.State != harnessmodel.NodeRetryWait {
				return fmt.Errorf("retry schedule %s has node in state %s", nr.ID, nr.State)
			}
			run, err := tx.GetWorkflowRun(ctx, schedule.WorkflowRunID)
			if err != nil {
				return err
			}
			if run.State != harnessmodel.WorkflowRunning {
				return nil
			}
			if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeReady, now); err != nil {
				return err
			}
			if err := tx.DeleteRetrySchedule(ctx, nr.ID); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "RetryReady", "node_run", string(nr.ID), map[string]any{
				"failedAttemptId": schedule.FailedAttemptID, "previousAttemptNumber": schedule.AttemptNumber,
			}); err != nil {
				return err
			}
			released = true
			return nil
		})
		if err != nil {
			if errors.Is(err, harnessstore.ErrNotFound) || errors.Is(err, harnessstore.ErrConflict) {
				continue
			}
			return ready, err
		}
		if released {
			ready = append(ready, candidate.NodeRunID)
		}
	}
	return ready, nil
}

// RetryDelay is exposed for query/UI layers without duplicating subtraction
// semantics around zero values.
func RetryDelay(schedule harnessmodel.RetrySchedule) time.Duration {
	if schedule.ScheduledAt.IsZero() || schedule.NotBefore.Before(schedule.ScheduledAt) {
		return 0
	}
	return schedule.NotBefore.Sub(schedule.ScheduledAt)
}
