package router

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	harnesssqlite "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

func floatPtr(f float64) *float64 {
	return &f
}

func setupTestStore(t testing.TB) (*harnesssqlite.DB, func()) {
	ctx := context.Background()
	db, err := harnesssqlite.Open(ctx, filepath.Join(t.TempDir(), "router_test.db"), harnesssqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return db, func() { _ = db.Close() }
}

func seedTestAccounts(t testing.TB, db harnessstore.Store, now time.Time) {
	ctx := context.Background()
	accAG := harnessmodel.ProviderAccount{
		ID:        "acc_ag",
		Provider:  harnessmodel.ProviderAntigravity,
		Name:      "Antigravity Main",
		State:     harnessmodel.ProviderAccountActive,
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now,
	}
	modelAG := harnessmodel.ProviderModelDescriptor{
		AccountID:    accAG.ID,
		Provider:     accAG.Provider,
		ID:           "gemini-2.5-pro",
		DisplayName:  "Gemini Pro",
		Capabilities: []string{"tools", "file_edit", "review"},
		ContextLimit: 128000,
		Enabled:      true,
	}
	capAG := harnessmodel.ProviderCapacitySnapshot{
		AccountID:  accAG.ID,
		Provider:   accAG.Provider,
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
	}

	accCodex := harnessmodel.ProviderAccount{
		ID:        "acc_codex",
		Provider:  harnessmodel.ProviderCodex,
		Name:      "Codex Secondary",
		State:     harnessmodel.ProviderAccountActive,
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now,
	}
	modelCodex := harnessmodel.ProviderModelDescriptor{
		AccountID:    accCodex.ID,
		Provider:     accCodex.Provider,
		ID:           "codex-max",
		DisplayName:  "Codex Max",
		Capabilities: []string{"tools", "file_edit", "review"},
		ContextLimit: 128000,
		Enabled:      true,
	}
	capCodex := harnessmodel.ProviderCapacitySnapshot{
		AccountID:  accCodex.ID,
		Provider:   accCodex.Provider,
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
	}

	err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.UpsertProviderAccount(ctx, accAG); err != nil {
			return err
		}
		if err := tx.UpsertProviderModel(ctx, modelAG, now); err != nil {
			return err
		}
		if err := tx.AppendProviderCapacity(ctx, capAG); err != nil {
			return err
		}
		if err := tx.UpsertProviderAccount(ctx, accCodex); err != nil {
			return err
		}
		if err := tx.UpsertProviderModel(ctx, modelCodex, now); err != nil {
			return err
		}
		return tx.AppendProviderCapacity(ctx, capCodex)
	})
	if err != nil {
		t.Fatalf("seed accounts failed: %v", err)
	}
}

func makeReadOnlyEnvelope(planDigest string) harnessmodel.TaskEnvelope {
	return harnessmodel.TaskEnvelope{
		ID:           "tenv_ro_001",
		TaskID:       "T-014",
		PlanDigest:   planDigest,
		TaskClass:    harnessmodel.TaskClassReview,
		Title:        "Read-only code review",
		Objective:    "Audit provider router implementation",
		Instructions: "Verify no disk mutations occur during execution",
		Workspace: harnessmodel.WorkspaceSpec{
			RootPath: "c:/repo",
			RepoID:   "repo1",
			ReadOnly: true,
		},
		Role:                 "worker",
		RequiredCapabilities: []string{"tools", "review"},
		MaxTokens:            5000,
		CreatedAt:            time.Now().UTC(),
	}
}

func TestRouterReadOnlyEnforcement(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	router := NewRouter(db, Options{})

	// Workspace.ReadOnly = false must be rejected
	writeEnv := makeReadOnlyEnvelope(planDigest)
	writeEnv.Workspace.ReadOnly = false
	writeEnv.Workspace.Isolated = true
	writeEnv.Workspace.IsolationRoot = "c:/repo/.scratch/t014"

	_, err := router.Route(ctx, writeEnv, planText)
	if err == nil {
		t.Fatal("expected ErrReadOnlyRequired for non-read-only envelope, got nil")
	}
	if !errors.Is(err, ErrReadOnlyRequired) {
		t.Fatalf("expected ErrReadOnlyRequired, got %v", err)
	}
}

func TestRouterPlanDriftEnforcement(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	router := NewRouter(db, Options{})

	env := makeReadOnlyEnvelope(planDigest)

	// Drifted plan text must be rejected
	tamperedPlan := []byte("# MASTER PLAN\n\nTask details... (tampered!)")
	_, err := router.Route(ctx, env, tamperedPlan)
	if err == nil {
		t.Fatal("expected ErrStalePlan for drifted plan text, got nil")
	}
	if !errors.Is(err, harnessmodel.ErrStalePlan) {
		t.Fatalf("expected ErrStalePlan, got %v", err)
	}
}

func TestRouterNoViableProvider(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	router := NewRouter(db, Options{})
	env := makeReadOnlyEnvelope(planDigest)

	// Empty store has no accounts
	_, err := router.Route(ctx, env, planText)
	if err == nil {
		t.Fatal("expected ErrNoViableProvider for empty store, got nil")
	}
	if !errors.Is(err, ErrNoViableProvider) {
		t.Fatalf("expected ErrNoViableProvider, got %v", err)
	}
}

func TestRouterSuccessfulRouteAndExecute(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	now := time.Unix(2000, 0).UTC()
	seedTestAccounts(t, db, now)

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	router := NewRouter(db, Options{
		Now: func() time.Time { return now },
	})

	env := makeReadOnlyEnvelope(planDigest)

	route, err := router.Route(ctx, env, planText)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	if route.Assignment.ID == "" {
		t.Fatal("route.Assignment.ID is empty")
	}
	if route.Assignment.PlanDigest != planDigest {
		t.Fatalf("route.Assignment.PlanDigest = %q, want %q", route.Assignment.PlanDigest, planDigest)
	}
	if route.Assignment.State != harnessmodel.ProviderAssignmentActive {
		t.Fatalf("route.Assignment.State = %s, want ACTIVE", route.Assignment.State)
	}
	if route.Reservation == nil {
		t.Fatal("expected non-nil reservation for token demand")
	}
	if route.Reservation.State != harnessmodel.ProviderReservationActive {
		t.Fatalf("route.Reservation.State = %s, want ACTIVE", route.Reservation.State)
	}

	// Verify persistence in store
	var loadedAsn harnessmodel.ProviderAssignment
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		var err error
		loadedAsn, err = r.GetProviderAssignment(ctx, route.Assignment.ID)
		return err
	}); err != nil {
		t.Fatalf("read assignment from store: %v", err)
	}
	if loadedAsn.State != harnessmodel.ProviderAssignmentActive {
		t.Fatalf("stored assignment state = %s, want ACTIVE", loadedAsn.State)
	}

	// Execute successfully
	execCalled := false
	execFn := func(ctx context.Context, e harnessmodel.TaskEnvelope, a harnessmodel.ProviderAssignment) (string, int64, error) {
		execCalled = true
		if !e.Workspace.ReadOnly {
			t.Error("executor received non-read-only envelope")
		}
		return "Analysis: code is compliant and secure", 1200, nil
	}

	result, err := router.Execute(ctx, route, execFn)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !execCalled {
		t.Fatal("execFn was not called")
	}
	if !result.Success {
		t.Fatalf("result.Success = false, error = %s", result.Error)
	}
	if result.TokensUsed != 1200 {
		t.Fatalf("result.TokensUsed = %d, want 1200", result.TokensUsed)
	}

	// Verify assignment transitioned to COMPLETED and reservation RELEASED
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		asn, err := r.GetProviderAssignment(ctx, route.Assignment.ID)
		if err != nil {
			return err
		}
		if asn.State != harnessmodel.ProviderAssignmentCompleted {
			t.Errorf("assignment state = %s, want COMPLETED", asn.State)
		}

		res, err := r.GetProviderReservation(ctx, route.Reservation.ID)
		if err != nil {
			return err
		}
		if res.State != harnessmodel.ProviderReservationReleased {
			t.Errorf("reservation state = %s, want RELEASED", res.State)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Attempting to re-execute completed route should fail
	_, err = router.Execute(ctx, route, execFn)
	if err == nil {
		t.Fatal("expected error re-executing settled route, got nil")
	}
}

func TestRouterExecutionFailure(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	now := time.Unix(2000, 0).UTC()
	seedTestAccounts(t, db, now)

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	router := NewRouter(db, Options{
		Now: func() time.Time { return now },
	})

	env := makeReadOnlyEnvelope(planDigest)

	route, err := router.Route(ctx, env, planText)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	expectedErr := errors.New("upstream provider rate limit")
	failFn := func(ctx context.Context, e harnessmodel.TaskEnvelope, a harnessmodel.ProviderAssignment) (string, int64, error) {
		return "", 0, expectedErr
	}

	result, err := router.Execute(ctx, route, failFn)
	if err == nil {
		t.Fatal("expected error from Execute, got nil")
	}
	if !errors.Is(err, ErrExecutionFailed) {
		t.Fatalf("expected ErrExecutionFailed, got %v", err)
	}
	if result.Success {
		t.Fatal("result.Success should be false")
	}
	if result.Fault == nil || result.Fault.Kind != "RATE_LIMITED" {
		t.Fatalf("unexpected result.Fault: %+v", result.Fault)
	}
	if result.RetryAction == "" {
		t.Fatal("result.RetryAction is empty")
	}

	// Verify assignment transitioned to RELEASED and reservation RELEASED
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		asn, err := r.GetProviderAssignment(ctx, route.Assignment.ID)
		if err != nil {
			return err
		}
		if asn.State != harnessmodel.ProviderAssignmentReleased {
			t.Errorf("assignment state = %s, want RELEASED", asn.State)
		}

		res, err := r.GetProviderReservation(ctx, route.Reservation.ID)
		if err != nil {
			return err
		}
		if res.State != harnessmodel.ProviderReservationReleased {
			t.Errorf("reservation state = %s, want RELEASED", res.State)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func makeIsolatedWriteEnvelope(planDigest string) harnessmodel.TaskEnvelope {
	return harnessmodel.TaskEnvelope{
		ID:           "tenv_write_001",
		TaskID:       "T-017",
		PlanDigest:   planDigest,
		TaskClass:    harnessmodel.TaskClassCodegen,
		Title:        "Isolated code edit",
		Objective:    "Implement isolated write router",
		Instructions: "Write new implementation in isolated sandbox workspace",
		Workspace: harnessmodel.WorkspaceSpec{
			RootPath:      "c:/repo",
			RepoID:        "repo1",
			ReadOnly:      false,
			Isolated:      true,
			IsolationRoot: "c:/repo/.scratch/t017",
		},
		Role:                 "worker",
		RequiredCapabilities: []string{"tools", "file_edit"},
		MaxTokens:            6000,
		CreatedAt:            time.Now().UTC(),
	}
}

func TestRouterIsolatedWriteEnforcement(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	router := NewRouter(db, Options{})

	// 1. Non-isolated write envelope must be rejected during envelope validation or routing
	unisolatedEnv := makeIsolatedWriteEnvelope(planDigest)
	unisolatedEnv.Workspace.Isolated = false
	unisolatedEnv.Workspace.IsolationRoot = ""

	_, err := router.RouteIsolatedWrite(ctx, unisolatedEnv, planText)
	if err == nil {
		t.Fatal("expected error for unisolated write envelope, got nil")
	}

	// 2. ReadOnly envelope passed to RouteIsolatedWrite must be rejected
	roEnv := makeReadOnlyEnvelope(planDigest)
	_, err = router.RouteIsolatedWrite(ctx, roEnv, planText)
	if err == nil {
		t.Fatal("expected ErrIsolatedWriteRequired for ReadOnly envelope in RouteIsolatedWrite, got nil")
	}
	if !errors.Is(err, ErrIsolatedWriteRequired) {
		t.Fatalf("expected ErrIsolatedWriteRequired, got %v", err)
	}

	// 3. Plan drift in isolated write must be rejected
	tamperedPlan := []byte("# MASTER PLAN\n\nTampered plan content!")
	validEnv := makeIsolatedWriteEnvelope(planDigest)
	_, err = router.RouteIsolatedWrite(ctx, validEnv, tamperedPlan)
	if err == nil {
		t.Fatal("expected ErrStalePlan for drifted plan in RouteIsolatedWrite, got nil")
	}
	if !errors.Is(err, harnessmodel.ErrStalePlan) {
		t.Fatalf("expected ErrStalePlan, got %v", err)
	}
}

func TestRouterIsolatedWriteSuccessfulRouteAndExecute(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	now := time.Unix(3000, 0).UTC()
	seedTestAccounts(t, db, now)

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	router := NewRouter(db, Options{
		Now: func() time.Time { return now },
	})

	env := makeIsolatedWriteEnvelope(planDigest)

	route, err := router.RouteIsolatedWrite(ctx, env, planText)
	if err != nil {
		t.Fatalf("RouteIsolatedWrite failed: %v", err)
	}

	if route.Assignment.ID == "" {
		t.Fatal("route.Assignment.ID is empty")
	}
	if route.Assignment.PlanDigest != planDigest {
		t.Fatalf("route.Assignment.PlanDigest = %q, want %q", route.Assignment.PlanDigest, planDigest)
	}
	if route.Assignment.State != harnessmodel.ProviderAssignmentActive {
		t.Fatalf("route.Assignment.State = %s, want ACTIVE", route.Assignment.State)
	}
	if route.Reservation == nil {
		t.Fatal("expected non-nil reservation")
	}
	if route.Reservation.State != harnessmodel.ProviderReservationActive {
		t.Fatalf("route.Reservation.State = %s, want ACTIVE", route.Reservation.State)
	}

	// Execute isolated write callback
	execCalled := false
	execFn := func(ctx context.Context, e harnessmodel.TaskEnvelope, a harnessmodel.ProviderAssignment) (IsolatedWriteOutput, error) {
		execCalled = true
		if e.Workspace.ReadOnly || !e.Workspace.Isolated {
			t.Error("write executor received unisolated workspace")
		}
		if e.Workspace.IsolationRoot != "c:/repo/.scratch/t017" {
			t.Errorf("unexpected isolation root: %s", e.Workspace.IsolationRoot)
		}
		return IsolatedWriteOutput{
			ModifiedFiles: []string{"internal/harness/router.go"},
			DiffSummary:   "+150 -10",
			Output:        "isolated write successful",
			TokensUsed:    2400,
		}, nil
	}

	result, err := router.ExecuteIsolatedWrite(ctx, route, execFn)
	if err != nil {
		t.Fatalf("ExecuteIsolatedWrite failed: %v", err)
	}
	if !execCalled {
		t.Fatal("write executor was not called")
	}
	if !result.Success {
		t.Fatalf("result.Success = false, error: %s", result.Error)
	}
	if result.TokensUsed != 2400 {
		t.Fatalf("result.TokensUsed = %d, want 2400", result.TokensUsed)
	}

	// Verify persistence in SQLite
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		asn, err := r.GetProviderAssignment(ctx, route.Assignment.ID)
		if err != nil {
			return err
		}
		if asn.State != harnessmodel.ProviderAssignmentCompleted {
			t.Errorf("assignment state = %s, want COMPLETED", asn.State)
		}

		res, err := r.GetProviderReservation(ctx, route.Reservation.ID)
		if err != nil {
			return err
		}
		if res.State != harnessmodel.ProviderReservationReleased {
			t.Errorf("reservation state = %s, want RELEASED", res.State)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Re-executing completed route must fail
	_, err = router.ExecuteIsolatedWrite(ctx, route, execFn)
	if err == nil {
		t.Fatal("expected error re-executing settled write route, got nil")
	}
}

func TestRouterIsolatedWriteExecutionFailure(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	now := time.Unix(3000, 0).UTC()
	seedTestAccounts(t, db, now)

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	router := NewRouter(db, Options{
		Now: func() time.Time { return now },
	})

	env := makeIsolatedWriteEnvelope(planDigest)

	route, err := router.RouteIsolatedWrite(ctx, env, planText)
	if err != nil {
		t.Fatalf("RouteIsolatedWrite failed: %v", err)
	}

	failFn := func(ctx context.Context, e harnessmodel.TaskEnvelope, a harnessmodel.ProviderAssignment) (IsolatedWriteOutput, error) {
		return IsolatedWriteOutput{}, errors.New("timeout connecting to provider backend")
	}

	result, err := router.ExecuteIsolatedWrite(ctx, route, failFn)
	if err == nil {
		t.Fatal("expected error from ExecuteIsolatedWrite, got nil")
	}
	if !errors.Is(err, ErrExecutionFailed) {
		t.Fatalf("expected ErrExecutionFailed, got %v", err)
	}
	if result.Success {
		t.Fatal("result.Success should be false")
	}
	if result.Fault == nil || result.Fault.Kind != "TRANSIENT_NETWORK" {
		t.Fatalf("unexpected result.Fault: %+v", result.Fault)
	}

	// Verify assignment transitioned to RELEASED and reservation RELEASED
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		asn, err := r.GetProviderAssignment(ctx, route.Assignment.ID)
		if err != nil {
			return err
		}
		if asn.State != harnessmodel.ProviderAssignmentReleased {
			t.Errorf("assignment state = %s, want RELEASED", asn.State)
		}

		res, err := r.GetProviderReservation(ctx, route.Reservation.ID)
		if err != nil {
			return err
		}
		if res.State != harnessmodel.ProviderReservationReleased {
			t.Errorf("reservation state = %s, want RELEASED", res.State)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
