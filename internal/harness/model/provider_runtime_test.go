package model

import (
	"math"
	"testing"
	"time"
)

func TestProviderRuntimeDomainTransitionsAndNumericBounds(t *testing.T) {
	now := time.Unix(1700, 0).UTC()
	assignment := ProviderAssignment{
		ID: "pasn_1", AttemptID: "att_1", AccountID: "pacc_1", ModelID: "model-x",
		State: ProviderAssignmentActive, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := assignment.Validate(); err != nil {
		t.Fatal(err)
	}
	if !ValidProviderAssignmentTransition(ProviderAssignmentActive, ProviderAssignmentSuperseded) {
		t.Fatal("ACTIVE -> SUPERSEDED should be valid")
	}
	if ValidProviderAssignmentTransition(ProviderAssignmentCompleted, ProviderAssignmentActive) {
		t.Fatal("terminal assignment unexpectedly reopened")
	}

	reservation := ProviderReservation{
		ID: "pres_1", AssignmentID: assignment.ID, AccountID: assignment.AccountID, WindowID: "primary",
		Metric: QuotaMetricTokens, Amount: 100, State: ProviderReservationActive, Revision: 1,
		CreatedAt: now, ExpiresAt: now.Add(time.Minute), UpdatedAt: now,
	}
	if err := reservation.Validate(); err != nil {
		t.Fatal(err)
	}
	if !ValidProviderReservationTransition(ProviderReservationActive, ProviderReservationSettled) {
		t.Fatal("ACTIVE -> SETTLED should be valid")
	}
	if ValidProviderReservationTransition(ProviderReservationReleased, ProviderReservationSettled) {
		t.Fatal("terminal reservation unexpectedly transitioned again")
	}

	for name, amount := range map[string]float64{"nan": math.NaN(), "positive-inf": math.Inf(1), "negative-inf": math.Inf(-1), "zero": 0, "negative": -1} {
		t.Run("reservation-"+name, func(t *testing.T) {
			candidate := reservation
			candidate.Amount = amount
			if err := candidate.Validate(); err == nil {
				t.Fatalf("amount %v unexpectedly accepted", amount)
			}
		})
	}

	usage := ProviderUsageSample{
		Key: "usage-1", AssignmentID: assignment.ID, AccountID: assignment.AccountID,
		Metric: QuotaMetricTokens, Amount: 0, ObservedAt: now, CreatedAt: now,
	}
	if err := usage.Validate(); err != nil {
		t.Fatal(err)
	}
	usage.Amount = math.NaN()
	if err := usage.Validate(); err == nil {
		t.Fatal("NaN usage unexpectedly accepted")
	}

	reservation.Amount = 1
	reservation.Metric = QuotaMetricOpaque
	if err := reservation.Validate(); err == nil {
		t.Fatal("OPAQUE reservation unexpectedly accepted")
	}
}
