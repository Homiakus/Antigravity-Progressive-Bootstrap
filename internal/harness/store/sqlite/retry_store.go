package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func (t *transaction) CreateRetrySchedule(ctx context.Context, schedule harnessmodel.RetrySchedule) error {
	if schedule.NodeRunID == "" || schedule.WorkflowRunID == "" || schedule.FailedAttemptID == "" || schedule.AttemptNumber < 1 || !schedule.FailureClass.Valid() || schedule.ScheduledAt.IsZero() || schedule.NotBefore.IsZero() || schedule.NotBefore.Before(schedule.ScheduledAt) {
		return fmt.Errorf("invalid retry schedule")
	}
	_, err := t.tx.ExecContext(ctx, `
INSERT INTO retry_schedule(node_run_id, workflow_run_id, failed_attempt_id, attempt_number, failure_class, policy_ref, service_key, scheduled_at, not_before, not_before_ns)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(schedule.NodeRunID), string(schedule.WorkflowRunID), string(schedule.FailedAttemptID), schedule.AttemptNumber,
		string(schedule.FailureClass), schedule.PolicyRef, schedule.ServiceKey, formatTime(schedule.ScheduledAt), formatTime(schedule.NotBefore), schedule.NotBefore.UnixNano())
	if err != nil {
		return fmt.Errorf("insert retry schedule: %w", err)
	}
	return nil
}

func (t *transaction) GetRetrySchedule(ctx context.Context, nodeRunID harnessmodel.NodeRunID) (harnessmodel.RetrySchedule, error) {
	return scanRetrySchedule(t.tx.QueryRowContext(ctx, `
SELECT node_run_id, workflow_run_id, failed_attempt_id, attempt_number, failure_class, policy_ref, service_key, scheduled_at, not_before
FROM retry_schedule WHERE node_run_id=?`, string(nodeRunID)))
}

func (t *transaction) ListDueRetries(ctx context.Context, now time.Time, limit int) ([]harnessmodel.RetrySchedule, error) {
	if now.IsZero() {
		return nil, fmt.Errorf("retry due time is required")
	}
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	rows, err := t.tx.QueryContext(ctx, `
SELECT node_run_id, workflow_run_id, failed_attempt_id, attempt_number, failure_class, policy_ref, service_key, scheduled_at, not_before
FROM retry_schedule
WHERE not_before_ns<=?
ORDER BY not_before_ns, workflow_run_id, node_run_id
LIMIT ?`, now.UnixNano(), limit)
	if err != nil {
		return nil, fmt.Errorf("list due retries: %w", err)
	}
	defer rows.Close()
	out := make([]harnessmodel.RetrySchedule, 0)
	for rows.Next() {
		schedule, err := scanRetrySchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, schedule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due retries: %w", err)
	}
	return out, nil
}

func (t *transaction) DeleteRetrySchedule(ctx context.Context, nodeRunID harnessmodel.NodeRunID) error {
	if nodeRunID == "" {
		return fmt.Errorf("node run id is required")
	}
	res, err := t.tx.ExecContext(ctx, `DELETE FROM retry_schedule WHERE node_run_id=?`, string(nodeRunID))
	if err != nil {
		return fmt.Errorf("delete retry schedule: %w", err)
	}
	return requireOneAffected(res)
}

func scanRetrySchedule(row interface{ Scan(...any) error }) (harnessmodel.RetrySchedule, error) {
	var schedule harnessmodel.RetrySchedule
	var nodeRunID, workflowRunID, failedAttemptID, failureClass, scheduledAt, notBefore string
	if err := row.Scan(&nodeRunID, &workflowRunID, &failedAttemptID, &schedule.AttemptNumber, &failureClass, &schedule.PolicyRef, &schedule.ServiceKey, &scheduledAt, &notBefore); err != nil {
		return harnessmodel.RetrySchedule{}, mapNotFound(err)
	}
	schedule.NodeRunID = harnessmodel.NodeRunID(nodeRunID)
	schedule.WorkflowRunID = harnessmodel.WorkflowRunID(workflowRunID)
	schedule.FailedAttemptID = harnessmodel.AttemptID(failedAttemptID)
	schedule.FailureClass = harnessmodel.ErrorClass(failureClass)
	var err error
	if schedule.ScheduledAt, err = parseTime(scheduledAt); err != nil {
		return harnessmodel.RetrySchedule{}, fmt.Errorf("parse retry scheduled_at: %w", err)
	}
	if schedule.NotBefore, err = parseTime(notBefore); err != nil {
		return harnessmodel.RetrySchedule{}, fmt.Errorf("parse retry not_before: %w", err)
	}
	return schedule, nil
}

func (t *transaction) GetRetryBudget(ctx context.Context, scope harnessmodel.RetryBudgetScope, scopeKey string) (harnessmodel.RetryBudget, error) {
	var budget harnessmodel.RetryBudget
	var scopeValue, windowStart, updatedAt string
	var windowStartNS, windowNS int64
	if err := t.tx.QueryRowContext(ctx, `
SELECT scope, scope_key, window_start, window_start_ns, window_ns, limit_count, used_count, updated_at
FROM retry_budgets WHERE scope=? AND scope_key=?`, string(scope), scopeKey).
		Scan(&scopeValue, &budget.ScopeKey, &windowStart, &windowStartNS, &windowNS, &budget.Limit, &budget.Used, &updatedAt); err != nil {
		return harnessmodel.RetryBudget{}, mapNotFound(err)
	}
	budget.Scope = harnessmodel.RetryBudgetScope(scopeValue)
	budget.Window = time.Duration(windowNS)
	var err error
	if budget.WindowStart, err = parseTime(windowStart); err != nil {
		return harnessmodel.RetryBudget{}, fmt.Errorf("parse retry budget window_start: %w", err)
	}
	if budget.WindowStart.UnixNano() != windowStartNS {
		return harnessmodel.RetryBudget{}, fmt.Errorf("retry budget %s/%s has inconsistent window timestamp", scope, scopeKey)
	}
	if budget.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return harnessmodel.RetryBudget{}, fmt.Errorf("parse retry budget updated_at: %w", err)
	}
	return budget, nil
}

// ReserveRetryBudget is a single-statement reservation. The UPSERT performs
// increment or window reset inside SQLite, so concurrent callers cannot lose a
// token through read-modify-write races. A no-row RETURNING result means either
// exhaustion or a policy-shape conflict and is classified by a read in the same
// transaction.
func (t *transaction) ReserveRetryBudget(ctx context.Context, scope harnessmodel.RetryBudgetScope, scopeKey string, window time.Duration, limit int, now time.Time) (harnessmodel.RetryBudget, bool, error) {
	if (scope != harnessmodel.RetryBudgetWorkflow && scope != harnessmodel.RetryBudgetService) || scopeKey == "" || window <= 0 || limit < 1 || now.IsZero() {
		return harnessmodel.RetryBudget{}, false, fmt.Errorf("invalid retry budget reservation")
	}
	now = now.UTC()
	nowNS := now.UnixNano()
	windowNS := int64(window)
	var budget harnessmodel.RetryBudget
	var scopeValue, windowStart, updatedAt string
	var storedWindowStartNS, storedWindowNS int64
	err := t.tx.QueryRowContext(ctx, `
INSERT INTO retry_budgets(
    scope, scope_key, window_start, window_start_ns, window_ns,
    limit_count, used_count, updated_at
)
VALUES(?, ?, ?, ?, ?, ?, 1, ?)
ON CONFLICT(scope, scope_key) DO UPDATE SET
    window_start = CASE
        WHEN excluded.window_start_ns >= retry_budgets.window_start_ns + retry_budgets.window_ns
        THEN excluded.window_start ELSE retry_budgets.window_start END,
    window_start_ns = CASE
        WHEN excluded.window_start_ns >= retry_budgets.window_start_ns + retry_budgets.window_ns
        THEN excluded.window_start_ns ELSE retry_budgets.window_start_ns END,
    used_count = CASE
        WHEN excluded.window_start_ns >= retry_budgets.window_start_ns + retry_budgets.window_ns
        THEN 1 ELSE retry_budgets.used_count + 1 END,
    updated_at = excluded.updated_at
WHERE retry_budgets.window_ns = excluded.window_ns
  AND retry_budgets.limit_count = excluded.limit_count
  AND (
      excluded.window_start_ns >= retry_budgets.window_start_ns + retry_budgets.window_ns
      OR retry_budgets.used_count < retry_budgets.limit_count
  )
RETURNING scope, scope_key, window_start, window_start_ns, window_ns, limit_count, used_count, updated_at`,
		string(scope), scopeKey, formatTime(now), nowNS, windowNS, limit, formatTime(now)).
		Scan(&scopeValue, &budget.ScopeKey, &windowStart, &storedWindowStartNS, &storedWindowNS, &budget.Limit, &budget.Used, &updatedAt)
	if err == nil {
		budget.Scope = harnessmodel.RetryBudgetScope(scopeValue)
		budget.Window = time.Duration(storedWindowNS)
		var parseErr error
		if budget.WindowStart, parseErr = parseTime(windowStart); parseErr != nil {
			return harnessmodel.RetryBudget{}, false, fmt.Errorf("parse reserved retry budget window_start: %w", parseErr)
		}
		if budget.WindowStart.UnixNano() != storedWindowStartNS {
			return harnessmodel.RetryBudget{}, false, fmt.Errorf("reserved retry budget has inconsistent window timestamp")
		}
		if budget.UpdatedAt, parseErr = parseTime(updatedAt); parseErr != nil {
			return harnessmodel.RetryBudget{}, false, fmt.Errorf("parse reserved retry budget updated_at: %w", parseErr)
		}
		return budget, true, nil
	}
	if err != sql.ErrNoRows {
		return harnessmodel.RetryBudget{}, false, fmt.Errorf("reserve retry budget: %w", err)
	}

	current, readErr := t.GetRetryBudget(ctx, scope, scopeKey)
	if readErr != nil {
		return harnessmodel.RetryBudget{}, false, readErr
	}
	if current.Window != window || current.Limit != limit {
		return current, false, fmt.Errorf("retry budget %s/%s policy changed inside active window: %w", scope, scopeKey, harnessstore.ErrConflict)
	}
	if now.Before(current.WindowStart.Add(current.Window)) && current.Used >= current.Limit {
		return current, false, nil
	}
	return current, false, harnessstore.ErrConflict
}

func (t *transaction) GetCircuitBreaker(ctx context.Context, serviceKey string) (harnessmodel.CircuitBreaker, error) {
	var breaker harnessmodel.CircuitBreaker
	var state, updatedAt string
	var openedAt, nextProbeAt sql.NullString
	var probe int
	if err := t.tx.QueryRowContext(ctx, `
SELECT service_key, revision, state, consecutive_failures, failure_threshold, opened_at, next_probe_at, probe_in_flight, updated_at
FROM circuit_breakers WHERE service_key=?`, serviceKey).
		Scan(&breaker.ServiceKey, &breaker.Revision, &state, &breaker.ConsecutiveFailures, &breaker.FailureThreshold, &openedAt, &nextProbeAt, &probe, &updatedAt); err != nil {
		return harnessmodel.CircuitBreaker{}, mapNotFound(err)
	}
	breaker.State = harnessmodel.CircuitState(state)
	breaker.ProbeInFlight = probe != 0
	var err error
	if openedAt.Valid {
		if breaker.OpenedAt, err = parseTime(openedAt.String); err != nil {
			return harnessmodel.CircuitBreaker{}, fmt.Errorf("parse circuit opened_at: %w", err)
		}
	}
	if nextProbeAt.Valid {
		if breaker.NextProbeAt, err = parseTime(nextProbeAt.String); err != nil {
			return harnessmodel.CircuitBreaker{}, fmt.Errorf("parse circuit next_probe_at: %w", err)
		}
	}
	if breaker.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return harnessmodel.CircuitBreaker{}, fmt.Errorf("parse circuit updated_at: %w", err)
	}
	return breaker, nil
}

func (t *transaction) CreateCircuitBreaker(ctx context.Context, breaker harnessmodel.CircuitBreaker) error {
	if err := validateCircuit(breaker); err != nil {
		return err
	}
	res, err := t.tx.ExecContext(ctx, `
INSERT INTO circuit_breakers(service_key, revision, state, consecutive_failures, failure_threshold, opened_at, next_probe_at, probe_in_flight, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(service_key) DO NOTHING`,
		breaker.ServiceKey, breaker.Revision, string(breaker.State), breaker.ConsecutiveFailures, breaker.FailureThreshold,
		nullTime(breaker.OpenedAt), nullTime(breaker.NextProbeAt), boolInt(breaker.ProbeInFlight), formatTime(breaker.UpdatedAt))
	if err != nil {
		return fmt.Errorf("create circuit breaker: %w", err)
	}
	if err := requireOneAffected(res); err != nil {
		return harnessstore.ErrConflict
	}
	return nil
}

func (t *transaction) CompareAndSwapCircuitBreaker(ctx context.Context, expectedRevision uint64, breaker harnessmodel.CircuitBreaker) error {
	if expectedRevision == 0 || breaker.Revision != expectedRevision+1 {
		return fmt.Errorf("invalid circuit breaker revision transition %d -> %d", expectedRevision, breaker.Revision)
	}
	if err := validateCircuit(breaker); err != nil {
		return err
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE circuit_breakers
SET revision=?, state=?, consecutive_failures=?, failure_threshold=?, opened_at=?, next_probe_at=?, probe_in_flight=?, updated_at=?
WHERE service_key=? AND revision=?`,
		breaker.Revision, string(breaker.State), breaker.ConsecutiveFailures, breaker.FailureThreshold,
		nullTime(breaker.OpenedAt), nullTime(breaker.NextProbeAt), boolInt(breaker.ProbeInFlight), formatTime(breaker.UpdatedAt),
		breaker.ServiceKey, expectedRevision)
	if err != nil {
		return fmt.Errorf("compare-and-swap circuit breaker: %w", err)
	}
	if err := requireOneAffected(res); err != nil {
		return harnessstore.ErrConflict
	}
	return nil
}

func validateCircuit(breaker harnessmodel.CircuitBreaker) error {
	if breaker.ServiceKey == "" || breaker.Revision == 0 || breaker.FailureThreshold < 1 || breaker.ConsecutiveFailures < 0 || breaker.UpdatedAt.IsZero() {
		return fmt.Errorf("invalid circuit breaker")
	}
	switch breaker.State {
	case harnessmodel.CircuitClosed, harnessmodel.CircuitOpen, harnessmodel.CircuitHalfOpen:
		return nil
	default:
		return fmt.Errorf("invalid circuit breaker state %q", breaker.State)
	}
}

func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return formatTime(value)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
