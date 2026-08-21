package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 5,
		Name:    "durable_retry_runtime_and_integer_deadlines",
		SQL: `
ALTER TABLE ready_queue ADD COLUMN not_before_ns INTEGER;
CREATE INDEX ready_queue_due_ns
    ON ready_queue(workflow_run_id, not_before_ns, effective_priority DESC, priority DESC, ready_at, node_run_id);
CREATE INDEX ready_queue_global_due_ns
    ON ready_queue(not_before_ns, workflow_run_id, effective_priority DESC, priority DESC, ready_at);

CREATE TABLE retry_schedule_history (
    failed_attempt_id TEXT PRIMARY KEY,
    node_run_id TEXT NOT NULL,
    workflow_run_id TEXT NOT NULL,
    attempt_number INTEGER NOT NULL CHECK(attempt_number > 0),
    failure_class TEXT NOT NULL,
    policy_ref TEXT NOT NULL DEFAULT '',
    service_key TEXT NOT NULL DEFAULT '',
    scheduled_at TEXT NOT NULL,
    not_before TEXT NOT NULL,
    not_before_ns INTEGER NOT NULL,
    FOREIGN KEY(node_run_id) REFERENCES node_runs(id) ON DELETE CASCADE,
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE,
    FOREIGN KEY(failed_attempt_id) REFERENCES attempts(id) ON DELETE CASCADE
);

CREATE INDEX retry_schedule_history_by_node
    ON retry_schedule_history(node_run_id, attempt_number);

CREATE TABLE retry_schedule (
    node_run_id TEXT PRIMARY KEY,
    workflow_run_id TEXT NOT NULL,
    failed_attempt_id TEXT NOT NULL UNIQUE,
    attempt_number INTEGER NOT NULL CHECK(attempt_number > 0),
    failure_class TEXT NOT NULL,
    policy_ref TEXT NOT NULL DEFAULT '',
    service_key TEXT NOT NULL DEFAULT '',
    scheduled_at TEXT NOT NULL,
    not_before TEXT NOT NULL,
    not_before_ns INTEGER NOT NULL,
    FOREIGN KEY(node_run_id) REFERENCES node_runs(id) ON DELETE CASCADE,
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE,
    FOREIGN KEY(failed_attempt_id) REFERENCES attempts(id) ON DELETE CASCADE
);

CREATE INDEX retry_schedule_due
    ON retry_schedule(not_before_ns, workflow_run_id, node_run_id);

CREATE TRIGGER retry_schedule_journal_insert
AFTER INSERT ON retry_schedule
BEGIN
    INSERT INTO retry_schedule_history(
        failed_attempt_id, node_run_id, workflow_run_id, attempt_number,
        failure_class, policy_ref, service_key, scheduled_at, not_before, not_before_ns
    ) VALUES(
        NEW.failed_attempt_id, NEW.node_run_id, NEW.workflow_run_id, NEW.attempt_number,
        NEW.failure_class, NEW.policy_ref, NEW.service_key, NEW.scheduled_at, NEW.not_before, NEW.not_before_ns
    );
END;

CREATE TABLE retry_budgets (
    scope TEXT NOT NULL,
    scope_key TEXT NOT NULL,
    window_start TEXT NOT NULL,
    window_start_ns INTEGER NOT NULL,
    window_ns INTEGER NOT NULL CHECK(window_ns > 0),
    limit_count INTEGER NOT NULL CHECK(limit_count > 0),
    used_count INTEGER NOT NULL DEFAULT 0 CHECK(used_count >= 0),
    updated_at TEXT NOT NULL,
    PRIMARY KEY(scope, scope_key),
    CHECK(used_count <= limit_count),
    CHECK(window_start_ns <= 9223372036854775807 - window_ns)
);

CREATE INDEX retry_budgets_window
    ON retry_budgets(scope, window_start_ns, updated_at);

CREATE TABLE circuit_breakers (
    service_key TEXT PRIMARY KEY,
    revision INTEGER NOT NULL DEFAULT 1 CHECK(revision > 0 AND revision <= 9223372036854775807),
    state TEXT NOT NULL,
    consecutive_failures INTEGER NOT NULL DEFAULT 0 CHECK(consecutive_failures >= 0),
    failure_threshold INTEGER NOT NULL CHECK(failure_threshold > 0),
    opened_at TEXT,
    next_probe_at TEXT,
    probe_in_flight INTEGER NOT NULL DEFAULT 0 CHECK(probe_in_flight IN (0,1)),
    updated_at TEXT NOT NULL
);

CREATE INDEX circuit_breakers_by_state_probe
    ON circuit_breakers(state, next_probe_at, service_key);
`,
	})
}
