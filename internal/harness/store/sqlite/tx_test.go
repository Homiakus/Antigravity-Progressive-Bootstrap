package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/harness/events"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func openTestStore(t *testing.T) *DB {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "state.db"), Options{BusyTimeout: time.Second, MaxOpenConns: 8, MaxIdleConns: 8})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func testDefinition(now time.Time) harnessmodel.WorkflowDefinition {
	return harnessmodel.WorkflowDefinition{
		ID:              "wfd_test",
		Version:         1,
		Name:            "test",
		CreatedAt:       now,
		CompilerVersion: "test",
		Nodes: []harnessmodel.NodeSpec{
			{ID: "a", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess},
			{ID: "b", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Dependencies: []harnessmodel.NodeID{"a"}},
		},
	}
}

func testRun(now time.Time) harnessmodel.WorkflowRun {
	return harnessmodel.WorkflowRun{ID: "wfr_test", DefinitionID: "wfd_test", DefinitionVersion: 1, State: harnessmodel.WorkflowCreated, CreatedAt: now, UpdatedAt: now}
}

func seedRun(t *testing.T, db *DB, now time.Time) {
	t.Helper()
	if err := db.Update(context.Background(), func(tx harnessstore.Tx) error {
		if err := tx.CreateWorkflowDefinition(context.Background(), testDefinition(now)); err != nil {
			return err
		}
		return tx.CreateWorkflowRun(context.Background(), testRun(now))
	}); err != nil {
		t.Fatal(err)
	}
}

func TestDefinitionRunRoundTrip(t *testing.T) {
	db := openTestStore(t)
	now := time.Unix(100, 123).UTC()
	seedRun(t, db, now)
	if err := db.View(context.Background(), func(r harnessstore.Reader) error {
		def, err := r.GetWorkflowDefinition(context.Background(), "wfd_test", 1)
		if err != nil {
			return err
		}
		if len(def.Nodes) != 2 || len(def.Nodes[1].Dependencies) != 1 {
			t.Fatalf("unexpected definition round-trip: %+v", def)
		}
		run, err := r.GetWorkflowRun(context.Background(), "wfr_test")
		if err != nil {
			return err
		}
		if run.State != harnessmodel.WorkflowCreated || !run.CreatedAt.Equal(now) {
			t.Fatalf("unexpected workflow run: %+v", run)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestStateEventOutboxAreAtomic(t *testing.T) {
	db := openTestStore(t)
	now := time.Unix(200, 0).UTC()
	seedRun(t, db, now)
	err := db.Update(context.Background(), func(tx harnessstore.Tx) error {
		if err := tx.UpdateWorkflowRunState(context.Background(), "wfr_test", harnessmodel.WorkflowRunning, now.Add(time.Second)); err != nil {
			return err
		}
		_, err := tx.AppendEvent(context.Background(), events.Event{ID: "evt_rollback", WorkflowRunID: "wfr_test", Type: "WorkflowRunning", Timestamp: now.Add(time.Second), EntityType: "workflow_run", EntityID: "wfr_test", PayloadVersion: 1, Payload: json.RawMessage(`{"state":"RUNNING"}`)}, &events.OutboxMessage{Topic: "workflow.events"})
		if err != nil {
			return err
		}
		return errors.New("fault after event before commit")
	})
	if err == nil {
		t.Fatal("expected rollback error")
	}
	if err := db.View(context.Background(), func(r harnessstore.Reader) error {
		run, err := r.GetWorkflowRun(context.Background(), "wfr_test")
		if err != nil {
			return err
		}
		if run.State != harnessmodel.WorkflowCreated {
			t.Fatalf("state escaped rollback: %s", run.State)
		}
		evs, err := r.ListEvents(context.Background(), "wfr_test", 0, 10)
		if err != nil {
			return err
		}
		if len(evs) != 0 {
			t.Fatalf("event escaped rollback: %+v", evs)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.Update(context.Background(), func(tx harnessstore.Tx) error {
		if err := tx.UpdateWorkflowRunState(context.Background(), "wfr_test", harnessmodel.WorkflowRunning, now.Add(2*time.Second)); err != nil {
			return err
		}
		e, err := tx.AppendEvent(context.Background(), events.Event{ID: "evt_commit", WorkflowRunID: "wfr_test", Type: "WorkflowRunning", Timestamp: now.Add(2 * time.Second), EntityType: "workflow_run", EntityID: "wfr_test", PayloadVersion: 1, Payload: json.RawMessage(`{"state":"RUNNING"}`)}, &events.OutboxMessage{Topic: "workflow.events"})
		if err != nil {
			return err
		}
		if e.WorkflowSeq != 1 {
			t.Fatalf("rolled-back sequence was consumed: got %d want 1", e.WorkflowSeq)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var outboxCount int
	if err := db.SQLDB().QueryRow(`SELECT COUNT(*) FROM outbox WHERE event_id='evt_commit'`).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if outboxCount != 1 {
		t.Fatalf("outbox/event atomicity broken: count=%d", outboxCount)
	}
}

func TestForeignKeyFailureRollsBackDefinition(t *testing.T) {
	db := openTestStore(t)
	now := time.Unix(300, 0).UTC()
	bad := testDefinition(now)
	bad.Nodes[1].Dependencies = []harnessmodel.NodeID{"missing"}
	if err := db.Update(context.Background(), func(tx harnessstore.Tx) error {
		return tx.CreateWorkflowDefinition(context.Background(), bad)
	}); err == nil {
		t.Fatal("expected foreign key error")
	}
	if err := db.View(context.Background(), func(r harnessstore.Reader) error {
		_, err := r.GetWorkflowDefinition(context.Background(), "wfd_test", 1)
		if !errors.Is(err, harnessstore.ErrNotFound) {
			t.Fatalf("partial definition visible after rollback: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestWALReaderDoesNotBlockOnUncommittedWriter(t *testing.T) {
	db := openTestStore(t)
	now := time.Unix(400, 0).UTC()
	seedRun(t, db, now)
	written := make(chan struct{})
	release := make(chan struct{})
	writerDone := make(chan error, 1)
	go func() {
		writerDone <- db.Update(context.Background(), func(tx harnessstore.Tx) error {
			if err := tx.UpdateWorkflowRunState(context.Background(), "wfr_test", harnessmodel.WorkflowRunning, now.Add(time.Second)); err != nil {
				return err
			}
			close(written)
			<-release
			return nil
		})
	}()
	<-written
	if err := db.View(context.Background(), func(r harnessstore.Reader) error {
		run, err := r.GetWorkflowRun(context.Background(), "wfr_test")
		if err != nil {
			return err
		}
		if run.State != harnessmodel.WorkflowCreated {
			t.Fatalf("reader observed uncommitted state: %s", run.State)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-writerDone; err != nil {
		t.Fatal(err)
	}
}

func TestBusyWriterWaitsThenCommits(t *testing.T) {
	db := openTestStore(t)
	now := time.Unix(500, 0).UTC()
	seedRun(t, db, now)
	locked := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = db.Update(context.Background(), func(tx harnessstore.Tx) error {
			if err := tx.UpdateWorkflowRunState(context.Background(), "wfr_test", harnessmodel.WorkflowRunning, now.Add(time.Second)); err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- db.Update(context.Background(), func(tx harnessstore.Tx) error {
			return tx.UpdateWorkflowRunState(context.Background(), "wfr_test", harnessmodel.WorkflowPaused, now.Add(2*time.Second))
		})
	}()
	time.Sleep(50 * time.Millisecond)
	close(release)
	if err := <-secondDone; err != nil {
		t.Fatalf("second writer did not survive transient busy lock: %v", err)
	}
	wg.Wait()
	if err := db.View(context.Background(), func(r harnessstore.Reader) error {
		run, err := r.GetWorkflowRun(context.Background(), "wfr_test")
		if err != nil {
			return err
		}
		if run.State != harnessmodel.WorkflowPaused {
			t.Fatalf("unexpected final state: %s", run.State)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
