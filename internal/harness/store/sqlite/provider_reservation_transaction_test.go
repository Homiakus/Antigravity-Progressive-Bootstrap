package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/provider/demand"
	providerreservation "github.com/homiakus/agctl/internal/harness/provider/reservation"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestAtomicProviderReservationSecondClaimFailureRollsBackWholeTransaction(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(33_000, 0).UTC()
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/a-first", reservationTestModel, 100, now, nil),
		tokenWindow("tokens/z-fail", "", 100, now, nil),
	})
	if _, err := db.SQLDB().ExecContext(ctx, `
CREATE TRIGGER fail_second_provider_reservation
BEFORE INSERT ON provider_reservations
WHEN NEW.window_id='tokens/z-fail'
BEGIN
  SELECT RAISE(ABORT, 'injected second reservation write failure');
END`); err != nil {
		t.Fatal(err)
	}

	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	if _, err := service.Reserve(ctx, reservationRequest(testProviderAttemptID, "pasn_second_write_rollback", 10, now)); err == nil {
		t.Fatal("injected second reservation write failure unexpectedly succeeded")
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		if _, err := r.GetProviderAssignment(ctx, "pasn_second_write_rollback"); !errors.Is(err, harnessstore.ErrNotFound) {
			t.Fatalf("assignment survived partial multi-window write: %v", err)
		}
		claims, err := r.ListAllActiveProviderReservations(ctx, testProviderAccountID, now)
		if err != nil {
			return err
		}
		for _, claim := range claims {
			if claim.AssignmentID == "pasn_second_write_rollback" {
				t.Fatalf("first claim survived second claim failure: %+v", claim)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAtomicProviderReservationPreservesFractionNativeUnit(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(34_000, 0).UTC()
	fraction := 0.75
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{{
		ID: "fraction/provider", ModelID: reservationTestModel,
		Metric: harnessmodel.QuotaMetricFraction, RemainingFraction: &fraction,
		ObservedAt: now, Confidence: 1,
	}})
	createReservationAttempts(t, db, now, "att_fraction_second")

	fractionDemand := func(amount float64) demand.Estimate {
		return demand.Estimate{
			Key: demand.Key{
				TaskClass: "review", Provider: harnessmodel.ProviderCodex,
				ModelID: reservationTestModel, RepositoryID: "repo-test", ContextClass: "small",
				Metric: harnessmodel.QuotaMetricFraction,
			},
			Available: true, Source: demand.SourceExact, SampleCount: 10,
			P50: amount / 2, P80: amount, Reservation: amount, Confidence: 1,
		}
	}
	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	firstReq := reservationRequest(testProviderAttemptID, "pasn_fraction_first", 0, now)
	firstReq.Demand = fractionDemand(0.50)
	first, err := service.Reserve(ctx, firstReq)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Reservations) != 1 || first.Reservations[0].Metric != harnessmodel.QuotaMetricFraction || first.Reservations[0].Amount != 0.50 {
		t.Fatalf("fraction claim lost native unit: %+v", first)
	}

	secondReq := reservationRequest("att_fraction_second", "pasn_fraction_second", 0, now)
	secondReq.Demand = fractionDemand(0.26)
	if _, err := service.Reserve(ctx, secondReq); !errors.Is(err, providerreservation.ErrInsufficientCapacity) {
		t.Fatalf("fraction oversubscription error=%v want insufficient capacity", err)
	}
}
