package sqlite

import (
	"context"
	"fmt"
	"strings"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func (t *transaction) CountActiveAttempts(ctx context.Context, workflowRunID harnessmodel.WorkflowRunID) (int, error) {
	if workflowRunID == "" {
		return 0, fmt.Errorf("workflow run id is required")
	}
	var count int
	if err := t.tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM attempts a
JOIN node_runs nr ON nr.id=a.node_run_id
WHERE nr.workflow_run_id=? AND a.state IN ('CLAIMED','RUNNING')`, string(workflowRunID)).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active attempts: %w", err)
	}
	return count, nil
}

func (t *transaction) ListSignalWaits(ctx context.Context, workflowRunID harnessmodel.WorkflowRunID, name string, limit int) ([]harnessmodel.SignalWait, error) {
	if workflowRunID == "" || strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("workflow run id and signal name are required")
	}
	limit = boundedWaitLimit(limit)
	rows, err := t.tx.QueryContext(ctx, `
SELECT node_run_id, workflow_run_id, signal_name, state, created_at, delivered_signal_id, resolved_at
FROM signal_waits
WHERE workflow_run_id=? AND signal_name=? AND state='WAITING'
ORDER BY created_at, node_run_id
LIMIT ?`, string(workflowRunID), name, limit)
	if err != nil {
		return nil, fmt.Errorf("list signal waits: %w", err)
	}
	defer rows.Close()
	out := make([]harnessmodel.SignalWait, 0)
	for rows.Next() {
		wait, err := scanSignalWait(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, wait)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate signal waits: %w", err)
	}
	return out, nil
}
