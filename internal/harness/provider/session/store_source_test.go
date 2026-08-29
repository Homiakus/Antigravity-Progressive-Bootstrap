package session

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	harnesssqlite "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

func TestStoreSourceLoadsOneDurableSessionSnapshot(t *testing.T) {
	ctx := context.Background()
	db, err := harnesssqlite.Open(ctx, filepath.Join(t.TempDir(), "harness.db"), harnesssqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 8, 29, 8, 0, 0, 0, time.UTC)
	account := harnessmodel.ProviderAccount{
		ID: "account-store", Provider: harnessmodel.ProviderAntigravity, Name: "store-test",
		State: harnessmodel.ProviderAccountActive, CreatedAt: now.Add(-time.Hour), UpdatedAt: now,
	}
	model := harnessmodel.ProviderModelDescriptor{
		AccountID: account.ID, Provider: account.Provider, ID: "model-store", DisplayName: "Store Model",
		Capabilities: []string{"code"}, ContextLimit: 2000, Enabled: true,
	}
	providerSession := harnessmodel.ProviderSessionSnapshot{
		ID: "session-store", Provider: account.Provider, AccountID: account.ID, ModelID: model.ID,
		State: harnessmodel.ProviderSessionActive, ContextUsed: 500, ContextLimit: 2000,
		LastUsedAt: now, WorkspaceFingerprint: "sha256:store-repo",
	}
	capacity := harnessmodel.ProviderCapacitySnapshot{
		AccountID: account.ID, Provider: account.Provider, Health: harnessmodel.ProviderHealthHealthy, ObservedAt: now,
	}

	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.UpsertProviderAccount(ctx, account); err != nil {
			return err
		}
		if err := tx.UpsertProviderModel(ctx, model, now); err != nil {
			return err
		}
		if err := tx.AppendProviderCapacity(ctx, capacity); err != nil {
			return err
		}
		return tx.UpsertProviderSession(ctx, providerSession, now)
	}); err != nil {
		t.Fatal(err)
	}

	broker := Broker{
		Source: StoreSource{Store: db},
		Policy: Policy{AcquireReuseHeadroomFraction: 0.30, RetainReuseHeadroomFraction: 0.15},
	}
	decision, err := broker.Decide(ctx, account.ID, Request{
		ModelID: model.ID, RequiredCapabilities: []string{"code"}, WorkspaceFingerprint: providerSession.WorkspaceFingerprint,
		RequiredContext: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionReuse || decision.SessionID != providerSession.ID {
		t.Fatalf("decision = %+v, want REUSE %s", decision, providerSession.ID)
	}
	if decision.Headroom != 1500 || decision.HeadroomFraction != 0.75 {
		t.Fatalf("headroom = %d/%f, want 1500/0.75", decision.Headroom, decision.HeadroomFraction)
	}
}

func TestStoreSourceTreatsMissingCapacityAsUnknownHealthNotUnlimitedEvidence(t *testing.T) {
	ctx := context.Background()
	db, err := harnesssqlite.Open(ctx, filepath.Join(t.TempDir(), "harness.db"), harnesssqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 8, 29, 8, 30, 0, 0, time.UTC)
	account := harnessmodel.ProviderAccount{
		ID: "account-no-capacity", Provider: harnessmodel.ProviderCodex,
		State: harnessmodel.ProviderAccountActive, CreatedAt: now.Add(-time.Hour), UpdatedAt: now,
	}
	model := harnessmodel.ProviderModelDescriptor{
		AccountID: account.ID, Provider: account.Provider, ID: "codex-model", ContextLimit: 1000, Enabled: true,
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.UpsertProviderAccount(ctx, account); err != nil {
			return err
		}
		return tx.UpsertProviderModel(ctx, model, now)
	}); err != nil {
		t.Fatal(err)
	}

	snapshot, err := (StoreSource{Store: db}).Snapshot(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Health != harnessmodel.ProviderHealthUnknown {
		t.Fatalf("health = %s, want UNKNOWN", snapshot.Health)
	}
	if len(snapshot.Sessions) != 0 {
		t.Fatalf("sessions = %v, want none without authoritative Codex thread->model binding", snapshot.Sessions)
	}

	decision, err := Evaluate(snapshot, Request{ModelID: model.ID, RequiredContext: 100}, Policy{AcquireReuseHeadroomFraction: 0.30, RetainReuseHeadroomFraction: 0.15})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionNew {
		t.Fatalf("decision = %+v, want NEW; UNKNOWN health must not be treated as EXHAUSTED or fabricated headroom", decision)
	}
}
