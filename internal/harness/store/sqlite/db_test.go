package sqlite

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildDSNAppliesPragmasPerConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "with space", "state.db")
	dsn := buildDSN(path, Options{BusyTimeout: 7 * time.Second})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	values := u.Query()["_pragma"]
	want := []string{"busy_timeout(7000)", "foreign_keys(ON)", "journal_mode(WAL)", "synchronous(FULL)"}
	if strings.Join(values, "|") != strings.Join(want, "|") {
		t.Fatalf("pragmas = %#v, want %#v", values, want)
	}
}

func TestOpenAppliesDurabilityPragmas(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p, err := db.Pragmas(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if p.JournalMode != "wal" || !p.ForeignKeys || p.BusyTimeoutMS != 5000 || p.Synchronous != synchronousFull {
		t.Fatalf("unexpected pragmas: %+v", p)
	}
}

func TestOpenReopenExistingDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	first, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.SQLDB().ExecContext(ctx, `CREATE TABLE durable_probe(id INTEGER PRIMARY KEY, value TEXT NOT NULL); INSERT INTO durable_probe(id,value) VALUES(1,'committed')`); err != nil {
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
	var got string
	if err := second.SQLDB().QueryRowContext(ctx, `SELECT value FROM durable_probe WHERE id=1`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != "committed" {
		t.Fatalf("value=%q", got)
	}
}

func TestForeignKeysAreActuallyEnforced(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.SQLDB().ExecContext(ctx, `CREATE TABLE parent(id INTEGER PRIMARY KEY); CREATE TABLE child(id INTEGER PRIMARY KEY, parent_id INTEGER NOT NULL REFERENCES parent(id))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQLDB().ExecContext(ctx, `INSERT INTO child(id,parent_id) VALUES(1,999)`); err == nil {
		t.Fatal("foreign-key violation unexpectedly succeeded")
	}
}
