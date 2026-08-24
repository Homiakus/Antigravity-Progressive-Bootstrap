package replan

import (
	"context"
	"fmt"
	"strings"

	harnessengine "github.com/homiakus/agctl/internal/harness/engine"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/model"
)

// BuildHarnessMutation translates an adaptive replan proposal into a transactional GraphMutation.
func BuildHarnessMutation(run harnessmodel.WorkflowRun, triggerNodeRun harnessmodel.NodeRun, proposal model.ReplanProposal, cfg model.ReplanConfig) (harnessengine.GraphMutation, error) {
	if proposal.Confidence < cfg.MinConfidence {
		return harnessengine.GraphMutation{}, fmt.Errorf("proposal confidence %.2f is below threshold %.2f", proposal.Confidence, cfg.MinConfidence)
	}
	if cfg.RequireEvidence && len(nonEmpty(proposal.Evidence)) == 0 {
		return harnessengine.GraphMutation{}, fmt.Errorf("proposal contains no concrete evidence")
	}
	if len(proposal.Actions) == 0 {
		return harnessengine.GraphMutation{}, fmt.Errorf("proposal contains no actions")
	}
	for _, a := range proposal.Actions {
		if !riskAllowed(a.Risk, cfg.AutoApplyRiskMax) {
			return harnessengine.GraphMutation{}, fmt.Errorf("action %s risk %q exceeds auto-apply threshold %q", a.ID, a.Risk, cfg.AutoApplyRiskMax)
		}
	}

	nextRev := run.CurrentGraphRevision + 1
	prefix := fmt.Sprintf("r%d-", nextRev)
	actions, err := normalizeActions(proposal.Actions, prefix)
	if err != nil {
		return harnessengine.GraphMutation{}, err
	}
	if len(topoActions(actions)) != len(actions) {
		return harnessengine.GraphMutation{}, fmt.Errorf("replan proposal contains an action dependency cycle")
	}

	var addNodes []harnessmodel.NodeSpec
	for _, a := range actions {
		var deps []harnessmodel.NodeID
		for _, d := range a.DependsOn {
			deps = append(deps, harnessmodel.NodeID(d))
		}
		if len(deps) == 0 && triggerNodeRun.NodeID != "" {
			deps = append(deps, triggerNodeRun.NodeID)
		}

		addNodes = append(addNodes, harnessmodel.NodeSpec{
			ID:           harnessmodel.NodeID(a.ID),
			Kind:         harnessmodel.NodeKindAction,
			ExecutorKind: harnessmodel.ExecutorProcess,
			Dependencies: deps,
			Metadata: map[string]string{
				"title":     a.Title,
				"objective": a.Objective,
				"risk":      a.Risk,
			},
		})
	}

	return harnessengine.GraphMutation{
		WorkflowRunID:    run.ID,
		ExpectedRevision: run.CurrentGraphRevision,
		TriggerNodeRunID: triggerNodeRun.ID,
		Reason:           proposal.Reason,
		Evidence:         strings.Join(proposal.Evidence, "; "),
		AddNodes:         addNodes,
	}, nil
}

// ApplyHarnessMutation converts a proposal and applies it atomically through the engine.
func ApplyHarnessMutation(ctx context.Context, eng *harnessengine.Engine, run harnessmodel.WorkflowRun, triggerNodeRun harnessmodel.NodeRun, proposal model.ReplanProposal, cfg model.ReplanConfig) (harnessengine.MutationResult, error) {
	if eng == nil {
		return harnessengine.MutationResult{}, fmt.Errorf("engine is required")
	}
	mutation, err := BuildHarnessMutation(run, triggerNodeRun, proposal, cfg)
	if err != nil {
		return harnessengine.MutationResult{}, err
	}
	return eng.ApplyGraphMutation(ctx, mutation)
}
