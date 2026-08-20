package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestSchedulerSchemaV3Exists(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range []string{"ready_queue", "workflow_schedule_state"} {
		var name string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
			t.Fatalf("missing scheduler table %s: %v", table, err)
		}
	}
	for _, column := range []string{"priority", "effective_priority"} {
		rows, err := db.SQLDB().QueryContext(ctx, `PRAGMA table_info(node_runs)`)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for rows.Next() {
			var cid int
			var name, typ string
			var notnull, pk int
			var dflt any
			if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			if name == column {
				found = true
			}
		}
		rows.Close()
		if !found {
			t.Fatalf("node_runs missing scheduler column %s", column)
		}
	}
}

func TestUpgradeFromVersionTwoAddsSchedulerProjection(t *testing.T) {
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
	if err := applyMigration(ctx, raw, migrations[1]); err != nil {
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
	var count int
	if err := db.SQLDB().QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version=3`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("migration v3 rows=%d want=1", count)
	}
}
