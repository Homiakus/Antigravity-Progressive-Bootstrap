package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 6,
		Name:    "durable_timers_signals_and_approvals",
		SQL: `
CREATE TABLE timers (
    timer_id TEXT PRIMARY KEY,
    workflow_run_id TEXT NOT NULL,
    node_run_id TEXT,
    kind TEXT NOT NULL,
    payload BLOB NOT NULL DEFAULT X'',
    state TEXT NOT NULL CHECK(state IN ('PENDING','FIRED','CANCELLED')),
    due_at TEXT NOT NULL,
    due_at_ns INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    resolved_at TEXT,
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE,
    FOREIGN KEY(node_run_id) REFERENCES node_runs(id) ON DELETE CASCADE
);

CREATE INDEX timers_due
    ON timers(state, due_at_ns, workflow_run_id, timer_id);
CREATE INDEX timers_by_node
    ON timers(node_run_id, state, due_at_ns);

CREATE TABLE signals (
    signal_id TEXT PRIMARY KEY,
    workflow_run_id TEXT NOT NULL,
    signal_name TEXT NOT NULL,
    message_id TEXT NOT NULL,
    payload BLOB NOT NULL DEFAULT X'',
    state TEXT NOT NULL CHECK(state IN ('PENDING','CONSUMED')),
    received_at TEXT NOT NULL,
    received_at_ns INTEGER NOT NULL,
    consumed_by_node_run_id TEXT,
    consumed_at TEXT,
    UNIQUE(workflow_run_id, signal_name, message_id),
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE,
    FOREIGN KEY(consumed_by_node_run_id) REFERENCES node_runs(id) ON DELETE RESTRICT,
    CHECK(
        (state='PENDING' AND consumed_by_node_run_id IS NULL AND consumed_at IS NULL)
        OR
        (state='CONSUMED' AND consumed_by_node_run_id IS NOT NULL AND consumed_at IS NOT NULL)
    )
);

CREATE INDEX signals_pending
    ON signals(workflow_run_id, signal_name, state, received_at_ns, signal_id);

CREATE TABLE signal_waits (
    node_run_id TEXT PRIMARY KEY,
    workflow_run_id TEXT NOT NULL,
    signal_name TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('WAITING','DELIVERED','CANCELLED','TIMED_OUT')),
    created_at TEXT NOT NULL,
    delivered_signal_id TEXT UNIQUE,
    resolved_at TEXT,
    FOREIGN KEY(node_run_id) REFERENCES node_runs(id) ON DELETE CASCADE,
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE,
    FOREIGN KEY(delivered_signal_id) REFERENCES signals(signal_id) ON DELETE RESTRICT,
    CHECK(
        (state='WAITING' AND delivered_signal_id IS NULL AND resolved_at IS NULL)
        OR
        (state='DELIVERED' AND delivered_signal_id IS NOT NULL AND resolved_at IS NOT NULL)
        OR
        (state IN ('CANCELLED','TIMED_OUT') AND delivered_signal_id IS NULL AND resolved_at IS NOT NULL)
    )
);

CREATE INDEX signal_waits_pending
    ON signal_waits(workflow_run_id, signal_name, state, created_at, node_run_id);

CREATE TABLE approvals (
    approval_id TEXT PRIMARY KEY,
    workflow_run_id TEXT NOT NULL,
    node_run_id TEXT NOT NULL,
    requested_capability TEXT NOT NULL,
    risk TEXT NOT NULL,
    reason TEXT NOT NULL,
    requested_at TEXT NOT NULL,
    expires_at TEXT,
    expires_at_ns INTEGER,
    state TEXT NOT NULL CHECK(state IN ('PENDING','APPROVED','REJECTED','EXPIRED','CANCELLED')),
    actor TEXT NOT NULL DEFAULT '',
    resolved_at TEXT,
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE,
    FOREIGN KEY(node_run_id) REFERENCES node_runs(id) ON DELETE CASCADE,
    CHECK((expires_at IS NULL) = (expires_at_ns IS NULL)),
    CHECK(
        (state='PENDING' AND actor='' AND resolved_at IS NULL)
        OR
        (state IN ('APPROVED','REJECTED') AND actor<>'' AND resolved_at IS NOT NULL)
        OR
        (state IN ('EXPIRED','CANCELLED') AND resolved_at IS NOT NULL)
    )
);

CREATE INDEX approvals_pending
    ON approvals(state, expires_at_ns, requested_at, approval_id);
CREATE INDEX approvals_by_workflow
    ON approvals(workflow_run_id, state, requested_at, approval_id);
`,
	})
}
