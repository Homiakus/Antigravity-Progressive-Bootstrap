package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func (t *transaction) CreateWorkflowScheduleState(ctx context.Context, s harnessmodel.WorkflowScheduleState) error {
	if s.WorkflowRunID == "" || s.Weight <= 0 || s.ServiceCount < 0 || s.UpdatedAt.IsZero() {
		return fmt.Errorf("invalid workflow schedule state")
	}
	_, err := t.tx.ExecContext(ctx, `
INSERT INTO workflow_schedule_state(workflow_run_id, weight, service_count, last_selected_at, updated_at)
VALUES(?, ?, ?, ?, ?)`, string(s.WorkflowRunID), s.Weight, s.ServiceCount, nullableTime(s.LastSelectedAt), formatTime(s.UpdatedAt))
	if err != nil {
		return fmt.Errorf("insert workflow schedule state: %w", err)
	}
	return nil
}

func (t *transaction) EnqueueReadyNode(ctx context.Context, nodeRunID harnessmodel.NodeRunID, readyAt, notBefore time.Time, resourceClass string) error {
	if nodeRunID == "" || readyAt.IsZero() {
		return fmt.Errorf("node run id and ready time are required")
	}
	res, err := t.tx.ExecContext(ctx, `
INSERT INTO ready_queue(
    node_run_id, workflow_run_id, priority, effective_priority,
    ready_at, not_before, resource_class, wait_reason, wait_detail, updated_at
)
SELECT nr.id, nr.workflow_run_id, nr.priority, nr.effective_priority,
       ?, ?, ?, '', '', ?
FROM node_runs nr
WHERE nr.id=? AND nr.state=?
ON CONFLICT(node_run_id) DO UPDATE SET
    workflow_run_id=excluded.workflow_run_id,
    priority=excluded.priority,
    effective_priority=excluded.effective_priority,
    ready_at=excluded.ready_at,
    not_before=excluded.not_before,
    resource_class=excluded.resource_class,
    wait_reason='',
    wait_detail='',
    updated_at=excluded.updated_at`,
		formatTime(readyAt), nullableTime(notBefore), resourceClass, formatTime(readyAt), string(nodeRunID), string(harnessmodel.NodeReady))
	if err != nil {
		return fmt.Errorf("enqueue ready node: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read ready enqueue count: %w", err)
	}
	if n != 1 {
		return harnessstore.ErrConflict
	}
	return nil
}

func (t *transaction) RemoveReadyNode(ctx context.Context, nodeRunID harnessmodel.NodeRunID) error {
	if nodeRunID == "" {
		return fmt.Errorf("node run id is required")
	}
	if _, err := t.tx.ExecContext(ctx, `DELETE FROM ready_queue WHERE node_run_id=?`, string(nodeRunID)); err != nil {
		return fmt.Errorf("remove ready node: %w", err)
	}
	return nil
}

func (t *transaction) SetReadyWait(ctx context.Context, nodeRunID harnessmodel.NodeRunID, reason harnessmodel.WaitReason, detail string, updatedAt time.Time) error {
	if nodeRunID == "" || updatedAt.IsZero() {
		return fmt.Errorf("node run id and updated time are required")
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE ready_queue SET wait_reason=?, wait_detail=?, updated_at=? WHERE node_run_id=?`, string(reason), detail, formatTime(updatedAt), string(nodeRunID))
	if err != nil {
		return fmt.Errorf("set ready wait reason: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read wait update count: %w", err)
	}
	if n != 1 {
		return harnessstore.ErrNotFound
	}
	return nil
}

func (t *transaction) RecordWorkflowService(ctx context.Context, runID harnessmodel.WorkflowRunID, selectedAt time.Time) error {
	if runID == "" || selectedAt.IsZero() {
		return fmt.Errorf("workflow run id and selection time are required")
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE workflow_schedule_state
SET service_count=service_count+1, last_selected_at=?, updated_at=?
WHERE workflow_run_id=?`, formatTime(selectedAt), formatTime(selectedAt), string(runID))
	if err != nil {
		return fmt.Errorf("record workflow scheduler service: %w", err)
	}
	return requireOneAffected(res)
}

func (t *transaction) GetReadyNode(ctx context.Context, nodeRunID harnessmodel.NodeRunID) (harnessmodel.ReadyNode, error) {
	return scanReadyNode(t.tx.QueryRowContext(ctx, readyNodeSelect+`
WHERE rq.node_run_id=? AND nr.state=?`, string(nodeRunID), string(harnessmodel.NodeReady)))
}

func (t *transaction) ListReadyWorkflowLanes(ctx context.Context, now time.Time, limit int) ([]harnessmodel.WorkflowScheduleState, error) {
	if limit <= 0 || limit > 10000 {
		limit = 256
	}
	rows, err := t.tx.QueryContext(ctx, `
SELECT ws.workflow_run_id, ws.weight, ws.service_count, ws.last_selected_at, ws.updated_at
FROM workflow_schedule_state ws
JOIN workflow_runs wr ON wr.id=ws.workflow_run_id
WHERE wr.state=?
  AND EXISTS (
      SELECT 1
      FROM ready_queue rq
      JOIN node_runs nr ON nr.id=rq.node_run_id
      WHERE rq.workflow_run_id=ws.workflow_run_id
        AND nr.state=?
        AND (rq.not_before IS NULL OR rq.not_before<=?)
  )
ORDER BY (CAST(ws.service_count AS REAL) / CAST(ws.weight AS REAL)) ASC,
         CASE WHEN ws.last_selected_at IS NULL THEN 0 ELSE 1 END ASC,
         ws.last_selected_at ASC,
         ws.workflow_run_id ASC
LIMIT ?`, string(harnessmodel.WorkflowRunning), string(harnessmodel.NodeReady), formatTime(now), limit)
	if err != nil {
		return nil, fmt.Errorf("list ready workflow lanes: %w", err)
	}
	defer rows.Close()
	out := make([]harnessmodel.WorkflowScheduleState, 0)
	for rows.Next() {
		var s harnessmodel.WorkflowScheduleState
		var runID, updated string
		var selected sql.NullString
		if err := rows.Scan(&runID, &s.Weight, &s.ServiceCount, &selected, &updated); err != nil {
			return nil, fmt.Errorf("scan ready workflow lane: %w", err)
		}
		s.WorkflowRunID = harnessmodel.WorkflowRunID(runID)
		var err error
		if s.UpdatedAt, err = parseTime(updated); err != nil {
			return nil, fmt.Errorf("parse workflow schedule updated_at: %w", err)
		}
		if selected.Valid && selected.String != "" {
			if s.LastSelectedAt, err = parseTime(selected.String); err != nil {
				return nil, fmt.Errorf("parse workflow last_selected_at: %w", err)
			}
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ready workflow lanes: %w", err)
	}
	return out, nil
}

const readyNodeSelect = `
SELECT rq.node_run_id, rq.workflow_run_id, nr.node_id,
       rq.priority, rq.effective_priority, rq.ready_at, rq.not_before,
       rq.resource_class, rq.wait_reason, rq.wait_detail, n.spec_json
FROM ready_queue rq
JOIN node_runs nr ON nr.id=rq.node_run_id
JOIN nodes n
  ON n.definition_id=nr.definition_id
 AND n.definition_version=nr.definition_version
 AND n.node_id=nr.node_id
`

func (t *transaction) ListReadyNodes(ctx context.Context, runID harnessmodel.WorkflowRunID, now time.Time, limit int) ([]harnessmodel.ReadyNode, error) {
	if limit <= 0 || limit > 10000 {
		limit = 64
	}
	rows, err := t.tx.QueryContext(ctx, readyNodeSelect+`
WHERE rq.workflow_run_id=?
  AND nr.state=?
  AND (rq.not_before IS NULL OR rq.not_before<=?)
ORDER BY rq.effective_priority DESC, rq.priority DESC, rq.ready_at ASC, rq.node_run_id ASC
LIMIT ?`, string(runID), string(harnessmodel.NodeReady), formatTime(now), limit)
	if err != nil {
		return nil, fmt.Errorf("list ready nodes: %w", err)
	}
	defer rows.Close()
	out := make([]harnessmodel.ReadyNode, 0)
	for rows.Next() {
		node, err := scanReadyNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ready nodes: %w", err)
	}
	return out, nil
}

func scanReadyNode(row interface{ Scan(...any) error }) (harnessmodel.ReadyNode, error) {
	var node harnessmodel.ReadyNode
	var nodeRunID, runID, nodeID, readyAt string
	var notBefore sql.NullString
	var reason string
	var specJSON []byte
	if err := row.Scan(&nodeRunID, &runID, &nodeID, &node.Priority, &node.EffectivePriority, &readyAt, &notBefore, &node.ResourceClass, &reason, &node.WaitDetail, &specJSON); err != nil {
		return harnessmodel.ReadyNode{}, mapNotFound(err)
	}
	node.NodeRunID = harnessmodel.NodeRunID(nodeRunID)
	node.WorkflowRunID = harnessmodel.WorkflowRunID(runID)
	node.NodeID = harnessmodel.NodeID(nodeID)
	node.WaitReason = harnessmodel.WaitReason(reason)
	var err error
	if node.ReadyAt, err = parseTime(readyAt); err != nil {
		return harnessmodel.ReadyNode{}, fmt.Errorf("parse ready_at: %w", err)
	}
	if notBefore.Valid && notBefore.String != "" {
		if node.NotBefore, err = parseTime(notBefore.String); err != nil {
			return harnessmodel.ReadyNode{}, fmt.Errorf("parse not_before: %w", err)
		}
	}
	var spec harnessmodel.NodeSpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		return harnessmodel.ReadyNode{}, fmt.Errorf("decode ready node spec: %w", err)
	}
	node.Resources = spec.Resources
	return node, nil
}
