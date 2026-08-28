package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 14,
		Name:    "provider_observation",
		SQL: `
CREATE TABLE provider_accounts (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL CHECK(length(provider) BETWEEN 1 AND 64),
    name TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK(state IN ('ACTIVE','DRAINING','DISABLED')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX provider_accounts_by_provider_state
    ON provider_accounts(provider, state, id);

CREATE TABLE provider_models (
    account_id TEXT NOT NULL,
    model_id TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    capabilities_json BLOB NOT NULL,
    context_limit INTEGER NOT NULL DEFAULT 0 CHECK(context_limit >= 0),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
    observed_at TEXT NOT NULL,
    PRIMARY KEY(account_id, model_id),
    FOREIGN KEY(account_id) REFERENCES provider_accounts(id) ON DELETE CASCADE
);

CREATE INDEX provider_models_by_account_enabled
    ON provider_models(account_id, enabled, model_id);

CREATE TABLE provider_capacity_snapshots (
    snapshot_id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id TEXT NOT NULL,
    health TEXT NOT NULL CHECK(health IN ('UNKNOWN','HEALTHY','DEGRADED','EXHAUSTED','UNAVAILABLE')),
    active_runs INTEGER NOT NULL DEFAULT 0 CHECK(active_runs >= 0),
    observed_at TEXT NOT NULL,
    FOREIGN KEY(account_id) REFERENCES provider_accounts(id) ON DELETE CASCADE
);

CREATE INDEX provider_capacity_latest
    ON provider_capacity_snapshots(account_id, observed_at DESC, snapshot_id DESC);

CREATE TABLE provider_quota_windows (
    snapshot_id INTEGER NOT NULL,
    window_id TEXT NOT NULL,
    model_id TEXT NOT NULL DEFAULT '',
    metric TEXT NOT NULL CHECK(metric IN ('TOKENS','REQUESTS','COST','FRACTION','OPAQUE')),
    limit_value REAL CHECK(limit_value IS NULL OR limit_value >= 0),
    remaining_value REAL CHECK(remaining_value IS NULL OR remaining_value >= 0),
    remaining_fraction REAL CHECK(remaining_fraction IS NULL OR (remaining_fraction >= 0 AND remaining_fraction <= 1)),
    reset_at TEXT,
    observed_at TEXT NOT NULL,
    confidence REAL NOT NULL CHECK(confidence >= 0 AND confidence <= 1),
    PRIMARY KEY(snapshot_id, window_id, model_id),
    FOREIGN KEY(snapshot_id) REFERENCES provider_capacity_snapshots(snapshot_id) ON DELETE CASCADE
);

CREATE INDEX provider_quota_windows_by_snapshot
    ON provider_quota_windows(snapshot_id, window_id, model_id);

CREATE TABLE provider_sessions (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    model_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('ACTIVE','DRAINING','EXHAUSTED','CLOSED')),
    context_used INTEGER NOT NULL DEFAULT 0 CHECK(context_used >= 0),
    context_limit INTEGER NOT NULL DEFAULT 0 CHECK(context_limit >= 0),
    last_used_at TEXT NOT NULL,
    workspace_fingerprint TEXT NOT NULL DEFAULT '',
    observed_at TEXT NOT NULL,
    CHECK(context_limit = 0 OR context_used <= context_limit),
    FOREIGN KEY(account_id) REFERENCES provider_accounts(id) ON DELETE CASCADE
);

CREATE INDEX provider_sessions_by_account_state
    ON provider_sessions(account_id, state, last_used_at DESC, id);

CREATE INDEX provider_sessions_by_workspace
    ON provider_sessions(account_id, workspace_fingerprint, state, last_used_at DESC, id);
`,
	})
}
