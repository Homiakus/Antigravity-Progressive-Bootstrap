package sqlite

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestProviderObservationStoreRoundTripAndLatestOrdering(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	t0 := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	account := harnessmodel.ProviderAccount{
		ID: "pacc_test", Provider: harnessmodel.ProviderCodex, Name: "primary",
		State: harnessmodel.ProviderAccountActive, CreatedAt: t0, UpdatedAt: t0,
	}
	model := harnessmodel.ProviderModelDescriptor{
		AccountID: account.ID, Provider: account.Provider, ID: "model-b", DisplayName: "Model B",
		Capabilities: []string{"code", "tools"}, ContextLimit: 200000, Enabled: true,
	}
	session := harnessmodel.ProviderSessionSnapshot{
		ID: "pses_test", Provider: account.Provider, AccountID: account.ID, ModelID: model.ID,
		State: harnessmodel.ProviderSessionActive, ContextUsed: 1234, ContextLimit: 200000,
		LastUsedAt: t0.Add(30 * time.Second), WorkspaceFingerprint: "repo@abc",
	}
	fractionOld := 0.80
	fractionNew := 0.55
	reset := t0.Add(5 * time.Hour)

	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.UpsertProviderAccount(ctx, account); err != nil {
			return err
		}
		if err := tx.UpsertProviderModel(ctx, model, t0); err != nil {
			return err
		}
		if err := tx.UpsertProviderSession(ctx, session, t0.Add(time.Minute)); err != nil {
			return err
		}
		if err := tx.AppendProviderCapacity(ctx, harnessmodel.ProviderCapacitySnapshot{
			AccountID: account.ID, Provider: account.Provider, Health: harnessmodel.ProviderHealthHealthy,
			ObservedAt: t0, Windows: []harnessmodel.QuotaWindow{{
				ID: "primary", Metric: harnessmodel.QuotaMetricFraction, RemainingFraction: &fractionOld,
				ObservedAt: t0, Confidence: 1,
			}},
		}); err != nil {
			return err
		}
		return tx.AppendProviderCapacity(ctx, harnessmodel.ProviderCapacitySnapshot{
			AccountID: account.ID, Provider: account.Provider, Health: harnessmodel.ProviderHealthDegraded,
			ActiveRuns: 2, ObservedAt: t0.Add(time.Minute), Windows: []harnessmodel.QuotaWindow{
				{ID: "zeta", ModelID: model.ID, Metric: harnessmodel.QuotaMetricFraction, RemainingFraction: &fractionNew, ResetAt: &reset, ObservedAt: t0.Add(time.Minute), Confidence: 0.9},
				{ID: "alpha", Metric: harnessmodel.QuotaMetricOpaque, ObservedAt: t0.Add(time.Minute), Confidence: 0.5},
			},
		})
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		updated := account
		updated.Name = "renamed"
		updated.CreatedAt = t0.Add(time.Hour)
		updated.UpdatedAt = t0.Add(2 * time.Hour)
		return tx.UpsertProviderAccount(ctx, updated)
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		gotAccount, err := reader.GetProviderAccount(ctx, account.ID)
		if err != nil {
			return err
		}
		if gotAccount.Name != "renamed" || !gotAccount.CreatedAt.Equal(t0) {
			return fmt.Errorf("account=%+v; createdAt must remain durable", gotAccount)
		}
		accounts, err := reader.ListProviderAccounts(ctx, harnessmodel.ProviderCodex, harnessmodel.ProviderAccountActive)
		if err != nil {
			return err
		}
		if len(accounts) != 1 || accounts[0].ID != account.ID {
			return fmt.Errorf("filtered accounts=%+v", accounts)
		}
		models, err := reader.ListProviderModels(ctx, account.ID)
		if err != nil {
			return err
		}
		if len(models) != 1 || models[0].ID != model.ID || models[0].Provider != account.Provider || len(models[0].Capabilities) != 2 {
			return fmt.Errorf("models=%+v", models)
		}
		sessions, err := reader.ListProviderSessions(ctx, account.ID)
		if err != nil {
			return err
		}
		if len(sessions) != 1 || sessions[0].ID != session.ID || sessions[0].WorkspaceFingerprint != session.WorkspaceFingerprint {
			return fmt.Errorf("sessions=%+v", sessions)
		}
		latest, err := reader.GetLatestProviderCapacity(ctx, account.ID)
		if err != nil {
			return err
		}
		if latest.Health != harnessmodel.ProviderHealthDegraded || latest.ActiveRuns != 2 || !latest.ObservedAt.Equal(t0.Add(time.Minute)) {
			return fmt.Errorf("latest=%+v", latest)
		}
		if len(latest.Windows) != 2 || latest.Windows[0].ID != "alpha" || latest.Windows[1].ID != "zeta" {
			return fmt.Errorf("quota windows not deterministic: %+v", latest.Windows)
		}
		if latest.Windows[1].RemainingFraction == nil || *latest.Windows[1].RemainingFraction != fractionNew || latest.Windows[1].ResetAt == nil || !latest.Windows[1].ResetAt.Equal(reset) {
			return fmt.Errorf("zeta window=%+v", latest.Windows[1])
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestProviderCapacityAppendRollsBackSnapshotAndWindowsAtomically(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC)
	account := harnessmodel.ProviderAccount{ID: "pacc_atomic", Provider: harnessmodel.ProviderAntigravity, State: harnessmodel.ProviderAccountActive, CreatedAt: now, UpdatedAt: now}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.UpsertProviderAccount(ctx, account) }); err != nil {
		t.Fatal(err)
	}

	snapshot := harnessmodel.ProviderCapacitySnapshot{
		AccountID: account.ID, Provider: account.Provider, Health: harnessmodel.ProviderHealthHealthy, ObservedAt: now,
		Windows: []harnessmodel.QuotaWindow{
			{ID: "duplicate", Metric: harnessmodel.QuotaMetricOpaque, ObservedAt: now, Confidence: 1},
			{ID: "duplicate", Metric: harnessmodel.QuotaMetricOpaque, ObservedAt: now, Confidence: 1},
		},
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.AppendProviderCapacity(ctx, snapshot) }); err == nil {
		t.Fatal("duplicate quota windows unexpectedly committed")
	}

	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		_, err := reader.GetLatestProviderCapacity(ctx, account.ID)
		if !errors.Is(err, harnessstore.ErrNotFound) {
			return fmt.Errorf("latest after failed atomic append error=%v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestProviderObservationRejectsProviderIdentityMismatch(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC)
	account := harnessmodel.ProviderAccount{ID: "pacc_codex", Provider: harnessmodel.ProviderCodex, State: harnessmodel.ProviderAccountActive, CreatedAt: now, UpdatedAt: now}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.UpsertProviderAccount(ctx, account) }); err != nil {
		t.Fatal(err)
	}

	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.UpsertProviderModel(ctx, harnessmodel.ProviderModelDescriptor{
			AccountID: account.ID, Provider: harnessmodel.ProviderAntigravity, ID: "wrong", Enabled: true,
		}, now)
	}); err == nil {
		t.Fatal("model provider mismatch unexpectedly accepted")
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.AppendProviderCapacity(ctx, harnessmodel.ProviderCapacitySnapshot{
			AccountID: account.ID, Provider: harnessmodel.ProviderAntigravity, Health: harnessmodel.ProviderHealthHealthy, ObservedAt: now,
		})
	}); err == nil {
		t.Fatal("capacity provider mismatch unexpectedly accepted")
	}
}

func TestProviderObservationSupportsConcurrentWALReaders(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{MaxOpenConns: 8, MaxIdleConns: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	base := time.Date(2026, 8, 28, 15, 0, 0, 0, time.UTC)
	account := harnessmodel.ProviderAccount{ID: "pacc_wal", Provider: harnessmodel.ProviderCodex, State: harnessmodel.ProviderAccountActive, CreatedAt: base, UpdatedAt: base}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.UpsertProviderAccount(ctx, account); err != nil {
			return err
		}
		return tx.AppendProviderCapacity(ctx, harnessmodel.ProviderCapacitySnapshot{AccountID: account.ID, Provider: account.Provider, Health: harnessmodel.ProviderHealthHealthy, ObservedAt: base})
	}); err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 16)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if err := db.View(ctx, func(reader harnessstore.Reader) error {
					_, err := reader.GetLatestProviderCapacity(ctx, account.ID)
					return err
				}); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	for j := 1; j <= 20; j++ {
		observed := base.Add(time.Duration(j) * time.Second)
		if err := db.Update(ctx, func(tx harnessstore.Tx) error {
			return tx.AppendProviderCapacity(ctx, harnessmodel.ProviderCapacitySnapshot{AccountID: account.ID, Provider: account.Provider, Health: harnessmodel.ProviderHealthHealthy, ObservedAt: observed})
		}); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent WAL reader failed: %v", err)
	}
}
