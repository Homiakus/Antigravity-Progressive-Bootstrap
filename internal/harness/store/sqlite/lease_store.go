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

func (t *transaction) UpsertWorker(ctx context.Context, worker harnessmodel.Worker) error {
	if worker.ID == "" || worker.State == "" || worker.Trust == "" || worker.CreatedAt.IsZero() || worker.LastSeenAt.IsZero() {
		return fmt.Errorf("invalid worker")
	}
	caps, err := json.Marshal(worker.Capabilities)
	if err != nil {
		return fmt.Errorf("marshal worker capabilities: %w", err)
	}
	resources, err := json.Marshal(worker.Resources)
	if err != nil {
		return fmt.Errorf("marshal worker resources: %w", err)
	}
	_, err = t.tx.ExecContext(ctx, `
INSERT INTO workers(id, name, state, trust, capabilities_json, resources_json, created_at, last_seen_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    name=excluded.name,
    state=excluded.state,
    trust=excluded.trust,
    capabilities_json=excluded.capabilities_json,
    resources_json=excluded.resources_json,
    last_seen_at=excluded.last_seen_at`,
		string(worker.ID), worker.Name, string(worker.State), string(worker.Trust), caps, resources, formatTime(worker.CreatedAt), formatTime(worker.LastSeenAt))
	if err != nil {
		return fmt.Errorf("upsert worker: %w", err)
	}
	return nil
}

func (t *transaction) GetWorker(ctx context.Context, id harnessmodel.WorkerID) (harnessmodel.Worker, error) {
	var worker harnessmodel.Worker
	var workerID, state, trust, createdAt, lastSeenAt string
	var caps, resources []byte
	if err := t.tx.QueryRowContext(ctx, `
SELECT id, name, state, trust, capabilities_json, resources_json, created_at, last_seen_at
FROM workers WHERE id=?`, string(id)).Scan(&workerID, &worker.Name, &state, &trust, &caps, &resources, &createdAt, &lastSeenAt); err != nil {
		return harnessmodel.Worker{}, mapNotFound(err)
	}
	worker.ID = harnessmodel.WorkerID(workerID)
	worker.State = harnessmodel.WorkerState(state)
	worker.Trust = harnessmodel.WorkerTrust(trust)
	if err := json.Unmarshal(caps, &worker.Capabilities); err != nil {
		return harnessmodel.Worker{}, fmt.Errorf("decode worker capabilities: %w", err)
	}
	if err := json.Unmarshal(resources, &worker.Resources); err != nil {
		return harnessmodel.Worker{}, fmt.Errorf("decode worker resources: %w", err)
	}
	var err error
	if worker.CreatedAt, err = parseTime(createdAt); err != nil {
		return harnessmodel.Worker{}, fmt.Errorf("parse worker created_at: %w", err)
	}
	if worker.LastSeenAt, err = parseTime(lastSeenAt); err != nil {
		return harnessmodel.Worker{}, fmt.Errorf("parse worker last_seen_at: %w", err)
	}
	return worker, nil
}

func (t *transaction) TouchWorker(ctx context.Context, id harnessmodel.WorkerID, seenAt time.Time) error {
	if id == "" || seenAt.IsZero() {
		return fmt.Errorf("worker id and seen time are required")
	}
	res, err := t.tx.ExecContext(ctx, `UPDATE workers SET last_seen_at=? WHERE id=?`, formatTime(seenAt), string(id))
	if err != nil {
		return fmt.Errorf("touch worker: %w", err)
	}
	return requireOneAffected(res)
}

func (t *transaction) CreateLease(ctx context.Context, lease harnessmodel.Lease) error {
	if lease.ID == "" || lease.AttemptID == "" || lease.WorkerID == "" || lease.Epoch == 0 || lease.State != harnessmodel.LeaseActive || lease.ClaimedAt.IsZero() || lease.HeartbeatAt.IsZero() || lease.ExpiresAt.IsZero() || !lease.ExpiresAt.After(lease.HeartbeatAt) {
		return fmt.Errorf("invalid active lease")
	}
	_, err := t.tx.ExecContext(ctx, `
INSERT INTO leases(id, attempt_id, worker_id, epoch, state, claimed_at, heartbeat_at, expires_at, closed_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, NULL)`, string(lease.ID), string(lease.AttemptID), string(lease.WorkerID), lease.Epoch, string(lease.State), formatTime(lease.ClaimedAt), formatTime(lease.HeartbeatAt), formatTime(lease.ExpiresAt))
	if err != nil {
		return fmt.Errorf("insert lease: %w", err)
	}
	return nil
}

func (t *transaction) GetCurrentLease(ctx context.Context, attemptID harnessmodel.AttemptID) (harnessmodel.Lease, error) {
	return scanLease(t.tx.QueryRowContext(ctx, `
SELECT id, attempt_id, worker_id, epoch, state, claimed_at, heartbeat_at, expires_at, closed_at
FROM leases WHERE attempt_id=? AND state=?`, string(attemptID), string(harnessmodel.LeaseActive)))
}

func (t *transaction) RenewLease(ctx context.Context, attemptID harnessmodel.AttemptID, workerID harnessmodel.WorkerID, epoch uint64, heartbeatAt, expiresAt time.Time) (harnessmodel.Lease, error) {
	if attemptID == "" || workerID == "" || epoch == 0 || heartbeatAt.IsZero() || expiresAt.IsZero() || !expiresAt.After(heartbeatAt) {
		return harnessmodel.Lease{}, fmt.Errorf("invalid lease heartbeat")
	}
	lease, err := scanLease(t.tx.QueryRowContext(ctx, `
UPDATE leases
SET heartbeat_at=?, expires_at=?
WHERE attempt_id=? AND worker_id=? AND epoch=? AND state=? AND expires_at>=?
RETURNING id, attempt_id, worker_id, epoch, state, claimed_at, heartbeat_at, expires_at, closed_at`,
		formatTime(heartbeatAt), formatTime(expiresAt), string(attemptID), string(workerID), epoch, string(harnessmodel.LeaseActive), formatTime(heartbeatAt)))
	if err != nil {
		if err == harnessstore.ErrNotFound {
			return harnessmodel.Lease{}, harnessstore.ErrConflict
		}
		return harnessmodel.Lease{}, err
	}
	return lease, nil
}

func (t *transaction) CloseLease(ctx context.Context, attemptID harnessmodel.AttemptID, workerID harnessmodel.WorkerID, epoch uint64, target harnessmodel.LeaseState, closedAt time.Time) error {
	if attemptID == "" || workerID == "" || epoch == 0 || closedAt.IsZero() || (target != harnessmodel.LeaseExpired && target != harnessmodel.LeaseReleased) {
		return fmt.Errorf("invalid lease close")
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE leases SET state=?, closed_at=?
WHERE attempt_id=? AND worker_id=? AND epoch=? AND state=?`, string(target), formatTime(closedAt), string(attemptID), string(workerID), epoch, string(harnessmodel.LeaseActive))
	if err != nil {
		return fmt.Errorf("close lease: %w", err)
	}
	return requireOneAffected(res)
}

func scanLease(row interface{ Scan(...any) error }) (harnessmodel.Lease, error) {
	var lease harnessmodel.Lease
	var id, attemptID, workerID, state, claimedAt, heartbeatAt, expiresAt string
	var closedAt sql.NullString
	if err := row.Scan(&id, &attemptID, &workerID, &lease.Epoch, &state, &claimedAt, &heartbeatAt, &expiresAt, &closedAt); err != nil {
		return harnessmodel.Lease{}, mapNotFound(err)
	}
	lease.ID = harnessmodel.LeaseID(id)
	lease.AttemptID = harnessmodel.AttemptID(attemptID)
	lease.WorkerID = harnessmodel.WorkerID(workerID)
	lease.State = harnessmodel.LeaseState(state)
	var err error
	if lease.ClaimedAt, err = parseTime(claimedAt); err != nil {
		return harnessmodel.Lease{}, fmt.Errorf("parse lease claimed_at: %w", err)
	}
	if lease.HeartbeatAt, err = parseTime(heartbeatAt); err != nil {
		return harnessmodel.Lease{}, fmt.Errorf("parse lease heartbeat_at: %w", err)
	}
	if lease.ExpiresAt, err = parseTime(expiresAt); err != nil {
		return harnessmodel.Lease{}, fmt.Errorf("parse lease expires_at: %w", err)
	}
	if closedAt.Valid && closedAt.String != "" {
		if lease.ClosedAt, err = parseTime(closedAt.String); err != nil {
			return harnessmodel.Lease{}, fmt.Errorf("parse lease closed_at: %w", err)
		}
	}
	return lease, nil
}
