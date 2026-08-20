package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 4,
		Name:    "durable_workers_leases_fencing",
		SQL: `
CREATE TABLE workers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL,
    trust TEXT NOT NULL,
    capabilities_json BLOB NOT NULL,
    resources_json BLOB NOT NULL,
    created_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL
);

CREATE INDEX workers_by_state_seen
    ON workers(state, last_seen_at, id);

CREATE TABLE leases (
    id TEXT PRIMARY KEY,
    attempt_id TEXT NOT NULL,
    worker_id TEXT NOT NULL,
    epoch INTEGER NOT NULL CHECK(epoch > 0),
    state TEXT NOT NULL,
    claimed_at TEXT NOT NULL,
    heartbeat_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    closed_at TEXT,
    UNIQUE(attempt_id, epoch),
    FOREIGN KEY(attempt_id) REFERENCES attempts(id) ON DELETE CASCADE,
    FOREIGN KEY(worker_id) REFERENCES workers(id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX leases_one_active_per_attempt
    ON leases(attempt_id)
    WHERE state='ACTIVE';

CREATE INDEX leases_active_expiry
    ON leases(state, expires_at, attempt_id);
CREATE INDEX leases_by_worker
    ON leases(worker_id, state, expires_at);
`,
	})
}
