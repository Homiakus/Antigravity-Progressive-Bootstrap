package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 15,
		Name:    "remote_session_requests",
		SQL: `
CREATE TABLE remote_session_requests (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    source_message_id TEXT NOT NULL,
    repository_id TEXT NOT NULL,
    account_id TEXT NOT NULL,
    chat_id INTEGER NOT NULL,
    thread_id INTEGER NOT NULL DEFAULT 0,
    requester_user_id INTEGER NOT NULL,
    instance_strategy TEXT NOT NULL CHECK(instance_strategy IN ('AUTO','DEDICATED','EXISTING')),
    instance_id TEXT NOT NULL DEFAULT '',
    conversation_strategy TEXT NOT NULL CHECK(conversation_strategy IN ('NEW','EXISTING')),
    provider_conversation_id TEXT NOT NULL DEFAULT '',
    isolation_mode TEXT NOT NULL CHECK(isolation_mode IN ('SHARED_READ','EXCLUSIVE_WRITE','WORKTREE')),
    state TEXT NOT NULL CHECK(state IN ('PENDING','PROVISIONING','BINDING','SUCCEEDED','FAILED','CANCELLED')),
    session_id TEXT,
    requested_at TEXT NOT NULL,
    started_at TEXT,
    completed_at TEXT,
    error TEXT NOT NULL DEFAULT '',
    UNIQUE(source, source_message_id),
    CHECK(instance_strategy <> 'EXISTING' OR length(instance_id) > 0),
    CHECK(conversation_strategy <> 'EXISTING' OR length(provider_conversation_id) > 0),
    CHECK(state NOT IN ('BINDING','SUCCEEDED') OR session_id IS NOT NULL),
    FOREIGN KEY(repository_id) REFERENCES remote_repositories(id) ON DELETE RESTRICT,
    FOREIGN KEY(session_id) REFERENCES remote_sessions(id) ON DELETE RESTRICT
);

CREATE INDEX remote_session_requests_actionable
    ON remote_session_requests(state, requested_at, id);
CREATE INDEX remote_session_requests_topic
    ON remote_session_requests(chat_id, thread_id, requested_at DESC, id);
CREATE INDEX remote_session_requests_session
    ON remote_session_requests(session_id, id)
    WHERE session_id IS NOT NULL;
`,
	})
}
