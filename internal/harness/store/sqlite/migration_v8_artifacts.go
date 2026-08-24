package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 8,
		Name:    "durable_artifacts_and_provenance",
		SQL: `
CREATE TABLE artifacts (
    artifact_id TEXT PRIMARY KEY,
    workflow_run_id TEXT NOT NULL,
    producer_node_run_id TEXT NOT NULL DEFAULT '',
    producer_attempt_id TEXT NOT NULL DEFAULT '',
    content_digest TEXT NOT NULL,
    artifact_type TEXT NOT NULL,
    name TEXT NOT NULL,
    uri TEXT NOT NULL,
    size_bytes INTEGER NOT NULL CHECK(size_bytes >= 0),
    created_at TEXT NOT NULL,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE
);

CREATE INDEX artifacts_by_workflow
    ON artifacts(workflow_run_id, created_at, artifact_id);
CREATE INDEX artifacts_by_digest
    ON artifacts(content_digest, artifact_id);
CREATE INDEX artifacts_by_producer_node
    ON artifacts(producer_node_run_id, artifact_id);

CREATE TABLE artifact_provenance (
    artifact_id TEXT NOT NULL,
    node_run_id TEXT NOT NULL,
    relation TEXT NOT NULL CHECK(relation IN ('PRODUCED_BY','CONSUMED_BY')),
    created_at TEXT NOT NULL,
    PRIMARY KEY(artifact_id, node_run_id, relation),
    FOREIGN KEY(artifact_id) REFERENCES artifacts(artifact_id) ON DELETE CASCADE,
    FOREIGN KEY(node_run_id) REFERENCES node_runs(id) ON DELETE CASCADE
);

CREATE INDEX artifact_provenance_by_node
    ON artifact_provenance(node_run_id, relation, artifact_id);
`,
	})
}
