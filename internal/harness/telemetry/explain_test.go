package telemetry

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

func TestExplainerNodeAndWorkflowState(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	db, err := sqlitestore.Open(ctx, filepath.Join(tempDir, "state.db"), sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Unix(1000, 0).UTC()
	clock := func() time.Time { return now }

	def := harnessmodel.WorkflowDefinition{
		ID:              "wfd_exp",
		Version:         1,
		Name:            "explain-test",
		CreatedAt:       now,
		CompilerVersion: "test",
		Nodes: []harnessmodel.NodeSpec{
			{ID: "root", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess},
			{ID: "child", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Dependencies: []harnessmodel.NodeID{"root"}},
		},
	}

	err = db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CreateWorkflowDefinition(ctx, def); err != nil {
			return err
		}
		if err := tx.CreateWorkflowRun(ctx, harnessmodel.WorkflowRun{
			ID:                   "wfr_exp",
			DefinitionID:         def.ID,
			DefinitionVersion:    1,
			State:                harnessmodel.WorkflowRunning,
			CurrentGraphRevision: 1,
			CreatedAt:            now,
			UpdatedAt:            now,
		}); err != nil {
			return err
		}
		if err := tx.CreateGraphRevision(ctx, harnessmodel.GraphRevision{
			WorkflowRunID: "wfr_exp",
			Number:        1,
			CreatedAt:     now,
		}); err != nil {
			return err
		}
		if err := tx.CreateWorkflowProgress(ctx, harnessmodel.WorkflowProgress{
			WorkflowRunID: "wfr_exp",
			TotalNodes:    2,
			TerminalNodes: 0,
			FailedNodes:   0,
			UpdatedAt:     now,
		}); err != nil {
			return err
		}
		if err := tx.CreateNodeRun(ctx, harnessmodel.NodeRun{
			ID:                    "nr_root",
			WorkflowRunID:         "wfr_exp",
			NodeID:                "root",
			GraphRevision:         1,
			Generation:            1,
			State:                 harnessmodel.NodeReady,
			RemainingDependencies: 0,
			CreatedAt:             now,
			UpdatedAt:             now,
		}); err != nil {
			return err
		}
		return tx.CreateNodeRun(ctx, harnessmodel.NodeRun{
			ID:                    "nr_child",
			WorkflowRunID:         "wfr_exp",
			NodeID:                "child",
			GraphRevision:         1,
			Generation:            1,
			State:                 harnessmodel.NodePendingDependencies,
			RemainingDependencies: 1,
			CreatedAt:             now,
			UpdatedAt:             now,
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	explainer := NewExplainer(db, clock)

	// 1. Explain root node (READY)
	rootExp, err := explainer.ExplainNode(ctx, "nr_root")
	if err != nil {
		t.Fatal(err)
	}
	if rootExp.State != harnessmodel.NodeReady {
		t.Fatalf("expected state READY, got %s", rootExp.State)
	}

	// 2. Explain child node (PENDING_DEPENDENCIES)
	childExp, err := explainer.ExplainNode(ctx, "nr_child")
	if err != nil {
		t.Fatal(err)
	}
	if childExp.RemainingDependencies != 1 {
		t.Fatalf("expected 1 remaining dependency, got %d", childExp.RemainingDependencies)
	}

	// 3. Explain workflow
	wfExp, err := explainer.ExplainWorkflow(ctx, "wfr_exp")
	if err != nil {
		t.Fatal(err)
	}
	if wfExp.TotalNodes != 2 || wfExp.State != harnessmodel.WorkflowRunning {
		t.Fatalf("unexpected workflow explanation: %+v", wfExp)
	}
}

func TestMetricsCollectorSnapshots(t *testing.T) {
	metrics := NewMetricsCollector()
	metrics.IncAttempts()
	metrics.IncAttempts()
	metrics.IncFailures()
	metrics.IncRetries()
	metrics.RecordLLMUsage(500, 0.02)
	metrics.RecordNodeDuration(100 * time.Millisecond)
	metrics.RecordNodeDuration(300 * time.Millisecond)

	snap := metrics.Snapshot()
	if snap.TotalAttempts != 2 || snap.TotalFailures != 1 || snap.TotalRetries != 1 {
		t.Fatalf("unexpected metrics snapshot: %+v", snap)
	}
	if snap.TotalLLMTokens != 500 || snap.AverageNodeDuration != 200*time.Millisecond {
		t.Fatalf("unexpected avg/llm metrics: %+v", snap)
	}
}
