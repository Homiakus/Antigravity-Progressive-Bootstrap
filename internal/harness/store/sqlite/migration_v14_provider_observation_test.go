package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

func TestUpgradeFromVersionThirteenIncludesProviderObservationSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	openFixtureAtVersion(t, path, 13)
	db, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	version, err := db.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != SchemaVersion {
		t.Fatalf("schema version=%d want=%d", version, SchemaVersion)
	}
	for _, table := range []string{
		"provider_accounts",
		"provider_models",
		"provider_capacity_snapshots",
		"provider_quota_windows",
		"provider_sessions",
	} {
		var got string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&got); err != nil {
			t.Fatalf("missing v14 table %s after upgrade: %v", table, err)
		}
	}
	for _, index := range []string{
		"provider_accounts_by_provider_state",
		"provider_models_by_account_enabled",
		"provider_capacity_latest",
		"provider_quota_windows_by_snapshot",
		"provider_sessions_by_account_state",
		"provider_sessions_by_workspace",
	} {
		var got string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='index' AND name=?`, index).Scan(&got); err != nil {
			t.Fatalf("missing v14 index %s after upgrade: %v", index, err)
		}
	}
}

func TestProviderObservationSchemaRejectsInvalidCapacityAndForeignKeys(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_capacity_snapshots(account_id, health, active_runs, observed_at)
VALUES('missing-account','HEALTHY',0,'2026-08-28T12:00:00Z')`); err == nil {
		t.Fatal("capacity snapshot without provider account unexpectedly accepted")
	}

	if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_accounts(id, provider, name, state, created_at, updated_at)
VALUES('pacc_1','CODEX','default','ACTIVE','2026-08-28T12:00:00Z','2026-08-28T12:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	res, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_capacity_snapshots(account_id, health, active_runs, observed_at)
VALUES('pacc_1','HEALTHY',0,'2026-08-28T12:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	snapshotID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_quota_windows(snapshot_id, window_id, metric, remaining_fraction, observed_at, confidence)
VALUES(?, 'primary', 'FRACTION', 1.1, '2026-08-28T12:00:00Z', 1.0)`, snapshotID); err == nil {
		t.Fatal("out-of-range remaining_fraction unexpectedly accepted")
	}
}

func TestProviderSessionMayReferenceObservedModelBeforeCatalogRefresh(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_accounts(id, provider, state, created_at, updated_at)
VALUES('pacc_1','ANTIGRAVITY','ACTIVE','2026-08-28T12:00:00Z','2026-08-28T12:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_sessions(id, account_id, model_id, state, context_used, context_limit, last_used_at, observed_at)
VALUES('pses_1','pacc_1','model-not-yet-cataloged','ACTIVE',10,100,'2026-08-28T12:00:00Z','2026-08-28T12:00:00Z')`); err != nil {
		t.Fatalf("session should not require provider_models ingestion ordering: %v", err)
	}
	if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_sessions(id, account_id, model_id, state, context_used, context_limit, last_used_at, observed_at)
VALUES('pses_overflow','pacc_1','model-x','ACTIVE',101,100,'2026-08-28T12:00:00Z','2026-08-28T12:00:00Z')`); err == nil {
		t.Fatal("session context overflow unexpectedly accepted")
	}
}
