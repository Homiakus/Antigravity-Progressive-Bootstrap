package budget

import (
	"errors"
	"testing"
	"time"
)

func TestTrackerStepAndModelBudgets(t *testing.T) {
	now := time.Unix(1000, 0)
	clock := func() time.Time { return now }

	limits := Limits{
		MaxSteps:      3,
		MaxModelCalls: 2,
		MaxTokens:     1000,
	}

	tracker := NewTracker(limits, clock)

	// Step 1, 2, 3 ok
	if err := tracker.RecordStep(); err != nil {
		t.Fatal(err)
	}
	if err := tracker.RecordStep(); err != nil {
		t.Fatal(err)
	}
	if err := tracker.RecordStep(); err != nil {
		t.Fatal(err)
	}

	// Step 4 exceeds
	if err := tracker.RecordStep(); err == nil || !errors.Is(err, ErrStepBudgetExceeded) {
		t.Fatalf("expected ErrStepBudgetExceeded, got %v", err)
	}

	// Model calls
	if err := tracker.RecordModelCall(400, 0.01); err != nil {
		t.Fatal(err)
	}
	if err := tracker.RecordModelCall(400, 0.01); err != nil {
		t.Fatal(err)
	}

	// Model call 3 exceeds
	if err := tracker.RecordModelCall(100, 0.01); err == nil || !errors.Is(err, ErrModelCallBudgetExceeded) {
		t.Fatalf("expected ErrModelCallBudgetExceeded, got %v", err)
	}
}

func TestTrackerTimeoutAndFailureBudget(t *testing.T) {
	now := time.Unix(1000, 0)
	clock := func() time.Time { return now }

	limits := Limits{
		Timeout:       10 * time.Second,
		FailureBudget: 2,
	}
	tracker := NewTracker(limits, clock)

	// 2 failures ok
	if err := tracker.RecordFailure(); err != nil {
		t.Fatal(err)
	}
	if err := tracker.RecordFailure(); err != nil {
		t.Fatal(err)
	}

	// 3rd failure exceeds failure budget
	if err := tracker.RecordFailure(); err == nil || !errors.Is(err, ErrFailureBudgetExceeded) {
		t.Fatalf("expected ErrFailureBudgetExceeded, got %v", err)
	}

	// Advance clock beyond timeout
	now = now.Add(15 * time.Second)
	if err := tracker.Check(); err == nil || !errors.Is(err, ErrDeadlineExceeded) {
		t.Fatalf("expected ErrDeadlineExceeded, got %v", err)
	}
}
