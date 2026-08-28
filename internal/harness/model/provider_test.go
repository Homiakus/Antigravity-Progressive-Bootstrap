package model

import (
	"testing"
	"time"
)

func TestProviderDomainValidation(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	remaining := 0.42
	snap := ProviderCapacitySnapshot{
		AccountID:  "pacc_1787918400000_00000000000000000000",
		Provider:   ProviderCodex,
		Health:     ProviderHealthHealthy,
		ActiveRuns: 1,
		ObservedAt: now,
		Windows: []QuotaWindow{{
			ID:                "primary",
			Metric:            QuotaMetricFraction,
			RemainingFraction: &remaining,
			ObservedAt:        now,
			Confidence:        1,
		}},
	}
	if err := snap.Validate(); err != nil {
		t.Fatalf("valid capacity snapshot rejected: %v", err)
	}

	bad := snap
	outOfRange := 1.1
	bad.Windows[0].RemainingFraction = &outOfRange
	if err := bad.Validate(); err == nil {
		t.Fatal("out-of-range remaining fraction unexpectedly accepted")
	}
}

func TestProviderSessionValidationRejectsContextOverflow(t *testing.T) {
	session := ProviderSessionSnapshot{
		ID:           "pses_1787918400000_00000000000000000000",
		Provider:     ProviderAntigravity,
		AccountID:    "pacc_1787918400000_00000000000000000000",
		ModelID:      "model-x",
		State:        ProviderSessionActive,
		ContextUsed:  101,
		ContextLimit: 100,
		LastUsedAt:   time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC),
	}
	if err := session.Validate(); err == nil {
		t.Fatal("context overflow unexpectedly accepted")
	}
}

func TestProviderGeneratedIDKinds(t *testing.T) {
	kinds := []IDKind{IDProviderAccount, IDProviderSession, IDProviderAssignment, IDProviderReservation}
	for _, kind := range kinds {
		id := string(kind) + "_1787918400000_00000000000000000000"
		if err := ValidateGeneratedID(id, kind); err != nil {
			t.Errorf("ValidateGeneratedID(%q, %q): %v", id, kind, err)
		}
	}
}
