package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func seedLeaseAttempt(t *testing.T, db *DB, now time.Time) {
	t.Helper()
	seedRun(t, db, now)
	if err := db.Update(context.Background(), func(tx harnessstore.Tx) error {
		if err := tx.CreateGraphRevision(context.Background(), harnessmodel.GraphRevision{WorkflowRunID: "wfr_test", Number: 1, CreatedAt: now, Reason: "lease test"}); err != nil {
			return err
		}
		if err := tx.CreateWorkflowProgress(context.Background(), harnessmodel.WorkflowProgress{WorkflowRunID: "wfr_test", TotalNodes: 1, UpdatedAt: now}); err != nil {
			return err
		}
		if err := tx.CreateNodeRun(context.Background(), harnessmodel.NodeRun{ID: "nr_lease", WorkflowRunID: "wfr_test", NodeID: "a", GraphRevision: 1, Generation: 1, State: harnessmodel.NodeQueued, CreatedAt: now, UpdatedAt: now}); err != nil {
			return err
		}
		if _, err := tx.CreateNextAttempt(context.Background(), harnessmodel.Attempt{ID: "att_lease", NodeRunID: "nr_lease", State: harnessmodel.AttemptCreated, CreatedAt: now}); err != nil {
			return err
		}
		if err := tx.UpsertWorker(context.Background(), harnessmodel.Worker{ID: "worker-a", Name: "a", State: harnessmodel.WorkerActive, Trust: harnessmodel.WorkerTrustedLocal, CreatedAt: now, LastSeenAt: now}); err != nil {
			return err
		}
		return tx.UpsertWorker(context.Background(), harnessmodel.Worker{ID: "worker-b", Name: "b", State: harnessmodel.WorkerActive, Trust: harnessmodel.WorkerTrustedLocal, CreatedAt: now, LastSeenAt: now})
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLeaseStoreEnforcesSingleActiveLeaseAndMonotonicEpoch(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(5000, 0).UTC()
	seedLeaseAttempt(t, db, now)
	first := harnessmodel.Lease{ID: "lease_one", AttemptID: "att_lease", WorkerID: "worker-a", Epoch: 1, State: harnessmodel.LeaseActive, ClaimedAt: now, HeartbeatAt: now, ExpiresAt: now.Add(30 * time.Second)}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateLease(ctx, first) }); err != nil {
		t.Fatal(err)
	}
	secondActive := harnessmodel.Lease{ID: "lease_two", AttemptID: "att_lease", WorkerID: "worker-b", Epoch: 2, State: harnessmodel.LeaseActive, ClaimedAt: now, HeartbeatAt: now, ExpiresAt: now.Add(30 * time.Second)}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateLease(ctx, secondActive) }); err == nil {
		t.Fatal("second active lease for one attempt was accepted")
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.CloseLease(ctx, first.AttemptID, first.WorkerID, first.Epoch, harnessmodel.LeaseExpired, now.Add(31*time.Second))
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateLease(ctx, secondActive) }); err != nil {
		t.Fatalf("new epoch after close rejected: %v", err)
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		current, err := reader.GetCurrentLease(ctx, "att_lease")
		if err != nil {
			return err
		}
		if current.ID != secondActive.ID || current.Epoch != 2 || current.WorkerID != "worker-b" {
			t.Fatalf("unexpected current lease: %+v", current)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLeaseStoreRenewRequiresCurrentFenceAndNonExpiredLease(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(6000, 0).UTC()
	seedLeaseAttempt(t, db, now)
	lease := harnessmodel.Lease{ID: "lease_renew", AttemptID: "att_lease", WorkerID: "worker-a", Epoch: 7, State: harnessmodel.LeaseActive, ClaimedAt: now, HeartbeatAt: now, ExpiresAt: now.Add(10 * time.Second)}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateLease(ctx, lease) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		_, err := tx.RenewLease(ctx, lease.AttemptID, "worker-b", lease.Epoch, now.Add(time.Second), now.Add(20*time.Second))
		return err
	}); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("wrong worker renew error=%v want ErrConflict", err)
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		_, err := tx.RenewLease(ctx, lease.AttemptID, lease.WorkerID, lease.Epoch+1, now.Add(time.Second), now.Add(20*time.Second))
		return err
	}); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("wrong epoch renew error=%v want ErrConflict", err)
	}
	var renewed harnessmodel.Lease
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		var err error
		renewed, err = tx.RenewLease(ctx, lease.AttemptID, lease.WorkerID, lease.Epoch, now.Add(5*time.Second), now.Add(35*time.Second))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !renewed.HeartbeatAt.Equal(now.Add(5*time.Second)) || !renewed.ExpiresAt.Equal(now.Add(35*time.Second)) {
		t.Fatalf("lease heartbeat not persisted: %+v", renewed)
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		_, err := tx.RenewLease(ctx, lease.AttemptID, lease.WorkerID, lease.Epoch, now.Add(40*time.Second), now.Add(70*time.Second))
		return err
	}); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("expired lease renew error=%v want ErrConflict", err)
	}
}

func TestLeaseCloseIsFencedAndTerminal(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(7000, 0).UTC()
	seedLeaseAttempt(t, db, now)
	lease := harnessmodel.Lease{ID: "lease_close", AttemptID: "att_lease", WorkerID: "worker-a", Epoch: 1, State: harnessmodel.LeaseActive, ClaimedAt: now, HeartbeatAt: now, ExpiresAt: now.Add(30 * time.Second)}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateLease(ctx, lease) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.CloseLease(ctx, lease.AttemptID, lease.WorkerID, lease.Epoch+1, harnessmodel.LeaseReleased, now.Add(time.Second))
	}); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("stale close error=%v want ErrConflict", err)
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.CloseLease(ctx, lease.AttemptID, lease.WorkerID, lease.Epoch, harnessmodel.LeaseReleased, now.Add(time.Second))
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		_, err := reader.GetCurrentLease(ctx, lease.AttemptID)
		return err
	}); !errors.Is(err, harnessstore.ErrNotFound) {
		t.Fatalf("closed lease still current: %v", err)
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.CloseLease(ctx, lease.AttemptID, lease.WorkerID, lease.Epoch, harnessmodel.LeaseReleased, now.Add(2*time.Second))
	}); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("second close error=%v want ErrConflict", err)
	}
}
