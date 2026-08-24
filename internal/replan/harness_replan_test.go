package replan

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	harnessengine "github.com/homiakus/agctl/internal/harness/engine"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
	"github.com/homiakus/agctl/internal/model"
)

func TestBuildHarnessMutationValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinConfidence = 0.8
	cfg.RequireEvidence = true
	cfg.AutoApplyRiskMax = "write-medium"

	run := harnessmodel.WorkflowRun{
		ID:                   "wfr_rep_1",
		CurrentGraphRevision: 1,
	}
	trigger := harnessmodel.NodeRun{
		ID:     "nr_step1",
		NodeID: "step1",
	}

	// 1. Low confidence
	lowConf := model.ReplanProposal{
		Confidence: 0.5,
		Evidence:   []string{"logs showed timeout"},
		Actions:    []model.ReplanAction{{ID: "act1", Title: "Act 1", Objective: "do read", Risk: "read-low"}},
	}
	if _, err := BuildHarnessMutation(run, trigger, lowConf, cfg); err == nil {
		t.Fatal("expected error on low confidence")
	}

	// 2. Missing evidence
	noEvidence := model.ReplanProposal{
		Confidence: 0.9,
		Actions:    []model.ReplanAction{{ID: "act1", Title: "Act 1", Objective: "do read", Risk: "read-low"}},
	}
	if _, err := BuildHarnessMutation(run, trigger, noEvidence, cfg); err == nil {
		t.Fatal("expected error on missing evidence")
	}

	// 3. High risk
	highRisk := model.ReplanProposal{
		Confidence: 0.9,
		Evidence:   []string{"evidence"},
		Actions:    []model.ReplanAction{{ID: "act1", Title: "Act 1", Objective: "do destructive", Risk: "destructive-critical"}},
	}
	if _, err := BuildHarnessMutation(run, trigger, highRisk, cfg); err == nil {
		t.Fatal("expected error on high risk action")
	}

	// 4. Valid proposal
	validProposal := model.ReplanProposal{
		Reason:     "add recovery step",
		Confidence: 0.95,
		Evidence:   []string{"compilation failed with missing package"},
		Actions: []model.ReplanAction{
			{ID: "install_pkg", Title: "Install Package", Objective: "install go package", Risk: "write-medium"},
		},
	}
	mut, err := BuildHarnessMutation(run, trigger, validProposal, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if mut.ExpectedRevision != 1 || len(mut.AddNodes) != 1 {
		t.Fatalf("unexpected mutation: %+v", mut)
	}
	if mut.AddNodes[0].ID != "r2-install-pkg" {
		t.Fatalf("expected prefixed node id r2-install-pkg, got %s", mut.AddNodes[0].ID)
	}
}

func TestApplyHarnessMutationEndToEnd(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	db, err := sqlitestore.Open(ctx, filepath.Join(tempDir, "state.db"), sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Unix(170_000, 0).UTC()
	eng, err := harnessengine.New(db, harnessengine.Options{
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	def := harnessmodel.WorkflowDefinition{
		ID:              "wfd_rep_test",
		Version:         1,
		Name:            "replan-test",
		CreatedAt:       now,
		CompilerVersion: "test",
		Nodes: []harnessmodel.NodeSpec{
			{ID: "init", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess},
		},
	}

	run, err := eng.StartWorkflow(ctx, def)
	if err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	proposal := model.ReplanProposal{
		Reason:     "adaptive repair on compilation error",
		Confidence: 0.9,
		Evidence:   []string{"build error in init"},
		Actions: []model.ReplanAction{
			{ID: "fix_deps", Title: "Fix Dependencies", Objective: "download deps", Risk: "write-medium"},
			{ID: "rebuild", Title: "Rebuild", Objective: "build project", Risk: "write-medium", DependsOn: []string{"fix_deps"}},
		},
	}

	res, err := ApplyHarnessMutation(ctx, eng, run, harnessmodel.NodeRun{ID: "nr_init", NodeID: "init"}, proposal, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.NewRevision != 2 || len(res.AddedNodes) != 2 {
		t.Fatalf("unexpected mutation result: %+v", res)
	}
}
