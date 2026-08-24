package engine

import (
	"context"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestApplyGraphMutationOptimisticConcurrencyAndEnqueuing(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-mut-1", clock.current)

	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("mutation-test"))
	if err != nil {
		t.Fatal(err)
	}

	// 1. Optimistic concurrency mismatch
	_, err = eng.ApplyGraphMutation(ctx, GraphMutation{
		WorkflowRunID:    run.ID,
		ExpectedRevision: 999, // Wrong revision
		Reason:           "invalid revision test",
	})
	if err == nil {
		t.Fatal("expected conflict on expected revision mismatch")
	}

	// 2. Successful graph mutation adding dynamic node 'b' (no dependencies -> READY) and 'c' (depends on 'b')
	mutRes, err := eng.ApplyGraphMutation(ctx, GraphMutation{
		WorkflowRunID:    run.ID,
		ExpectedRevision: 1,
		Reason:           "adaptive repair branch",
		AddNodes: []harnessmodel.NodeSpec{
			{ID: "repair_b", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess},
			{ID: "repair_c", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Dependencies: []harnessmodel.NodeID{"repair_b"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mutRes.NewRevision != 2 || len(mutRes.AddedNodes) != 2 {
		t.Fatalf("unexpected mutation result: %+v", mutRes)
	}

	// Verify revision advanced in DB
	err = db.View(ctx, func(r harnessstore.Reader) error {
		updatedRun, err := r.GetWorkflowRun(ctx, run.ID)
		if err != nil {
			return err
		}
		if updatedRun.CurrentGraphRevision != 2 {
			t.Fatalf("expected CurrentGraphRevision=2, got %d", updatedRun.CurrentGraphRevision)
		}

		// repair_b should be READY and in ready queue
		nodeB, err := r.GetNodeRun(ctx, mutRes.AddedNodes[0])
		if err != nil {
			return err
		}
		if nodeB.State != harnessmodel.NodeReady {
			t.Fatalf("expected node repair_b to be READY, got %s", nodeB.State)
		}

		// repair_c should be PENDING_DEPENDENCIES
		nodeC, err := r.GetNodeRun(ctx, mutRes.AddedNodes[1])
		if err != nil {
			return err
		}
		if nodeC.State != harnessmodel.NodePendingDependencies || nodeC.RemainingDependencies != 1 {
			t.Fatalf("expected node repair_c PENDING with 1 dep, got %s (%d)", nodeC.State, nodeC.RemainingDependencies)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Worker claims repair_b
	claimB, err := eng.ClaimNode(ctx, mutRes.AddedNodes[0], "worker-mut-1", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if claimB.Attempt.ID == "" {
		t.Fatal("expected valid attempt for claimed dynamic node")
	}
}

func TestApplyGraphMutationSupersedeNodes(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-mut-2", clock.current)

	def := harnessmodel.WorkflowDefinition{
		ID:              "wfd_supersede",
		Version:         1,
		Name:            "supersede-test",
		CreatedAt:       clock.current,
		CompilerVersion: "test",
		Nodes: []harnessmodel.NodeSpec{
			{ID: "step1", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess},
			{ID: "obsolete_step2", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Dependencies: []harnessmodel.NodeID{"step1"}},
		},
	}

	run, err := eng.StartWorkflow(ctx, def)
	if err != nil {
		t.Fatal(err)
	}

	step2Node := nodeRunFor(t, db, run.ID, "obsolete_step2")

	// Mutate graph: cancel/supersede obsolete_step2 and add replacement
	mutRes, err := eng.ApplyGraphMutation(ctx, GraphMutation{
		WorkflowRunID:     run.ID,
		ExpectedRevision:  1,
		Reason:            "replace obsolete step with new step",
		SupersedeNodeRuns: []harnessmodel.NodeRunID{step2Node.ID},
		AddNodes: []harnessmodel.NodeSpec{
			{ID: "new_step2", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Dependencies: []harnessmodel.NodeID{"step1"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mutRes.NewRevision != 2 {
		t.Fatalf("expected revision 2, got %d", mutRes.NewRevision)
	}

	// Verify step2Node is cancelled
	err = db.View(ctx, func(r harnessstore.Reader) error {
		nr, err := r.GetNodeRun(ctx, step2Node.ID)
		if err != nil {
			return err
		}
		if nr.State != harnessmodel.NodeCancelled {
			t.Fatalf("expected superseded node state CANCELLED, got %s", nr.State)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
