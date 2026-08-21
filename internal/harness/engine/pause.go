package engine

import (
	"context"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type PauseResult struct {
	WorkflowRun    harnessmodel.WorkflowRun `json:"workflowRun"`
	ActiveAttempts int                      `json:"activeAttempts"`
	Idempotent     bool                     `json:"idempotent,omitempty"`
}

// PauseWorkflow atomically closes the scheduling gate for a running workflow.
// Claims already committed before this transaction are part of the drain set;
// claims that arrive after it observe PAUSING and are rejected by ClaimNode.
func (e *Engine) PauseWorkflow(ctx context.Context, runID harnessmodel.WorkflowRunID) (PauseResult, error) {
	if runID == "" {
		return PauseResult{}, fmt.Errorf("workflow run id is required")
	}
	now := e.now().UTC()
	var result PauseResult
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		run, err := tx.GetWorkflowRun(ctx, runID)
		if err != nil {
			return err
		}
		switch run.State {
		case harnessmodel.WorkflowRunning:
			if err := transitionWorkflow(ctx, tx, &run, harnessmodel.WorkflowPausing, now); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "WorkflowPauseRequested", "workflow_run", string(run.ID), nil); err != nil {
				return err
			}
		case harnessmodel.WorkflowPausing:
			result.Idempotent = true
		case harnessmodel.WorkflowPaused:
			result.WorkflowRun = run
			result.Idempotent = true
			return nil
		default:
			return fmt.Errorf("cannot pause workflow %s from state %s", run.ID, run.State)
		}

		active, err := tx.CountActiveAttempts(ctx, run.ID)
		if err != nil {
			return err
		}
		result.ActiveAttempts = active
		if active == 0 {
			if _, err := e.finalizePauseIfDrained(ctx, tx, &run, now); err != nil {
				return err
			}
		}
		result.WorkflowRun = run
		return nil
	})
	return result, err
}

// ResumeWorkflow reopens scheduling for the same durable WorkflowRun. PAUSING
// is deliberately not resumable: callers must allow its already-owned
// attempts to drain to PAUSED first, avoiding a pause/resume race that could
// otherwise blur which claims belong to the drain set.
func (e *Engine) ResumeWorkflow(ctx context.Context, runID harnessmodel.WorkflowRunID) (harnessmodel.WorkflowRun, error) {
	if runID == "" {
		return harnessmodel.WorkflowRun{}, fmt.Errorf("workflow run id is required")
	}
	now := e.now().UTC()
	var result harnessmodel.WorkflowRun
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		run, err := tx.GetWorkflowRun(ctx, runID)
		if err != nil {
			return err
		}
		switch run.State {
		case harnessmodel.WorkflowPaused:
			if err := transitionWorkflow(ctx, tx, &run, harnessmodel.WorkflowRunning, now); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "WorkflowResumed", "workflow_run", string(run.ID), nil); err != nil {
				return err
			}
		case harnessmodel.WorkflowRunning:
			// Idempotent resume is useful for CLI/API retries.
		case harnessmodel.WorkflowPausing:
			return fmt.Errorf("workflow %s is still PAUSING; active attempts must drain before resume", run.ID)
		default:
			return fmt.Errorf("cannot resume workflow %s from state %s", run.ID, run.State)
		}
		result = run
		return nil
	})
	return result, err
}

func workflowAllowsDrain(state harnessmodel.WorkflowState) bool {
	return state == harnessmodel.WorkflowRunning || state == harnessmodel.WorkflowPausing
}

// finalizePauseIfDrained is called in the same transaction that removes an
// Attempt from the active CLAIMED/RUNNING set. It makes PAUSING -> PAUSED a
// durable consequence of the last active Attempt finishing rather than a
// polling side effect.
func (e *Engine) finalizePauseIfDrained(ctx context.Context, tx harnessstore.Tx, run *harnessmodel.WorkflowRun, at time.Time) (bool, error) {
	if run.State != harnessmodel.WorkflowPausing {
		return false, nil
	}
	active, err := tx.CountActiveAttempts(ctx, run.ID)
	if err != nil {
		return false, err
	}
	if active != 0 {
		return false, nil
	}
	if err := transitionWorkflow(ctx, tx, run, harnessmodel.WorkflowPaused, at); err != nil {
		return false, err
	}
	if _, err := e.appendEvent(ctx, tx, run.ID, at, "WorkflowPaused", "workflow_run", string(run.ID), map[string]any{"activeAttempts": 0}); err != nil {
		return false, err
	}
	return true, nil
}
