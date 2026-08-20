package engine

import (
	"context"
	"testing"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestReadyQueueProjectionTracksNodeStateAtomically(t *testing.T) {
	ctx := context.Background()
	eng, db, _, _ := newTestEngine(t)
	def := dagDefinition()
	def.ID = "ready-projection"
	def.Nodes = def.Nodes[:2]
	run, err := eng.StartWorkflow(ctx, def)
	if err != nil {
		t.Fatal(err)
	}

	a := nodeRunFor(t, db, run.ID, "a")
	b := nodeRunFor(t, db, run.ID, "b")
	var count int
	if err := db.SQLDB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ready_queue WHERE node_run_id=?`, string(a.ID)).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("root READY projection count=%d want=1", count)
	}
	if err := db.SQLDB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ready_queue WHERE node_run_id=?`, string(b.ID)).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("dependent appeared in READY projection early: %d", count)
	}

	attempt, err := eng.StartAttempt(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SQLDB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ready_queue WHERE node_run_id=?`, string(a.ID)).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("READY projection survived READY->RUNNING: %d", count)
	}

	if _, err := eng.CompleteAttemptSuccess(ctx, attempt.ID); err != nil {
		t.Fatal(err)
	}
	b = nodeRunFor(t, db, run.ID, "b")
	if b.State != harnessmodel.NodeReady {
		t.Fatalf("dependent state=%s want READY", b.State)
	}
	if err := db.SQLDB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ready_queue WHERE node_run_id=?`, string(b.ID)).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("dependent READY projection count=%d want=1", count)
	}
}
