package sqlite

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/provider/capacity"
	"github.com/homiakus/agctl/internal/harness/provider/demand"
	providerreservation "github.com/homiakus/agctl/internal/harness/provider/reservation"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

const reservationTestModel harnessmodel.ProviderModelID = "gpt-reservation-test"

func seedReservationBroker(t *testing.T, db *DB, now time.Time, windows []harnessmodel.QuotaWindow) {
	t.Helper()
	seedProviderRuntimeParents(t, db, now)
	ctx := context.Background()
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.UpsertProviderModel(ctx, harnessmodel.ProviderModelDescriptor{
			AccountID: testProviderAccountID,
			Provider:  harnessmodel.ProviderCodex,
			ID:        reservationTestModel,
			Enabled:   true,
		}, now); err != nil {
			return err
		}
		return tx.AppendProviderCapacity(ctx, harnessmodel.ProviderCapacitySnapshot{
			AccountID: testProviderAccountID,
			Provider:  harnessmodel.ProviderCodex,
			Health:    harnessmodel.ProviderHealthHealthy,
			Windows:   windows,
			ObservedAt: now,
		})
	}); err != nil {
		t.Fatal(err)
	}
}

func tokenWindow(id string, modelID harnessmodel.ProviderModelID, remaining float64, observed time.Time, resetAt *time.Time) harnessmodel.QuotaWindow {
	limit := remaining
	return harnessmodel.QuotaWindow{
		ID: id, ModelID: modelID, Metric: harnessmodel.QuotaMetricTokens,
		Limit: &limit, Remaining: &remaining, ObservedAt: observed, Confidence: 1,
		ResetAt: resetAt,
	}
}

func reservationPolicy() providerreservation.Policy {
	return providerreservation.Policy{
		Capacity: capacity.ConservativePolicy(),
		ClaimTTL: 4 * time.Minute,
	}
}

func tokenDemand(amount float64) demand.Estimate {
	return demand.Estimate{
		Key: demand.Key{
			TaskClass: "implement", Provider: harnessmodel.ProviderCodex,
			ModelID: reservationTestModel, RepositoryID: "repo-test", ContextClass: "medium",
			Metric: harnessmodel.QuotaMetricTokens,
		},
		Available: true, Source: demand.SourceExact, SampleCount: 10,
		P50: amount / 2, P80: amount, Reservation: amount, Confidence: 1,
	}
}

func reservationRequest(attemptID harnessmodel.AttemptID, assignmentID harnessmodel.ProviderAssignmentID, amount float64, now time.Time) providerreservation.Request {
	return providerreservation.Request{
		AssignmentID: assignmentID,
		AttemptID: attemptID,
		AccountID: testProviderAccountID,
		ModelID: reservationTestModel,
		Demand: tokenDemand(amount),
		DecisionAt: now,
	}
}

func createReservationAttempts(t *testing.T, db *DB, now time.Time, ids ...harnessmodel.AttemptID) {
	t.Helper()
	ctx := context.Background()
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		for i, id := range ids {
			if id == testProviderAttemptID {
				continue
			}
			if _, err := tx.CreateNextAttempt(ctx, harnessmodel.Attempt{
				ID: id, NodeRunID: "nr_provider", State: harnessmodel.AttemptCreated,
				CreatedAt: now.Add(time.Duration(i+1) * time.Nanosecond),
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAtomicProviderReservationClaimsEveryApplicableWindow(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(20_000, 0).UTC()
	reset := now.Add(30 * time.Second)
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/short", reservationTestModel, 1000, now, nil),
		tokenWindow("tokens/account-daily", "", 600, now, &reset),
		tokenWindow("tokens/other-model", "other-model", 1, now, nil),
	})

	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	got, err := service.Reserve(ctx, reservationRequest(testProviderAttemptID, "pasn_atomic", 200, now))
	if err != nil {
		t.Fatal(err)
	}
	if got.Replayed || len(got.Reservations) != 2 || len(got.Claims) != 2 {
		t.Fatalf("unexpected reservation result: %+v", got)
	}
	seen := map[string]harnessmodel.ProviderReservation{}
	for _, claim := range got.Reservations {
		seen[claim.WindowID] = claim
		if claim.Amount != 200 || claim.Metric != harnessmodel.QuotaMetricTokens {
			t.Fatalf("unexpected claim: %+v", claim)
		}
	}
	if seen["tokens/short"].ModelID != reservationTestModel {
		t.Fatalf("model-scoped claim lost scope: %+v", seen["tokens/short"])
	}
	if seen["tokens/account-daily"].ModelID != "" {
		t.Fatalf("account-wide claim acquired model scope: %+v", seen["tokens/account-daily"])
	}
	if !seen["tokens/account-daily"].ExpiresAt.Equal(reset) {
		t.Fatalf("reset boundary did not cap expiry: got=%s want=%s", seen["tokens/account-daily"].ExpiresAt, reset)
	}
}

func TestAtomicProviderReservationSubtractsCompleteClaimsAndRollsBackOnInsufficientCapacity(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(21_000, 0).UTC()
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/shared", "", 600, now, nil),
	})
	createReservationAttempts(t, db, now, "att_second")
	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	if _, err := service.Reserve(ctx, reservationRequest(testProviderAttemptID, "pasn_first", 300, now)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reserve(ctx, reservationRequest("att_second", "pasn_second", 301, now)); !errors.Is(err, providerreservation.ErrInsufficientCapacity) {
		t.Fatalf("second reservation error=%v want insufficient capacity", err)
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		if _, err := r.GetProviderAssignment(ctx, "pasn_second"); !errors.Is(err, harnessstore.ErrNotFound) {
			t.Fatalf("failed reservation leaked assignment: %v", err)
		}
		claims, err := r.ListAllActiveProviderReservations(ctx, testProviderAccountID, now)
		if err != nil {
			return err
		}
		if len(claims) != 1 || claims[0].Amount != 300 {
			t.Fatalf("unexpected active claims after rollback: %+v", claims)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAtomicProviderReservationReplayAndConflict(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(22_000, 0).UTC()
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/replay", reservationTestModel, 500, now, nil),
	})
	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	req := reservationRequest(testProviderAttemptID, "pasn_replay", 100, now)
	first, err := service.Reserve(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Reserve(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replayed || len(second.Reservations) != 1 || second.Reservations[0].ID != first.Reservations[0].ID {
		t.Fatalf("idempotent replay mismatch: first=%+v second=%+v", first, second)
	}
	conflict := req
	conflict.AssignmentID = "pasn_other"
	if _, err := service.Reserve(ctx, conflict); !errors.Is(err, providerreservation.ErrReservationConflict) {
		t.Fatalf("conflicting replay error=%v want conflict", err)
	}
}

func TestAtomicProviderReservationZeroDemandCreatesAssignmentOnly(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(23_000, 0).UTC()
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/zero", reservationTestModel, 100, now, nil),
	})
	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	got, err := service.Reserve(ctx, reservationRequest(testProviderAttemptID, "pasn_zero", 0, now))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Reservations) != 0 || got.Assignment.ID != "pasn_zero" {
		t.Fatalf("zero-demand reservation should be assignment-only: %+v", got)
	}
	claims, err := func() ([]harnessmodel.ProviderReservation, error) {
		var out []harnessmodel.ProviderReservation
		err := db.View(ctx, func(r harnessstore.Reader) error {
			var err error
			out, err = r.ListAllActiveProviderReservations(ctx, testProviderAccountID, now)
			return err
		})
		return out, err
	}()
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 0 {
		t.Fatalf("zero-demand assignment created durable claim: %+v", claims)
	}
}

func TestAtomicProviderReservationWriteFailureRollsBackAssignment(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(24_000, 0).UTC()
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/rollback", reservationTestModel, 100, now, nil),
	})
	if _, err := db.SQLDB().ExecContext(ctx, `
CREATE TRIGGER fail_provider_reservation
BEFORE INSERT ON provider_reservations
BEGIN
  SELECT RAISE(ABORT, 'injected reservation write failure');
END`); err != nil {
		t.Fatal(err)
	}
	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	if _, err := service.Reserve(ctx, reservationRequest(testProviderAttemptID, "pasn_rollback", 10, now)); err == nil {
		t.Fatal("injected reservation write failure unexpectedly succeeded")
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		if _, err := r.GetProviderAssignment(ctx, "pasn_rollback"); !errors.Is(err, harnessstore.ErrNotFound) {
			t.Fatalf("assignment survived failed reservation transaction: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAtomicProviderReservationRejectsStaleCapacity(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	observed := time.Unix(25_000, 0).UTC()
	seedReservationBroker(t, db, observed, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/stale", reservationTestModel, 100, observed, nil),
	})
	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	decision := observed.Add(reservationPolicy().Capacity.ExpireAfter + time.Second)
	if _, err := service.Reserve(ctx, reservationRequest(testProviderAttemptID, "pasn_stale", 1, decision)); !errors.Is(err, providerreservation.ErrCapacityUnavailable) {
		t.Fatalf("stale capacity error=%v want capacity unavailable", err)
	}
}

func TestAtomicProviderReservationConcurrentClaimsNeverOversubscribe(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(26_000, 0).UTC()
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/concurrent", "", 100, now, nil),
	})

	const workers = 100
	ids := make([]harnessmodel.AttemptID, 0, workers-1)
	for i := 1; i < workers; i++ {
		ids = append(ids, harnessmodel.AttemptID(fmt.Sprintf("att_concurrent_%03d", i)))
	}
	createReservationAttempts(t, db, now, ids...)

	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	var success atomic.Int64
	var insufficient atomic.Int64
	var unexpected atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		attemptID := testProviderAttemptID
		if i > 0 {
			attemptID = harnessmodel.AttemptID(fmt.Sprintf("att_concurrent_%03d", i))
		}
		wg.Add(1)
		go func(i int, attemptID harnessmodel.AttemptID) {
			defer wg.Done()
			assignmentID := harnessmodel.ProviderAssignmentID(fmt.Sprintf("pasn_concurrent_%03d", i))
			_, err := service.Reserve(ctx, reservationRequest(attemptID, assignmentID, 10, now))
			switch {
			case err == nil:
				success.Add(1)
			case errors.Is(err, providerreservation.ErrInsufficientCapacity):
				insufficient.Add(1)
			default:
				unexpected.Add(1)
				t.Logf("unexpected concurrent reservation error: %v", err)
			}
		}(i, attemptID)
	}
	wg.Wait()
	if got := success.Load(); got != 10 {
		t.Fatalf("successful reservations=%d want=10; insufficient=%d unexpected=%d", got, insufficient.Load(), unexpected.Load())
	}
	if insufficient.Load() != 90 || unexpected.Load() != 0 {
		t.Fatalf("concurrent outcomes success=%d insufficient=%d unexpected=%d", success.Load(), insufficient.Load(), unexpected.Load())
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		claims, err := r.ListAllActiveProviderReservations(ctx, testProviderAccountID, now)
		if err != nil {
			return err
		}
		var total float64
		for _, claim := range claims {
			if claim.WindowID == "tokens/concurrent" {
				total += claim.Amount
			}
		}
		if total != 100 {
			t.Fatalf("active reservation total=%v want=100; claims=%d", total, len(claims))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
