package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 9,
		Name:    "durable_node_cache_and_partial_recomputation",
		SQL: `
CREATE TABLE node_cache_entries (
    cache_key TEXT PRIMARY KEY,
    workflow_run_id TEXT NOT NULL,
    node_run_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL DEFAULT '',
    output_artifacts_json TEXT NOT NULL DEFAULT '[]',
    result_payload_json TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    last_hit_at TEXT NOT NULL,
    hit_count INTEGER NOT NULL DEFAULT 0 CHECK(hit_count >= 0),
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE
);

CREATE INDEX node_cache_by_run
    ON node_cache_entries(workflow_run_id, created_at);
CREATE INDEX node_cache_by_last_hit
    ON node_cache_entries(last_hit_at);
`,
	})
}
