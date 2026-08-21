package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

// WaitUntil moves a WAIT control-flow node out of the scheduler and persists a
// durable wakeup deadline. No worker, process or goroutine is held while the
// timer is pending.
func (e *Engine) WaitUntil(ctx context.Context, nodeRunID harnessmodel.NodeRunID, dueAt time.Time, payload []byte) (harnessmodel.Timer, error) {
	if nodeRunID == "" {
		return harnessmodel.Timer{}, fmt.Errorf("node run id is required")
	}
	now := e.now().UTC()
	dueAt = dueAt.UTC()
	if dueAt.IsZero() || !dueAt.After(now) {
		return harnessmodel.Timer{}, fmt.Errorf("timer due time must be after current time")
	}
	rawTimerID, err := e.nextID(harnessmodel.IDTimer)
	if err != nil {
		return harnessmodel.Timer{}, err
	}
	var timer harnessmodel.Timer
	err = e.store.Update(ctx, func(tx harnessstore.Tx) error {
		nr, err := tx.GetNodeRun(ctx, nodeRunID)
		if err != nil {
			return err
		}
		if nr.State != harnessmodel.NodeReady {
			return fmt.Errorf("cannot wait node %s from state %s", nr.ID, nr.State)
		}
		run, err := tx.GetWorkflowRun(ctx, nr.WorkflowRunID)
		if err != nil {
			return err
		}
		if run.State != harnessmodel.WorkflowRunning {
			return fmt.Errorf("cannot schedule timer for node %s while workflow %s is %s", nr.ID, run.ID, run.State)
		}
		def, err := tx.GetWorkflowDefinition(ctx, run.DefinitionID, run.DefinitionVersion)
		if err != nil {
			return err
		}
		node, ok := findNodeSpec(def, nr.NodeID)
		if !ok {
			return fmt.Errorf("node spec %s not found in durable definition", nr.NodeID)
		}
		if node.Kind != harnessmodel.NodeKindWait {
			return fmt.Errorf("node %s kind %s cannot use durable node timer", nr.NodeID, node.Kind)
		}
		if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeWaiting, now); err != nil {
			return err
		}
		if err := tx.RemoveReadyNode(ctx, nr.ID); err != nil {
			return err
		}
		timer = harnessmodel.Timer{
			ID: harnessmodel.TimerID(rawTimerID), WorkflowRunID: run.ID, NodeRunID: nr.ID,
			Kind: harnessmodel.TimerNodeWait, Payload: append([]byte(nil), payload...),
			State: harnessmodel.TimerPending, DueAt: dueAt, CreatedAt: now,
		}
		if err := tx.CreateTimer(ctx, timer); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "NodeWaiting", "node_run", string(nr.ID), map[string]any{"nodeId": nr.NodeID, "waitKind": "timer", "dueAt": dueAt}); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "TimerScheduled", "timer", string(timer.ID), map[string]any{"nodeRunId": nr.ID, "kind": timer.Kind, "dueAt": dueAt}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return harnessmodel.Timer{}, err
	}
	return timer, nil
}

// ReleaseDueTimers atomically fires due NODE_WAIT timers and restores their
// nodes to READY. A paused workflow can accumulate READY nodes safely because
// scheduler/ClaimNode both require WorkflowRunning before dispatch.
func (e *Engine) ReleaseDueTimers(ctx context.Context, limit int) ([]harnessmodel.NodeRunID, error) {
	now := e.now().UTC()
	var due []harnessmodel.Timer
	if err := e.store.View(ctx, func(reader harnessstore.Reader) error {
		var err error
		due, err = reader.ListDueTimers(ctx, now, limit)
		return err
	}); err != nil {
		return nil, err
	}
	ready := make([]harnessmodel.NodeRunID, 0, len(due))
	for _, candidate := range due {
		var released bool
		err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
			timer, err := tx.GetTimer(ctx, candidate.ID)
			if err != nil {
				return err
			}
			if timer.State != harnessmodel.TimerPending || timer.DueAt.After(now) {
				return nil
			}
			if timer.Kind != harnessmodel.TimerNodeWait {
				// Other Stage 8 timer kinds are resolved by their specialized
				// services; do not consume them here.
				return nil
			}
			nr, err := tx.GetNodeRun(ctx, timer.NodeRunID)
			if err != nil {
				return err
			}
			if nr.WorkflowRunID != timer.WorkflowRunID {
				return fmt.Errorf("timer %s workflow/node mismatch: %w", timer.ID, harnessstore.ErrConflict)
			}
			if nr.State != harnessmodel.NodeWaiting {
				return fmt.Errorf("timer %s has node %s in state %s: %w", timer.ID, nr.ID, nr.State, harnessstore.ErrConflict)
			}
			if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeReady, now); err != nil {
				return err
			}
			if err := tx.EnqueueReadyNode(ctx, nr.ID, now, time.Time{}, ""); err != nil {
				return err
			}
			timer.State = harnessmodel.TimerFired
			timer.ResolvedAt = now
			if err := tx.CompareAndSwapTimer(ctx, harnessmodel.TimerPending, timer); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, timer.WorkflowRunID, now, "TimerFired", "timer", string(timer.ID), map[string]any{"nodeRunId": nr.ID, "kind": timer.Kind}); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, timer.WorkflowRunID, now, "NodeReady", "node_run", string(nr.ID), map[string]any{"nodeId": nr.NodeID, "wakeReason": "timer", "timerId": timer.ID}); err != nil {
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
