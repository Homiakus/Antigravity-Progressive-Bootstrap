package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

// CancelWorkflowRuntime fences all remaining work for a workflow inside the
// caller's existing SQLite transaction. Historical events, consumed signals,
// terminal attempts/nodes and retry history are intentionally preserved.
func (t *transaction) CancelWorkflowRuntime(ctx context.Context, runID harnessmodel.WorkflowRunID, at time.Time) (harnessstore.WorkflowCancellationStats, error) {
	if runID == "" || at.IsZero() {
		return harnessstore.WorkflowCancellationStats{}, fmt.Errorf("workflow run id and cancellation time are required")
	}
	at = at.UTC()
	stamp := formatTime(at)
	var stats harnessstore.WorkflowCancellationStats

	res, err := t.tx.ExecContext(ctx, `
UPDATE leases
SET state=?, closed_at=?
WHERE state=?
  AND attempt_id IN (
      SELECT a.id
      FROM attempts a
      JOIN node_runs nr ON nr.id=a.node_run_id
      WHERE nr.workflow_run_id=? AND a.state IN (?,?)
  )`, string(harnessmodel.LeaseReleased), stamp, string(harnessmodel.LeaseActive), string(runID), string(harnessmodel.AttemptClaimed), string(harnessmodel.AttemptRunning))
	if err != nil {
		return stats, fmt.Errorf("release workflow leases: %w", err)
	}
	if stats.Leases, err = rowsAffected(res); err != nil {
		return stats, err
	}

	res, err = t.tx.ExecContext(ctx, `
UPDATE attempts
SET state=?, finished_at=?, error_class='CANCELLED', error_message='workflow cancelled'
WHERE id IN (
    SELECT a.id
    FROM attempts a
    JOIN node_runs nr ON nr.id=a.node_run_id
    WHERE nr.workflow_run_id=? AND a.state IN (?,?)
)`, string(harnessmodel.AttemptCancelled), stamp, string(runID), string(harnessmodel.AttemptClaimed), string(harnessmodel.AttemptRunning))
	if err != nil {
		return stats, fmt.Errorf("cancel workflow attempts: %w", err)
	}
	if stats.Attempts, err = rowsAffected(res); err != nil {
		return stats, err
	}

	res, err = t.tx.ExecContext(ctx, `
UPDATE timers SET state=?, resolved_at=?
WHERE workflow_run_id=? AND state=?`, string(harnessmodel.TimerCancelled), stamp, string(runID), string(harnessmodel.TimerPending))
	if err != nil {
		return stats, fmt.Errorf("cancel workflow timers: %w", err)
	}
	if stats.Timers, err = rowsAffected(res); err != nil {
		return stats, err
	}

	res, err = t.tx.ExecContext(ctx, `
UPDATE signal_waits SET state=?, resolved_at=?
WHERE workflow_run_id=? AND state=?`, string(harnessmodel.SignalWaitCancelled), stamp, string(runID), string(harnessmodel.SignalWaitWaiting))
	if err != nil {
		return stats, fmt.Errorf("cancel workflow signal waits: %w", err)
	}
	if stats.SignalWaits, err = rowsAffected(res); err != nil {
		return stats, err
	}

	res, err = t.tx.ExecContext(ctx, `
UPDATE approvals SET state=?, resolved_at=?
WHERE workflow_run_id=? AND state=?`, string(harnessmodel.ApprovalCancelled), stamp, string(runID), string(harnessmodel.ApprovalPending))
	if err != nil {
		return stats, fmt.Errorf("cancel workflow approvals: %w", err)
	}
	if stats.Approvals, err = rowsAffected(res); err != nil {
		return stats, err
	}

	res, err = t.tx.ExecContext(ctx, `DELETE FROM retry_schedule WHERE workflow_run_id=?`, string(runID))
	if err != nil {
		return stats, fmt.Errorf("cancel workflow retries: %w", err)
	}
	if stats.Retries, err = rowsAffected(res); err != nil {
		return stats, err
	}

	if _, err := t.tx.ExecContext(ctx, `DELETE FROM ready_queue WHERE workflow_run_id=?`, string(runID)); err != nil {
		return stats, fmt.Errorf("remove workflow ready projection: %w", err)
	}

	res, err = t.tx.ExecContext(ctx, `
UPDATE node_runs
SET state=?, updated_at=?
WHERE workflow_run_id=?
  AND state NOT IN (?,?,?,?,?)`,
		string(harnessmodel.NodeCancelled), stamp, string(runID),
		string(harnessmodel.NodeSucceeded), string(harnessmodel.NodeFailed), string(harnessmodel.NodeTimedOut), string(harnessmodel.NodeCancelled), string(harnessmodel.NodeSkipped))
	if err != nil {
		return stats, fmt.Errorf("cancel workflow nodes: %w", err)
	}
	if stats.Nodes, err = rowsAffected(res); err != nil {
		return stats, err
	}

	res, err = t.tx.ExecContext(ctx, `
UPDATE workflow_progress
SET terminal_nodes=total_nodes, updated_at=?
WHERE workflow_run_id=?`, stamp, string(runID))
	if err != nil {
		return stats, fmt.Errorf("finalize cancelled workflow progress: %w", err)
	}
	if n, countErr := res.RowsAffected(); countErr != nil {
		return stats, fmt.Errorf("read cancelled workflow progress count: %w", countErr)
	} else if n != 1 {
		return stats, harnessstore.ErrNotFound
	}

	return stats, nil
}

func rowsAffected(res sql.Result) (int, error) {
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read affected row count: %w", err)
	}
	return int(n), nil
}
