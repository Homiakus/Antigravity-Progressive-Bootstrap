package engine

import (
	"context"
	"fmt"
	"time"

	harnesslease "github.com/homiakus/agctl/internal/harness/lease"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type ClaimResult struct {
	WorkflowRun harnessmodel.WorkflowRun `json:"workflowRun"`
	NodeRun     harnessmodel.NodeRun     `json:"nodeRun"`
	Attempt     harnessmodel.Attempt     `json:"attempt"`
	Lease       harnessmodel.Lease       `json:"lease"`
}

func (e *Engine) ClaimNode(ctx context.Context, nodeRunID harnessmodel.NodeRunID, workerID harnessmodel.WorkerID, ttl time.Duration) (ClaimResult, error) {
	if nodeRunID == "" || workerID == "" {
		return ClaimResult{}, fmt.Errorf("node run id and worker id are required")
	}
	ttl, err := harnesslease.NormalizeTTL(ttl)
	if err != nil {
		return ClaimResult{}, err
	}
	now := e.now().UTC()
	var result ClaimResult
	err = e.store.Update(ctx, func(tx harnessstore.Tx) error {
		worker, err := tx.GetWorker(ctx, workerID)
		if err != nil {
			return err
		}
		if worker.State != harnessmodel.WorkerActive {
			return fmt.Errorf("worker %s is %s, not ACTIVE", workerID, worker.State)
		}
		nr, err := tx.GetNodeRun(ctx, nodeRunID)
		if err != nil {
			return err
		}
		run, err := tx.GetWorkflowRun(ctx, nr.WorkflowRunID)
		if err != nil {
			return err
		}
		if run.State != harnessmodel.WorkflowRunning {
			return fmt.Errorf("cannot claim node %s while workflow %s is %s", nr.ID, run.ID, run.State)
		}
		if nr.State != harnessmodel.NodeReady {
			return fmt.Errorf("cannot claim node %s from state %s", nr.ID, nr.State)
		}
		if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeQueued, now); err != nil {
			return err
		}

		rawAttemptID, err := e.nextID(harnessmodel.IDAttempt)
		if err != nil {
			return err
		}
		attempt, err := tx.CreateNextAttempt(ctx, harnessmodel.Attempt{
			ID: harnessmodel.AttemptID(rawAttemptID), NodeRunID: nr.ID,
			State: harnessmodel.AttemptCreated, WorkerID: workerID, LeaseEpoch: 1, CreatedAt: now,
		})
		if err != nil {
			return err
		}
		if err := transitionAttempt(ctx, tx, &attempt, harnessmodel.AttemptClaimed); err != nil {
			return err
		}

		rawLeaseID, err := e.nextID(harnessmodel.IDLease)
		if err != nil {
			return err
		}
		lease := harnessmodel.Lease{
			ID: harnessmodel.LeaseID(rawLeaseID), AttemptID: attempt.ID, WorkerID: workerID,
			Epoch: 1, State: harnessmodel.LeaseActive, ClaimedAt: now, HeartbeatAt: now, ExpiresAt: now.Add(ttl),
		}
		if err := tx.CreateLease(ctx, lease); err != nil {
			return err
		}
		if err := tx.TouchWorker(ctx, workerID, now); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "NodeQueued", "node_run", string(nr.ID), map[string]any{"nodeId": nr.NodeID}); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "AttemptClaimed", "attempt", string(attempt.ID), map[string]any{"nodeRunId": nr.ID, "workerId": workerID, "leaseEpoch": lease.Epoch}); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "LeaseClaimed", "lease", string(lease.ID), map[string]any{"attemptId": attempt.ID, "workerId": workerID, "epoch": lease.Epoch, "expiresAt": lease.ExpiresAt}); err != nil {
			return err
		}
		result = ClaimResult{WorkflowRun: run, NodeRun: nr, Attempt: attempt, Lease: lease}
		return nil
	})
	return result, err
}

func (e *Engine) StartClaimedAttempt(ctx context.Context, attemptID harnessmodel.AttemptID, workerID harnessmodel.WorkerID, epoch uint64) (harnessmodel.Attempt, error) {
	if attemptID == "" || workerID == "" || epoch == 0 {
		return harnessmodel.Attempt{}, fmt.Errorf("attempt id, worker id and lease epoch are required")
	}
	now := e.now().UTC()
	var result harnessmodel.Attempt
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		attempt, err := tx.GetAttempt(ctx, attemptID)
		if err != nil {
			return err
		}
		if attempt.State != harnessmodel.AttemptClaimed {
			return fmt.Errorf("cannot start attempt %s from state %s", attempt.ID, attempt.State)
		}
		if err := authorizeLease(ctx, tx, attempt, workerID, epoch, now); err != nil {
			return err
		}
		nr, err := tx.GetNodeRun(ctx, attempt.NodeRunID)
		if err != nil {
			return err
		}
		if nr.State != harnessmodel.NodeQueued {
			return fmt.Errorf("claimed attempt %s has node %s in state %s", attempt.ID, nr.ID, nr.State)
		}
		if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeRunning, now); err != nil {
			return err
		}
		attempt.StartedAt = now
		if err := transitionAttempt(ctx, tx, &attempt, harnessmodel.AttemptRunning); err != nil {
			return err
		}
		if err := tx.TouchWorker(ctx, workerID, now); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, nr.WorkflowRunID, now, "AttemptStarted", "attempt", string(attempt.ID), map[string]any{"nodeRunId": nr.ID, "workerId": workerID, "leaseEpoch": epoch}); err != nil {
			return err
		}
		result = attempt
		return nil
	})
	return result, err
}

func (e *Engine) HeartbeatLease(ctx context.Context, attemptID harnessmodel.AttemptID, workerID harnessmodel.WorkerID, epoch uint64, ttl time.Duration) (harnessmodel.Lease, error) {
	ttl, err := harnesslease.NormalizeTTL(ttl)
	if err != nil {
		return harnessmodel.Lease{}, err
	}
	now := e.now().UTC()
	var result harnessmodel.Lease
	err = e.store.Update(ctx, func(tx harnessstore.Tx) error {
		attempt, err := tx.GetAttempt(ctx, attemptID)
		if err != nil {
			return err
		}
		if attempt.State != harnessmodel.AttemptClaimed && attempt.State != harnessmodel.AttemptRunning {
			return fmt.Errorf("attempt %s cannot heartbeat from state %s", attempt.ID, attempt.State)
		}
		if err := authorizeLease(ctx, tx, attempt, workerID, epoch, now); err != nil {
			return err
		}
		result, err = tx.RenewLease(ctx, attemptID, workerID, epoch, now, now.Add(ttl))
		if err != nil {
			return err
		}
		return tx.TouchWorker(ctx, workerID, now)
	})
	return result, err
}

func (e *Engine) ReclaimExpiredAttempt(ctx context.Context, attemptID harnessmodel.AttemptID, workerID harnessmodel.WorkerID, ttl time.Duration) (harnessmodel.Lease, error) {
	if attemptID == "" || workerID == "" {
		return harnessmodel.Lease{}, fmt.Errorf("attempt id and worker id are required")
	}
	ttl, err := harnesslease.NormalizeTTL(ttl)
	if err != nil {
		return harnessmodel.Lease{}, err
	}
	now := e.now().UTC()
	var result harnessmodel.Lease
	err = e.store.Update(ctx, func(tx harnessstore.Tx) error {
		worker, err := tx.GetWorker(ctx, workerID)
		if err != nil {
			return err
		}
		if worker.State != harnessmodel.WorkerActive {
			return fmt.Errorf("worker %s is %s, not ACTIVE", workerID, worker.State)
		}
		attempt, err := tx.GetAttempt(ctx, attemptID)
		if err != nil {
			return err
		}
		if attempt.State != harnessmodel.AttemptClaimed && attempt.State != harnessmodel.AttemptRunning {
			return fmt.Errorf("attempt %s cannot be reclaimed from state %s", attempt.ID, attempt.State)
		}
		current, err := tx.GetCurrentLease(ctx, attempt.ID)
		if err != nil {
			return err
		}
		if !harnesslease.Expired(current, now) {
			return fmt.Errorf("attempt %s lease epoch %d is still valid until %s", attempt.ID, current.Epoch, current.ExpiresAt.UTC().Format(time.RFC3339Nano))
		}
		if err := tx.CloseLease(ctx, attempt.ID, current.WorkerID, current.Epoch, harnessmodel.LeaseExpired, now); err != nil {
			return err
		}
		rawLeaseID, err := e.nextID(harnessmodel.IDLease)
		if err != nil {
			return err
		}
		result = harnessmodel.Lease{
			ID: harnessmodel.LeaseID(rawLeaseID), AttemptID: attempt.ID, WorkerID: workerID,
			Epoch: current.Epoch + 1, State: harnessmodel.LeaseActive, ClaimedAt: now, HeartbeatAt: now, ExpiresAt: now.Add(ttl),
		}
		if err := tx.CreateLease(ctx, result); err != nil {
			return err
		}
		attempt.WorkerID = workerID
		attempt.LeaseEpoch = result.Epoch
		if err := tx.CompareAndSwapAttempt(ctx, attempt.State, attempt); err != nil {
			return err
		}
		if err := tx.TouchWorker(ctx, workerID, now); err != nil {
			return err
		}
		nr, err := tx.GetNodeRun(ctx, attempt.NodeRunID)
		if err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, nr.WorkflowRunID, now, "LeaseLost", "lease", string(current.ID), map[string]any{"attemptId": attempt.ID, "workerId": current.WorkerID, "epoch": current.Epoch, "expiredAt": current.ExpiresAt}); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, nr.WorkflowRunID, now, "LeaseReclaimed", "lease", string(result.ID), map[string]any{"attemptId": attempt.ID, "workerId": workerID, "epoch": result.Epoch, "previousEpoch": current.Epoch}); err != nil {
			return err
		}
		return nil
	})
	return result, err
}

func authorizeLease(ctx context.Context, tx harnessstore.Tx, attempt harnessmodel.Attempt, workerID harnessmodel.WorkerID, epoch uint64, now time.Time) error {
	if attempt.WorkerID != workerID || attempt.LeaseEpoch != epoch {
		return harnesslease.ErrStaleFence
	}
	current, err := tx.GetCurrentLease(ctx, attempt.ID)
	if err != nil {
		return harnesslease.ErrStaleFence
	}
	return harnesslease.ValidateAuthority(current, workerID, epoch, now)
}
