package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestEngineStoreCASDependenciesAttemptsAndProgress(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Unix(1100, 0).UTC()
	seedRun(t, db, now)

	root := harnessmodel.NodeRun{ID: "nr_root", WorkflowRunID: "wfr_test", NodeID: "a", GraphRevision: 1, Generation: 1, State: harnessmodel.NodeReady, RemainingDependencies: 0, CreatedAt: now, UpdatedAt: now}
	child := harnessmodel.NodeRun{ID: "nr_child", WorkflowRunID: "wfr_test", NodeID: "b", GraphRevision: 1, Generation: 1, State: harnessmodel.NodePendingDependencies, RemainingDependencies: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CreateGraphRevision(ctx, harnessmodel.GraphRevision{WorkflowRunID: "wfr_test", Number: 1, CreatedAt: now, Reason: "test"}); err != nil {
			return err
		}
		if err := tx.CreateWorkflowProgress(ctx, harnessmodel.WorkflowProgress{WorkflowRunID: "wfr_test", TotalNodes: 2, UpdatedAt: now}); err != nil {
			return err
		}
		if err := tx.CreateNodeRun(ctx, root); err != nil {
			return err
		}
		return tx.CreateNodeRun(ctx, child)
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		deps, err := tx.ListDependentNodeRuns(ctx, "wfr_test", "a")
		if err != nil {
			return err
		}
		if len(deps) != 1 || deps[0].ID != child.ID {
			t.Fatalf("unexpected dependents: %+v", deps)
		}
		remaining, err := tx.DecrementNodeRemainingDependencies(ctx, child.ID, now.Add(time.Second))
		if err != nil {
			return err
		}
		if remaining != 0 {
			t.Fatalf("remaining=%d want=0", remaining)
		}
		updatedChild, err := tx.GetNodeRun(ctx, child.ID)
		if err != nil {
			return err
		}
		updatedChild.State = harnessmodel.NodeReady
		updatedChild.UpdatedAt = now.Add(time.Second)
		if err := tx.CompareAndSwapNodeRun(ctx, harnessmodel.NodePendingDependencies, updatedChild); err != nil {
			return err
		}
		if err := tx.CompareAndSwapNodeRun(ctx, harnessmodel.NodePendingDependencies, updatedChild); !errors.Is(err, harnessstore.ErrConflict) {
			t.Fatalf("stale node CAS=%v want ErrConflict", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		a, err := tx.CreateNextAttempt(ctx, harnessmodel.Attempt{ID: "att_one", NodeRunID: root.ID, State: harnessmodel.AttemptCreated, CreatedAt: now.Add(2 * time.Second)})
		if err != nil {
			return err
		}
		if a.Number != 1 {
			t.Fatalf("attempt number=%d want=1", a.Number)
		}
		second, err := tx.CreateNextAttempt(ctx, harnessmodel.Attempt{ID: "att_two", NodeRunID: root.ID, State: harnessmodel.AttemptCreated, CreatedAt: now.Add(3 * time.Second)})
		if err != nil {
			return err
		}
		if second.Number != 2 {
			t.Fatalf("attempt number=%d want=2", second.Number)
		}
		loaded, err := tx.GetAttempt(ctx, a.ID)
		if err != nil {
			return err
		}
		loaded.State = harnessmodel.AttemptClaimed
		if err := tx.CompareAndSwapAttempt(ctx, harnessmodel.AttemptRunning, loaded); !errors.Is(err, harnessstore.ErrConflict) {
			t.Fatalf("stale attempt CAS=%v want ErrConflict", err)
		}
		if err := tx.CompareAndSwapAttempt(ctx, harnessmodel.AttemptCreated, loaded); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		run, err := tx.GetWorkflowRun(ctx, "wfr_test")
		if err != nil {
			return err
		}
		run.State = harnessmodel.WorkflowRunning
		run.UpdatedAt = now.Add(4 * time.Second)
		if err := tx.CompareAndSwapWorkflowRun(ctx, harnessmodel.WorkflowQueued, run); !errors.Is(err, harnessstore.ErrConflict) {
			t.Fatalf("stale workflow CAS=%v want ErrConflict", err)
		}
		if err := tx.CompareAndSwapWorkflowRun(ctx, harnessmodel.WorkflowCreated, run); err != nil {
			return err
		}
		p, err := tx.IncrementWorkflowProgress(ctx, "wfr_test", false, now.Add(5*time.Second))
		if err != nil {
			return err
		}
		if p.TerminalNodes != 1 || p.FailedNodes != 0 {
			t.Fatalf("unexpected progress after success: %+v", p)
		}
		p, err = tx.IncrementWorkflowProgress(ctx, "wfr_test", true, now.Add(6*time.Second))
		if err != nil {
			return err
		}
		if p.TerminalNodes != 2 || p.FailedNodes != 1 {
			t.Fatalf("unexpected progress after failure: %+v", p)
		}
		if _, err := tx.IncrementWorkflowProgress(ctx, "wfr_test", false, now.Add(7*time.Second)); !errors.Is(err, harnessstore.ErrConflict) {
			t.Fatalf("progress overflow=%v want ErrConflict", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
