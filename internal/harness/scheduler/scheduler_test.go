package scheduler

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/harness/engine"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/resource"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

func schedulerFixture(t *testing.T) (*engine.Engine, *Scheduler, *sqlitestore.DB) {
	t.Helper()
	db, err := sqlitestore.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"), sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	eng, err := engine.New(db, engine.Options{})
	if err != nil {
		t.Fatal(err)
	}
	sched, err := New(db, Options{Capacity: resource.Capacity{
		CPUWeight: 1000, MemoryBytes: 1 << 40, GPUCount: 0, MaxVRAMBytes: 0,
		DiskBytes: 1 << 40, BuildSlots: 64, BrowserSlots: 64,
	}})
	if err != nil {
		t.Fatal(err)
	}
	return eng, sched, db
}

func independentDefinition(id string, count int) harnessmodel.WorkflowDefinition {
	nodes := make([]harnessmodel.NodeSpec, 0, count)
	for i := 0; i < count; i++ {
		nodes = append(nodes, harnessmodel.NodeSpec{ID: harnessmodel.NodeID(fmt.Sprintf("n-%06d", i)), Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess})
	}
	return harnessmodel.WorkflowDefinition{ID: harnessmodel.WorkflowDefinitionID(id), Version: 1, Name: id, CompilerVersion: "scheduler-test", CreatedAt: time.Unix(1000, 0).UTC(), Nodes: nodes}
}

func finishDecision(t *testing.T, ctx context.Context, eng *engine.Engine, decision Decision) {
	t.Helper()
	attempt, err := eng.StartAttempt(ctx, decision.Node.NodeRunID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.CompleteAttemptSuccess(ctx, attempt.ID); err != nil {
		t.Fatal(err)
	}
}

func TestFairnessSmallWorkflowGetsBoundedService(t *testing.T) {
	ctx := context.Background()
	eng, sched, _ := schedulerFixture(t)
	large, err := eng.StartWorkflow(ctx, independentDefinition("large", 100))
	if err != nil {
		t.Fatal(err)
	}
	small, err := eng.StartWorkflow(ctx, independentDefinition("small", 3))
	if err != nil {
		t.Fatal(err)
	}
	_ = large

	seenSmallAt := -1
	for i := 0; i < 4; i++ {
		decision, ok, err := sched.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatal("scheduler returned no decision while workflows are READY")
		}
		if decision.Node.WorkflowRunID == small.ID && seenSmallAt < 0 {
			seenSmallAt = i
		}
		finishDecision(t, ctx, eng, decision)
	}
	if seenSmallAt < 0 || seenSmallAt > 1 {
		t.Fatalf("small workflow first serviced at decision %d, want <=1", seenSmallAt)
	}
}

func TestHardResourceConstraintNeverFallsBackToInfeasibleNode(t *testing.T) {
	ctx := context.Background()
	eng, sched, _ := schedulerFixture(t)
	def := harnessmodel.WorkflowDefinition{
		ID: "resource-hard", Version: 1, Name: "resource-hard", CompilerVersion: "scheduler-test", CreatedAt: time.Unix(1000, 0).UTC(),
		Nodes: []harnessmodel.NodeSpec{
			{ID: "gpu", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Priority: 100, Resources: harnessmodel.ResourceSpec{GPUCount: 1, MinVRAMBytes: 12 << 30}},
			{ID: "cpu", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Priority: 1, Resources: harnessmodel.ResourceSpec{CPUWeight: 1}},
		},
	}
	run, err := eng.StartWorkflow(ctx, def)
	if err != nil {
		t.Fatal(err)
	}
	decision, ok, err := sched.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected feasible CPU decision")
	}
	if decision.Node.NodeID != "cpu" {
		t.Fatalf("scheduler selected %s, must not select infeasible gpu node", decision.Node.NodeID)
	}

	var gpuID harnessmodel.NodeRunID
	if err := sched.store.View(ctx, func(reader harnessstore.Reader) error {
		var rowsErr error
		rows, queryErr := sched.(*Scheduler)
		_ = rows
		_ = queryErr
		return rowsErr
	}); err != nil {
		_ = run
	}
	if err := sched.store.View(ctx, func(reader harnessstore.Reader) error {
		var id string
		if err := sched.store.(*sqlitestore.DB).SQLDB().QueryRowContext(ctx, `SELECT id FROM node_runs WHERE workflow_run_id=? AND node_id='gpu'`, string(run.ID)).Scan(&id); err != nil {
			return err
		}
		gpuID = harnessmodel.NodeRunID(id)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	explanation, err := sched.ExplainNode(ctx, gpuID)
	if err != nil {
		t.Fatal(err)
	}
	if explanation.Reason != harnessmodel.WaitResource {
		t.Fatalf("gpu wait reason=%s want RESOURCE (%s)", explanation.Reason, explanation.Detail)
	}
}

func TestPriorityInheritanceRaisesBlockingAncestor(t *testing.T) {
	ctx := context.Background()
	eng, sched, _ := schedulerFixture(t)
	def := harnessmodel.WorkflowDefinition{
		ID: "priority-inheritance", Version: 1, Name: "priority-inheritance", CompilerVersion: "scheduler-test", CreatedAt: time.Unix(1000, 0).UTC(),
		Nodes: []harnessmodel.NodeSpec{
			{ID: "ancestor", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Priority: 1},
			{ID: "urgent", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Priority: 100, Dependencies: []harnessmodel.NodeID{"ancestor"}},
		},
	}
	run, err := eng.StartWorkflow(ctx, def)
	if err != nil {
		t.Fatal(err)
	}
	decision, ok, err := sched.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || decision.Node.NodeID != "ancestor" || decision.Node.EffectivePriority != 100 {
		t.Fatalf("unexpected inherited priority decision: ok=%v node=%+v", ok, decision.Node)
	}

	var urgentID string
	if err := sched.store.(*sqlitestore.DB).SQLDB().QueryRowContext(ctx, `SELECT id FROM node_runs WHERE workflow_run_id=? AND node_id='urgent'`, string(run.ID)).Scan(&urgentID); err != nil {
		t.Fatal(err)
	}
	explanation, err := sched.ExplainNode(ctx, harnessmodel.NodeRunID(urgentID))
	if err != nil {
		t.Fatal(err)
	}
	if explanation.Reason != harnessmodel.WaitDependency || explanation.RemainingDependencies != 1 {
		t.Fatalf("unexpected dependent explanation: %+v", explanation)
	}
}
