package engine

import (
	"context"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type DependencyReplacement struct {
	NodeID          harnessmodel.NodeID `json:"nodeId"`
	OldDependencyID harnessmodel.NodeID `json:"oldDependencyId"`
	NewDependencyID harnessmodel.NodeID `json:"newDependencyId"`
}

type GraphMutation struct {
	WorkflowRunID        harnessmodel.WorkflowRunID `json:"workflowRunId"`
	ExpectedRevision     int                        `json:"expectedRevision"`
	TriggerNodeRunID     harnessmodel.NodeRunID     `json:"triggerNodeRunId,omitempty"`
	Reason               string                     `json:"reason"`
	Evidence             string                     `json:"evidence,omitempty"`
	AddNodes             []harnessmodel.NodeSpec    `json:"addNodes,omitempty"`
	ReplaceDependencies []DependencyReplacement    `json:"replaceDependencies,omitempty"`
	SupersedeNodeRuns    []harnessmodel.NodeRunID   `json:"supersedeNodeRuns,omitempty"`
}

type MutationResult struct {
	NewRevision int                      `json:"newRevision"`
	AddedNodes  []harnessmodel.NodeRunID `json:"addedNodes"`
}

func (e *Engine) ApplyGraphMutation(ctx context.Context, mutation GraphMutation) (MutationResult, error) {
	if mutation.WorkflowRunID == "" {
		return MutationResult{}, fmt.Errorf("workflow run id is required")
	}
	if mutation.ExpectedRevision < 0 {
		return MutationResult{}, fmt.Errorf("expected revision must be non-negative")
	}

	now := e.now().UTC()
	var result MutationResult

	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		run, err := tx.GetWorkflowRun(ctx, mutation.WorkflowRunID)
		if err != nil {
			return err
		}
		if run.State.Terminal() || run.State == harnessmodel.WorkflowCancelling {
			return fmt.Errorf("workflow %s is in state %s; cannot mutate graph", run.ID, run.State)
		}
		if run.CurrentGraphRevision != mutation.ExpectedRevision {
			return fmt.Errorf("optimistic concurrency conflict: expected revision %d, current is %d: %w",
				mutation.ExpectedRevision, run.CurrentGraphRevision, harnessstore.ErrConflict)
		}

		newRev := run.CurrentGraphRevision + 1
		result.NewRevision = newRev

		rev := harnessmodel.GraphRevision{
			WorkflowRunID: run.ID,
			Number:        newRev,
			ParentNumber:  run.CurrentGraphRevision,
			Reason:        mutation.Reason,
			CreatedAt:     now,
		}
		if err := tx.CreateGraphRevision(ctx, rev); err != nil {
			return fmt.Errorf("create graph revision: %w", err)
		}

		// 1. Process AddNodes
		var addedNodeRuns []harnessmodel.NodeRun
		for _, node := range mutation.AddNodes {
			if err := harnessmodel.ValidateNodeID(node.ID); err != nil {
				return err
			}
			if err := tx.AddWorkflowNode(ctx, run.DefinitionID, run.DefinitionVersion, node); err != nil {
				return err
			}
			for _, dep := range node.Dependencies {
				if err := tx.AddWorkflowDependency(ctx, run.DefinitionID, run.DefinitionVersion, node.ID, dep); err != nil {
					return err
				}
			}

			rawID, err := e.nextID(harnessmodel.IDNodeRun)
			if err != nil {
				return err
			}
			nrID := harnessmodel.NodeRunID(rawID)

			remaining := len(node.Dependencies)
			nodeState := harnessmodel.NodePendingDependencies
			if remaining == 0 {
				nodeState = harnessmodel.NodeReady
			}

			nr := harnessmodel.NodeRun{
				ID:                    nrID,
				WorkflowRunID:         run.ID,
				NodeID:                node.ID,
				GraphRevision:         newRev,
				Generation:            1,
				State:                 nodeState,
				RemainingDependencies: remaining,
				Priority:              node.Priority,
				EffectivePriority:     node.Priority,
				CreatedAt:             now,
				UpdatedAt:             now,
			}
			if err := tx.CreateNodeRun(ctx, nr); err != nil {
				return fmt.Errorf("create dynamic node run %s: %w", node.ID, err)
			}
			if nodeState == harnessmodel.NodeReady {
				if err := tx.EnqueueReadyNode(ctx, nr.ID, now, time.Time{}, ""); err != nil {
					return err
				}
			}
			addedNodeRuns = append(addedNodeRuns, nr)
			result.AddedNodes = append(result.AddedNodes, nrID)
		}

		if len(mutation.AddNodes) > 0 {
			if err := tx.UpdateWorkflowProgressTotalNodes(ctx, run.ID, len(mutation.AddNodes)); err != nil {
				return err
			}
		}

		// 2. Process ReplaceDependencies
		for _, rep := range mutation.ReplaceDependencies {
			if err := tx.RemoveWorkflowDependency(ctx, run.DefinitionID, run.DefinitionVersion, rep.NodeID, rep.OldDependencyID); err != nil {
				return err
			}
			if err := tx.AddWorkflowDependency(ctx, run.DefinitionID, run.DefinitionVersion, rep.NodeID, rep.NewDependencyID); err != nil {
				return err
			}
		}

		// 3. Process SupersedeNodeRuns
		for _, nrID := range mutation.SupersedeNodeRuns {
			nr, err := tx.GetNodeRun(ctx, nrID)
			if err != nil {
				return err
			}
			if nr.State == harnessmodel.NodePendingDependencies || nr.State == harnessmodel.NodeReady || nr.State == harnessmodel.NodeWaiting {
				if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeCancelled, now); err != nil {
					return err
				}
				_ = tx.RemoveReadyNode(ctx, nr.ID)
			}
		}

		// 4. Record GraphMutated event
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "GraphMutated", "graph_revision", fmt.Sprintf("%d", newRev), map[string]any{
			"previousRevision":     run.CurrentGraphRevision,
			"newRevision":          newRev,
			"reason":               mutation.Reason,
			"evidence":             mutation.Evidence,
			"triggerNodeRunId":     mutation.TriggerNodeRunID,
			"addedNodesCount":      len(mutation.AddNodes),
			"supersededNodesCount": len(mutation.SupersedeNodeRuns),
		}); err != nil {
			return err
		}

		return nil
	})

	return result, err
}
