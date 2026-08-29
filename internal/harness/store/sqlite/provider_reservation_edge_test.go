package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	providerreservation "github.com/homiakus/agctl/internal/harness/provider/reservation"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestAtomicProviderReservationExactBoundarySucceeds(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(27_000, 0).UTC()
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/boundary", "", 100, now, nil),
	})
	createReservationAttempts(t, db, now, "att_boundary_second")
	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	if _, err := service.Reserve(ctx, reservationRequest(testProviderAttemptID, "pasn_boundary_first", 40, now)); err != nil {
		t.Fatal(err)
	}
	got, err := service.Reserve(ctx, reservationRequest("att_boundary_second", "pasn_boundary_second", 60, now))
	if err != nil {
		t.Fatalf("exact remaining boundary rejected: %v", err)
	}
	if len(got.Claims) != 1 || got.Claims[0].RemainingAfter != 0 {
		t.Fatalf("exact-boundary claim mismatch: %+v", got.Claims)
	}
}

func TestAtomicProviderReservationExpiredClaimDoesNotConsumeCapacity(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(28_000, 0).UTC()
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/expiry", "", 100, now, nil),
	})
	createReservationAttempts(t, db, now, "att_expiry_second")
	policy := reservationPolicy()
	policy.ClaimTTL = 10 * time.Second
	service := providerreservation.Service{Store: db, Policy: policy}
	if _, err := service.Reserve(ctx, reservationRequest(testProviderAttemptID, "pasn_expiry_first", 100, now)); err != nil {
		t.Fatal(err)
	}
	decision := now.Add(11 * time.Second)
	got, err := service.Reserve(ctx, reservationRequest("att_expiry_second", "pasn_expiry_second", 100, decision))
	if err != nil {
		t.Fatalf("expired claim still consumed capacity: %v", err)
	}
	if len(got.Claims) != 1 || got.Claims[0].AlreadyClaimed != 0 {
		t.Fatalf("expired claim entered complete active set: %+v", got.Claims)
	}
}

func TestAtomicProviderReservationContradictorySameWindowClaimFailsClosed(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(29_000, 0).UTC()
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/contradict", "", 100, now, nil),
	})
	createReservationAttempts(t, db, now, "att_legacy", "att_target")

	legacyAssignment := harnessmodel.ProviderAssignment{
		ID: "pasn_legacy", AttemptID: "att_legacy", AccountID: testProviderAccountID, ModelID: reservationTestModel,
		State: harnessmodel.ProviderAssignmentActive, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	legacyClaim := harnessmodel.ProviderReservation{
		ID: "pres_legacy", AssignmentID: legacyAssignment.ID, AccountID: testProviderAccountID,
		WindowID: "tokens/contradict", ModelID: reservationTestModel, Metric: harnessmodel.QuotaMetricTokens,
		Amount: 10, State: harnessmodel.ProviderReservationActive, Revision: 1,
		CreatedAt: now, ExpiresAt: now.Add(time.Minute), UpdatedAt: now,
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CreateProviderAssignment(ctx, legacyAssignment); err != nil {
			return err
		}
		return tx.CreateProviderReservation(ctx, legacyClaim)
	}); err != nil {
		t.Fatal(err)
	}

	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	_, err := service.Reserve(ctx, reservationRequest("att_target", "pasn_target", 1, now))
	if !errors.Is(err, providerreservation.ErrReservationConflict) {
		t.Fatalf("contradictory same-window dimension error=%v want reservation conflict", err)
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		if _, err := r.GetProviderAssignment(ctx, "pasn_target"); !errors.Is(err, harnessstore.ErrNotFound) {
			t.Fatalf("dimension conflict leaked assignment: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAtomicProviderReservationIDsAreAssignmentScoped(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(30_000, 0).UTC()
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/id-scope", "", 100, now, nil),
	})
	createReservationAttempts(t, db, now, "att_id_second")
	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	first, err := service.Reserve(ctx, reservationRequest(testProviderAttemptID, "pasn_id_first", 10, now))
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Reserve(ctx, reservationRequest("att_id_second", "pasn_id_second", 10, now))
	if err != nil {
		t.Fatal(err)
	}
	if first.Reservations[0].ID == second.Reservations[0].ID {
		t.Fatalf("reservation id ignored assignment identity: %s", first.Reservations[0].ID)
	}
}

func TestAtomicProviderReservationDisabledModelFailsClosed(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(31_000, 0).UTC()
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/disabled", reservationTestModel, 100, now, nil),
	})
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.UpsertProviderModel(ctx, harnessmodel.ProviderModelDescriptor{
			AccountID: testProviderAccountID, Provider: harnessmodel.ProviderCodex,
			ID: reservationTestModel, Enabled: false,
		}, now.Add(time.Second))
	}); err != nil {
		t.Fatal(err)
	}
	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	if _, err := service.Reserve(ctx, reservationRequest(testProviderAttemptID, "pasn_disabled", 1, now.Add(time.Second))); !errors.Is(err, providerreservation.ErrCapacityUnavailable) {
		t.Fatalf("disabled model error=%v want capacity unavailable", err)
	}
}

func TestAtomicProviderReservationCancelledBeforeTransaction(t *testing.T) {
	db := openTestStore(t)
	now := time.Unix(32_000, 0).UTC()
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/cancel", reservationTestModel, 100, now, nil),
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	if _, err := service.Reserve(ctx, reservationRequest(testProviderAttemptID, "pasn_cancel", 1, now)); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request error=%v want context.Canceled", err)
	}
	if err := db.View(context.Background(), func(r harnessstore.Reader) error {
		if _, err := r.GetProviderAssignment(context.Background(), "pasn_cancel"); !errors.Is(err, harnessstore.ErrNotFound) {
			t.Fatalf("cancelled request entered transaction: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
