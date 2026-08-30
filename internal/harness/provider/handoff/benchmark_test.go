package handoff

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	harnesssqlite "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

func BenchmarkSafeProviderHandoff100Operations(b *testing.B) {
	ctx := context.Background()
	db, err := harnesssqlite.Open(ctx, filepath.Join(b.TempDir(), "handoff_bench.db"), harnesssqlite.Options{})
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	now := time.Unix(2000, 0).UTC()
	seedTestData(b, db, now)

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	mgr := NewManager(db, Options{
		Now: func() time.Time { return now },
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Seed 100 fresh attempts and initial active assignments
		b.StopTimer()
		attemptIDs := make([]harnessmodel.AttemptID, 100)
		asnIDs := make([]harnessmodel.ProviderAssignmentID, 100)

		err := db.Update(ctx, func(tx harnessstore.Tx) error {
			for j := 0; j < 100; j++ {
				wfrID := harnessmodel.WorkflowRunID(fmt.Sprintf("wfr_bench_%d_%d", i, j))
				nodeID := harnessmodel.NodeRunID(fmt.Sprintf("nr_bench_%d_%d", i, j))
				attID := harnessmodel.AttemptID(fmt.Sprintf("att_bench_%d_%d", i, j))
				asnID := harnessmodel.ProviderAssignmentID(fmt.Sprintf("pasn_bench_%d_%d", i, j))

				attemptIDs[j] = attID
				asnIDs[j] = asnID

				if err := tx.CreateWorkflowRun(ctx, harnessmodel.WorkflowRun{
					ID: wfrID, DefinitionID: "wfd_test", DefinitionVersion: 1, State: harnessmodel.WorkflowRunning, CreatedAt: now, UpdatedAt: now,
				}); err != nil {
					return err
				}
				if err := tx.CreateGraphRevision(ctx, harnessmodel.GraphRevision{
					WorkflowRunID: wfrID, Number: 1, CreatedAt: now, Reason: "bench_test",
				}); err != nil {
					return err
				}

				node := harnessmodel.NodeRun{
					ID:            nodeID,
					WorkflowRunID: wfrID,
					NodeID:        harnessmodel.NodeID("node_test"),
					GraphRevision: 1,
					Generation:    1,
					State:         harnessmodel.NodeReady,
					CreatedAt:     now,
					UpdatedAt:     now,
				}
				if err := tx.CreateNodeRun(ctx, node); err != nil {
					return err
				}

				if _, err := tx.CreateNextAttempt(ctx, harnessmodel.Attempt{
					ID:        attID,
					NodeRunID: nodeID,
					State:     harnessmodel.AttemptCreated,
					CreatedAt: now,
				}); err != nil {
					return err
				}

				asn := harnessmodel.ProviderAssignment{
					ID:         asnID,
					AttemptID:  attID,
					AccountID:  "acc_primary",
					ModelID:    "gemini-2.5-pro",
					State:      harnessmodel.ProviderAssignmentActive,
					Revision:   1,
					CreatedAt:  now,
					UpdatedAt:  now,
				}
				if err := tx.CreateProviderAssignment(ctx, asn); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()

		for j := 0; j < 100; j++ {
			env := makeEnvelope(planDigest)
			env.TaskID = fmt.Sprintf("task_bench_%d_%d", i, j)
			env.AttemptID = attemptIDs[j]

			req := HandoffRequest{
				Envelope:          env,
				PlanText:          planText,
				PriorAssignmentID: asnIDs[j],
				Reason:            "benchmark handoff",
			}

			res, err := mgr.Handoff(ctx, req)
			if err != nil || res.ReplacementAssignment.State != harnessmodel.ProviderAssignmentActive {
				b.Fatalf("handoff %d failed: %v", j, err)
			}
		}
	}
}
