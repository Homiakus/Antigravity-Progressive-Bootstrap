package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestLeaseSchemaV4Exists(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range []string{"workers", "leases"} {
		var name string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
			t.Fatalf("missing lease runtime table %s: %v", table, err)
		}
	}
}

func TestUpgradeFromVersionThreeAddsWorkersAndLeases(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	raw, err := sql.Open("sqlite", buildDSN(path, Options{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureMigrationTable(ctx, raw); err != nil {
		t.Fatal(err)
	}
	for version := 1; version <= 3; version++ {
		m, ok := migrationByVersion(version)
		if !ok {
			t.Fatalf("missing migration v%d", version)
		}
		if err := applyMigration(ctx, raw, m); err != nil {
			t.Fatal(err)
		}
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
	for _, table := range []string{"workers", "leases"} {
		var name string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
			t.Fatalf("v3->v4 missing %s: %v", table, err)
		}
	}
}
