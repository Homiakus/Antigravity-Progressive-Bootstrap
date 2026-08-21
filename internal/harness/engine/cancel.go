package engine

import (
	"context"
	"fmt"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type CancelResult struct {
	WorkflowRun harnessmodel.WorkflowRun              `json:"workflowRun"`
	Stats       harnessstore.WorkflowCancellationStats `json:"stats"`
	Idempotent  bool                                  `json:"idempotent,omitempty"`
}

// CancelWorkflow is state-authoritative cancellation. It first closes the
// scheduling gate, then fences active attempts/releases leases and cancels all
// remaining durable waits in the same SQLite transaction. A physical process
// that ignores cancellation may still run briefly, but any late completion is
// rejected because its Attempt is terminal and its lease is no longer active.
func (e *Engine) CancelWorkflow(ctx context.Context, runID harnessmodel.WorkflowRunID) (CancelResult, error) {
	if runID == "" {
		return CancelResult{}, fmt.Errorf("workflow run id is required")
	}
	now := e.now().UTC()
	var result CancelResult
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		run, err := tx.GetWorkflowRun(ctx, runID)
		if err != nil {
			return err
		}
		if run.State == harnessmodel.WorkflowCancelled {
			result.WorkflowRun = run
			result.Idempotent = true
			return nil
		}
		if run.State == harnessmodel.WorkflowSucceeded || run.State == harnessmodel.WorkflowFailed {
			return fmt.Errorf("cannot cancel terminal workflow %s in state %s", run.ID, run.State)
		}

		switch run.State {
		case harnessmodel.WorkflowCreated, harnessmodel.WorkflowValidating:
			if err := transitionWorkflow(ctx, tx, &run, harnessmodel.WorkflowCancelled, now); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "WorkflowCancelled", "workflow_run", string(run.ID), map[string]any{"earlyLifecycle": true}); err != nil {
				return err
			}
			result.WorkflowRun = run
			return nil
		case harnessmodel.WorkflowCancelling:
			result.Idempotent = true
		case harnessmodel.WorkflowQueued, harnessmodel.WorkflowRunning, harnessmodel.WorkflowPausing, harnessmodel.WorkflowPaused, harnessmodel.WorkflowBlocked:
			if err := transitionWorkflow(ctx, tx, &run, harnessmodel.WorkflowCancelling, now); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "WorkflowCancellationRequested", "workflow_run", string(run.ID), nil); err != nil {
				return err
			}
		default:
			return fmt.Errorf("cannot cancel workflow %s from state %s", run.ID, run.State)
		}

		stats, err := tx.CancelWorkflowRuntime(ctx, run.ID, now)
		if err != nil {
			return err
		}
		if err := transitionWorkflow(ctx, tx, &run, harnessmodel.WorkflowCancelled, now); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "WorkflowCancelled", "workflow_run", string(run.ID), map[string]any{
			"cancelledNodes": stats.Nodes, "cancelledAttempts": stats.Attempts, "releasedLeases": stats.Leases,
			"cancelledTimers": stats.Timers, "cancelledSignalWaits": stats.SignalWaits,
			"cancelledApprovals": stats.Approvals, "cancelledRetries": stats.Retries,
		}); err != nil {
			return err
		}
		result = CancelResult{WorkflowRun: run, Stats: stats, Idempotent: result.Idempotent}
		return nil
	})
	return result, err
}
