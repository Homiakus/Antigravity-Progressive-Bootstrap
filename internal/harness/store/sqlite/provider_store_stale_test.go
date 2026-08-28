package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestProviderObservationUpsertsRejectStaleState(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	t0 := time.Date(2026, 8, 28, 16, 0, 0, 0, time.UTC)
	account := harnessmodel.ProviderAccount{
		ID: "pacc_stale", Provider: harnessmodel.ProviderCodex, Name: "fresh",
		State: harnessmodel.ProviderAccountActive, CreatedAt: t0, UpdatedAt: t0.Add(2 * time.Minute),
	}
	model := harnessmodel.ProviderModelDescriptor{
		AccountID: account.ID, Provider: account.Provider, ID: "model-stale", DisplayName: "fresh-model",
		ContextLimit: 200000, Enabled: true,
	}
	session := harnessmodel.ProviderSessionSnapshot{
		ID: "pses_stale", Provider: account.Provider, AccountID: account.ID, ModelID: model.ID,
		State: harnessmodel.ProviderSessionActive, ContextUsed: 100, ContextLimit: 200000,
		LastUsedAt: t0.Add(2 * time.Minute), WorkspaceFingerprint: "fresh",
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.UpsertProviderAccount(ctx, account); err != nil {
			return err
		}
		if err := tx.UpsertProviderModel(ctx, model, t0.Add(2*time.Minute)); err != nil {
			return err
		}
		return tx.UpsertProviderSession(ctx, session, t0.Add(2*time.Minute))
	}); err != nil {
		t.Fatal(err)
	}

	staleAccount := account
	staleAccount.Name = "stale-account"
	staleAccount.UpdatedAt = t0.Add(time.Minute)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.UpsertProviderAccount(ctx, staleAccount) }); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("stale account error=%v, want conflict", err)
	}

	staleModel := model
	staleModel.DisplayName = "stale-model"
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.UpsertProviderModel(ctx, staleModel, t0.Add(time.Minute)) }); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("stale model error=%v, want conflict", err)
	}

	staleSession := session
	staleSession.WorkspaceFingerprint = "stale"
	staleSession.LastUsedAt = t0.Add(time.Minute)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.UpsertProviderSession(ctx, staleSession, t0.Add(time.Minute)) }); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("stale session error=%v, want conflict", err)
	}

	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		got, err := reader.GetProviderAccount(ctx, account.ID)
		if err != nil {
			return err
		}
		if got.Name != "fresh" {
			t.Fatalf("stale account overwrote fresh state: %+v", got)
		}
		models, err := reader.ListProviderModels(ctx, account.ID)
		if err != nil {
			return err
		}
		if len(models) != 1 || models[0].DisplayName != "fresh-model" {
			t.Fatalf("stale model overwrote fresh state: %+v", models)
		}
		sessions, err := reader.ListProviderSessions(ctx, account.ID)
		if err != nil {
			return err
		}
		if len(sessions) != 1 || sessions[0].WorkspaceFingerprint != "fresh" {
			t.Fatalf("stale session overwrote fresh state: %+v", sessions)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestProviderSessionIdentityCannotMoveBetweenAccounts(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 8, 28, 17, 0, 0, 0, time.UTC)
	a := harnessmodel.ProviderAccount{ID: "pacc_a", Provider: harnessmodel.ProviderCodex, State: harnessmodel.ProviderAccountActive, CreatedAt: now, UpdatedAt: now}
	b := harnessmodel.ProviderAccount{ID: "pacc_b", Provider: harnessmodel.ProviderCodex, State: harnessmodel.ProviderAccountActive, CreatedAt: now, UpdatedAt: now}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.UpsertProviderAccount(ctx, a); err != nil {
			return err
		}
		return tx.UpsertProviderAccount(ctx, b)
	}); err != nil {
		t.Fatal(err)
	}

	s := harnessmodel.ProviderSessionSnapshot{ID: "pses_fixed", Provider: harnessmodel.ProviderCodex, AccountID: a.ID, ModelID: "model", State: harnessmodel.ProviderSessionActive, LastUsedAt: now}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.UpsertProviderSession(ctx, s, now) }); err != nil {
		t.Fatal(err)
	}
	s.AccountID = b.ID
	s.LastUsedAt = now.Add(time.Minute)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.UpsertProviderSession(ctx, s, now.Add(time.Minute)) }); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("session account move error=%v, want conflict", err)
	}
}
