package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 10,
		Name:    "durable_workspaces_and_git_worktrees",
		SQL: `
CREATE TABLE workspaces (
    workspace_id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    state TEXT NOT NULL,
    base_path TEXT NOT NULL,
    repository_id TEXT NOT NULL DEFAULT '',
    branch TEXT NOT NULL DEFAULT '',
    owner_workflow_run_id TEXT NOT NULL DEFAULT '',
    owner_node_run_id TEXT NOT NULL DEFAULT '',
    owner_attempt_id TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    metadata_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX workspaces_by_owner
    ON workspaces(owner_workflow_run_id, owner_node_run_id);
CREATE INDEX workspaces_by_repo
    ON workspaces(repository_id, branch);
CREATE INDEX workspaces_by_state
    ON workspaces(state);
`,
	})
}
