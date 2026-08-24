package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

const workspaceSelect = `
SELECT workspace_id, kind, state, base_path, repository_id, branch,
       owner_workflow_run_id, owner_node_run_id, owner_attempt_id,
       created_at, expires_at, metadata_json
FROM workspaces`

func (t *transaction) CreateWorkspace(ctx context.Context, ws harnessmodel.WorkspaceRecord) error {
	if err := ws.Validate(); err != nil {
		return err
	}
	metaJSON, err := json.Marshal(ws.Metadata)
	if err != nil {
		return fmt.Errorf("marshal workspace metadata: %w", err)
	}

	_, err = t.tx.ExecContext(ctx, `
INSERT INTO workspaces(
    workspace_id, kind, state, base_path, repository_id, branch,
    owner_workflow_run_id, owner_node_run_id, owner_attempt_id,
    created_at, expires_at, metadata_json
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		string(ws.ID), string(ws.Kind), string(ws.State), ws.BasePath, ws.RepositoryID, ws.Branch,
		string(ws.OwnerWorkflowRunID), string(ws.OwnerNodeRunID), string(ws.OwnerAttemptID),
		formatTime(ws.CreatedAt), formatTime(ws.ExpiresAt), string(metaJSON))
	if err != nil {
		return fmt.Errorf("create workspace %s: %w", ws.ID, err)
	}
	return nil
}

func (t *transaction) GetWorkspace(ctx context.Context, id harnessmodel.WorkspaceID) (harnessmodel.WorkspaceRecord, error) {
	if strings.TrimSpace(string(id)) == "" {
		return harnessmodel.WorkspaceRecord{}, fmt.Errorf("workspace id is required")
	}
	return scanWorkspace(t.tx.QueryRowContext(ctx, workspaceSelect+` WHERE workspace_id=?`, string(id)))
}

func (t *transaction) ListWorkspacesByOwner(ctx context.Context, runID harnessmodel.WorkflowRunID) ([]harnessmodel.WorkspaceRecord, error) {
	if runID == "" {
		return nil, fmt.Errorf("workflow run id is required")
	}
	rows, err := t.tx.QueryContext(ctx, workspaceSelect+` WHERE owner_workflow_run_id=? ORDER BY created_at`, string(runID))
	if err != nil {
		return nil, fmt.Errorf("list workspaces by owner %s: %w", runID, err)
	}
	defer rows.Close()

	var out []harnessmodel.WorkspaceRecord
	for rows.Next() {
		ws, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ws)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspaces: %w", err)
	}
	return out, nil
}

func (t *transaction) ListWorkspacesByRepo(ctx context.Context, repoID string) ([]harnessmodel.WorkspaceRecord, error) {
	if strings.TrimSpace(repoID) == "" {
		return nil, fmt.Errorf("repository id is required")
	}
	rows, err := t.tx.QueryContext(ctx, workspaceSelect+` WHERE repository_id=? ORDER BY created_at`, repoID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces by repo %s: %w", repoID, err)
	}
	defer rows.Close()

	var out []harnessmodel.WorkspaceRecord
	for rows.Next() {
		ws, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ws)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspaces: %w", err)
	}
	return out, nil
}

func (t *transaction) ListActiveWorkspaces(ctx context.Context) ([]harnessmodel.WorkspaceRecord, error) {
	rows, err := t.tx.QueryContext(ctx, workspaceSelect+` WHERE state IN ('ALLOCATED','ACTIVE') ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list active workspaces: %w", err)
	}
	defer rows.Close()

	var out []harnessmodel.WorkspaceRecord
	for rows.Next() {
		ws, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ws)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active workspaces: %w", err)
	}
	return out, nil
}

func (t *transaction) UpdateWorkspaceState(ctx context.Context, id harnessmodel.WorkspaceID, state harnessmodel.WorkspaceState, expiresAt time.Time) error {
	if strings.TrimSpace(string(id)) == "" || !state.Valid() {
		return fmt.Errorf("invalid workspace id or state")
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE workspaces
SET state=?, expires_at=?
WHERE workspace_id=?`, string(state), formatTime(expiresAt), string(id))
	if err != nil {
		return fmt.Errorf("update workspace state: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return harnessstore.ErrNotFound
	}
	return nil
}

func (t *transaction) DeleteWorkspace(ctx context.Context, id harnessmodel.WorkspaceID) error {
	if strings.TrimSpace(string(id)) == "" {
		return fmt.Errorf("workspace id is required")
	}
	res, err := t.tx.ExecContext(ctx, `DELETE FROM workspaces WHERE workspace_id=?`, string(id))
	if err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return harnessstore.ErrNotFound
	}
	return nil
}

func scanWorkspace(row interface{ Scan(...any) error }) (harnessmodel.WorkspaceRecord, error) {
	var id, kind, state, basePath, repoID, branch, ownerWfr, ownerNr, ownerAtt, created, expires, metaJSON string
	if err := row.Scan(&id, &kind, &state, &basePath, &repoID, &branch, &ownerWfr, &ownerNr, &ownerAtt, &created, &expires, &metaJSON); err != nil {
		return harnessmodel.WorkspaceRecord{}, mapNotFound(err)
	}
	cTime, err := parseTime(created)
	if err != nil {
		return harnessmodel.WorkspaceRecord{}, fmt.Errorf("parse workspace created_at: %w", err)
	}
	eTime, err := parseTime(expires)
	if err != nil {
		return harnessmodel.WorkspaceRecord{}, fmt.Errorf("parse workspace expires_at: %w", err)
	}
	var meta map[string]string
	if metaJSON != "" && metaJSON != "{}" {
		_ = json.Unmarshal([]byte(metaJSON), &meta)
	}

	return harnessmodel.WorkspaceRecord{
		ID:                 harnessmodel.WorkspaceID(id),
		Kind:               harnessmodel.WorkspaceKind(kind),
		State:              harnessmodel.WorkspaceState(state),
		BasePath:           basePath,
		RepositoryID:       repoID,
		Branch:             branch,
		OwnerWorkflowRunID: harnessmodel.WorkflowRunID(ownerWfr),
		OwnerNodeRunID:     harnessmodel.NodeRunID(ownerNr),
		OwnerAttemptID:     harnessmodel.AttemptID(ownerAtt),
		CreatedAt:          cTime,
		ExpiresAt:          eTime,
		Metadata:           meta,
	}, nil
}
