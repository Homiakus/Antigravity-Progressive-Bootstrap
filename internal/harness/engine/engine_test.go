package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

type sequenceIDs struct {
	n      int
	failAt int
}

func (g *sequenceIDs) New(kind harnessmodel.IDKind) (string, error) {
	next := g.n + 1
	if g.failAt > 0 && next == g.failAt {
		return "", fmt.Errorf("injected id failure at %d", next)
	}
	g.n = next
	return fmt.Sprintf("%s_%013d_%020x", kind, 1, next), nil
}

type testClock struct{ current time.Time }

func (c *testClock) Now() time.Time {
	c.current = c.current.Add(time.Second)
	return c.current
}

func newTestEngine(t *testing.T) (*Engine, *sqlitestore.DB, *sequenceIDs, *testClock) {
	t.Helper()
	db, err := sqlitestore.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"), sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ids := &sequenceIDs{}
	clock := &testClock{current: time.Unix(2000, 0).UTC()}
	eng, err := New(db, Options{IDs: ids, Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	return eng, db, ids, clock
}

func dagDefinition() harnessmodel.WorkflowDefinition {
	now := time.Unix(1900, 0).UTC()
	return harnessmodel.WorkflowDefinition{
		ID: "wfd_engine", Version: 1, Name: "engine-test", CreatedAt: now, CompilerVersion: "test",
		Nodes: []harnessmodel.NodeSpec{
			{ID: "a", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, CachePolicy: harnessmodel.CacheDisabled},
			{ID: "b", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Dependencies: []harnessmodel.NodeID{"a"}, CachePolicy: harnessmodel.CacheDisabled},
			{ID: "c", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Dependencies: []harnessmodel.NodeID{"a"}, CachePolicy: harnessmodel.CacheDisabled},
			{ID: "d", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Dependencies: []harnessmodel.NodeID{"b", "c"}, CachePolicy: harnessmodel.CacheDisabled},
		},
	}
}

func nodeRunFor(t *testing.T, db *sqlitestore.DB, runID harnessmodel.WorkflowRunID, nodeID harnessmodel.NodeID) harnessmodel.NodeRun {
	t.Helper()
	var id string
	if err := db.SQLDB().QueryRow(`SELECT id FROM node_runs WHERE workflow_run_id=? AND node_id=? ORDER BY generation DESC LIMIT 1`, string(runID), string(nodeID)).Scan(&id); err != nil {
		t.Fatal(err)
	}
	var nr harnessmodel.NodeRun
	if err := db.View(context.Background(), func(r harnessstore.Reader) error {
		var err error
		nr, err = r.GetNodeRun(context.Background(), harnessmodel.NodeRunID(id))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return nr
}

func workflowProgress(t *testing.T, db *sqlitestore.DB, runID harnessmodel.WorkflowRunID) harnessmodel.WorkflowProgress {
	t.Helper()
	var p harnessmodel.WorkflowProgress
	if err := db.View(context.Background(), func(r harnessstore.Reader) error {
		var err error
		p, err = r.GetWorkflowProgress(context.Background(), runID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return p
}

func workflowRun(t *testing.T, db *sqlitestore.DB, runID harnessmodel.WorkflowRunID) harnessmodel.WorkflowRun {
	t.Helper()
	var run harnessmodel.WorkflowRun
	if err := db.View(context.Background(), func(r harnessstore.Reader) error {
		var err error
		run, err = r.GetWorkflowRun(context.Background(), runID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestWorkflowFanOutFanInAndTerminalDetection(t *testing.T) {
	ctx := context.Background()
	eng, db, _, _ := newTestEngine(t)
	run, err := eng.StartWorkflow(ctx, dagDefinition())
	if err != nil {
		t.Fatal(err)
	}
	if run.State != harnessmodel.WorkflowRunning || run.CurrentGraphRevision != 1 {
		t.Fatalf("unexpected started run: %+v", run)
	}
	if p := workflowProgress(t, db, run.ID); p.TotalNodes != 4 || p.TerminalNodes != 0 {
		t.Fatalf("unexpected initial progress: %+v", p)
	}
	a := nodeRunFor(t, db, run.ID, "a")
	b := nodeRunFor(t, db, run.ID, "b")
	c := nodeRunFor(t, db, run.ID, "c")
	d := nodeRunFor(t, db, run.ID, "d")
	if a.State != harnessmodel.NodeReady || b.State != harnessmodel.NodePendingDependencies || c.State != harnessmodel.NodePendingDependencies || d.RemainingDependencies != 2 {
		t.Fatalf("unexpected initial DAG materialization: a=%+v b=%+v c=%+v d=%+v", a, b, c, d)
	}

	attemptA, err := eng.StartAttempt(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if attemptA.State != harnessmodel.AttemptRunning || attemptA.Number != 1 {
		t.Fatalf("unexpected attempt A: %+v", attemptA)
	}
	completeA, err := eng.CompleteAttemptSuccess(ctx, attemptA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(completeA.ReadyNodeRunIDs) != 2 {
		t.Fatalf("fan-out ready count=%d want=2", len(completeA.ReadyNodeRunIDs))
	}
	b = nodeRunFor(t, db, run.ID, "b")
	c = nodeRunFor(t, db, run.ID, "c")
	if b.State != harnessmodel.NodeReady || c.State != harnessmodel.NodeReady {
		t.Fatalf("fan-out children not READY: b=%s c=%s", b.State, c.State)
	}

	attemptB, err := eng.StartAttempt(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.CompleteAttemptSuccess(ctx, attemptB.ID); err != nil {
		t.Fatal(err)
	}
	d = nodeRunFor(t, db, run.ID, "d")
	if d.State != harnessmodel.NodePendingDependencies || d.RemainingDependencies != 1 {
		t.Fatalf("fan-in released early: %+v", d)
	}

	attemptC, err := eng.StartAttempt(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	completeC, err := eng.CompleteAttemptSuccess(ctx, attemptC.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(completeC.ReadyNodeRunIDs) != 1 {
		t.Fatalf("fan-in ready count=%d want=1", len(completeC.ReadyNodeRunIDs))
	}
	d = nodeRunFor(t, db, run.ID, "d")
	if d.State != harnessmodel.NodeReady || d.RemainingDependencies != 0 {
		t.Fatalf("fan-in child not READY: %+v", d)
	}
	duplicate, err := eng.CompleteAttemptSuccess(ctx, attemptC.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate.Idempotent {
		t.Fatal("duplicate completion was not idempotent")
	}
	d = nodeRunFor(t, db, run.ID, "d")
	if d.RemainingDependencies != 0 {
		t.Fatalf("duplicate completion decremented child twice: %+v", d)
	}
	if p := workflowProgress(t, db, run.ID); p.TerminalNodes != 3 {
		t.Fatalf("duplicate completion changed progress: %+v", p)
	}

	attemptD, err := eng.StartAttempt(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.CompleteAttemptSuccess(ctx, attemptD.ID); err != nil {
		t.Fatal(err)
	}
	finalRun := workflowRun(t, db, run.ID)
	if finalRun.State != harnessmodel.WorkflowSucceeded {
		t.Fatalf("workflow state=%s want SUCCEEDED", finalRun.State)
	}
	if p := workflowProgress(t, db, run.ID); p.TerminalNodes != 4 || p.FailedNodes != 0 {
		t.Fatalf("unexpected terminal progress: %+v", p)
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		evs, err := r.ListEvents(ctx, run.ID, 0, 1000)
		if err != nil {
			return err
		}
		for i, event := range evs {
			if event.WorkflowSeq != int64(i+1) {
				t.Fatalf("event sequence gap at %d: %+v", i, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFailureDoesNotReleaseDependentAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	eng, db, _, _ := newTestEngine(t)
	def := dagDefinition()
	def.ID = "wfd_failure"
	def.Nodes = def.Nodes[:2]
	run, err := eng.StartWorkflow(ctx, def)
	if err != nil {
		t.Fatal(err)
	}
	a := nodeRunFor(t, db, run.ID, "a")
	attempt, err := eng.StartAttempt(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := eng.CompleteAttemptFailure(ctx, attempt.ID, "TEST_FAILURE", "boom")
	if err != nil {
		t.Fatal(err)
	}
	if failed.WorkflowRun.State != harnessmodel.WorkflowFailed || failed.NodeRun.State != harnessmodel.NodeFailed {
		t.Fatalf("unexpected failure result: %+v", failed)
	}
	b := nodeRunFor(t, db, run.ID, "b")
	if b.State != harnessmodel.NodePendingDependencies || b.RemainingDependencies != 1 {
		t.Fatalf("failed parent released dependent: %+v", b)
	}
	if p := workflowProgress(t, db, run.ID); p.TerminalNodes != 1 || p.FailedNodes != 1 {
		t.Fatalf("unexpected failure progress: %+v", p)
	}
	dup, err := eng.CompleteAttemptFailure(ctx, attempt.ID, "TEST_FAILURE", "boom")
	if err != nil {
		t.Fatal(err)
	}
	if !dup.Idempotent {
		t.Fatal("duplicate failure was not idempotent")
	}
	if p := workflowProgress(t, db, run.ID); p.TerminalNodes != 1 || p.FailedNodes != 1 {
		t.Fatalf("duplicate failure changed progress: %+v", p)
	}
}

func TestStartWorkflowRollsBackOnEventFailure(t *testing.T) {
	ctx := context.Background()
	eng, db, ids, _ := newTestEngine(t)
	def := dagDefinition()
	def.ID = "wfd_rollback_start"
	def.Nodes = def.Nodes[:1]
	// StartWorkflow generates run id, node-run id, then the first event id.
	ids.failAt = 3
	if _, err := eng.StartWorkflow(ctx, def); err == nil {
		t.Fatal("expected injected event id failure")
	}
	for _, table := range []string{"workflow_definitions", "workflow_runs", "workflow_progress", "graph_revisions", "node_runs", "events"} {
		var count int
		if err := db.SQLDB().QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s leaked %d rows after rollback", table, count)
		}
	}
}

func TestCompletionRollsBackStateAndCountersWhenEventAppendFails(t *testing.T) {
	ctx := context.Background()
	eng, db, ids, _ := newTestEngine(t)
	def := dagDefinition()
	def.ID = "wfd_rollback_completion"
	def.Nodes = def.Nodes[:2]
	run, err := eng.StartWorkflow(ctx, def)
	if err != nil {
		t.Fatal(err)
	}
	a := nodeRunFor(t, db, run.ID, "a")
	attempt, err := eng.StartAttempt(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids.failAt = ids.n + 1
	if _, err := eng.CompleteAttemptSuccess(ctx, attempt.ID); err == nil {
		t.Fatal("expected injected completion event failure")
	}
	var loadedAttempt harnessmodel.Attempt
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		var err error
		loadedAttempt, err = r.GetAttempt(ctx, attempt.ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if loadedAttempt.State != harnessmodel.AttemptRunning {
		t.Fatalf("attempt state escaped rollback: %s", loadedAttempt.State)
	}
	a = nodeRunFor(t, db, run.ID, "a")
	b := nodeRunFor(t, db, run.ID, "b")
	if a.State != harnessmodel.NodeRunning || b.State != harnessmodel.NodePendingDependencies || b.RemainingDependencies != 1 {
		t.Fatalf("node/dependency state escaped rollback: a=%+v b=%+v", a, b)
	}
	if p := workflowProgress(t, db, run.ID); p.TerminalNodes != 0 || p.FailedNodes != 0 {
		t.Fatalf("progress escaped rollback: %+v", p)
	}
	if final := workflowRun(t, db, run.ID); final.State != harnessmodel.WorkflowRunning {
		t.Fatalf("workflow escaped rollback: %+v", final)
	}
}

func TestCannotStartAttemptWhenWorkflowNotRunning(t *testing.T) {
	ctx := context.Background()
	eng, db, _, _ := newTestEngine(t)
	def := dagDefinition()
	def.ID = "wfd_not_running"
	def.Nodes = def.Nodes[:2]
	run, err := eng.StartWorkflow(ctx, def)
	if err != nil {
		t.Fatal(err)
	}
	a := nodeRunFor(t, db, run.ID, "a")
	attempt, err := eng.StartAttempt(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.CompleteAttemptFailure(ctx, attempt.ID, "FAIL", "stop workflow"); err != nil {
		t.Fatal(err)
	}
	b := nodeRunFor(t, db, run.ID, "b")
	if _, err := eng.StartAttempt(ctx, b.ID); err == nil {
		t.Fatal("expected start rejection after workflow failure")
	}
}
