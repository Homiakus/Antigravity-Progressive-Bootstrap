package sqlite

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

const (
	testProviderAttemptID harnessmodel.AttemptID         = "att_provider"
	testProviderAccountID harnessmodel.ProviderAccountID = "pacc_provider"
)

func seedProviderRuntimeParents(t *testing.T, db *DB, now time.Time) {
	t.Helper()
	ctx := context.Background()
	seedRun(t, db, now)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CreateGraphRevision(ctx, harnessmodel.GraphRevision{WorkflowRunID: "wfr_test", Number: 1, CreatedAt: now, Reason: "provider-runtime-test"}); err != nil {
			return err
		}
		if err := tx.CreateNodeRun(ctx, harnessmodel.NodeRun{
			ID: "nr_provider", WorkflowRunID: "wfr_test", NodeID: "a", GraphRevision: 1, Generation: 1,
			State: harnessmodel.NodeReady, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return err
		}
		if _, err := tx.CreateNextAttempt(ctx, harnessmodel.Attempt{
			ID: testProviderAttemptID, NodeRunID: "nr_provider", State: harnessmodel.AttemptCreated, CreatedAt: now,
		}); err != nil {
			return err
		}
		return tx.UpsertProviderAccount(ctx, harnessmodel.ProviderAccount{
			ID: testProviderAccountID, Provider: harnessmodel.ProviderCodex, Name: "test", State: harnessmodel.ProviderAccountActive,
			CreatedAt: now, UpdatedAt: now,
		})
	}); err != nil {
		t.Fatal(err)
	}
}

func TestProviderRuntimeLedgerRoundTripCASAndIdempotency(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(1800, 0).UTC()
	seedProviderRuntimeParents(t, db, now)

	assignment := harnessmodel.ProviderAssignment{
		ID: "pasn_primary", AttemptID: testProviderAttemptID, AccountID: testProviderAccountID, ModelID: "gpt-test",
		State: harnessmodel.ProviderAssignmentActive, Revision: 1, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
	}
	reservation := harnessmodel.ProviderReservation{
		ID: "pres_primary", AssignmentID: assignment.ID, AccountID: assignment.AccountID, WindowID: "bucket/primary", ModelID: assignment.ModelID,
		Metric: harnessmodel.QuotaMetricTokens, Amount: 250, State: harnessmodel.ProviderReservationActive, Revision: 1,
		CreatedAt: now.Add(2 * time.Second), ExpiresAt: now.Add(time.Minute), UpdatedAt: now.Add(2 * time.Second),
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CreateProviderAssignment(ctx, assignment); err != nil {
			return err
		}
		return tx.CreateProviderReservation(ctx, reservation)
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.View(ctx, func(r harnessstore.Reader) error {
		active, err := r.GetActiveProviderAssignment(ctx, testProviderAttemptID)
		if err != nil {
			return err
		}
		if active.ID != assignment.ID || active.Revision != 1 {
			t.Fatalf("unexpected active assignment: %+v", active)
		}
		reservations, err := r.ListActiveProviderReservations(ctx, testProviderAccountID, now.Add(10*time.Second), 10)
		if err != nil {
			return err
		}
		if len(reservations) != 1 || reservations[0].ID != reservation.ID {
			t.Fatalf("unexpected active reservations: %+v", reservations)
		}
		due, err := r.ListDueProviderReservationExpirations(ctx, now.Add(2*time.Minute), 10)
		if err != nil {
			return err
		}
		if len(due) != 1 || due[0].ID != reservation.ID {
			t.Fatalf("unexpected due expirations: %+v", due)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// An assignment cannot become terminal while it still owns an ACTIVE claim.
	blocked := assignment
	blocked.State = harnessmodel.ProviderAssignmentCompleted
	blocked.Revision = 2
	blocked.UpdatedAt = now.Add(3 * time.Second)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.CompareAndSwapProviderAssignment(ctx, 1, blocked)
	}); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("terminal assignment with active reservation error=%v want ErrConflict", err)
	}

	usage := harnessmodel.ProviderUsageSample{
		Key: "codex:event:usage-1", AssignmentID: assignment.ID, ReservationID: reservation.ID, AccountID: assignment.AccountID,
		ModelID: assignment.ModelID, Metric: harnessmodel.QuotaMetricTokens, Amount: 123,
		ObservedAt: now.Add(4 * time.Second), CreatedAt: now.Add(5 * time.Second),
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		stored, created, err := tx.PutProviderUsageSample(ctx, usage)
		if err != nil {
			return err
		}
		if !created || stored.Key != usage.Key {
			t.Fatalf("first usage insert created=%v stored=%+v", created, stored)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	replay := usage
	replay.CreatedAt = now.Add(time.Hour) // local ingestion time may differ on replay.
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		stored, created, err := tx.PutProviderUsageSample(ctx, replay)
		if err != nil {
			return err
		}
		if created || !stored.CreatedAt.Equal(usage.CreatedAt) {
			t.Fatalf("idempotent replay created=%v stored=%+v", created, stored)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	conflictingReplay := replay
	conflictingReplay.Amount++
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		_, _, err := tx.PutProviderUsageSample(ctx, conflictingReplay)
		return err
	}); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("conflicting usage replay error=%v want ErrConflict", err)
	}

	settled := reservation
	settled.State = harnessmodel.ProviderReservationSettled
	settled.Revision = 2
	settled.UpdatedAt = now.Add(6 * time.Second)
	bound := assignment
	bound.SessionID = "pses_runtime"
	bound.Revision = 2
	bound.UpdatedAt = now.Add(7 * time.Second)
	completed := bound
	completed.State = harnessmodel.ProviderAssignmentCompleted
	completed.Revision = 3
	completed.UpdatedAt = now.Add(8 * time.Second)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CompareAndSwapProviderReservation(ctx, 1, settled); err != nil {
			return err
		}
		if err := tx.CompareAndSwapProviderAssignment(ctx, 1, bound); err != nil {
			return err
		}
		return tx.CompareAndSwapProviderAssignment(ctx, 2, completed)
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.CompareAndSwapProviderReservation(ctx, 1, settled)
	}); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("stale reservation CAS error=%v want ErrConflict", err)
	}

	circuit := harnessmodel.ProviderCircuitState{
		AccountID: testProviderAccountID, ModelID: "gpt-test", Revision: 1, State: harnessmodel.CircuitClosed, UpdatedAt: now.Add(9 * time.Second),
	}
	opened := circuit
	opened.Revision = 2
	opened.State = harnessmodel.CircuitOpen
	opened.ConsecutiveFailures = 3
	opened.OpenedAt = now.Add(10 * time.Second)
	opened.NextProbeAt = now.Add(time.Minute)
	opened.UpdatedAt = now.Add(10 * time.Second)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CreateProviderCircuitState(ctx, circuit); err != nil {
			return err
		}
		return tx.CompareAndSwapProviderCircuitState(ctx, 1, opened)
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.CompareAndSwapProviderCircuitState(ctx, 1, opened)
	}); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("stale provider circuit CAS error=%v want ErrConflict", err)
	}

	if err := db.View(ctx, func(r harnessstore.Reader) error {
		history, err := r.ListProviderAssignmentsByAttempt(ctx, testProviderAttemptID)
		if err != nil {
			return err
		}
		if len(history) != 1 || history[0].State != harnessmodel.ProviderAssignmentCompleted || history[0].SessionID != bound.SessionID {
			t.Fatalf("unexpected assignment history: %+v", history)
		}
		samples, err := r.ListProviderUsageSamplesByAssignment(ctx, assignment.ID, 10)
		if err != nil {
			return err
		}
		if len(samples) != 1 || samples[0].Amount != usage.Amount {
			t.Fatalf("unexpected usage samples: %+v", samples)
		}
		gotCircuit, err := r.GetProviderCircuitState(ctx, testProviderAccountID, "gpt-test")
		if err != nil {
			return err
		}
		if gotCircuit.Revision != 2 || gotCircuit.State != harnessmodel.CircuitOpen {
			t.Fatalf("unexpected provider circuit: %+v", gotCircuit)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestProviderRuntimeLedgerSerializesConcurrentClaimsAndCAS(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(1900, 0).UTC()
	seedProviderRuntimeParents(t, db, now)

	var assignmentWins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			candidate := harnessmodel.ProviderAssignment{
				ID: harnessmodel.ProviderAssignmentID("pasn_concurrent_" + time.Unix(int64(i), 0).UTC().Format("150405")),
				AttemptID: testProviderAttemptID, AccountID: testProviderAccountID, ModelID: "gpt-test",
				State: harnessmodel.ProviderAssignmentActive, Revision: 1, CreatedAt: now.Add(time.Duration(i) * time.Nanosecond), UpdatedAt: now.Add(time.Duration(i) * time.Nanosecond),
			}
			err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateProviderAssignment(ctx, candidate) })
			if err == nil {
				assignmentWins.Add(1)
				return
			}
			if !errors.Is(err, harnessstore.ErrConflict) {
				t.Errorf("concurrent assignment error=%v", err)
			}
		}()
	}
	wg.Wait()
	if got := assignmentWins.Load(); got != 1 {
		t.Fatalf("active assignment winners=%d want=1", got)
	}

	var active harnessmodel.ProviderAssignment
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		var err error
		active, err = r.GetActiveProviderAssignment(ctx, testProviderAttemptID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	reservation := harnessmodel.ProviderReservation{
		ID: "pres_concurrent", AssignmentID: active.ID, AccountID: active.AccountID, WindowID: "primary", ModelID: active.ModelID,
		Metric: harnessmodel.QuotaMetricRequests, Amount: 1, State: harnessmodel.ProviderReservationActive, Revision: 1,
		CreatedAt: now.Add(time.Second), ExpiresAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Second),
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateProviderReservation(ctx, reservation) }); err != nil {
		t.Fatal(err)
	}

	released := reservation
	released.State = harnessmodel.ProviderReservationReleased
	released.Revision = 2
	released.UpdatedAt = now.Add(2 * time.Second)
	var casWins atomic.Int32
	wg = sync.WaitGroup{}
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CompareAndSwapProviderReservation(ctx, 1, released) })
			if err == nil {
				casWins.Add(1)
				return
			}
			if !errors.Is(err, harnessstore.ErrConflict) {
				t.Errorf("concurrent reservation CAS error=%v", err)
			}
		}()
	}
	wg.Wait()
	if got := casWins.Load(); got != 1 {
		t.Fatalf("reservation CAS winners=%d want=1", got)
	}
}
