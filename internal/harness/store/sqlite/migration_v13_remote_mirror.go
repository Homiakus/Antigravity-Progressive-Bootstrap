package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 13,
		Name:    "remote_telegram_mirror_state",
		SQL: `
CREATE TABLE telegram_mirror_state (
    session_id TEXT PRIMARY KEY,
    chat_id INTEGER NOT NULL,
    thread_id INTEGER NOT NULL DEFAULT 0,
    stream_key TEXT NOT NULL DEFAULT '',
    message_id INTEGER NOT NULL DEFAULT 0 CHECK(message_id >= 0),
    last_event_seq INTEGER NOT NULL DEFAULT 0 CHECK(last_event_seq >= 0),
    rendered_text TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL,
    FOREIGN KEY(session_id) REFERENCES remote_sessions(id) ON DELETE CASCADE
);

CREATE INDEX telegram_mirror_state_chat
    ON telegram_mirror_state(chat_id, thread_id, session_id);
`,
	})
}
