package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 3,
		Name:    "incremental_scheduler_ready_queue",
		SQL: `
ALTER TABLE node_runs ADD COLUMN priority INTEGER NOT NULL DEFAULT 0;
ALTER TABLE node_runs ADD COLUMN effective_priority INTEGER NOT NULL DEFAULT 0;

CREATE TABLE ready_queue (
    node_run_id TEXT PRIMARY KEY,
    workflow_run_id TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    effective_priority INTEGER NOT NULL DEFAULT 0,
    ready_at TEXT NOT NULL,
    not_before TEXT,
    resource_class TEXT NOT NULL DEFAULT '',
    wait_reason TEXT NOT NULL DEFAULT '',
    wait_detail TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL,
    FOREIGN KEY(node_run_id) REFERENCES node_runs(id) ON DELETE CASCADE,
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE
);

CREATE INDEX ready_queue_due
    ON ready_queue(workflow_run_id, not_before, effective_priority DESC, priority DESC, ready_at, node_run_id);
CREATE INDEX ready_queue_global_due
    ON ready_queue(not_before, workflow_run_id, effective_priority DESC, priority DESC, ready_at);

CREATE TABLE workflow_schedule_state (
    workflow_run_id TEXT PRIMARY KEY,
    weight INTEGER NOT NULL DEFAULT 1 CHECK(weight > 0),
    service_count INTEGER NOT NULL DEFAULT 0 CHECK(service_count >= 0),
    last_selected_at TEXT,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE
);

CREATE INDEX workflow_schedule_fairness
    ON workflow_schedule_state(service_count, weight, last_selected_at, workflow_run_id);

INSERT INTO workflow_schedule_state(workflow_run_id, weight, service_count, last_selected_at, updated_at)
SELECT id, 1, 0, NULL, updated_at
FROM workflow_runs;

INSERT INTO ready_queue(node_run_id, workflow_run_id, priority, effective_priority, ready_at, not_before, resource_class, wait_reason, wait_detail, updated_at)
SELECT id, workflow_run_id, priority, effective_priority, updated_at, NULL, '', '', '', updated_at
FROM node_runs
WHERE state='READY';
`,
	})
}
