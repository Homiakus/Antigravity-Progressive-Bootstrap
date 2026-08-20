package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func (t *transaction) GetWorkflowProgress(ctx context.Context, runID harnessmodel.WorkflowRunID) (harnessmodel.WorkflowProgress, error) {
	var p harnessmodel.WorkflowProgress
	var id, updated string
	if err := t.tx.QueryRowContext(ctx, `
SELECT workflow_run_id, total_nodes, terminal_nodes, failed_nodes, updated_at
FROM workflow_progress WHERE workflow_run_id=?`, string(runID)).Scan(&id, &p.TotalNodes, &p.TerminalNodes, &p.FailedNodes, &updated); err != nil {
		return harnessmodel.WorkflowProgress{}, mapNotFound(err)
	}
	p.WorkflowRunID = harnessmodel.WorkflowRunID(id)
	var err error
	p.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return harnessmodel.WorkflowProgress{}, fmt.Errorf("parse workflow progress updated_at: %w", err)
	}
	return p, nil
}

func (t *transaction) CreateWorkflowProgress(ctx context.Context, p harnessmodel.WorkflowProgress) error {
	if p.WorkflowRunID == "" || p.TotalNodes < 0 || p.TerminalNodes < 0 || p.FailedNodes < 0 || p.TerminalNodes > p.TotalNodes || p.FailedNodes > p.TerminalNodes || p.UpdatedAt.IsZero() {
		return fmt.Errorf("invalid workflow progress")
	}
	if _, err := t.tx.ExecContext(ctx, `
INSERT INTO workflow_progress(workflow_run_id, total_nodes, terminal_nodes, failed_nodes, updated_at)
VALUES(?, ?, ?, ?, ?)`, string(p.WorkflowRunID), p.TotalNodes, p.TerminalNodes, p.FailedNodes, formatTime(p.UpdatedAt)); err != nil {
		return fmt.Errorf("insert workflow progress: %w", err)
	}
	return nil
}

func (t *transaction) IncrementWorkflowProgress(ctx context.Context, runID harnessmodel.WorkflowRunID, failed bool, updatedAt time.Time) (harnessmodel.WorkflowProgress, error) {
	if runID == "" || updatedAt.IsZero() {
		return harnessmodel.WorkflowProgress{}, fmt.Errorf("workflow run id and updated time are required")
	}
	failedDelta := 0
	if failed {
		failedDelta = 1
	}
	var p harnessmodel.WorkflowProgress
	var id, updated string
	err := t.tx.QueryRowContext(ctx, `
UPDATE workflow_progress
SET terminal_nodes=terminal_nodes+1,
    failed_nodes=failed_nodes+?,
    updated_at=?
WHERE workflow_run_id=? AND terminal_nodes < total_nodes
RETURNING workflow_run_id, total_nodes, terminal_nodes, failed_nodes, updated_at`, failedDelta, formatTime(updatedAt), string(runID)).Scan(&id, &p.TotalNodes, &p.TerminalNodes, &p.FailedNodes, &updated)
	if err != nil {
		if err == sql.ErrNoRows {
			return harnessmodel.WorkflowProgress{}, harnessstore.ErrConflict
		}
		return harnessmodel.WorkflowProgress{}, fmt.Errorf("increment workflow progress: %w", err)
	}
	p.WorkflowRunID = harnessmodel.WorkflowRunID(id)
	p.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return harnessmodel.WorkflowProgress{}, fmt.Errorf("parse workflow progress updated_at: %w", err)
	}
	return p, nil
}

func (t *transaction) GetNodeRun(ctx context.Context, id harnessmodel.NodeRunID) (harnessmodel.NodeRun, error) {
	return scanNodeRun(t.tx.QueryRowContext(ctx, `
SELECT id, workflow_run_id, node_id, graph_revision, generation, state, remaining_dependencies, created_at, updated_at
FROM node_runs WHERE id=?`, string(id)))
}

func scanNodeRun(row interface{ Scan(...any) error }) (harnessmodel.NodeRun, error) {
	var nr harnessmodel.NodeRun
	var id, runID, nodeID, state, created, updated string
	if err := row.Scan(&id, &runID, &nodeID, &nr.GraphRevision, &nr.Generation, &state, &nr.RemainingDependencies, &created, &updated); err != nil {
		return harnessmodel.NodeRun{}, mapNotFound(err)
	}
	nr.ID = harnessmodel.NodeRunID(id)
	nr.WorkflowRunID = harnessmodel.WorkflowRunID(runID)
	nr.NodeID = harnessmodel.NodeID(nodeID)
	nr.State = harnessmodel.NodeState(state)
	var err error
	if nr.CreatedAt, err = parseTime(created); err != nil {
		return harnessmodel.NodeRun{}, fmt.Errorf("parse node run created_at: %w", err)
	}
	if nr.UpdatedAt, err = parseTime(updated); err != nil {
		return harnessmodel.NodeRun{}, fmt.Errorf("parse node run updated_at: %w", err)
	}
	return nr, nil
}

func (t *transaction) ListDependentNodeRuns(ctx context.Context, runID harnessmodel.WorkflowRunID, parentNodeID harnessmodel.NodeID) ([]harnessmodel.NodeRun, error) {
	rows, err := t.tx.QueryContext(ctx, `
SELECT nr.id, nr.workflow_run_id, nr.node_id, nr.graph_revision, nr.generation, nr.state, nr.remaining_dependencies, nr.created_at, nr.updated_at
FROM node_runs nr
JOIN dependencies d
  ON d.definition_id=nr.definition_id
 AND d.definition_version=nr.definition_version
 AND d.node_id=nr.node_id
WHERE nr.workflow_run_id=?
  AND d.depends_on_node_id=?
  AND d.required=1
  AND nr.generation=(
      SELECT MAX(n2.generation)
      FROM node_runs n2
      WHERE n2.workflow_run_id=nr.workflow_run_id AND n2.node_id=nr.node_id
  )
ORDER BY nr.node_id`, string(runID), string(parentNodeID))
	if err != nil {
		return nil, fmt.Errorf("list dependent node runs: %w", err)
	}
	defer rows.Close()
	out := make([]harnessmodel.NodeRun, 0)
	for rows.Next() {
		nr, err := scanNodeRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, nr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dependent node runs: %w", err)
	}
	return out, nil
}

func (t *transaction) CompareAndSwapNodeRun(ctx context.Context, expected harnessmodel.NodeState, nr harnessmodel.NodeRun) error {
	if nr.ID == "" || expected == "" || nr.State == "" || nr.UpdatedAt.IsZero() || nr.RemainingDependencies < 0 {
		return fmt.Errorf("invalid node run CAS")
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE node_runs
SET state=?, remaining_dependencies=?, updated_at=?
WHERE id=? AND state=?`, string(nr.State), nr.RemainingDependencies, formatTime(nr.UpdatedAt), string(nr.ID), string(expected))
	if err != nil {
		return fmt.Errorf("CAS node run: %w", err)
	}
	return requireOneAffected(res)
}

func (t *transaction) DecrementNodeRemainingDependencies(ctx context.Context, id harnessmodel.NodeRunID, updatedAt time.Time) (int, error) {
	if id == "" || updatedAt.IsZero() {
		return 0, fmt.Errorf("node run id and updated time are required")
	}
	var remaining int
	if err := t.tx.QueryRowContext(ctx, `
UPDATE node_runs
SET remaining_dependencies=remaining_dependencies-1, updated_at=?
WHERE id=? AND remaining_dependencies>0 AND state=?
RETURNING remaining_dependencies`, formatTime(updatedAt), string(id), string(harnessmodel.NodePendingDependencies)).Scan(&remaining); err != nil {
		if err == sql.ErrNoRows {
			return 0, harnessstore.ErrConflict
		}
		return 0, fmt.Errorf("decrement node dependencies: %w", err)
	}
	return remaining, nil
}

func (t *transaction) GetAttempt(ctx context.Context, id harnessmodel.AttemptID) (harnessmodel.Attempt, error) {
	var a harnessmodel.Attempt
	var attemptID, nodeRunID, state string
	var worker, started, finished sql.NullString
	var created string
	if err := t.tx.QueryRowContext(ctx, `
SELECT id, node_run_id, attempt_number, state, worker_id, lease_epoch, created_at, started_at, finished_at, error_class, error_message
FROM attempts WHERE id=?`, string(id)).Scan(&attemptID, &nodeRunID, &a.Number, &state, &worker, &a.LeaseEpoch, &created, &started, &finished, &a.ErrorClass, &a.ErrorMessage); err != nil {
		return harnessmodel.Attempt{}, mapNotFound(err)
	}
	a.ID = harnessmodel.AttemptID(attemptID)
	a.NodeRunID = harnessmodel.NodeRunID(nodeRunID)
	a.State = harnessmodel.AttemptState(state)
	if worker.Valid {
		a.WorkerID = harnessmodel.WorkerID(worker.String)
	}
	var err error
	a.CreatedAt, err = parseTime(created)
	if err != nil {
		return harnessmodel.Attempt{}, fmt.Errorf("parse attempt created_at: %w", err)
	}
	if started.Valid && started.String != "" {
		a.StartedAt, err = parseTime(started.String)
		if err != nil {
			return harnessmodel.Attempt{}, fmt.Errorf("parse attempt started_at: %w", err)
		}
	}
	if finished.Valid && finished.String != "" {
		a.FinishedAt, err = parseTime(finished.String)
		if err != nil {
			return harnessmodel.Attempt{}, fmt.Errorf("parse attempt finished_at: %w", err)
		}
	}
	return a, nil
}

func (t *transaction) CreateNextAttempt(ctx context.Context, a harnessmodel.Attempt) (harnessmodel.Attempt, error) {
	if a.ID == "" || a.NodeRunID == "" || a.Number != 0 || a.State != harnessmodel.AttemptCreated || a.CreatedAt.IsZero() {
		return harnessmodel.Attempt{}, fmt.Errorf("invalid new attempt")
	}
	worker := any(nil)
	if a.WorkerID != "" {
		worker = string(a.WorkerID)
	}
	if err := t.tx.QueryRowContext(ctx, `
INSERT INTO attempts(id, node_run_id, attempt_number, state, worker_id, lease_epoch, created_at, started_at, finished_at, error_class, error_message)
SELECT ?, ?, COALESCE(MAX(attempt_number),0)+1, ?, ?, ?, ?, NULL, NULL, ?, ?
FROM attempts
WHERE node_run_id=?
RETURNING attempt_number`, string(a.ID), string(a.NodeRunID), string(a.State), worker, a.LeaseEpoch, formatTime(a.CreatedAt), a.ErrorClass, a.ErrorMessage, string(a.NodeRunID)).Scan(&a.Number); err != nil {
		return harnessmodel.Attempt{}, fmt.Errorf("insert attempt: %w", err)
	}
	return a, nil
}

func (t *transaction) CompareAndSwapAttempt(ctx context.Context, expected harnessmodel.AttemptState, a harnessmodel.Attempt) error {
	if a.ID == "" || expected == "" || a.State == "" || a.CreatedAt.IsZero() {
		return fmt.Errorf("invalid attempt CAS")
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE attempts
SET state=?, worker_id=?, lease_epoch=?, started_at=?, finished_at=?, error_class=?, error_message=?
WHERE id=? AND state=?`, string(a.State), nullableString(string(a.WorkerID)), a.LeaseEpoch, nullableTime(a.StartedAt), nullableTime(a.FinishedAt), a.ErrorClass, a.ErrorMessage, string(a.ID), string(expected))
	if err != nil {
		return fmt.Errorf("CAS attempt: %w", err)
	}
	return requireOneAffected(res)
}

func (t *transaction) CompareAndSwapWorkflowRun(ctx context.Context, expected harnessmodel.WorkflowState, run harnessmodel.WorkflowRun) error {
	if run.ID == "" || expected == "" || run.State == "" || run.UpdatedAt.IsZero() {
		return fmt.Errorf("invalid workflow run CAS")
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE workflow_runs SET state=?, updated_at=? WHERE id=? AND state=?`, string(run.State), formatTime(run.UpdatedAt), string(run.ID), string(expected))
	if err != nil {
		return fmt.Errorf("CAS workflow run: %w", err)
	}
	return requireOneAffected(res)
}

func requireOneAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected row count: %w", err)
	}
	if n != 1 {
		return harnessstore.ErrConflict
	}
	return nil
}

func nullableTime(v time.Time) any {
	if v.IsZero() {
		return nil
	}
	return formatTime(v)
}

func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
