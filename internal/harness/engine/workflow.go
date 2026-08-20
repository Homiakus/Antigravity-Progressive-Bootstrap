package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	harnesscompiler "github.com/homiakus/agctl/internal/harness/compiler"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstate "github.com/homiakus/agctl/internal/harness/state"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func (e *Engine) StartWorkflow(ctx context.Context, def harnessmodel.WorkflowDefinition) (harnessmodel.WorkflowRun, error) {
	if err := harnesscompiler.ValidateCompiled(def); err != nil {
		return harnessmodel.WorkflowRun{}, fmt.Errorf("validate workflow definition: %w", err)
	}
	now := e.now().UTC()
	rawRunID, err := e.nextID(harnessmodel.IDWorkflowRun)
	if err != nil {
		return harnessmodel.WorkflowRun{}, err
	}
	run := harnessmodel.WorkflowRun{
		ID:                harnessmodel.WorkflowRunID(rawRunID),
		DefinitionID:      def.ID,
		DefinitionVersion: def.Version,
		State:             harnessmodel.WorkflowCreated,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	err = e.store.Update(ctx, func(tx harnessstore.Tx) error {
		if err := ensureDefinition(ctx, tx, def); err != nil {
			return err
		}
		if err := tx.CreateWorkflowRun(ctx, run); err != nil {
			return err
		}
		if err := tx.CreateWorkflowProgress(ctx, harnessmodel.WorkflowProgress{WorkflowRunID: run.ID, TotalNodes: len(def.Nodes), UpdatedAt: now}); err != nil {
			return err
		}
		if err := tx.CreateGraphRevision(ctx, harnessmodel.GraphRevision{WorkflowRunID: run.ID, Number: 1, CreatedAt: now, Reason: "initial workflow graph"}); err != nil {
			return err
		}
		run.CurrentGraphRevision = 1

		ready := make([]harnessmodel.NodeRun, 0)
		for _, node := range def.Nodes {
			rawNodeRunID, err := e.nextID(harnessmodel.IDNodeRun)
			if err != nil {
				return err
			}
			nodeState := harnessmodel.NodePendingDependencies
			if len(node.Dependencies) == 0 {
				nodeState = harnessmodel.NodeReady
			}
			nr := harnessmodel.NodeRun{
				ID:                    harnessmodel.NodeRunID(rawNodeRunID),
				WorkflowRunID:         run.ID,
				NodeID:                node.ID,
				GraphRevision:         1,
				Generation:            1,
				CreatedAt:             now,
				UpdatedAt:             now,
				State:                 nodeState,
				RemainingDependencies: len(node.Dependencies),
			}
			if err := tx.CreateNodeRun(ctx, nr); err != nil {
				return err
			}
			if nodeState == harnessmodel.NodeReady {
				ready = append(ready, nr)
			}
		}

		for _, target := range []harnessmodel.WorkflowState{harnessmodel.WorkflowValidating, harnessmodel.WorkflowQueued, harnessmodel.WorkflowRunning} {
			if err := transitionWorkflow(ctx, tx, &run, target, now); err != nil {
				return err
			}
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "WorkflowStarted", "workflow_run", string(run.ID), map[string]any{"definitionId": def.ID, "definitionVersion": def.Version, "graphRevision": 1, "nodes": len(def.Nodes)}); err != nil {
			return err
		}
		for _, nr := range ready {
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "NodeReady", "node_run", string(nr.ID), map[string]any{"nodeId": nr.NodeID, "remainingDependencies": 0}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return harnessmodel.WorkflowRun{}, err
	}
	return run, nil
}

func ensureDefinition(ctx context.Context, tx harnessstore.Tx, def harnessmodel.WorkflowDefinition) error {
	existing, err := tx.GetWorkflowDefinition(ctx, def.ID, def.Version)
	if err == nil {
		a, marshalErr := json.Marshal(existing)
		if marshalErr != nil {
			return fmt.Errorf("marshal existing workflow definition: %w", marshalErr)
		}
		b, marshalErr := json.Marshal(def)
		if marshalErr != nil {
			return fmt.Errorf("marshal workflow definition: %w", marshalErr)
		}
		if !bytes.Equal(a, b) {
			return fmt.Errorf("workflow definition %s v%d already exists with different immutable content", def.ID, def.Version)
		}
		return nil
	}
	if !errors.Is(err, harnessstore.ErrNotFound) {
		return err
	}
	return tx.CreateWorkflowDefinition(ctx, def)
}

func transitionWorkflow(ctx context.Context, tx harnessstore.Tx, run *harnessmodel.WorkflowRun, target harnessmodel.WorkflowState, at time.Time) error {
	if err := harnessstate.TransitionWorkflow(run.State, target); err != nil {
		return err
	}
	expected := run.State
	run.State = target
	run.UpdatedAt = at.UTC()
	if err := tx.CompareAndSwapWorkflowRun(ctx, expected, *run); err != nil {
		return err
	}
	return nil
}
