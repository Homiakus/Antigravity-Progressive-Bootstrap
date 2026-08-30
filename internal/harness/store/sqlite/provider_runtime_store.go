package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

const providerAssignmentSelect = `
SELECT id, attempt_id, account_id, model_id, session_id, plan_digest, state, revision, created_at, updated_at
FROM provider_assignments`

func (t *transaction) GetProviderAssignment(ctx context.Context, id harnessmodel.ProviderAssignmentID) (harnessmodel.ProviderAssignment, error) {
	if id == "" {
		return harnessmodel.ProviderAssignment{}, fmt.Errorf("provider assignment id is required")
	}
	return scanProviderAssignment(t.tx.QueryRowContext(ctx, providerAssignmentSelect+` WHERE id=?`, string(id)))
}

func (t *transaction) GetActiveProviderAssignment(ctx context.Context, attemptID harnessmodel.AttemptID) (harnessmodel.ProviderAssignment, error) {
	if attemptID == "" {
		return harnessmodel.ProviderAssignment{}, fmt.Errorf("attempt id is required")
	}
	return scanProviderAssignment(t.tx.QueryRowContext(ctx, providerAssignmentSelect+` WHERE attempt_id=? AND state='ACTIVE'`, string(attemptID)))
}

func (t *transaction) ListProviderAssignmentsByAttempt(ctx context.Context, attemptID harnessmodel.AttemptID) ([]harnessmodel.ProviderAssignment, error) {
	if attemptID == "" {
		return nil, fmt.Errorf("attempt id is required")
	}
	rows, err := t.tx.QueryContext(ctx, providerAssignmentSelect+` WHERE attempt_id=? ORDER BY created_at, id`, string(attemptID))
	if err != nil {
		return nil, fmt.Errorf("list provider assignments: %w", err)
	}
	defer rows.Close()
	var out []harnessmodel.ProviderAssignment
	for rows.Next() {
		assignment, err := scanProviderAssignment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider assignments: %w", err)
	}
	return out, nil
}

func (t *transaction) CreateProviderAssignment(ctx context.Context, assignment harnessmodel.ProviderAssignment) error {
	if err := assignment.Validate(); err != nil {
		return err
	}
	if assignment.State != harnessmodel.ProviderAssignmentActive || assignment.Revision != 1 {
		return fmt.Errorf("provider assignment must start ACTIVE at revision 1")
	}
	_, err := t.tx.ExecContext(ctx, `
INSERT INTO provider_assignments(id, attempt_id, account_id, model_id, session_id, plan_digest, state, revision, created_at, updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?)`, string(assignment.ID), string(assignment.AttemptID), string(assignment.AccountID), string(assignment.ModelID),
		string(assignment.SessionID), assignment.PlanDigest, string(assignment.State), assignment.Revision, formatTime(assignment.CreatedAt), formatTime(assignment.UpdatedAt))
	if err != nil {
		if _, activeErr := t.GetActiveProviderAssignment(ctx, assignment.AttemptID); activeErr == nil {
			return fmt.Errorf("attempt %s already has an active provider assignment: %w", assignment.AttemptID, harnessstore.ErrConflict)
		}
		return fmt.Errorf("insert provider assignment %s: %w", assignment.ID, err)
	}
	return nil
}

func (t *transaction) CompareAndSwapProviderAssignment(ctx context.Context, expectedRevision uint64, assignment harnessmodel.ProviderAssignment) error {
	if expectedRevision == 0 || expectedRevision >= maxSQLiteIntegerRevision || assignment.Revision != expectedRevision+1 {
		return fmt.Errorf("invalid provider assignment revision transition %d -> %d", expectedRevision, assignment.Revision)
	}
	if err := assignment.Validate(); err != nil {
		return err
	}
	current, err := t.GetProviderAssignment(ctx, assignment.ID)
	if err != nil {
		return err
	}
	if current.Revision != expectedRevision {
		return harnessstore.ErrConflict
	}
	if assignment.AttemptID != current.AttemptID || assignment.AccountID != current.AccountID || assignment.ModelID != current.ModelID || !assignment.CreatedAt.Equal(current.CreatedAt) {
		return fmt.Errorf("provider assignment immutable identity changed: %w", harnessstore.ErrConflict)
	}
	if current.PlanDigest != "" && assignment.PlanDigest != "" && assignment.PlanDigest != current.PlanDigest {
		return fmt.Errorf("provider assignment plan digest cannot be mutated: %w", harnessstore.ErrConflict)
	}
	if assignment.PlanDigest == "" {
		assignment.PlanDigest = current.PlanDigest
	}
	if current.SessionID != "" && assignment.SessionID != current.SessionID {
		return fmt.Errorf("provider assignment session is already bound: %w", harnessstore.ErrConflict)
	}
	if !harnessmodel.ValidProviderAssignmentTransition(current.State, assignment.State) {
		return fmt.Errorf("invalid provider assignment state transition %s -> %s", current.State, assignment.State)
	}
	if assignment.UpdatedAt.Before(current.UpdatedAt) {
		return fmt.Errorf("provider assignment update is stale: %w", harnessstore.ErrConflict)
	}
	if assignment.State.Terminal() {
		var active int
		if err := t.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_reservations WHERE assignment_id=? AND state='ACTIVE'`, string(assignment.ID)).Scan(&active); err != nil {
			return fmt.Errorf("count active provider reservations: %w", err)
		}
		if active != 0 {
			return fmt.Errorf("provider assignment has %d active reservations: %w", active, harnessstore.ErrConflict)
		}
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE provider_assignments
SET session_id=?, plan_digest=?, state=?, revision=?, updated_at=?
WHERE id=? AND revision=?`, string(assignment.SessionID), assignment.PlanDigest, string(assignment.State), assignment.Revision, formatTime(assignment.UpdatedAt), string(assignment.ID), expectedRevision)
	if err != nil {
		return fmt.Errorf("compare-and-swap provider assignment: %w", err)
	}
	if err := requireOneAffected(res); err != nil {
		return harnessstore.ErrConflict
	}
	return nil
}

func scanProviderAssignment(row interface{ Scan(...any) error }) (harnessmodel.ProviderAssignment, error) {
	var assignment harnessmodel.ProviderAssignment
	var id, attemptID, accountID, modelID, sessionID, planDigest, state, createdAt, updatedAt string
	if err := row.Scan(&id, &attemptID, &accountID, &modelID, &sessionID, &planDigest, &state, &assignment.Revision, &createdAt, &updatedAt); err != nil {
		return harnessmodel.ProviderAssignment{}, mapNotFound(err)
	}
	assignment.ID = harnessmodel.ProviderAssignmentID(id)
	assignment.AttemptID = harnessmodel.AttemptID(attemptID)
	assignment.AccountID = harnessmodel.ProviderAccountID(accountID)
	assignment.ModelID = harnessmodel.ProviderModelID(modelID)
	assignment.SessionID = harnessmodel.ProviderSessionID(sessionID)
	assignment.PlanDigest = planDigest
	assignment.State = harnessmodel.ProviderAssignmentState(state)
	var err error
	if assignment.CreatedAt, err = parseTime(createdAt); err != nil {
		return harnessmodel.ProviderAssignment{}, fmt.Errorf("parse provider assignment created_at: %w", err)
	}
	if assignment.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return harnessmodel.ProviderAssignment{}, fmt.Errorf("parse provider assignment updated_at: %w", err)
	}
	return assignment, nil
}

const providerReservationSelect = `
SELECT id, assignment_id, account_id, window_id, model_id, metric, amount, state, revision, created_at, expires_at, updated_at
FROM provider_reservations`

func (t *transaction) GetProviderReservation(ctx context.Context, id harnessmodel.ProviderReservationID) (harnessmodel.ProviderReservation, error) {
	if id == "" {
		return harnessmodel.ProviderReservation{}, fmt.Errorf("provider reservation id is required")
	}
	return scanProviderReservation(t.tx.QueryRowContext(ctx, providerReservationSelect+` WHERE id=?`, string(id)))
}

func (t *transaction) ListProviderReservationsByAssignment(ctx context.Context, assignmentID harnessmodel.ProviderAssignmentID) ([]harnessmodel.ProviderReservation, error) {
	if assignmentID == "" {
		return nil, fmt.Errorf("provider assignment id is required")
	}
	rows, err := t.tx.QueryContext(ctx, providerReservationSelect+` WHERE assignment_id=? ORDER BY created_at, id`, string(assignmentID))
	if err != nil {
		return nil, fmt.Errorf("list provider reservations: %w", err)
	}
	defer rows.Close()
	return scanProviderReservations(rows)
}

func (t *transaction) ListActiveProviderReservations(ctx context.Context, accountID harnessmodel.ProviderAccountID, now time.Time, limit int) ([]harnessmodel.ProviderReservation, error) {
	if accountID == "" || now.IsZero() {
		return nil, fmt.Errorf("provider account id and current time are required")
	}
	nowNS, err := checkedUnixNano(now)
	if err != nil {
		return nil, fmt.Errorf("invalid provider reservation current time: %w", err)
	}
	limit = normalizedProviderLimit(limit)
	rows, err := t.tx.QueryContext(ctx, providerReservationSelect+`
WHERE account_id=? AND state='ACTIVE' AND expires_at_ns>?
ORDER BY expires_at_ns, id LIMIT ?`, string(accountID), nowNS, limit)
	if err != nil {
		return nil, fmt.Errorf("list active provider reservations: %w", err)
	}
	defer rows.Close()
	return scanProviderReservations(rows)
}

func (t *transaction) ListDueProviderReservationExpirations(ctx context.Context, now time.Time, limit int) ([]harnessmodel.ProviderReservation, error) {
	if now.IsZero() {
		return nil, fmt.Errorf("provider reservation expiry time is required")
	}
	nowNS, err := checkedUnixNano(now)
	if err != nil {
		return nil, fmt.Errorf("invalid provider reservation expiry time: %w", err)
	}
	limit = normalizedProviderLimit(limit)
	rows, err := t.tx.QueryContext(ctx, providerReservationSelect+`
WHERE state='ACTIVE' AND expires_at_ns<=?
ORDER BY expires_at_ns, account_id, id LIMIT ?`, nowNS, limit)
	if err != nil {
		return nil, fmt.Errorf("list due provider reservation expirations: %w", err)
	}
	defer rows.Close()
	return scanProviderReservations(rows)
}

func (t *transaction) CreateProviderReservation(ctx context.Context, reservation harnessmodel.ProviderReservation) error {
	if err := reservation.Validate(); err != nil {
		return err
	}
	if reservation.State != harnessmodel.ProviderReservationActive || reservation.Revision != 1 {
		return fmt.Errorf("provider reservation must start ACTIVE at revision 1")
	}
	assignment, err := t.GetProviderAssignment(ctx, reservation.AssignmentID)
	if err != nil {
		return err
	}
	if assignment.State != harnessmodel.ProviderAssignmentActive || assignment.AccountID != reservation.AccountID {
		return fmt.Errorf("provider reservation assignment is inactive or belongs to another account: %w", harnessstore.ErrConflict)
	}
	if reservation.ModelID != "" && reservation.ModelID != assignment.ModelID {
		return fmt.Errorf("provider reservation model differs from assignment model: %w", harnessstore.ErrConflict)
	}
	expiresNS, err := checkedUnixNano(reservation.ExpiresAt)
	if err != nil {
		return fmt.Errorf("provider reservation expiry is outside durable range: %w", err)
	}
	_, err = t.tx.ExecContext(ctx, `
INSERT INTO provider_reservations(
    id, assignment_id, account_id, window_id, model_id, metric, amount, state,
    revision, created_at, expires_at, expires_at_ns, updated_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, string(reservation.ID), string(reservation.AssignmentID), string(reservation.AccountID), reservation.WindowID,
		string(reservation.ModelID), string(reservation.Metric), reservation.Amount, string(reservation.State), reservation.Revision,
		formatTime(reservation.CreatedAt), formatTime(reservation.ExpiresAt), expiresNS, formatTime(reservation.UpdatedAt))
	if err != nil {
		return fmt.Errorf("insert provider reservation %s: %w", reservation.ID, err)
	}
	return nil
}

func (t *transaction) CompareAndSwapProviderReservation(ctx context.Context, expectedRevision uint64, reservation harnessmodel.ProviderReservation) error {
	if expectedRevision == 0 || expectedRevision >= maxSQLiteIntegerRevision || reservation.Revision != expectedRevision+1 {
		return fmt.Errorf("invalid provider reservation revision transition %d -> %d", expectedRevision, reservation.Revision)
	}
	if err := reservation.Validate(); err != nil {
		return err
	}
	current, err := t.GetProviderReservation(ctx, reservation.ID)
	if err != nil {
		return err
	}
	if current.Revision != expectedRevision {
		return harnessstore.ErrConflict
	}
	if reservation.AssignmentID != current.AssignmentID || reservation.AccountID != current.AccountID || reservation.WindowID != current.WindowID ||
		reservation.ModelID != current.ModelID || reservation.Metric != current.Metric || reservation.Amount != current.Amount ||
		!reservation.CreatedAt.Equal(current.CreatedAt) || !reservation.ExpiresAt.Equal(current.ExpiresAt) {
		return fmt.Errorf("provider reservation immutable fields changed: %w", harnessstore.ErrConflict)
	}
	if !harnessmodel.ValidProviderReservationTransition(current.State, reservation.State) {
		return fmt.Errorf("invalid provider reservation state transition %s -> %s", current.State, reservation.State)
	}
	if reservation.UpdatedAt.Before(current.UpdatedAt) {
		return fmt.Errorf("provider reservation update is stale: %w", harnessstore.ErrConflict)
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE provider_reservations SET state=?, revision=?, updated_at=? WHERE id=? AND revision=?`,
		string(reservation.State), reservation.Revision, formatTime(reservation.UpdatedAt), string(reservation.ID), expectedRevision)
	if err != nil {
		return fmt.Errorf("compare-and-swap provider reservation: %w", err)
	}
	if err := requireOneAffected(res); err != nil {
		return harnessstore.ErrConflict
	}
	return nil
}

func scanProviderReservation(row interface{ Scan(...any) error }) (harnessmodel.ProviderReservation, error) {
	var reservation harnessmodel.ProviderReservation
	var id, assignmentID, accountID, modelID, metric, state, createdAt, expiresAt, updatedAt string
	if err := row.Scan(&id, &assignmentID, &accountID, &reservation.WindowID, &modelID, &metric, &reservation.Amount, &state,
		&reservation.Revision, &createdAt, &expiresAt, &updatedAt); err != nil {
		return harnessmodel.ProviderReservation{}, mapNotFound(err)
	}
	reservation.ID = harnessmodel.ProviderReservationID(id)
	reservation.AssignmentID = harnessmodel.ProviderAssignmentID(assignmentID)
	reservation.AccountID = harnessmodel.ProviderAccountID(accountID)
	reservation.ModelID = harnessmodel.ProviderModelID(modelID)
	reservation.Metric = harnessmodel.QuotaMetricKind(metric)
	reservation.State = harnessmodel.ProviderReservationState(state)
	var err error
	if reservation.CreatedAt, err = parseTime(createdAt); err != nil {
		return harnessmodel.ProviderReservation{}, fmt.Errorf("parse provider reservation created_at: %w", err)
	}
	if reservation.ExpiresAt, err = parseTime(expiresAt); err != nil {
		return harnessmodel.ProviderReservation{}, fmt.Errorf("parse provider reservation expires_at: %w", err)
	}
	if reservation.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return harnessmodel.ProviderReservation{}, fmt.Errorf("parse provider reservation updated_at: %w", err)
	}
	return reservation, nil
}

func scanProviderReservations(rows *sql.Rows) ([]harnessmodel.ProviderReservation, error) {
	var out []harnessmodel.ProviderReservation
	for rows.Next() {
		reservation, err := scanProviderReservation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, reservation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider reservations: %w", err)
	}
	return out, nil
}

const providerUsageSelect = `
SELECT sample_key, assignment_id, reservation_id, account_id, model_id, metric, amount, observed_at, created_at
FROM provider_usage_samples`

func (t *transaction) GetProviderUsageSample(ctx context.Context, key string) (harnessmodel.ProviderUsageSample, error) {
	if key == "" {
		return harnessmodel.ProviderUsageSample{}, fmt.Errorf("provider usage sample key is required")
	}
	return scanProviderUsageSample(t.tx.QueryRowContext(ctx, providerUsageSelect+` WHERE sample_key=?`, key))
}

func (t *transaction) ListProviderUsageSamplesByAssignment(ctx context.Context, assignmentID harnessmodel.ProviderAssignmentID, limit int) ([]harnessmodel.ProviderUsageSample, error) {
	if assignmentID == "" {
		return nil, fmt.Errorf("provider assignment id is required")
	}
	limit = normalizedProviderLimit(limit)
	rows, err := t.tx.QueryContext(ctx, providerUsageSelect+` WHERE assignment_id=? ORDER BY observed_at, sample_key LIMIT ?`, string(assignmentID), limit)
	if err != nil {
		return nil, fmt.Errorf("list provider usage samples: %w", err)
	}
	defer rows.Close()
	var out []harnessmodel.ProviderUsageSample
	for rows.Next() {
		sample, err := scanProviderUsageSample(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sample)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider usage samples: %w", err)
	}
	return out, nil
}

func (t *transaction) PutProviderUsageSample(ctx context.Context, sample harnessmodel.ProviderUsageSample) (harnessmodel.ProviderUsageSample, bool, error) {
	if err := sample.Validate(); err != nil {
		return harnessmodel.ProviderUsageSample{}, false, err
	}
	assignment, err := t.GetProviderAssignment(ctx, sample.AssignmentID)
	if err != nil {
		return harnessmodel.ProviderUsageSample{}, false, err
	}
	if assignment.AccountID != sample.AccountID {
		return harnessmodel.ProviderUsageSample{}, false, fmt.Errorf("provider usage account differs from assignment: %w", harnessstore.ErrConflict)
	}
	if sample.ModelID != "" && sample.ModelID != assignment.ModelID {
		return harnessmodel.ProviderUsageSample{}, false, fmt.Errorf("provider usage model differs from assignment: %w", harnessstore.ErrConflict)
	}
	var reservationID any
	if sample.ReservationID != "" {
		reservation, err := t.GetProviderReservation(ctx, sample.ReservationID)
		if err != nil {
			return harnessmodel.ProviderUsageSample{}, false, err
		}
		if reservation.AssignmentID != sample.AssignmentID || reservation.AccountID != sample.AccountID || reservation.Metric != sample.Metric {
			return harnessmodel.ProviderUsageSample{}, false, fmt.Errorf("provider usage reservation linkage mismatch: %w", harnessstore.ErrConflict)
		}
		if reservation.ModelID != "" && sample.ModelID != "" && reservation.ModelID != sample.ModelID {
			return harnessmodel.ProviderUsageSample{}, false, fmt.Errorf("provider usage reservation model mismatch: %w", harnessstore.ErrConflict)
		}
		reservationID = string(sample.ReservationID)
	}
	res, err := t.tx.ExecContext(ctx, `
INSERT INTO provider_usage_samples(sample_key, assignment_id, reservation_id, account_id, model_id, metric, amount, observed_at, created_at)
VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(sample_key) DO NOTHING`, sample.Key, string(sample.AssignmentID), reservationID, string(sample.AccountID),
		string(sample.ModelID), string(sample.Metric), sample.Amount, formatTime(sample.ObservedAt), formatTime(sample.CreatedAt))
	if err != nil {
		return harnessmodel.ProviderUsageSample{}, false, fmt.Errorf("insert provider usage sample %q: %w", sample.Key, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return harnessmodel.ProviderUsageSample{}, false, fmt.Errorf("read provider usage insert count: %w", err)
	}
	if n == 1 {
		return sample, true, nil
	}
	existing, err := t.GetProviderUsageSample(ctx, sample.Key)
	if err != nil {
		return harnessmodel.ProviderUsageSample{}, false, err
	}
	if !sameProviderUsageSemantics(existing, sample) {
		return existing, false, fmt.Errorf("provider usage sample key %q was replayed with different data: %w", sample.Key, harnessstore.ErrConflict)
	}
	return existing, false, nil
}

func scanProviderUsageSample(row interface{ Scan(...any) error }) (harnessmodel.ProviderUsageSample, error) {
	var sample harnessmodel.ProviderUsageSample
	var assignmentID, accountID, modelID, metric, observedAt, createdAt string
	var reservationID sql.NullString
	if err := row.Scan(&sample.Key, &assignmentID, &reservationID, &accountID, &modelID, &metric, &sample.Amount, &observedAt, &createdAt); err != nil {
		return harnessmodel.ProviderUsageSample{}, mapNotFound(err)
	}
	sample.AssignmentID = harnessmodel.ProviderAssignmentID(assignmentID)
	if reservationID.Valid {
		sample.ReservationID = harnessmodel.ProviderReservationID(reservationID.String)
	}
	sample.AccountID = harnessmodel.ProviderAccountID(accountID)
	sample.ModelID = harnessmodel.ProviderModelID(modelID)
	sample.Metric = harnessmodel.QuotaMetricKind(metric)
	var err error
	if sample.ObservedAt, err = parseTime(observedAt); err != nil {
		return harnessmodel.ProviderUsageSample{}, fmt.Errorf("parse provider usage observed_at: %w", err)
	}
	if sample.CreatedAt, err = parseTime(createdAt); err != nil {
		return harnessmodel.ProviderUsageSample{}, fmt.Errorf("parse provider usage created_at: %w", err)
	}
	return sample, nil
}

func sameProviderUsageSemantics(a, b harnessmodel.ProviderUsageSample) bool {
	return a.Key == b.Key && a.AssignmentID == b.AssignmentID && a.ReservationID == b.ReservationID && a.AccountID == b.AccountID &&
		a.ModelID == b.ModelID && a.Metric == b.Metric && a.Amount == b.Amount && a.ObservedAt.Equal(b.ObservedAt)
}

const providerCircuitSelect = `
SELECT account_id, model_id, revision, state, consecutive_failures, opened_at, next_probe_at, probe_in_flight, updated_at
FROM provider_circuit_state`

func (t *transaction) GetProviderCircuitState(ctx context.Context, accountID harnessmodel.ProviderAccountID, modelID harnessmodel.ProviderModelID) (harnessmodel.ProviderCircuitState, error) {
	if accountID == "" {
		return harnessmodel.ProviderCircuitState{}, fmt.Errorf("provider account id is required")
	}
	return scanProviderCircuitState(t.tx.QueryRowContext(ctx, providerCircuitSelect+` WHERE account_id=? AND model_id=?`, string(accountID), string(modelID)))
}

func (t *transaction) CreateProviderCircuitState(ctx context.Context, state harnessmodel.ProviderCircuitState) error {
	if err := state.Validate(); err != nil {
		return err
	}
	if state.Revision != 1 {
		return fmt.Errorf("provider circuit must start at revision 1")
	}
	_, err := t.tx.ExecContext(ctx, `
INSERT INTO provider_circuit_state(account_id, model_id, revision, state, consecutive_failures, opened_at, next_probe_at, probe_in_flight, updated_at)
VALUES(?,?,?,?,?,?,?,?,?)`, string(state.AccountID), string(state.ModelID), state.Revision, string(state.State), state.ConsecutiveFailures,
		nullTime(state.OpenedAt), nullTime(state.NextProbeAt), boolInt(state.ProbeInFlight), formatTime(state.UpdatedAt))
	if err != nil {
		return fmt.Errorf("insert provider circuit state: %w", err)
	}
	return nil
}

func (t *transaction) CompareAndSwapProviderCircuitState(ctx context.Context, expectedRevision uint64, state harnessmodel.ProviderCircuitState) error {
	if expectedRevision == 0 || expectedRevision >= maxSQLiteIntegerRevision || state.Revision != expectedRevision+1 {
		return fmt.Errorf("invalid provider circuit revision transition %d -> %d", expectedRevision, state.Revision)
	}
	if err := state.Validate(); err != nil {
		return err
	}
	current, err := t.GetProviderCircuitState(ctx, state.AccountID, state.ModelID)
	if err != nil {
		return err
	}
	if current.Revision != expectedRevision || state.UpdatedAt.Before(current.UpdatedAt) {
		return harnessstore.ErrConflict
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE provider_circuit_state
SET revision=?, state=?, consecutive_failures=?, opened_at=?, next_probe_at=?, probe_in_flight=?, updated_at=?
WHERE account_id=? AND model_id=? AND revision=?`, state.Revision, string(state.State), state.ConsecutiveFailures,
		nullTime(state.OpenedAt), nullTime(state.NextProbeAt), boolInt(state.ProbeInFlight), formatTime(state.UpdatedAt),
		string(state.AccountID), string(state.ModelID), expectedRevision)
	if err != nil {
		return fmt.Errorf("compare-and-swap provider circuit state: %w", err)
	}
	if err := requireOneAffected(res); err != nil {
		return harnessstore.ErrConflict
	}
	return nil
}

func scanProviderCircuitState(row interface{ Scan(...any) error }) (harnessmodel.ProviderCircuitState, error) {
	var state harnessmodel.ProviderCircuitState
	var accountID, modelID, circuitState, updatedAt string
	var openedAt, nextProbeAt sql.NullString
	var probe int
	if err := row.Scan(&accountID, &modelID, &state.Revision, &circuitState, &state.ConsecutiveFailures, &openedAt, &nextProbeAt, &probe, &updatedAt); err != nil {
		return harnessmodel.ProviderCircuitState{}, mapNotFound(err)
	}
	state.AccountID = harnessmodel.ProviderAccountID(accountID)
	state.ModelID = harnessmodel.ProviderModelID(modelID)
	state.State = harnessmodel.CircuitState(circuitState)
	state.ProbeInFlight = probe != 0
	var err error
	if openedAt.Valid {
		if state.OpenedAt, err = parseTime(openedAt.String); err != nil {
			return harnessmodel.ProviderCircuitState{}, fmt.Errorf("parse provider circuit opened_at: %w", err)
		}
	}
	if nextProbeAt.Valid {
		if state.NextProbeAt, err = parseTime(nextProbeAt.String); err != nil {
			return harnessmodel.ProviderCircuitState{}, fmt.Errorf("parse provider circuit next_probe_at: %w", err)
		}
	}
	if state.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return harnessmodel.ProviderCircuitState{}, fmt.Errorf("parse provider circuit updated_at: %w", err)
	}
	return state, nil
}

func normalizedProviderLimit(limit int) int {
	if limit <= 0 || limit > 10000 {
		return 1000
	}
	return limit
}
