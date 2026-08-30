package handoff

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	harnesssqlite "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

type mockReconciler struct {
	reconcileFn func(ctx context.Context, req harnessexecutor.EffectReconcileRequest) (harnessexecutor.EffectReconcileResult, error)
}

func (m *mockReconciler) ReconcileEffect(ctx context.Context, req harnessexecutor.EffectReconcileRequest) (harnessexecutor.EffectReconcileResult, error) {
	if m.reconcileFn != nil {
		return m.reconcileFn(ctx, req)
	}
	return harnessexecutor.EffectReconcileResult{Status: harnessexecutor.EffectReconcileUnknown}, nil
}

func setupTestStore(t testing.TB) (*harnesssqlite.DB, func()) {
	ctx := context.Background()
	db, err := harnesssqlite.Open(ctx, filepath.Join(t.TempDir(), "handoff_test.db"), harnesssqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return db, func() { _ = db.Close() }
}

func floatPtr(f float64) *float64 {
	return &f
}

func seedTestData(t testing.TB, db *harnesssqlite.DB, now time.Time) (harnessmodel.ProviderAccountID, harnessmodel.ProviderAccountID) {
	ctx := context.Background()
	acc1 := harnessmodel.ProviderAccountID("acc_primary")
	acc2 := harnessmodel.ProviderAccountID("acc_backup")
	mod1 := harnessmodel.ProviderModelID("gemini-2.5-pro")
	mod2 := harnessmodel.ProviderModelID("codex-max")

	err := db.Update(ctx, func(tx harnessstore.Tx) error {
		// Accounts
		if err := tx.UpsertProviderAccount(ctx, harnessmodel.ProviderAccount{
			ID:        acc1,
			Provider:  harnessmodel.ProviderAntigravity,
			Name:      "Primary Account",
			State:     harnessmodel.ProviderAccountActive,
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now,
		}); err != nil {
			return err
		}
		if err := tx.UpsertProviderAccount(ctx, harnessmodel.ProviderAccount{
			ID:        acc2,
			Provider:  harnessmodel.ProviderCodex,
			Name:      "Backup Account",
			State:     harnessmodel.ProviderAccountActive,
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now,
		}); err != nil {
			return err
		}

		// Models
		if err := tx.UpsertProviderModel(ctx, harnessmodel.ProviderModelDescriptor{
			AccountID:    acc1,
			Provider:     harnessmodel.ProviderAntigravity,
			ID:           mod1,
			DisplayName:  "Primary Model",
			Capabilities: []string{"tools", "file_edit", "review"},
			ContextLimit: 128000,
			Enabled:      true,
		}, now); err != nil {
			return err
		}
		if err := tx.UpsertProviderModel(ctx, harnessmodel.ProviderModelDescriptor{
			AccountID:    acc2,
			Provider:     harnessmodel.ProviderCodex,
			ID:           mod2,
			DisplayName:  "Backup Model",
			Capabilities: []string{"tools", "file_edit", "review"},
			ContextLimit: 128000,
			Enabled:      true,
		}, now); err != nil {
			return err
		}

		// Capacities
		if err := tx.AppendProviderCapacity(ctx, harnessmodel.ProviderCapacitySnapshot{
			AccountID:  acc1,
			Provider:   harnessmodel.ProviderAntigravity,
			Health:     harnessmodel.ProviderHealthHealthy,
			ObservedAt: now,
			Windows: []harnessmodel.QuotaWindow{
				{
					ID:         "ag_tokens",
					Metric:     harnessmodel.QuotaMetricTokens,
					Remaining:  floatPtr(1000000),
					Limit:      floatPtr(1000000),
					Confidence: 1.0,
					ObservedAt: now,
				},
			},
		}); err != nil {
			return err
		}
		if err := tx.AppendProviderCapacity(ctx, harnessmodel.ProviderCapacitySnapshot{
			AccountID:  acc2,
			Provider:   harnessmodel.ProviderCodex,
			Health:     harnessmodel.ProviderHealthHealthy,
			ObservedAt: now,
			Windows: []harnessmodel.QuotaWindow{
				{
					ID:         "codex_tokens",
					Metric:     harnessmodel.QuotaMetricTokens,
					Remaining:  floatPtr(500000),
					Limit:      floatPtr(500000),
					Confidence: 1.0,
					ObservedAt: now,
				},
			},
		}); err != nil {
			return err
		}

		// Workflow Run & Node Run & Attempt
		defID := harnessmodel.WorkflowDefinitionID("wfd_test")
		wfrID := harnessmodel.WorkflowRunID("wfr_test")
		nrID := harnessmodel.NodeRunID("nr_test")
		nodeID := harnessmodel.NodeID("node_test")

		if err := tx.CreateWorkflowDefinition(ctx, harnessmodel.WorkflowDefinition{
			ID: defID, Version: 1, Name: "test_wf", CreatedAt: now,
			Nodes: []harnessmodel.NodeSpec{{ID: nodeID, Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorAgent}},
		}); err != nil {
			return err
		}
		if err := tx.CreateWorkflowRun(ctx, harnessmodel.WorkflowRun{
			ID: wfrID, DefinitionID: defID, DefinitionVersion: 1, State: harnessmodel.WorkflowRunning, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return err
		}
		if err := tx.CreateGraphRevision(ctx, harnessmodel.GraphRevision{
			WorkflowRunID: wfrID, Number: 1, CreatedAt: now, Reason: "test",
		}); err != nil {
			return err
		}
		if err := tx.CreateNodeRun(ctx, harnessmodel.NodeRun{
			ID: nrID, WorkflowRunID: wfrID, NodeID: nodeID, GraphRevision: 1, Generation: 1, State: harnessmodel.NodeReady, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return err
		}
		if _, err := tx.CreateNextAttempt(ctx, harnessmodel.Attempt{
			ID: "att_test", NodeRunID: nrID, State: harnessmodel.AttemptCreated, CreatedAt: now,
		}); err != nil {
			return err
		}

		// Initial Active Assignment on Primary
		initAsn := harnessmodel.ProviderAssignment{
			ID:         "pasn_init",
			AttemptID:  "att_test",
			AccountID:  acc1,
			ModelID:    mod1,
			State:      harnessmodel.ProviderAssignmentActive,
			Revision:   1,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := tx.CreateProviderAssignment(ctx, initAsn); err != nil {
			return err
		}

		// Initial Active Reservation
		initRes := harnessmodel.ProviderReservation{
			ID:           "pres_init",
			AssignmentID: "pasn_init",
			AccountID:    acc1,
			WindowID:     "ag_tokens",
			ModelID:      mod1,
			Metric:       harnessmodel.QuotaMetricTokens,
			Amount:       1000,
			State:        harnessmodel.ProviderReservationActive,
			ExpiresAt:    now.Add(15 * time.Minute),
			Revision:     1,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		return tx.CreateProviderReservation(ctx, initRes)
	})

	if err != nil {
		t.Fatal(err)
	}
	return acc1, acc2
}

func makeEnvelope(planDigest string) harnessmodel.TaskEnvelope {
	return harnessmodel.TaskEnvelope{
		ID:           "tenv_handoff_1",
		TaskID:       "T-016",
		PlanDigest:   planDigest,
		TaskClass:    harnessmodel.TaskClassReview,
		Title:        "Safe Handoff Task",
		Objective:    "Verify safe provider handoff under in-doubt conditions",
		Instructions: "Verify no duplicate side effects on handoff",
		Workspace: harnessmodel.WorkspaceSpec{
			RootPath: "c:/repo",
			RepoID:   "repo1",
			ReadOnly: true,
		},
		Role:                 "worker",
		RequiredCapabilities: []string{"tools", "review"},
		MaxTokens:            5000,
		AttemptID:            "att_test",
		CreatedAt:            time.Now().UTC(),
	}
}

func TestHandoffSuccessWithNoEffects(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	now := time.Unix(1000, 0).UTC()
	acc1, acc2 := seedTestData(t, db, now)

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	mgr := NewManager(db, Options{
		Now: func() time.Time { return now },
	})

	env := makeEnvelope(planDigest)
	req := HandoffRequest{
		Envelope:          env,
		PlanText:          planText,
		PriorAssignmentID: "pasn_init",
		Reason:            "rate limited on primary",
	}

	result, err := mgr.Handoff(ctx, req)
	if err != nil {
		t.Fatalf("Handoff failed: %v", err)
	}

	if result.PriorAssignment.State != harnessmodel.ProviderAssignmentSuperseded {
		t.Errorf("prior assignment state = %s, want SUPERSEDED", result.PriorAssignment.State)
	}
	if result.PriorReservation == nil || result.PriorReservation.State != harnessmodel.ProviderReservationReleased {
		t.Errorf("prior reservation not RELEASED: %+v", result.PriorReservation)
	}
	if result.ReplacementAssignment.State != harnessmodel.ProviderAssignmentActive {
		t.Errorf("replacement assignment state = %s, want ACTIVE", result.ReplacementAssignment.State)
	}
	if result.ReplacementAssignment.AccountID != acc2 {
		t.Errorf("replacement account = %s, want %s (backup)", result.ReplacementAssignment.AccountID, acc2)
	}
	if result.ReplacementReservation == nil || result.ReplacementReservation.State != harnessmodel.ProviderReservationActive {
		t.Errorf("replacement reservation not ACTIVE: %+v", result.ReplacementReservation)
	}

	// Verify database state: exactly 1 ACTIVE assignment, 1 SUPERSEDED
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		asns, err := r.ListProviderAssignmentsByAttempt(ctx, "att_test")
		if err != nil {
			return err
		}
		if len(asns) != 2 {
			t.Fatalf("expected 2 assignments, got %d", len(asns))
		}
		var activeCount, supersededCount int
		for _, a := range asns {
			if a.State == harnessmodel.ProviderAssignmentActive {
				activeCount++
				if a.AccountID != acc2 {
					t.Errorf("active assignment account = %s, want %s", a.AccountID, acc2)
				}
			}
			if a.State == harnessmodel.ProviderAssignmentSuperseded {
				supersededCount++
				if a.AccountID != acc1 {
					t.Errorf("superseded assignment account = %s, want %s", a.AccountID, acc1)
				}
			}
		}
		if activeCount != 1 || supersededCount != 1 {
			t.Errorf("activeCount=%d, supersededCount=%d, want 1 and 1", activeCount, supersededCount)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestHandoffInDoubtRejectionWithoutReconciler(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	now := time.Unix(1000, 0).UTC()
	seedTestData(t, db, now)

	// Add an in-doubt non-idempotent effect to the attempt
	key1, dig1, err := harnessmodel.BuildEffectIdentity("wfr_test", "nr_test", "stripe", "charge_card", []byte(`{"amount":100}`))
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(ctx, func(tx harnessstore.Tx) error {
		eff := harnessmodel.EffectIntent{
			ID:                  "eff_indoubt_1",
			WorkflowRunID:       "wfr_test",
			NodeRunID:           "nr_test",
			OriginAttemptID:     "att_test",
			LastAttemptID:       "att_test",
			OperationNamespace:  "stripe",
			Operation:           "charge_card",
			Class:               harnessmodel.EffectNonIdempotentUnknown,
			IdempotencyKey:      key1,
			SemanticInputDigest: dig1,
			State:               harnessmodel.EffectPrepared,
			PreparedAt:          now,
		}
		if _, _, err := tx.PutEffectIntent(ctx, eff); err != nil {
			return err
		}
		eff.State = harnessmodel.EffectDispatched
		eff.DispatchedAt = now
		if err := tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectPrepared, eff); err != nil {
			return err
		}
		eff.State = harnessmodel.EffectInDoubt
		return tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectDispatched, eff)
	})
	if err != nil {
		t.Fatal(err)
	}

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	mgr := NewManager(db, Options{
		Now: func() time.Time { return now },
	})

	env := makeEnvelope(planDigest)
	req := HandoffRequest{
		Envelope:          env,
		PlanText:          planText,
		PriorAssignmentID: "pasn_init",
		Reason:            "test in doubt",
	}

	_, err = mgr.Handoff(ctx, req)
	if err == nil {
		t.Fatal("expected error for un-reconciled IN_DOUBT effect, got nil")
	}
	if !errors.Is(err, ErrHandoffUnsafeInDoubt) {
		t.Fatalf("expected ErrHandoffUnsafeInDoubt, got %v", err)
	}

	// Verify database transaction rolled back: prior assignment remains ACTIVE
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		asn, err := r.GetProviderAssignment(ctx, "pasn_init")
		if err != nil {
			return err
		}
		if asn.State != harnessmodel.ProviderAssignmentActive {
			t.Errorf("prior assignment state changed to %s on rolled back handoff", asn.State)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestHandoffInDoubtReconciledAbsent(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	now := time.Unix(1000, 0).UTC()
	seedTestData(t, db, now)

	// Add an in-doubt effect to the attempt
	key2, dig2, err := harnessmodel.BuildEffectIdentity("wfr_test", "nr_test", "github", "create_issue", []byte(`{"title":"issue"}`))
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(ctx, func(tx harnessstore.Tx) error {
		eff := harnessmodel.EffectIntent{
			ID:                  "eff_indoubt_absent",
			WorkflowRunID:       "wfr_test",
			NodeRunID:           "nr_test",
			OriginAttemptID:     "att_test",
			LastAttemptID:       "att_test",
			OperationNamespace:  "github",
			Operation:           "create_issue",
			Class:               harnessmodel.EffectNonIdempotentUnknown,
			IdempotencyKey:      key2,
			SemanticInputDigest: dig2,
			State:               harnessmodel.EffectPrepared,
			PreparedAt:          now,
		}
		if _, _, err := tx.PutEffectIntent(ctx, eff); err != nil {
			return err
		}
		eff.State = harnessmodel.EffectDispatched
		eff.DispatchedAt = now
		if err := tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectPrepared, eff); err != nil {
			return err
		}
		eff.State = harnessmodel.EffectInDoubt
		return tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectDispatched, eff)
	})
	if err != nil {
		t.Fatal(err)
	}

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	mgr := NewManager(db, Options{
		Now: func() time.Time { return now },
	})

	reconciler := &mockReconciler{
		reconcileFn: func(ctx context.Context, req harnessexecutor.EffectReconcileRequest) (harnessexecutor.EffectReconcileResult, error) {
			return harnessexecutor.EffectReconcileResult{
				Status: harnessexecutor.EffectReconcileAbsent,
			}, nil
		},
	}

	env := makeEnvelope(planDigest)
	req := HandoffRequest{
		Envelope:          env,
		PlanText:          planText,
		PriorAssignmentID: "pasn_init",
		Reconciler:        reconciler,
		Reason:            "reconciled absent handoff",
	}

	res, err := mgr.Handoff(ctx, req)
	if err != nil {
		t.Fatalf("Handoff with reconciled absent failed: %v", err)
	}

	if len(res.ReconciledEffects) != 1 {
		t.Fatalf("expected 1 reconciled effect, got %d", len(res.ReconciledEffects))
	}
	if res.ReconciledEffects[0].State != harnessmodel.EffectFailed {
		t.Errorf("reconciled effect state = %s, want FAILED", res.ReconciledEffects[0].State)
	}
	if res.ReplacementAssignment.State != harnessmodel.ProviderAssignmentActive {
		t.Errorf("replacement assignment state = %s, want ACTIVE", res.ReplacementAssignment.State)
	}
}

func TestHandoffPlanDriftRejection(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	now := time.Unix(1000, 0).UTC()
	seedTestData(t, db, now)

	planTextOriginal := []byte("# MASTER PLAN\n\nOriginal content")
	planDigest := harnessmodel.ComputePlanDigest(planTextOriginal)

	planTextDrifted := []byte("# MASTER PLAN\n\nModified content causing drift")

	mgr := NewManager(db, Options{
		Now: func() time.Time { return now },
	})

	env := makeEnvelope(planDigest)
	req := HandoffRequest{
		Envelope:          env,
		PlanText:          planTextDrifted,
		PriorAssignmentID: "pasn_init",
		Reason:            "drift test",
	}

	_, err := mgr.Handoff(ctx, req)
	if err == nil {
		t.Fatal("expected error on plan drift, got nil")
	}
	if !errors.Is(err, harnessmodel.ErrStalePlan) {
		t.Fatalf("expected ErrStalePlan, got %v", err)
	}
}
