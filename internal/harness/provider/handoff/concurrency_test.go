package handoff

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestConcurrentSafeProviderHandoffs(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	now := time.Unix(2000, 0).UTC()
	seedTestData(t, db, now)

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	const workers = 64

	// Seed 64 distinct attempts and initial assignments
	err := db.Update(ctx, func(tx harnessstore.Tx) error {
		for i := 0; i < workers; i++ {
			wfrID := harnessmodel.WorkflowRunID(fmt.Sprintf("wfr_conc_%d", i))
			nodeID := harnessmodel.NodeRunID(fmt.Sprintf("nr_conc_%d", i))
			attID := harnessmodel.AttemptID(fmt.Sprintf("att_conc_%d", i))
			asnID := harnessmodel.ProviderAssignmentID(fmt.Sprintf("pasn_conc_%d", i))

			if err := tx.CreateWorkflowRun(ctx, harnessmodel.WorkflowRun{
				ID: wfrID, DefinitionID: "wfd_test", DefinitionVersion: 1, State: harnessmodel.WorkflowRunning, CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				return err
			}
			if err := tx.CreateGraphRevision(ctx, harnessmodel.GraphRevision{
				WorkflowRunID: wfrID, Number: 1, CreatedAt: now, Reason: "conc_test",
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
		t.Fatal(err)
	}

	mgr := NewManager(db, Options{
		Now: func() time.Time { return now },
	})

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			env := makeEnvelope(planDigest)
			env.TaskID = fmt.Sprintf("task_conc_%d", idx)
			env.NodeRunID = harnessmodel.NodeRunID(fmt.Sprintf("nr_conc_%d", idx))
			env.AttemptID = harnessmodel.AttemptID(fmt.Sprintf("att_conc_%d", idx))

			req := HandoffRequest{
				Envelope:          env,
				PlanText:          planText,
				PriorAssignmentID: harnessmodel.ProviderAssignmentID(fmt.Sprintf("pasn_conc_%d", idx)),
				Reason:            "concurrent handoff",
			}

			res, err := mgr.Handoff(ctx, req)
			if err != nil {
				t.Errorf("worker %d handoff error: %v", idx, err)
				return
			}
			if res.ReplacementAssignment.State != harnessmodel.ProviderAssignmentActive {
				t.Errorf("worker %d replacement state = %s, want ACTIVE", idx, res.ReplacementAssignment.State)
			}
		}(i)
	}

	wg.Wait()
}
