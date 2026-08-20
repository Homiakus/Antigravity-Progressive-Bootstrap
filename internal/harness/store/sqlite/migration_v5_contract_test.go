package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

func TestVersionFivePinsRetryAndCircuitFenceColumns(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	openFixtureAtVersion(t, path, 4)
	db, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	assertColumn := func(table, column string) {
		t.Helper()
		rows, err := db.SQLDB().QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		found := false
		for rows.Next() {
			var cid int
			var name, typ string
			var notNull int
			var defaultValue any
			var pk int
			if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
				t.Fatal(err)
			}
			if name == column {
				found = true
			}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if !found {
			t.Fatalf("v5 schema missing %s.%s", table, column)
		}
	}
	assertObject := func(kind, name string) {
		t.Helper()
		var got string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type=? AND name=?`, kind, name).Scan(&got); err != nil {
			t.Fatalf("v5 schema missing %s %s: %v", kind, name, err)
		}
	}

	assertColumn("ready_queue", "not_before_ns")
	assertColumn("retry_schedule", "not_before_ns")
	assertColumn("retry_schedule_history", "not_before_ns")
	assertColumn("retry_budgets", "window_start_ns")
	assertColumn("circuit_breakers", "revision")
	assertObject("table", "retry_schedule_history")
	assertObject("trigger", "retry_schedule_journal_insert")
}
