package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

var currentHarnessTables = []string{
	"schema_migrations", "workflow_definitions", "workflow_runs", "graph_revisions",
	"nodes", "dependencies", "node_runs", "attempts", "events", "outbox",
	"workflow_progress", "ready_queue", "workflow_schedule_state", "workers", "leases",
}

func assertCurrentSchema(t *testing.T, ctx context.Context, db *DB) {
	t.Helper()
	version, err := db.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != SchemaVersion {
		t.Fatalf("schema version=%d want=%d", version, SchemaVersion)
	}
	for _, table := range currentHarnessTables {
		var name string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
			t.Fatalf("missing table %s: %v", table, err)
		}
	}
	for version := 1; version <= SchemaVersion; version++ {
		var count int
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version=?`, version).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("migration v%d rows=%d want=1", version, count)
		}
	}
}

func TestFreshDatabaseCreatesCurrentSchema(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	assertCurrentSchema(t, ctx, db)
}

func TestMigrationReplayPrevention(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	first, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	assertCurrentSchema(t, ctx, second)
}

func TestMigrationChecksumDetectsReleasedSQLMutation(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	db, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQLDB().ExecContext(ctx, `UPDATE schema_migrations SET checksum='tampered' WHERE version=1`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, path, Options{}); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable migration error, got %v", err)
	}
}

func TestMigrationRejectsFutureSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	db, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQLDB().ExecContext(ctx, `INSERT INTO schema_migrations(version,name,checksum,applied_at) VALUES(999,'future','x','now')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, path, Options{}); err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("expected future schema rejection, got %v", err)
	}
}

func openFixtureAtVersion(t *testing.T, path string, version int) {
	t.Helper()
	ctx := context.Background()
	raw, err := sql.Open("sqlite", buildDSN(path, Options{}))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if err := ensureMigrationTable(ctx, raw); err != nil {
		t.Fatal(err)
	}
	for v := 1; v <= version; v++ {
		m, ok := migrationByVersion(v)
		if !ok {
			t.Fatalf("missing migration v%d", v)
		}
		if err := applyMigration(ctx, raw, m); err != nil {
			t.Fatalf("apply fixture migration v%d: %v", v, err)
		}
	}
}

func TestUpgradeFromHistoricalFixtures(t *testing.T) {
	for _, fromVersion := range []int{0, 1, 2, 3} {
		t.Run("v"+string(rune('0'+fromVersion)), func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "state.db")
			openFixtureAtVersion(t, path, fromVersion)
			db, err := Open(ctx, path, Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			assertCurrentSchema(t, ctx, db)
		})
	}
}

func TestVersionThreeToFourCreatesWorkerLeaseConstraints(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	openFixtureAtVersion(t, path, 3)
	db, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, name := range []string{"workers_by_state_seen", "leases_one_active_per_attempt", "leases_active_expiry", "leases_by_worker"} {
		var got string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='index' AND name=?`, name).Scan(&got); err != nil {
			t.Fatalf("missing v4 index %s: %v", name, err)
		}
	}
}
