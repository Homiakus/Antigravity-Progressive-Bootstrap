package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestFreshDatabaseCreatesCurrentSchema(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{})
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
	for _, table := range []string{"schema_migrations", "workflow_definitions", "workflow_runs", "graph_revisions", "nodes", "dependencies", "node_runs", "attempts", "events", "outbox", "workflow_progress"} {
		var name string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
			t.Fatalf("missing table %s: %v", table, err)
		}
	}
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
	for version := 1; version <= SchemaVersion; version++ {
		var count int
		if err := second.SQLDB().QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version=?`, version).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("migration v%d rows=%d want=1", version, count)
		}
	}
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

func TestUpgradeFromVersionZeroFixture(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	raw, err := sql.Open("sqlite", buildDSN(path, Options{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureMigrationTable(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
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
}

func TestUpgradeFromVersionOneFixture(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	raw, err := sql.Open("sqlite", buildDSN(path, Options{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureMigrationTable(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if err := applyMigration(ctx, raw, migrations[0]); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

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
		t.Fatalf("upgraded schema version=%d want=%d", version, SchemaVersion)
	}
	var name string
	if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='workflow_progress'`).Scan(&name); err != nil {
		t.Fatalf("v1->v2 migration missing workflow_progress: %v", err)
	}
}
