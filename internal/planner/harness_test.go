package planner

import (
	"bytes"
	"testing"
	"time"

	harnesscompiler "github.com/homiakus/agctl/internal/harness/compiler"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	legacymodel "github.com/homiakus/agctl/internal/model"
)

func TestHarnessDraftPreservesLegacyGraphSemantics(t *testing.T) {
	plan := legacymodel.ExecutionPlan{
		ID:          "plan-1",
		Prompt:      "test",
		Workspace:   "/tmp/work",
		CreatedAt:   "2026-08-20T12:00:00Z",
		GeneratedBy: "legacy-planner",
		Nodes: []legacymodel.PlanNode{
			{ID: "inspect", Title: "Inspect", Objective: "inspect repo", Agent: "architect", Resources: legacymodel.ResourceRequest{CPUWeight: 15, ReadOnly: true}, Risk: "read-low"},
			{ID: "implement", Title: "Implement", Objective: "write change", Agent: "implementer", DependsOn: []string{"inspect"}, Resources: legacymodel.ResourceRequest{CPUWeight: 45, BuildSlots: 1, ExclusiveWorkspace: true}, Risk: "write-medium"},
		},
	}
	draft, err := HarnessDraft(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Nodes) != 2 || draft.Nodes[1].Dependencies[0] != "inspect" {
		t.Fatalf("legacy dependency graph was not preserved: %+v", draft.Nodes)
	}
	if draft.Nodes[0].ExecutorKind != harnessmodel.ExecutorAgent || draft.Nodes[1].Resources.BuildSlots != 1 {
		t.Fatalf("legacy execution/resource semantics were not preserved: %+v", draft.Nodes)
	}
}

func TestCompileHarnessDefinitionFromLegacyPlan(t *testing.T) {
	plan := legacymodel.ExecutionPlan{ID: "plan-1", CreatedAt: "2026-08-20T12:00:00Z", Nodes: []legacymodel.PlanNode{{ID: "inspect", Agent: "architect"}}}
	gen := harnessmodel.TimeSortableIDGenerator{Now: func() time.Time { return time.UnixMilli(1) }, Random: bytes.NewReader(make([]byte, 10))}
	def, err := CompileHarnessDefinition(plan, harnesscompiler.Options{IDs: gen})
	if err != nil {
		t.Fatal(err)
	}
	if def.ID == "" || len(def.Nodes) != 1 || def.Nodes[0].ID != "inspect" {
		t.Fatalf("unexpected compiled definition: %+v", def)
	}
}
