package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

func TestVersionElevenToTwelveCreatesTelegramRuntime(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	openFixtureAtVersion(t, path, 11)
	db, err := Open(ctx, path, Options{})
	if err != nil { t.Fatal(err) }
	defer db.Close()
	for _, table := range []string{"telegram_runtime", "telegram_principals", "telegram_pairings", "telegram_callback_replay"} {
		var got string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&got); err != nil {
			t.Fatalf("missing v12 table %s: %v", table, err)
		}
	}
}
