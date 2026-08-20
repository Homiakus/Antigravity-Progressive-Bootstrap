package planner

import (
	"fmt"
	"strings"
	"time"

	harnesscompiler "github.com/homiakus/agctl/internal/harness/compiler"
	"github.com/homiakus/agctl/internal/harness/ir"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	legacymodel "github.com/homiakus/agctl/internal/model"
)

// HarnessDraft converts the legacy 3.2 ExecutionPlan into the new durable
// workflow IR without mutating or persisting the legacy plan. It is the
// strangler seam used while the JSON task runtime remains the default path.
func HarnessDraft(plan legacymodel.ExecutionPlan) (ir.Definition, error) {
	if strings.TrimSpace(plan.ID) == "" {
		return ir.Definition{}, fmt.Errorf("legacy plan id is required")
	}
	createdAt := time.Time{}
	if strings.TrimSpace(plan.CreatedAt) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, plan.CreatedAt)
		if err != nil {
			return ir.Definition{}, fmt.Errorf("parse legacy plan createdAt: %w", err)
		}
		createdAt = parsed.UTC()
	}

	nodes := make([]harnessmodel.NodeSpec, 0, len(plan.Nodes))
	for _, n := range plan.Nodes {
		deps := make([]harnessmodel.NodeID, 0, len(n.DependsOn))
		for _, dep := range n.DependsOn {
			deps = append(deps, harnessmodel.NodeID(dep))
		}
		metadata := map[string]string{
			"legacyPlanId": plan.ID,
			"title":        n.Title,
			"objective":    n.Objective,
			"agent":        n.Agent,
		}
		if len(n.Tags) > 0 {
			metadata["tags"] = strings.Join(n.Tags, ",")
		}
		if len(n.Verification) > 0 {
			metadata["verification"] = strings.Join(n.Verification, "\n")
		}
		nodes = append(nodes, harnessmodel.NodeSpec{
			ID:           harnessmodel.NodeID(n.ID),
			Kind:         harnessmodel.NodeKindAction,
			ExecutorKind: harnessmodel.ExecutorAgent,
			Dependencies: deps,
			Resources: harnessmodel.ResourceSpec{
				CPUWeight:          n.Resources.CPUWeight,
				BuildSlots:         n.Resources.BuildSlots,
				BrowserSlots:       n.Resources.BrowserSlots,
				ExclusiveWorkspace: n.Resources.ExclusiveWorkspace,
				ReadOnly:           n.Resources.ReadOnly,
			},
			Policy:      harnessmodel.PolicySpec{Risk: n.Risk},
			CachePolicy: harnessmodel.CacheDisabled,
			Metadata:    metadata,
		})
	}

	return ir.Definition{
		Name:      plan.ID,
		CreatedAt: createdAt,
		Nodes:     nodes,
		Metadata: map[string]string{
			"legacyPlanId": plan.ID,
			"generatedBy":  plan.GeneratedBy,
			"prompt":       plan.Prompt,
			"workspace":    plan.Workspace,
		},
	}, nil
}

// CompileHarnessDefinition compiles a legacy plan through the new Harness
// compiler. The returned definition is immutable execution input; this helper
// intentionally performs no runtime persistence.
func CompileHarnessDefinition(plan legacymodel.ExecutionPlan, opts harnesscompiler.Options) (harnessmodel.WorkflowDefinition, error) {
	draft, err := HarnessDraft(plan)
	if err != nil {
		return harnessmodel.WorkflowDefinition{}, err
	}
	return harnesscompiler.Compile(draft, opts)
}
