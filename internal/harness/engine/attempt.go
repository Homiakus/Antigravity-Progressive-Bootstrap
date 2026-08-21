package engine

import (
	"context"
	"fmt"
	"time"

	harnesslease "github.com/homiakus/agctl/internal/harness/lease"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstate "github.com/homiakus/agctl/internal/harness/state"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type CompletionResult struct {
	Attempt         harnessmodel.Attempt      `json:"attempt"`
	NodeRun         harnessmodel.NodeRun      `json:"nodeRun"`
	WorkflowRun     harnessmodel.WorkflowRun  `json:"workflowRun"`
	ReadyNodeRunIDs []harnessmodel.NodeRunID `json:"readyNodeRunIds,omitempty"`
	Idempotent      bool                      `json:"idempotent,omitempty"`
}

type completionFence struct {
	WorkerID harnessmodel.WorkerID
	Epoch    uint64
}

// StartAttempt is the transitional unleased compatibility path used by legacy
// Stage 3 callers/tests. New runtime code must use ClaimNode +
// StartClaimedAttempt. Unfenced completion below explicitly rejects any Attempt
// whose LeaseEpoch is non-zero, so this helper cannot bypass lease ownership.
func (e *Engine) StartAttempt(ctx context.Context, nodeRunID harnessmodel.NodeRunID) (harnessmodel.Attempt, error) {
	if nodeRunID == "" {
		return harnessmodel.Attempt{}, fmt.Errorf("node run id is required")
	}
	now := e.now().UTC()
	var attempt harnessmodel.Attempt
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		nr, err := tx.GetNodeRun(ctx, nodeRunID)
		if err != nil {
			return err
		}
		run, err := tx.GetWorkflowRun(ctx, nr.WorkflowRunID)
		if err != nil {
			return err
		}
		if run.State != harnessmodel.WorkflowRunning {
			return fmt.Errorf("cannot start node %s while workflow %s is %s", nr.ID, run.ID, run.State)
		}
		if nr.State != harnessmodel.NodeReady {
			return fmt.Errorf("cannot start node %s from state %s", nr.ID, nr.State)
		}
		if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeQueued, now); err != nil {
			return err
		}
		rawAttemptID, err := e.nextID(harnessmodel.IDAttempt)
		if err != nil {
			return err
		}
		attempt, err = tx.CreateNextAttempt(ctx, harnessmodel.Attempt{ID: harnessmodel.AttemptID(rawAttemptID), NodeRunID: nr.ID, State: harnessmodel.AttemptCreated, CreatedAt: now})
		if err != nil {
			return err
		}
		if err := transitionAttempt(ctx, tx, &attempt, harnessmodel.AttemptClaimed); err != nil {
			return err
		}
		if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeRunning, now); err != nil {
			return err
		}
		attempt.StartedAt = now
		if err := transitionAttempt(ctx, tx, &attempt, harnessmodel.AttemptRunning); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, nr.WorkflowRunID, now, "NodeQueued", "node_run", string(nr.ID), map[string]any{"nodeId": nr.NodeID, "compatibilityUnleased": true}); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, nr.WorkflowRunID, now, "AttemptStarted", "attempt", string(attempt.ID), map[string]any{"nodeRunId": nr.ID, "attemptNumber": attempt.Number, "compatibilityUnleased": true}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return harnessmodel.Attempt{}, err
	}
	return attempt, nil
}

func (e *Engine) CompleteAttemptSuccess(ctx context.Context, attemptID harnessmodel.AttemptID) (CompletionResult, error) {
	return e.completeAttemptSuccess(ctx, attemptID, nil)
}

func (e *Engine) CompleteAttemptSuccessFenced(ctx context.Context, attemptID harnessmodel.AttemptID, workerID harnessmodel.WorkerID, epoch uint64) (CompletionResult, error) {
	if workerID == "" || epoch == 0 {
		return CompletionResult{}, fmt.Errorf("worker id and lease epoch are required")
	}
	return e.completeAttemptSuccess(ctx, attemptID, &completionFence{WorkerID: workerID, Epoch: epoch})
}

func (e *Engine) completeAttemptSuccess(ctx context.Context, attemptID harnessmodel.AttemptID, fence *completionFence) (CompletionResult, error) {
	if attemptID == "" {
		return CompletionResult{}, fmt.Errorf("attempt id is required")
	}
	now := e.now().UTC()
	var result CompletionResult
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		attempt, err := tx.GetAttempt(ctx, attemptID)
		if err != nil {
			return err
		}
		if attempt.State == harnessmodel.AttemptSucceeded {
			if err := authorizeTerminalDuplicate(attempt, fence); err != nil {
				return err
			}
			result.Attempt = attempt
			result.Idempotent = true
			if nr, err := tx.GetNodeRun(ctx, attempt.NodeRunID); err == nil {
				result.NodeRun = nr
				result.WorkflowRun, _ = tx.GetWorkflowRun(ctx, nr.WorkflowRunID)
			}
			return nil
		}
		// Ownership/fencing is authoritative before lifecycle semantics. A stale
		// worker must never learn or act on the current attempt state as if it
		// still owned the attempt; in particular a reclaimed CLAIMED attempt must
		// reject the old epoch with ErrStaleFence rather than a state error.
		if err := authorizeCompletion(ctx, tx, attempt, fence, now); err != nil {
			return err
		}
		if attempt.State != harnessmodel.AttemptRunning {
			return fmt.Errorf("cannot succeed attempt %s from state %s", attempt.ID, attempt.State)
		}
		nr, err := tx.GetNodeRun(ctx, attempt.NodeRunID)
		if err != nil {
			return err
		}
		if nr.State != harnessmodel.NodeRunning {
			return fmt.Errorf("attempt %s is RUNNING but node %s is %s", attempt.ID, nr.ID, nr.State)
		}
		run, err := tx.GetWorkflowRun(ctx, nr.WorkflowRunID)
		if err != nil {
			return err
		}

		attempt.FinishedAt = now
		if err := transitionAttempt(ctx, tx, &attempt, harnessmodel.AttemptSucceeded); err != nil {
			return err
		}
		if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeSucceeded, now); err != nil {
			return err
		}
		if fence != nil {
			if err := tx.CloseLease(ctx, attempt.ID, fence.WorkerID, fence.Epoch, harnessmodel.LeaseReleased, now); err != nil {
				return err
			}
		}
		progress, err := tx.IncrementWorkflowProgress(ctx, run.ID, false, now)
		if err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "AttemptSucceeded", "attempt", string(attempt.ID), map[string]any{"nodeRunId": nr.ID, "attemptNumber": attempt.Number, "workerId": attempt.WorkerID, "leaseEpoch": attempt.LeaseEpoch}); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "NodeSucceeded", "node_run", string(nr.ID), map[string]any{"nodeId": nr.NodeID}); err != nil {
			return err
		}

		readyIDs := make([]harnessmodel.NodeRunID, 0)
		if run.State == harnessmodel.WorkflowRunning {
			dependents, err := tx.ListDependentNodeRuns(ctx, run.ID, nr.NodeID)
			if err != nil {
				return err
			}
			for _, child := range dependents {
				remaining, err := tx.DecrementNodeRemainingDependencies(ctx, child.ID, now)
				if err != nil {
					return fmt.Errorf("release dependency %s -> %s: %w", nr.NodeID, child.NodeID, err)
				}
				if remaining != 0 {
					continue
				}
				child, err = tx.GetNodeRun(ctx, child.ID)
				if err != nil {
					return err
				}
				if err := transitionNode(ctx, tx, &child, harnessmodel.NodeReady, now); err != nil {
					return err
				}
				readyIDs = append(readyIDs, child.ID)
				if _, err := e.appendEvent(ctx, tx, run.ID, now, "NodeReady", "node_run", string(child.ID), map[string]any{"nodeId": child.NodeID, "remainingDependencies": 0}); err != nil {
					return err
				}
			}
		}

		if progress.TerminalNodes == progress.TotalNodes && progress.FailedNodes == 0 && run.State == harnessmodel.WorkflowRunning {
			if err := transitionWorkflow(ctx, tx, &run, harnessmodel.WorkflowSucceeded, now); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "WorkflowSucceeded", "workflow_run", string(run.ID), map[string]any{"terminalNodes": progress.TerminalNodes, "totalNodes": progress.TotalNodes}); err != nil {
				return err
			}
		}
		result = CompletionResult{Attempt: attempt, NodeRun: nr, WorkflowRun: run, ReadyNodeRunIDs: readyIDs}
		return nil
	})
	return result, err
}

func (e *Engine) CompleteAttemptFailure(ctx context.Context, attemptID harnessmodel.AttemptID, errorClass, errorMessage string) (CompletionResult, error) {
	return e.completeAttemptFailure(ctx, attemptID, errorClass, errorMessage, nil)
}

func (e *Engine) CompleteAttemptFailureFenced(ctx context.Context, attemptID harnessmodel.AttemptID, workerID harnessmodel.WorkerID, epoch uint64, errorClass, errorMessage string) (CompletionResult, error) {
	if workerID == "" || epoch == 0 {
		return CompletionResult{}, fmt.Errorf("worker id and lease epoch are required")
	}
	return e.completeAttemptFailure(ctx, attemptID, errorClass, errorMessage, &completionFence{WorkerID: workerID, Epoch: epoch})
}

func (e *Engine) completeAttemptFailure(ctx context.Context, attemptID harnessmodel.AttemptID, errorClass, errorMessage string, fence *completionFence) (CompletionResult, error) {
	if attemptID == "" {
		return CompletionResult{}, fmt.Errorf("attempt id is required")
	}
	now := e.now().UTC()
	var result CompletionResult
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		attempt, err := tx.GetAttempt(ctx, attemptID)
		if err != nil {
			return err
		}
		if attempt.State == harnessmodel.AttemptFailed {
			if err := authorizeTerminalDuplicate(attempt, fence); err != nil {
				return err
			}
			result.Attempt = attempt
			result.Idempotent = true
			if nr, err := tx.GetNodeRun(ctx, attempt.NodeRunID); err == nil {
				result.NodeRun = nr
				result.WorkflowRun, _ = tx.GetWorkflowRun(ctx, nr.WorkflowRunID)
			}
			return nil
		}
		if err := authorizeCompletion(ctx, tx, attempt, fence, now); err != nil {
			return err
		}
		if attempt.State != harnessmodel.AttemptRunning {
			return fmt.Errorf("cannot fail attempt %s from state %s", attempt.ID, attempt.State)
		}
		nr, err := tx.GetNodeRun(ctx, attempt.NodeRunID)
		if err != nil {
			return err
		}
		if nr.State != harnessmodel.NodeRunning {
			return fmt.Errorf("attempt %s is RUNNING but node %s is %s", attempt.ID, nr.ID, nr.State)
		}
		run, err := tx.GetWorkflowRun(ctx, nr.WorkflowRunID)
		if err != nil {
			return err
		}

		attempt.ErrorClass = errorClass
		attempt.ErrorMessage = errorMessage
		attempt.FinishedAt = now
		if err := transitionAttempt(ctx, tx, &attempt, harnessmodel.AttemptFailed); err != nil {
			return err
		}
		if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeFailed, now); err != nil {
			return err
		}
		if fence != nil {
			if err := tx.CloseLease(ctx, attempt.ID, fence.WorkerID, fence.Epoch, harnessmodel.LeaseReleased, now); err != nil {
				return err
			}
		}
		progress, err := tx.IncrementWorkflowProgress(ctx, run.ID, true, now)
		if err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "AttemptFailed", "attempt", string(attempt.ID), map[string]any{"nodeRunId": nr.ID, "attemptNumber": attempt.Number, "errorClass": errorClass, "error": errorMessage, "workerId": attempt.WorkerID, "leaseEpoch": attempt.LeaseEpoch}); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "NodeFailed", "node_run", string(nr.ID), map[string]any{"nodeId": nr.NodeID, "errorClass": errorClass}); err != nil {
			return err
		}
		if run.State == harnessmodel.WorkflowRunning {
			if err := transitionWorkflow(ctx, tx, &run, harnessmodel.WorkflowFailed, now); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "WorkflowFailed", "workflow_run", string(run.ID), map[string]any{"failedNodeId": nr.NodeID, "failedNodes": progress.FailedNodes}); err != nil {
				return err
			}
		}
		result = CompletionResult{Attempt: attempt, NodeRun: nr, WorkflowRun: run}
		return nil
	})
	return result, err
}

func authorizeCompletion(ctx context.Context, tx harnessstore.Tx, attempt harnessmodel.Attempt, fence *completionFence, now time.Time) error {
	if fence == nil {
		if attempt.LeaseEpoch != 0 || attempt.WorkerID != "" {
			return fmt.Errorf("attempt %s is lease-owned; fenced completion is required", attempt.ID)
		}
		return nil
	}
	return authorizeLease(ctx, tx, attempt, fence.WorkerID, fence.Epoch, now)
}

func authorizeTerminalDuplicate(attempt harnessmodel.Attempt, fence *completionFence) error {
	if fence == nil {
		if attempt.LeaseEpoch != 0 || attempt.WorkerID != "" {
			return fmt.Errorf("attempt %s is lease-owned; fenced completion is required", attempt.ID)
		}
		return nil
	}
	if attempt.WorkerID != fence.WorkerID || attempt.LeaseEpoch != fence.Epoch {
		return harnesslease.ErrStaleFence
	}
	return nil
}

func transitionNode(ctx context.Context, tx harnessstore.Tx, nr *harnessmodel.NodeRun, target harnessmodel.NodeState, at time.Time) error {
	if err := harnessstate.TransitionNode(nr.State, target); err != nil {
		return err
	}
	expected := nr.State
	nr.State = target
	nr.UpdatedAt = at.UTC()
	return tx.CompareAndSwapNodeRun(ctx, expected, *nr)
}

func transitionAttempt(ctx context.Context, tx harnessstore.Tx, attempt *harnessmodel.Attempt, target harnessmodel.AttemptState) error {
	if err := harnessstate.TransitionAttempt(attempt.State, target); err != nil {
		return err
	}
	expected := attempt.State
	attempt.State = target
	return tx.CompareAndSwapAttempt(ctx, expected, *attempt)
}
