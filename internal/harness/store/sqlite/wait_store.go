package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func (t *transaction) CreateTimer(ctx context.Context, timer harnessmodel.Timer) error {
	if timer.ID == "" || timer.WorkflowRunID == "" || strings.TrimSpace(string(timer.Kind)) == "" || timer.State != harnessmodel.TimerPending || timer.DueAt.IsZero() || timer.CreatedAt.IsZero() || !timer.ResolvedAt.IsZero() {
		return fmt.Errorf("invalid timer")
	}
	createdNS, err := checkedUnixNano(timer.CreatedAt)
	if err != nil {
		return fmt.Errorf("invalid timer created_at: %w", err)
	}
	dueNS, err := checkedUnixNano(timer.DueAt)
	if err != nil {
		return fmt.Errorf("invalid timer due_at: %w", err)
	}
	if dueNS < createdNS {
		return fmt.Errorf("timer due_at precedes created_at")
	}
	if err := t.validateNodeWorkflow(ctx, timer.NodeRunID, timer.WorkflowRunID); err != nil {
		return err
	}
	_, err = t.tx.ExecContext(ctx, `
INSERT INTO timers(timer_id, workflow_run_id, node_run_id, kind, payload, state, due_at, due_at_ns, created_at, resolved_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		string(timer.ID), string(timer.WorkflowRunID), nullableNodeRunID(timer.NodeRunID), string(timer.Kind), cloneBytes(timer.Payload), string(timer.State), formatTime(timer.DueAt), dueNS, formatTime(timer.CreatedAt))
	if err != nil {
		return fmt.Errorf("insert timer: %w", err)
	}
	return nil
}

func (t *transaction) GetTimer(ctx context.Context, timerID harnessmodel.TimerID) (harnessmodel.Timer, error) {
	return scanTimer(t.tx.QueryRowContext(ctx, `
SELECT timer_id, workflow_run_id, node_run_id, kind, payload, state, due_at, created_at, resolved_at
FROM timers WHERE timer_id=?`, string(timerID)))
}

func (t *transaction) ListDueTimers(ctx context.Context, now time.Time, limit int) ([]harnessmodel.Timer, error) {
	nowNS, err := checkedUnixNano(now)
	if err != nil {
		return nil, fmt.Errorf("invalid timer due query time: %w", err)
	}
	limit = boundedWaitLimit(limit)
	rows, err := t.tx.QueryContext(ctx, `
SELECT timer_id, workflow_run_id, node_run_id, kind, payload, state, due_at, created_at, resolved_at
FROM timers
WHERE state='PENDING' AND due_at_ns<=?
ORDER BY due_at_ns, workflow_run_id, timer_id
LIMIT ?`, nowNS, limit)
	if err != nil {
		return nil, fmt.Errorf("list due timers: %w", err)
	}
	defer rows.Close()
	out := make([]harnessmodel.Timer, 0)
	for rows.Next() {
		timer, err := scanTimer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, timer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due timers: %w", err)
	}
	return out, nil
}

func (t *transaction) CompareAndSwapTimer(ctx context.Context, expected harnessmodel.TimerState, timer harnessmodel.Timer) error {
	if timer.ID == "" || !expected.Valid() || !timer.State.Valid() || expected == timer.State {
		return fmt.Errorf("invalid timer state transition")
	}
	if expected != harnessmodel.TimerPending || (timer.State != harnessmodel.TimerFired && timer.State != harnessmodel.TimerCancelled) || timer.ResolvedAt.IsZero() {
		return fmt.Errorf("invalid timer transition %s -> %s", expected, timer.State)
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE timers SET state=?, resolved_at=? WHERE timer_id=? AND state=?`,
		string(timer.State), formatTime(timer.ResolvedAt), string(timer.ID), string(expected))
	if err != nil {
		return fmt.Errorf("compare-and-swap timer: %w", err)
	}
	if err := requireOneAffected(res); err != nil {
		return harnessstore.ErrConflict
	}
	return nil
}

func scanTimer(row interface{ Scan(...any) error }) (harnessmodel.Timer, error) {
	var timer harnessmodel.Timer
	var id, workflowID, kind, state, dueAt, createdAt string
	var nodeID, resolvedAt sql.NullString
	var payload []byte
	if err := row.Scan(&id, &workflowID, &nodeID, &kind, &payload, &state, &dueAt, &createdAt, &resolvedAt); err != nil {
		return harnessmodel.Timer{}, mapNotFound(err)
	}
	timer.ID = harnessmodel.TimerID(id)
	timer.WorkflowRunID = harnessmodel.WorkflowRunID(workflowID)
	if nodeID.Valid {
		timer.NodeRunID = harnessmodel.NodeRunID(nodeID.String)
	}
	timer.Kind = harnessmodel.TimerKind(kind)
	timer.Payload = cloneBytes(payload)
	timer.State = harnessmodel.TimerState(state)
	var err error
	if timer.DueAt, err = parseTime(dueAt); err != nil {
		return harnessmodel.Timer{}, fmt.Errorf("parse timer due_at: %w", err)
	}
	if timer.CreatedAt, err = parseTime(createdAt); err != nil {
		return harnessmodel.Timer{}, fmt.Errorf("parse timer created_at: %w", err)
	}
	if resolvedAt.Valid {
		if timer.ResolvedAt, err = parseTime(resolvedAt.String); err != nil {
			return harnessmodel.Timer{}, fmt.Errorf("parse timer resolved_at: %w", err)
		}
	}
	return timer, nil
}

func (t *transaction) PutSignal(ctx context.Context, signal harnessmodel.Signal) (harnessmodel.Signal, bool, error) {
	if signal.ID == "" || signal.WorkflowRunID == "" || strings.TrimSpace(signal.Name) == "" || strings.TrimSpace(signal.MessageID) == "" || signal.State != harnessmodel.SignalPending || signal.ReceivedAt.IsZero() || signal.ConsumedByNodeRunID != "" || !signal.ConsumedAt.IsZero() {
		return harnessmodel.Signal{}, false, fmt.Errorf("invalid signal")
	}
	receivedNS, err := checkedUnixNano(signal.ReceivedAt)
	if err != nil {
		return harnessmodel.Signal{}, false, fmt.Errorf("invalid signal received_at: %w", err)
	}
	res, err := t.tx.ExecContext(ctx, `
INSERT INTO signals(signal_id, workflow_run_id, signal_name, message_id, payload, state, received_at, received_at_ns, consumed_by_node_run_id, consumed_at)
VALUES(?, ?, ?, ?, ?, 'PENDING', ?, ?, NULL, NULL)
ON CONFLICT(workflow_run_id, signal_name, message_id) DO NOTHING`,
		string(signal.ID), string(signal.WorkflowRunID), signal.Name, signal.MessageID, cloneBytes(signal.Payload), formatTime(signal.ReceivedAt), receivedNS)
	if err != nil {
		return harnessmodel.Signal{}, false, fmt.Errorf("put signal: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 1 {
		return signal, true, nil
	}
	existing, err := t.GetSignalByMessage(ctx, signal.WorkflowRunID, signal.Name, signal.MessageID)
	if err != nil {
		return harnessmodel.Signal{}, false, err
	}
	if !bytes.Equal(existing.Payload, signal.Payload) {
		return existing, false, fmt.Errorf("signal message %q was replayed with different payload: %w", signal.MessageID, harnessstore.ErrConflict)
	}
	return existing, false, nil
}

func (t *transaction) GetSignal(ctx context.Context, signalID harnessmodel.SignalID) (harnessmodel.Signal, error) {
	return scanSignal(t.tx.QueryRowContext(ctx, `
SELECT signal_id, workflow_run_id, signal_name, message_id, payload, state, received_at, consumed_by_node_run_id, consumed_at
FROM signals WHERE signal_id=?`, string(signalID)))
}

func (t *transaction) GetSignalByMessage(ctx context.Context, workflowRunID harnessmodel.WorkflowRunID, name, messageID string) (harnessmodel.Signal, error) {
	return scanSignal(t.tx.QueryRowContext(ctx, `
SELECT signal_id, workflow_run_id, signal_name, message_id, payload, state, received_at, consumed_by_node_run_id, consumed_at
FROM signals WHERE workflow_run_id=? AND signal_name=? AND message_id=?`, string(workflowRunID), name, messageID))
}

func (t *transaction) ListPendingSignals(ctx context.Context, workflowRunID harnessmodel.WorkflowRunID, name string, limit int) ([]harnessmodel.Signal, error) {
	if workflowRunID == "" || strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("workflow run id and signal name are required")
	}
	limit = boundedWaitLimit(limit)
	rows, err := t.tx.QueryContext(ctx, `
SELECT signal_id, workflow_run_id, signal_name, message_id, payload, state, received_at, consumed_by_node_run_id, consumed_at
FROM signals
WHERE workflow_run_id=? AND signal_name=? AND state='PENDING'
ORDER BY received_at_ns, signal_id
LIMIT ?`, string(workflowRunID), name, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending signals: %w", err)
	}
	defer rows.Close()
	out := make([]harnessmodel.Signal, 0)
	for rows.Next() {
		signal, err := scanSignal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, signal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending signals: %w", err)
	}
	return out, nil
}

func scanSignal(row interface{ Scan(...any) error }) (harnessmodel.Signal, error) {
	var signal harnessmodel.Signal
	var id, workflowID, state, receivedAt string
	var consumedNodeID, consumedAt sql.NullString
	var payload []byte
	if err := row.Scan(&id, &workflowID, &signal.Name, &signal.MessageID, &payload, &state, &receivedAt, &consumedNodeID, &consumedAt); err != nil {
		return harnessmodel.Signal{}, mapNotFound(err)
	}
	signal.ID = harnessmodel.SignalID(id)
	signal.WorkflowRunID = harnessmodel.WorkflowRunID(workflowID)
	signal.Payload = cloneBytes(payload)
	signal.State = harnessmodel.SignalState(state)
	var err error
	if signal.ReceivedAt, err = parseTime(receivedAt); err != nil {
		return harnessmodel.Signal{}, fmt.Errorf("parse signal received_at: %w", err)
	}
	if consumedNodeID.Valid {
		signal.ConsumedByNodeRunID = harnessmodel.NodeRunID(consumedNodeID.String)
	}
	if consumedAt.Valid {
		if signal.ConsumedAt, err = parseTime(consumedAt.String); err != nil {
			return harnessmodel.Signal{}, fmt.Errorf("parse signal consumed_at: %w", err)
		}
	}
	return signal, nil
}

func (t *transaction) CreateSignalWait(ctx context.Context, wait harnessmodel.SignalWait) error {
	if wait.NodeRunID == "" || wait.WorkflowRunID == "" || strings.TrimSpace(wait.SignalName) == "" || wait.State != harnessmodel.SignalWaitWaiting || wait.CreatedAt.IsZero() || wait.DeliveredSignalID != "" || !wait.ResolvedAt.IsZero() {
		return fmt.Errorf("invalid signal wait")
	}
	if err := t.validateNodeWorkflow(ctx, wait.NodeRunID, wait.WorkflowRunID); err != nil {
		return err
	}
	_, err := t.tx.ExecContext(ctx, `
INSERT INTO signal_waits(node_run_id, workflow_run_id, signal_name, state, created_at, delivered_signal_id, resolved_at)
VALUES(?, ?, ?, 'WAITING', ?, NULL, NULL)`, string(wait.NodeRunID), string(wait.WorkflowRunID), wait.SignalName, formatTime(wait.CreatedAt))
	if err != nil {
		return fmt.Errorf("insert signal wait: %w", err)
	}
	return nil
}

func (t *transaction) GetSignalWait(ctx context.Context, nodeRunID harnessmodel.NodeRunID) (harnessmodel.SignalWait, error) {
	return scanSignalWait(t.tx.QueryRowContext(ctx, `
SELECT node_run_id, workflow_run_id, signal_name, state, created_at, delivered_signal_id, resolved_at
FROM signal_waits WHERE node_run_id=?`, string(nodeRunID)))
}

func (t *transaction) CompareAndSwapSignalWait(ctx context.Context, expected harnessmodel.SignalWaitState, wait harnessmodel.SignalWait) error {
	if wait.NodeRunID == "" || !expected.Valid() || !wait.State.Valid() || expected != harnessmodel.SignalWaitWaiting || (wait.State != harnessmodel.SignalWaitCancelled && wait.State != harnessmodel.SignalWaitTimedOut) || wait.DeliveredSignalID != "" || wait.ResolvedAt.IsZero() {
		return fmt.Errorf("invalid signal wait state transition")
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE signal_waits SET state=?, resolved_at=?
WHERE node_run_id=? AND state=? AND delivered_signal_id IS NULL`, string(wait.State), formatTime(wait.ResolvedAt), string(wait.NodeRunID), string(expected))
	if err != nil {
		return fmt.Errorf("compare-and-swap signal wait: %w", err)
	}
	if err := requireOneAffected(res); err != nil {
		return harnessstore.ErrConflict
	}
	return nil
}

func (t *transaction) DeliverSignal(ctx context.Context, nodeRunID harnessmodel.NodeRunID, signalID harnessmodel.SignalID, at time.Time) error {
	if nodeRunID == "" || signalID == "" || at.IsZero() {
		return fmt.Errorf("node run id, signal id and delivery time are required")
	}
	wait, err := t.GetSignalWait(ctx, nodeRunID)
	if err != nil {
		return err
	}
	if wait.State != harnessmodel.SignalWaitWaiting {
		return harnessstore.ErrConflict
	}
	signal, err := t.GetSignal(ctx, signalID)
	if err != nil {
		return err
	}
	if signal.State != harnessmodel.SignalPending || signal.WorkflowRunID != wait.WorkflowRunID || signal.Name != wait.SignalName {
		return harnessstore.ErrConflict
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE signals SET state='CONSUMED', consumed_by_node_run_id=?, consumed_at=?
WHERE signal_id=? AND state='PENDING' AND workflow_run_id=? AND signal_name=?`,
		string(nodeRunID), formatTime(at), string(signalID), string(wait.WorkflowRunID), wait.SignalName)
	if err != nil {
		return fmt.Errorf("consume signal: %w", err)
	}
	if err := requireOneAffected(res); err != nil {
		return harnessstore.ErrConflict
	}
	res, err = t.tx.ExecContext(ctx, `
UPDATE signal_waits SET state='DELIVERED', delivered_signal_id=?, resolved_at=?
WHERE node_run_id=? AND state='WAITING' AND delivered_signal_id IS NULL`, string(signalID), formatTime(at), string(nodeRunID))
	if err != nil {
		return fmt.Errorf("deliver signal to waiter: %w", err)
	}
	if err := requireOneAffected(res); err != nil {
		return harnessstore.ErrConflict
	}
	return nil
}

func scanSignalWait(row interface{ Scan(...any) error }) (harnessmodel.SignalWait, error) {
	var wait harnessmodel.SignalWait
	var nodeID, workflowID, state, createdAt string
	var signalID, resolvedAt sql.NullString
	if err := row.Scan(&nodeID, &workflowID, &wait.SignalName, &state, &createdAt, &signalID, &resolvedAt); err != nil {
		return harnessmodel.SignalWait{}, mapNotFound(err)
	}
	wait.NodeRunID = harnessmodel.NodeRunID(nodeID)
	wait.WorkflowRunID = harnessmodel.WorkflowRunID(workflowID)
	wait.State = harnessmodel.SignalWaitState(state)
	var err error
	if wait.CreatedAt, err = parseTime(createdAt); err != nil {
		return harnessmodel.SignalWait{}, fmt.Errorf("parse signal wait created_at: %w", err)
	}
	if signalID.Valid {
		wait.DeliveredSignalID = harnessmodel.SignalID(signalID.String)
	}
	if resolvedAt.Valid {
		if wait.ResolvedAt, err = parseTime(resolvedAt.String); err != nil {
			return harnessmodel.SignalWait{}, fmt.Errorf("parse signal wait resolved_at: %w", err)
		}
	}
	return wait, nil
}

func (t *transaction) CreateApproval(ctx context.Context, approval harnessmodel.Approval) error {
	if approval.ID == "" || approval.WorkflowRunID == "" || approval.NodeRunID == "" || strings.TrimSpace(approval.RequestedCapability) == "" || strings.TrimSpace(approval.Risk) == "" || strings.TrimSpace(approval.Reason) == "" || approval.State != harnessmodel.ApprovalPending || approval.RequestedAt.IsZero() || approval.Actor != "" || !approval.ResolvedAt.IsZero() {
		return fmt.Errorf("invalid approval")
	}
	if err := t.validateNodeWorkflow(ctx, approval.NodeRunID, approval.WorkflowRunID); err != nil {
		return err
	}
	var expiresAt any
	var expiresNS any
	if !approval.ExpiresAt.IsZero() {
		requestedNS, err := checkedUnixNano(approval.RequestedAt)
		if err != nil {
			return fmt.Errorf("invalid approval requested_at: %w", err)
		}
		ns, err := checkedUnixNano(approval.ExpiresAt)
		if err != nil {
			return fmt.Errorf("invalid approval expires_at: %w", err)
		}
		if ns <= requestedNS {
			return fmt.Errorf("approval expires_at must be after requested_at")
		}
		expiresAt, expiresNS = formatTime(approval.ExpiresAt), ns
	}
	_, err := t.tx.ExecContext(ctx, `
INSERT INTO approvals(approval_id, workflow_run_id, node_run_id, requested_capability, risk, reason, requested_at, expires_at, expires_at_ns, state, actor, resolved_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, 'PENDING', '', NULL)`,
		string(approval.ID), string(approval.WorkflowRunID), string(approval.NodeRunID), approval.RequestedCapability, approval.Risk, approval.Reason,
		formatTime(approval.RequestedAt), expiresAt, expiresNS)
	if err != nil {
		return fmt.Errorf("insert approval: %w", err)
	}
	return nil
}

func (t *transaction) GetApproval(ctx context.Context, approvalID harnessmodel.ApprovalID) (harnessmodel.Approval, error) {
	return scanApproval(t.tx.QueryRowContext(ctx, `
SELECT approval_id, workflow_run_id, node_run_id, requested_capability, risk, reason, requested_at, expires_at, state, actor, resolved_at
FROM approvals WHERE approval_id=?`, string(approvalID)))
}

func (t *transaction) ListPendingApprovals(ctx context.Context, workflowRunID harnessmodel.WorkflowRunID, limit int) ([]harnessmodel.Approval, error) {
	limit = boundedWaitLimit(limit)
	query := `
SELECT approval_id, workflow_run_id, node_run_id, requested_capability, risk, reason, requested_at, expires_at, state, actor, resolved_at
FROM approvals WHERE state='PENDING'`
	args := make([]any, 0, 2)
	if workflowRunID != "" {
		query += ` AND workflow_run_id=?`
		args = append(args, string(workflowRunID))
	}
	query += ` ORDER BY CASE WHEN expires_at_ns IS NULL THEN 1 ELSE 0 END, expires_at_ns, requested_at, approval_id LIMIT ?`
	args = append(args, limit)
	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list pending approvals: %w", err)
	}
	defer rows.Close()
	out := make([]harnessmodel.Approval, 0)
	for rows.Next() {
		approval, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, approval)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending approvals: %w", err)
	}
	return out, nil
}

func (t *transaction) CompareAndSwapApproval(ctx context.Context, expected harnessmodel.ApprovalState, approval harnessmodel.Approval) error {
	if approval.ID == "" || expected != harnessmodel.ApprovalPending || !approval.State.Valid() || approval.State == harnessmodel.ApprovalPending || approval.ResolvedAt.IsZero() {
		return fmt.Errorf("invalid approval state transition")
	}
	switch approval.State {
	case harnessmodel.ApprovalApproved, harnessmodel.ApprovalRejected:
		if strings.TrimSpace(approval.Actor) == "" {
			return fmt.Errorf("approval actor is required")
		}
		if !approval.ExpiresAt.IsZero() && !approval.ResolvedAt.Before(approval.ExpiresAt) {
			return fmt.Errorf("approval cannot be resolved after expiration")
		}
	case harnessmodel.ApprovalExpired:
		if approval.ExpiresAt.IsZero() || approval.ResolvedAt.Before(approval.ExpiresAt) {
			return fmt.Errorf("approval cannot expire before expires_at")
		}
		if approval.Actor != "" {
			return fmt.Errorf("expired approval must not have actor")
		}
	case harnessmodel.ApprovalCancelled:
		if approval.Actor != "" {
			return fmt.Errorf("cancelled approval must not have actor")
		}
	default:
		return fmt.Errorf("invalid approval target state %q", approval.State)
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE approvals SET state=?, actor=?, resolved_at=?
WHERE approval_id=? AND state=?`, string(approval.State), approval.Actor, formatTime(approval.ResolvedAt), string(approval.ID), string(expected))
	if err != nil {
		return fmt.Errorf("compare-and-swap approval: %w", err)
	}
	if err := requireOneAffected(res); err != nil {
		return harnessstore.ErrConflict
	}
	return nil
}

func scanApproval(row interface{ Scan(...any) error }) (harnessmodel.Approval, error) {
	var approval harnessmodel.Approval
	var id, workflowID, nodeID, requestedAt, state string
	var expiresAt, resolvedAt sql.NullString
	if err := row.Scan(&id, &workflowID, &nodeID, &approval.RequestedCapability, &approval.Risk, &approval.Reason, &requestedAt, &expiresAt, &state, &approval.Actor, &resolvedAt); err != nil {
		return harnessmodel.Approval{}, mapNotFound(err)
	}
	approval.ID = harnessmodel.ApprovalID(id)
	approval.WorkflowRunID = harnessmodel.WorkflowRunID(workflowID)
	approval.NodeRunID = harnessmodel.NodeRunID(nodeID)
	approval.State = harnessmodel.ApprovalState(state)
	var err error
	if approval.RequestedAt, err = parseTime(requestedAt); err != nil {
		return harnessmodel.Approval{}, fmt.Errorf("parse approval requested_at: %w", err)
	}
	if expiresAt.Valid {
		if approval.ExpiresAt, err = parseTime(expiresAt.String); err != nil {
			return harnessmodel.Approval{}, fmt.Errorf("parse approval expires_at: %w", err)
		}
	}
	if resolvedAt.Valid {
		if approval.ResolvedAt, err = parseTime(resolvedAt.String); err != nil {
			return harnessmodel.Approval{}, fmt.Errorf("parse approval resolved_at: %w", err)
		}
	}
	return approval, nil
}

func (t *transaction) validateNodeWorkflow(ctx context.Context, nodeRunID harnessmodel.NodeRunID, workflowRunID harnessmodel.WorkflowRunID) error {
	if nodeRunID == "" {
		return nil
	}
	node, err := t.GetNodeRun(ctx, nodeRunID)
	if err != nil {
		return err
	}
	if node.WorkflowRunID != workflowRunID {
		return fmt.Errorf("node run %s belongs to workflow %s, not %s: %w", nodeRunID, node.WorkflowRunID, workflowRunID, harnessstore.ErrConflict)
	}
	return nil
}

func nullableNodeRunID(id harnessmodel.NodeRunID) any {
	if id == "" {
		return nil
	}
	return string(id)
}

func cloneBytes(value []byte) []byte {
	if len(value) == 0 {
		return []byte{}
	}
	return append([]byte(nil), value...)
}

func boundedWaitLimit(limit int) int {
	if limit <= 0 || limit > 10000 {
		return 1000
	}
	return limit
}
