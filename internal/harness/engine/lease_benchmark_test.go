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

func benchmarkLeaseEngine(b *testing.B, now time.Time) (*Engine, *sqlitestore.DB) {
	b.Helper()
	db, err := sqlitestore.Open(context.Background(), filepath.Join(b.TempDir(), "state.db"), sqlitestore.Options{})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = db.Close() })
	eng, err := New(db, Options{Now: func() time.Time { return now }})
	if err != nil {
		b.Fatal(err)
	}
	return eng, db
}

func benchmarkWorker(b *testing.B, db harnessstore.Store, id harnessmodel.WorkerID, now time.Time) {
	b.Helper()
	if err := db.Update(context.Background(), func(tx harnessstore.Tx) error {
		return tx.UpsertWorker(context.Background(), harnessmodel.Worker{
			ID: id, Name: string(id), State: harnessmodel.WorkerActive, Trust: harnessmodel.WorkerTrustedLocal,
			CreatedAt: now, LastSeenAt: now,
		})
	}); err != nil {
		b.Fatal(err)
	}
}

func benchmarkIndependentDefinition(count int, now time.Time) harnessmodel.WorkflowDefinition {
	nodes := make([]harnessmodel.NodeSpec, count)
	for i := range nodes {
		nodes[i] = harnessmodel.NodeSpec{ID: harnessmodel.NodeID(fmt.Sprintf("n-%08d", i)), Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess}
	}
	return harnessmodel.WorkflowDefinition{ID: "lease-benchmark", Version: 1, Name: "lease-benchmark", CompilerVersion: "benchmark", CreatedAt: now, Nodes: nodes}
}

func BenchmarkHeartbeatLease(b *testing.B) {
	ctx := context.Background()
	now := time.Unix(10000, 0).UTC()
	eng, db := benchmarkLeaseEngine(b, now)
	benchmarkWorker(b, db, "worker-heartbeat-bench", now)
	run, err := eng.StartWorkflow(ctx, benchmarkIndependentDefinition(1, now))
	if err != nil {
		b.Fatal(err)
	}
	var nodeRunID string
	if err := db.SQLDB().QueryRowContext(ctx, `SELECT id FROM node_runs WHERE workflow_run_id=? LIMIT 1`, string(run.ID)).Scan(&nodeRunID); err != nil {
		b.Fatal(err)
	}
	claim, err := eng.ClaimNode(ctx, harnessmodel.NodeRunID(nodeRunID), "worker-heartbeat-bench", 30*time.Second)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, claim.Lease.WorkerID, claim.Lease.Epoch); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := eng.HeartbeatLease(ctx, claim.Attempt.ID, claim.Lease.WorkerID, claim.Lease.Epoch, 30*time.Second); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkClaimNode(b *testing.B) {
	if b.N < 1 {
		return
	}
	ctx := context.Background()
	now := time.Unix(11000, 0).UTC()
	eng, db := benchmarkLeaseEngine(b, now)
	benchmarkWorker(b, db, "worker-claim-bench", now)
	run, err := eng.StartWorkflow(ctx, benchmarkIndependentDefinition(b.N, now))
	if err != nil {
		b.Fatal(err)
	}
	rows, err := db.SQLDB().QueryContext(ctx, `SELECT id FROM node_runs WHERE workflow_run_id=? ORDER BY node_id`, string(run.ID))
	if err != nil {
		b.Fatal(err)
	}
	ids := make([]harnessmodel.NodeRunID, 0, b.N)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			b.Fatal(err)
		}
		ids = append(ids, harnessmodel.NodeRunID(id))
	}
	if err := rows.Close(); err != nil {
		b.Fatal(err)
	}
	if err := rows.Err(); err != nil {
		b.Fatal(err)
	}
	if len(ids) != b.N {
		b.Fatalf("loaded node runs=%d want=%d", len(ids), b.N)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := eng.ClaimNode(ctx, ids[i], "worker-claim-bench", 30*time.Second); err != nil {
			b.Fatal(err)
		}
	}
}
