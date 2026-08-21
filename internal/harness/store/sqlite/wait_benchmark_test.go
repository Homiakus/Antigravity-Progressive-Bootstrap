package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func BenchmarkDurableWaitDueTimerScan10K(b *testing.B) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(b.TempDir(), "state.db"), Options{})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	now := time.Unix(90_000, 0).UTC()
	seedBenchmarkRun(b, db, now)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CreateGraphRevision(ctx, harnessmodel.GraphRevision{WorkflowRunID: "wfr_test", Number: 1, CreatedAt: now, Reason: "timer benchmark"}); err != nil {
			return err
		}
		if err := tx.CreateWorkflowProgress(ctx, harnessmodel.WorkflowProgress{WorkflowRunID: "wfr_test", TotalNodes: 1, UpdatedAt: now}); err != nil {
			return err
		}
		node := harnessmodel.NodeRun{ID: "nr_timer_bench", WorkflowRunID: "wfr_test", NodeID: "a", GraphRevision: 1, Generation: 1, State: harnessmodel.NodeWaiting, CreatedAt: now, UpdatedAt: now}
		if err := tx.CreateNodeRun(ctx, node); err != nil {
			return err
		}
		for i := 0; i < 10_000; i++ {
			timer := harnessmodel.Timer{
				ID: harnessmodel.TimerID(fmt.Sprintf("tmr_bench_%05d", i)), WorkflowRunID: "wfr_test", NodeRunID: node.ID,
				Kind: harnessmodel.TimerNodeWait, State: harnessmodel.TimerPending,
				DueAt: now.Add(-time.Duration(i+1) * time.Millisecond), CreatedAt: now.Add(-time.Hour),
			}
			if err := tx.CreateTimer(ctx, timer); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := db.View(ctx, func(r harnessstore.Reader) error {
			due, err := r.ListDueTimers(ctx, now, 10_000)
			if err != nil {
				return err
			}
			if len(due) != 10_000 {
				b.Fatalf("due timers=%d want=10000", len(due))
			}
			return nil
		}); err != nil {
			b.Fatal(err)
		}
	}
}
