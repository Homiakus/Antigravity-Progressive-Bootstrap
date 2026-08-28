package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 12,
		Name:    "telegram_remote_runtime",
		SQL: `
CREATE TABLE telegram_runtime (
    bot_key TEXT PRIMARY KEY,
    next_update_id INTEGER NOT NULL DEFAULT 0 CHECK(next_update_id >= 0),
    updated_at TEXT NOT NULL
);

CREATE TABLE telegram_principals (
    user_id INTEGER PRIMARY KEY,
    role TEXT NOT NULL CHECK(role IN ('OWNER','OPERATOR','VIEWER')),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
    paired_at TEXT NOT NULL
);

CREATE INDEX telegram_principals_role_enabled
    ON telegram_principals(role, enabled, user_id);

CREATE TABLE telegram_pairings (
    token_hash TEXT PRIMARY KEY,
    role TEXT NOT NULL CHECK(role IN ('OWNER','OPERATOR','VIEWER')),
    intended_chat_id INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    consumed_at TEXT,
    consumed_by_user_id INTEGER,
    consumed_chat_id INTEGER
);

CREATE INDEX telegram_pairings_expiry
    ON telegram_pairings(consumed_at, expires_at, token_hash);

CREATE TABLE telegram_callback_replay (
    callback_query_id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    chat_id INTEGER NOT NULL,
    received_at TEXT NOT NULL
);

CREATE INDEX telegram_callback_replay_received
    ON telegram_callback_replay(received_at, callback_query_id);
`,
	})
}
