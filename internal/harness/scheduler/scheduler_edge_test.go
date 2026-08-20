package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/harness/engine"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/resource"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestBoundedClassificationFindsFeasibleNodeBehindBlockedPrefix(t *testing.T) {
	ctx := context.Background()
	eng, _, db := schedulerFixture(t)
	def := harnessmodel.WorkflowDefinition{
		ID: "classification-batches", Version: 1, Name: "classification-batches", CompilerVersion: "scheduler-test", CreatedAt: time.Unix(1000, 0).UTC(),
		Nodes: []harnessmodel.NodeSpec{
			{ID: "gpu-a", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Priority: 300, Resources: harnessmodel.ResourceSpec{GPUCount: 1}},
			{ID: "gpu-b", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Priority: 200, Resources: harnessmodel.ResourceSpec{GPUCount: 1}},
			{ID: "cpu", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Priority: 1, Resources: harnessmodel.ResourceSpec{CPUWeight: 1}},
		},
	}
	if _, err := eng.StartWorkflow(ctx, def); err != nil {
		t.Fatal(err)
	}
	sched, err := New(db, Options{Capacity: resource.Capacity{CPUWeight: 10}, CandidateLimit: 1, ClassificationBatches: 4})
	if err != nil {
		t.Fatal(err)
	}
	decision, ok, err := sched.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || decision.Node.NodeID != "cpu" {
		t.Fatalf("decision ok=%v node=%s, want feasible cpu behind blocked prefix", ok, decision.Node.NodeID)
	}
}

func TestNotBeforeIsDurableAndExplainable(t *testing.T) {
	ctx := context.Background()
	eng, _, db := schedulerFixture(t)
	run, err := eng.StartWorkflow(ctx, independentDefinition("not-before", 1))
	if err != nil {
		t.Fatal(err)
	}
	var nodeRunID harnessmodel.NodeRunID
	if err := db.SQLDB().QueryRowContext(ctx, `SELECT id FROM node_runs WHERE workflow_run_id=? LIMIT 1`, string(run.ID)).Scan(&nodeRunID); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(5000, 0).UTC()
	future := now.Add(time.Hour)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.EnqueueReadyNode(ctx, nodeRunID, now, future, "cpu")
	}); err != nil {
		t.Fatal(err)
	}
	sched, err := New(db, Options{Capacity: resource.Capacity{CPUWeight: 100}, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if decision, ok, err := sched.Next(ctx); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatalf("scheduler selected not-before node early: %+v", decision)
	}
	explanation, err := sched.ExplainNode(ctx, nodeRunID)
	if err != nil {
		t.Fatal(err)
	}
	if explanation.Reason != harnessmodel.WaitNotBefore {
		t.Fatalf("reason=%s want NOT_BEFORE: %+v", explanation.Reason, explanation)
	}
	now = future.Add(time.Second)
	decision, ok, err := sched.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || decision.Node.NodeRunID != nodeRunID {
		t.Fatalf("due node was not selected: ok=%v decision=%+v", ok, decision)
	}
}

func TestWorkflowWeightChangesServiceShare(t *testing.T) {
	ctx := context.Background()
	eng, sched, _ := schedulerFixture(t)
	heavy, err := eng.StartWorkflow(ctx, independentDefinition("weighted-heavy", 40))
	if err != nil {
		t.Fatal(err)
	}
	light, err := eng.StartWorkflow(ctx, independentDefinition("weighted-light", 40))
	if err != nil {
		t.Fatal(err)
	}
	if err := sched.SetWorkflowWeight(ctx, heavy.ID, 4); err != nil {
		t.Fatal(err)
	}
	if err := sched.SetWorkflowWeight(ctx, light.ID, 1); err != nil {
		t.Fatal(err)
	}
	counts := map[harnessmodel.WorkflowRunID]int{}
	for i := 0; i < 20; i++ {
		decision, ok, err := sched.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatal("scheduler returned no weighted decision")
		}
		counts[decision.Node.WorkflowRunID]++
		finishDecision(t, ctx, eng, decision)
	}
	if counts[heavy.ID] <= counts[light.ID] {
		t.Fatalf("weight 4 workflow did not receive larger share: heavy=%d light=%d", counts[heavy.ID], counts[light.ID])
	}
	if counts[heavy.ID] < 3*counts[light.ID] {
		t.Fatalf("weighted share too weak for 4:1 over 20 decisions: heavy=%d light=%d", counts[heavy.ID], counts[light.ID])
	}
}
