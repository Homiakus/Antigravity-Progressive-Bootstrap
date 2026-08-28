package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

func TestVersionTwelveToThirteenCreatesMirrorState(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	openFixtureAtVersion(t, path, 12)
	db, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var name string
	if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='telegram_mirror_state'`).Scan(&name); err != nil {
		t.Fatalf("missing telegram_mirror_state: %v", err)
	}
}
