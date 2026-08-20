package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/harness/events"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func seedBenchmarkRun(b *testing.B, db *DB, now time.Time) {
	b.Helper()
	if err := db.Update(context.Background(), func(tx harnessstore.Tx) error {
		if err := tx.CreateWorkflowDefinition(context.Background(), testDefinition(now)); err != nil {
			return err
		}
		return tx.CreateWorkflowRun(context.Background(), testRun(now))
	}); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkStateTransition(b *testing.B) {
	db, err := Open(context.Background(), filepath.Join(b.TempDir(), "state.db"), Options{})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	now := time.Unix(600, 0).UTC()
	seedBenchmarkRun(b, db, now)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state := harnessmodel.WorkflowRunning
		if i%2 == 1 {
			state = harnessmodel.WorkflowPaused
		}
		if err := db.Update(context.Background(), func(tx harnessstore.Tx) error {
			return tx.UpdateWorkflowRunState(context.Background(), "wfr_test", state, now.Add(time.Duration(i+1)*time.Nanosecond))
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEventAppend(b *testing.B) {
	db, err := Open(context.Background(), filepath.Join(b.TempDir(), "state.db"), Options{})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	now := time.Unix(700, 0).UTC()
	seedBenchmarkRun(b, db, now)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event := events.Event{ID: harnessmodel.EventID(fmt.Sprintf("evt_bench_%d", i)), WorkflowRunID: "wfr_test", Type: "Benchmark", Timestamp: now.Add(time.Duration(i) * time.Nanosecond), EntityType: "workflow_run", EntityID: "wfr_test", PayloadVersion: 1, Payload: json.RawMessage(`{}`)}
		if err := db.Update(context.Background(), func(tx harnessstore.Tx) error {
			_, err := tx.AppendEvent(context.Background(), event, nil)
			return err
		}); err != nil {
			b.Fatal(err)
		}
	}
}
