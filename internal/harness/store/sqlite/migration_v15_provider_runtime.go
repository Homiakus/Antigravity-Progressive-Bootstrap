package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 15,
		Name:    "provider_runtime_ledger",
		SQL: `
CREATE TABLE provider_assignments (
    id TEXT PRIMARY KEY,
    attempt_id TEXT NOT NULL,
    account_id TEXT NOT NULL,
    model_id TEXT NOT NULL CHECK(length(model_id) BETWEEN 1 AND 256),
    session_id TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK(state IN ('ACTIVE','COMPLETED','SUPERSEDED','RELEASED')),
    revision INTEGER NOT NULL CHECK(revision > 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(id, account_id),
    FOREIGN KEY(attempt_id) REFERENCES attempts(id) ON DELETE CASCADE,
    FOREIGN KEY(account_id) REFERENCES provider_accounts(id) ON DELETE RESTRICT
);

CREATE INDEX provider_assignments_by_attempt
    ON provider_assignments(attempt_id, created_at, id);
CREATE INDEX provider_assignments_by_account_state
    ON provider_assignments(account_id, state, updated_at, id);
CREATE UNIQUE INDEX provider_assignments_one_active_per_attempt
    ON provider_assignments(attempt_id) WHERE state='ACTIVE';

CREATE TABLE provider_reservations (
    id TEXT PRIMARY KEY,
    assignment_id TEXT NOT NULL,
    account_id TEXT NOT NULL,
    window_id TEXT NOT NULL CHECK(length(window_id) BETWEEN 1 AND 512),
    model_id TEXT NOT NULL DEFAULT '',
    metric TEXT NOT NULL CHECK(metric IN ('TOKENS','REQUESTS','COST','FRACTION')),
    amount REAL NOT NULL CHECK(amount > 0),
    state TEXT NOT NULL CHECK(state IN ('ACTIVE','SETTLED','RELEASED','EXPIRED')),
    revision INTEGER NOT NULL CHECK(revision > 0),
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    expires_at_ns INTEGER NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(id, assignment_id),
    FOREIGN KEY(assignment_id, account_id)
        REFERENCES provider_assignments(id, account_id) ON DELETE CASCADE
);

CREATE INDEX provider_reservations_by_account_state_expiry
    ON provider_reservations(account_id, state, expires_at_ns, id);
CREATE INDEX provider_reservations_by_assignment
    ON provider_reservations(assignment_id, state, id);
CREATE UNIQUE INDEX provider_reservations_one_active_dimension
    ON provider_reservations(assignment_id, window_id, model_id, metric)
    WHERE state='ACTIVE';

CREATE TABLE provider_usage_samples (
    sample_key TEXT PRIMARY KEY CHECK(length(sample_key) BETWEEN 1 AND 256),
    assignment_id TEXT NOT NULL,
    reservation_id TEXT,
    account_id TEXT NOT NULL,
    model_id TEXT NOT NULL DEFAULT '',
    metric TEXT NOT NULL CHECK(metric IN ('TOKENS','REQUESTS','COST','FRACTION')),
    amount REAL NOT NULL CHECK(amount >= 0),
    observed_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY(assignment_id, account_id)
        REFERENCES provider_assignments(id, account_id) ON DELETE CASCADE,
    FOREIGN KEY(reservation_id, assignment_id)
        REFERENCES provider_reservations(id, assignment_id) ON DELETE RESTRICT
);

CREATE INDEX provider_usage_samples_by_assignment
    ON provider_usage_samples(assignment_id, observed_at, sample_key);
CREATE INDEX provider_usage_samples_by_account_metric
    ON provider_usage_samples(account_id, metric, observed_at, sample_key);

CREATE TABLE provider_circuit_state (
    account_id TEXT NOT NULL,
    model_id TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL CHECK(revision > 0),
    state TEXT NOT NULL CHECK(state IN ('CLOSED','OPEN','HALF_OPEN')),
    consecutive_failures INTEGER NOT NULL DEFAULT 0 CHECK(consecutive_failures >= 0),
    opened_at TEXT,
    next_probe_at TEXT,
    probe_in_flight INTEGER NOT NULL DEFAULT 0 CHECK(probe_in_flight IN (0,1)),
    updated_at TEXT NOT NULL,
    PRIMARY KEY(account_id, model_id),
    FOREIGN KEY(account_id) REFERENCES provider_accounts(id) ON DELETE CASCADE
);

CREATE INDEX provider_circuit_by_state_probe
    ON provider_circuit_state(state, next_probe_at, account_id, model_id);
`,
	})
}
