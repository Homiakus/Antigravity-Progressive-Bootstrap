package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 11,
		Name:    "remote_control_core",
		SQL: `
CREATE TABLE remote_repositories (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    canonical_path TEXT NOT NULL UNIQUE,
    git_root TEXT NOT NULL,
    git_remote TEXT NOT NULL DEFAULT '',
    default_branch TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
    created_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL
);

CREATE INDEX remote_repositories_enabled_name
    ON remote_repositories(enabled, name COLLATE NOCASE, id);

CREATE TABLE remote_instances (
    cockpit_instance_id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    user_data_dir TEXT NOT NULL,
    working_dir TEXT NOT NULL DEFAULT '',
    account_id TEXT NOT NULL DEFAULT '',
    pid INTEGER NOT NULL DEFAULT 0 CHECK(pid >= 0),
    desired_state TEXT NOT NULL,
    observed_state TEXT NOT NULL,
    bridge_id TEXT NOT NULL DEFAULT '',
    last_reconciled_at TEXT NOT NULL,
    last_error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX remote_instances_observed_state
    ON remote_instances(observed_state, cockpit_instance_id);
CREATE INDEX remote_instances_account
    ON remote_instances(account_id, observed_state, cockpit_instance_id);

CREATE TABLE remote_conversations (
    id TEXT PRIMARY KEY,
    provider_conversation_id TEXT NOT NULL,
    cockpit_instance_id TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL,
    mirror_mode TEXT NOT NULL,
    last_activity_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(cockpit_instance_id, provider_conversation_id),
    FOREIGN KEY(cockpit_instance_id) REFERENCES remote_instances(cockpit_instance_id) ON DELETE RESTRICT
);

CREATE INDEX remote_conversations_instance_activity
    ON remote_conversations(cockpit_instance_id, last_activity_at DESC, id);
CREATE INDEX remote_conversations_workspace
    ON remote_conversations(workspace_id, state, id);

CREATE TABLE remote_sessions (
    id TEXT PRIMARY KEY,
    host_id TEXT NOT NULL,
    cockpit_instance_id TEXT NOT NULL,
    cockpit_account_id TEXT NOT NULL DEFAULT '',
    repository_id TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    workspace_path TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    workflow_run_id TEXT,
    desired_state TEXT NOT NULL,
    observed_state TEXT NOT NULL,
    isolation_mode TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(cockpit_instance_id) REFERENCES remote_instances(cockpit_instance_id) ON DELETE RESTRICT,
    FOREIGN KEY(repository_id) REFERENCES remote_repositories(id) ON DELETE RESTRICT,
    FOREIGN KEY(conversation_id) REFERENCES remote_conversations(id) ON DELETE RESTRICT,
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id) ON DELETE SET NULL
);

CREATE INDEX remote_sessions_instance_state
    ON remote_sessions(cockpit_instance_id, observed_state, id);
CREATE INDEX remote_sessions_repository_state
    ON remote_sessions(repository_id, observed_state, id);
CREATE INDEX remote_sessions_workspace_state
    ON remote_sessions(workspace_id, observed_state, id);
CREATE INDEX remote_sessions_workflow
    ON remote_sessions(workflow_run_id, id);

CREATE TABLE telegram_bindings (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    chat_id INTEGER NOT NULL,
    thread_id INTEGER NOT NULL DEFAULT 0,
    owner_user_id INTEGER NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
    created_at TEXT NOT NULL,
    FOREIGN KEY(session_id) REFERENCES remote_sessions(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX telegram_bindings_active_topic
    ON telegram_bindings(chat_id, thread_id)
    WHERE enabled=1;
CREATE INDEX telegram_bindings_session
    ON telegram_bindings(session_id, enabled, id);

CREATE TABLE remote_commands (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    source_message_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    payload BLOB NOT NULL,
    state TEXT NOT NULL,
    requested_at TEXT NOT NULL,
    started_at TEXT,
    completed_at TEXT,
    error TEXT NOT NULL DEFAULT '',
    UNIQUE(source, source_message_id),
    FOREIGN KEY(session_id) REFERENCES remote_sessions(id) ON DELETE CASCADE
);

CREATE INDEX remote_commands_session_state
    ON remote_commands(session_id, state, requested_at, id);

CREATE TABLE remote_events (
    event_id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    session_seq INTEGER NOT NULL CHECK(session_seq > 0),
    source TEXT NOT NULL,
    type TEXT NOT NULL,
    source_event_id TEXT NOT NULL DEFAULT '',
    payload BLOB NOT NULL,
    timestamp TEXT NOT NULL,
    UNIQUE(session_id, session_seq),
    FOREIGN KEY(session_id) REFERENCES remote_sessions(id) ON DELETE CASCADE
);

CREATE INDEX remote_events_session_seq
    ON remote_events(session_id, session_seq);
CREATE INDEX remote_events_type_time
    ON remote_events(type, timestamp, event_id);
CREATE UNIQUE INDEX remote_events_source_dedupe
    ON remote_events(source, source_event_id)
    WHERE source_event_id <> '';

CREATE TABLE remote_outbox (
    outbox_id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL,
    transport TEXT NOT NULL,
    payload BLOB NOT NULL,
    created_at TEXT NOT NULL,
    delivered_at TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK(attempt_count >= 0),
    next_attempt_at TEXT,
    UNIQUE(event_id, transport),
    FOREIGN KEY(event_id) REFERENCES remote_events(event_id) ON DELETE CASCADE
);

CREATE INDEX remote_outbox_pending
    ON remote_outbox(transport, delivered_at, next_attempt_at, outbox_id);
`,
	})
}
