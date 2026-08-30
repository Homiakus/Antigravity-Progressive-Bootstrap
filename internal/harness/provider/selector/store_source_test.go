package selector

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	harnesssqlite "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

func TestStoreSourceIntegration(t *testing.T) {
	ctx := context.Background()
	db, err := harnesssqlite.Open(ctx, filepath.Join(t.TempDir(), "selector_test.db"), harnesssqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Now().UTC()

	// 1. Setup account 1 (Antigravity)
	acc1 := harnessmodel.ProviderAccount{
		ID:        "acc_antigravity",
		Provider:  harnessmodel.ProviderAntigravity,
		Name:      "AG Account",
		State:     harnessmodel.ProviderAccountActive,
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now,
	}
	model1 := harnessmodel.ProviderModelDescriptor{
		AccountID:    acc1.ID,
		Provider:     acc1.Provider,
		ID:           "model_ag",
		DisplayName:  "Antigravity Pro",
		Capabilities: []string{"tools", "streaming"},
		ContextLimit: 128000,
		Enabled:      true,
	}
	cap1 := harnessmodel.ProviderCapacitySnapshot{
		AccountID:  acc1.ID,
		Provider:   acc1.Provider,
		Health:     harnessmodel.ProviderHealthHealthy,
		ObservedAt: now,
		Windows: []harnessmodel.QuotaWindow{
			{
				ID:                "ag_window",
				Metric:            harnessmodel.QuotaMetricFraction,
				RemainingFraction: floatPtr(0.90),
				Confidence:        1.0,
				ObservedAt:        now,
			},
		},
	}
	sess1 := harnessmodel.ProviderSessionSnapshot{
		ID:                   "sess_ag_active",
		Provider:             acc1.Provider,
		AccountID:            acc1.ID,
		ModelID:              model1.ID,
		State:                harnessmodel.ProviderSessionActive,
		ContextUsed:          500,
		ContextLimit:         128000,
		LastUsedAt:           now,
		WorkspaceFingerprint: "fingerprint:test",
	}

	// 2. Setup account 2 (Codex)
	acc2 := harnessmodel.ProviderAccount{
		ID:        "acc_codex",
		Provider:  harnessmodel.ProviderCodex,
		Name:      "Codex Account",
		State:     harnessmodel.ProviderAccountActive,
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now,
	}
	model2 := harnessmodel.ProviderModelDescriptor{
		AccountID:    acc2.ID,
		Provider:     acc2.Provider,
		ID:           "model_codex",
		DisplayName:  "Codex Max",
		Capabilities: []string{"tools", "streaming"},
		ContextLimit: 128000,
		Enabled:      true,
	}
	cap2 := harnessmodel.ProviderCapacitySnapshot{
		AccountID:  acc2.ID,
		Provider:   acc2.Provider,
		Health:     harnessmodel.ProviderHealthHealthy,
		ObservedAt: now,
		Windows: []harnessmodel.QuotaWindow{
			{
				ID:                "codex_window",
				Metric:            harnessmodel.QuotaMetricFraction,
				RemainingFraction: floatPtr(0.50),
				Confidence:        1.0,
				ObservedAt:        now,
			},
		},
	}

	// Insert into DB inside a transaction
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.UpsertProviderAccount(ctx, acc1); err != nil {
			return err
		}
		if err := tx.UpsertProviderModel(ctx, model1, now); err != nil {
			return err
		}
		if err := tx.AppendProviderCapacity(ctx, cap1); err != nil {
			return err
		}
		if err := tx.UpsertProviderSession(ctx, sess1, now); err != nil {
			return err
		}

		if err := tx.UpsertProviderAccount(ctx, acc2); err != nil {
			return err
		}
		if err := tx.UpsertProviderModel(ctx, model2, now); err != nil {
			return err
		}
		return tx.AppendProviderCapacity(ctx, cap2)
	}); err != nil {
		t.Fatal(err)
	}

	source := StoreSource{Store: db}
	req := Request{
		TaskClass:            "codegen",
		RequiredCapabilities: []string{"tools"},
		WorkspaceFingerprint: "fingerprint:test",
	}

	dec, err := source.Select(ctx, req, now, DefaultPolicy())
	if err != nil {
		t.Fatalf("source.Select failed: %v", err)
	}

	if dec.Selected == nil {
		t.Fatal("expected candidate to be selected")
	}
	if dec.Selected.AccountID != "acc_antigravity" || dec.Selected.ModelID != "model_ag" {
		t.Fatalf("expected acc_antigravity/model_ag to be selected, got %s/%s", dec.Selected.AccountID, dec.Selected.ModelID)
	}
	if dec.Selected.SessionDecision.Action != "REUSE" {
		t.Fatalf("expected REUSE session action, got %s", dec.Selected.SessionDecision.Action)
	}
	if len(dec.Evaluations) != 2 {
		t.Fatalf("expected 2 evaluations, got %d", len(dec.Evaluations))
	}
}
