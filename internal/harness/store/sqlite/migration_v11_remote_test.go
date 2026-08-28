package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

func TestVersionTenToElevenCreatesRemoteControlCore(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	openFixtureAtVersion(t, path, 10)
	db, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, table := range []string{
		"remote_repositories", "remote_instances", "remote_conversations", "remote_sessions",
		"telegram_bindings", "remote_commands", "remote_events", "remote_outbox",
	} {
		var got string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&got); err != nil {
			t.Fatalf("missing v11 table %s: %v", table, err)
		}
	}
	for _, index := range []string{
		"remote_repositories_enabled_name", "remote_instances_observed_state", "remote_conversations_instance_activity",
		"remote_sessions_instance_state", "telegram_bindings_active_topic", "remote_commands_session_state",
		"remote_events_source_dedupe", "remote_outbox_pending",
	} {
		var got string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='index' AND name=?`, index).Scan(&got); err != nil {
			t.Fatalf("missing v11 index %s: %v", index, err)
		}
	}
}
