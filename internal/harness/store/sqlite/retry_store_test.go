package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func seedRetryFixture(t *testing.T, db *DB, now time.Time) harnessmodel.RetrySchedule {
	t.Helper()
	seedRun(t, db, now)
	err := db.Update(context.Background(), func(tx harnessstore.Tx) error {
		if err := tx.CreateGraphRevision(context.Background(), harnessmodel.GraphRevision{WorkflowRunID: "wfr_test", Number: 1, CreatedAt: now, Reason: "retry-test"}); err != nil {
			return err
		}
		if err := tx.CreateWorkflowProgress(context.Background(), harnessmodel.WorkflowProgress{WorkflowRunID: "wfr_test", TotalNodes: 1, UpdatedAt: now}); err != nil {
			return err
		}
		node := harnessmodel.NodeRun{ID: "nr_retry", WorkflowRunID: "wfr_test", NodeID: "a", GraphRevision: 1, Generation: 1, State: harnessmodel.NodeRetryWait, CreatedAt: now, UpdatedAt: now}
		if err := tx.CreateNodeRun(context.Background(), node); err != nil {
			return err
		}
		attempt, err := tx.CreateNextAttempt(context.Background(), harnessmodel.Attempt{ID: "att_retry_1", NodeRunID: node.ID, State: harnessmodel.AttemptCreated, CreatedAt: now.Add(-time.Second)})
		if err != nil {
			return err
		}
		if attempt.Number != 1 {
			t.Fatalf("seed attempt number=%d want=1", attempt.Number)
		}
		attempt.State = harnessmodel.AttemptFailed
		attempt.StartedAt = now.Add(-time.Second)
		attempt.FinishedAt = now
		attempt.ErrorClass = string(harnessmodel.ErrorInfraTransient)
		if err := tx.CompareAndSwapAttempt(context.Background(), harnessmodel.AttemptCreated, attempt); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return harnessmodel.RetrySchedule{
		NodeRunID: "nr_retry", WorkflowRunID: "wfr_test", FailedAttemptID: "att_retry_1", AttemptNumber: 1,
		FailureClass: harnessmodel.ErrorInfraTransient, PolicyRef: "transient", ServiceKey: "github-api",
		ScheduledAt: now, NotBefore: now.Add(30 * time.Second),
	}
}

func TestRetryScheduleRoundTripDueOrderAndUniqueness(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(9000, 0).UTC()
	schedule := seedRetryFixture(t, db, now)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateRetrySchedule(ctx, schedule) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateRetrySchedule(ctx, schedule) }); err == nil {
		t.Fatal("duplicate retry schedule unexpectedly succeeded")
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		loaded, err := reader.GetRetrySchedule(ctx, schedule.NodeRunID)
		if err != nil {
			return err
		}
		if loaded.FailedAttemptID != schedule.FailedAttemptID || !loaded.NotBefore.Equal(schedule.NotBefore) || loaded.FailureClass != schedule.FailureClass {
			t.Fatalf("unexpected retry schedule round trip: %+v", loaded)
		}
		early, err := reader.ListDueRetries(ctx, schedule.NotBefore.Add(-time.Nanosecond), 10)
		if err != nil {
			return err
		}
		if len(early) != 0 {
			t.Fatalf("retry became due early: %+v", early)
		}
		due, err := reader.ListDueRetries(ctx, schedule.NotBefore, 10)
		if err != nil {
			return err
		}
		if len(due) != 1 || due[0].NodeRunID != schedule.NodeRunID {
			t.Fatalf("due retry missing: %+v", due)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRetryScheduleSurvivesDatabaseReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	db, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(10000, 0).UTC()
	schedule := seedRetryFixture(t, db, now)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateRetrySchedule(ctx, schedule) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		got, err := reader.GetRetrySchedule(ctx, schedule.NodeRunID)
		if err != nil {
			return err
		}
		if got.FailedAttemptID != schedule.FailedAttemptID || !got.NotBefore.Equal(schedule.NotBefore) {
			t.Fatalf("retry schedule changed after reopen: %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRetryBudgetReservationExhaustionAndWindowReset(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(11000, 0).UTC()
	window := time.Minute
	for i := 1; i <= 2; i++ {
		if err := db.Update(ctx, func(tx harnessstore.Tx) error {
			budget, allowed, err := tx.ReserveRetryBudget(ctx, harnessmodel.RetryBudgetWorkflow, "wfr_test", window, 2, now.Add(time.Duration(i)*time.Second))
			if err != nil {
				return err
			}
			if !allowed || budget.Used != i {
				t.Fatalf("reservation %d unexpected budget=%+v allowed=%v", i, budget, allowed)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		budget, allowed, err := tx.ReserveRetryBudget(ctx, harnessmodel.RetryBudgetWorkflow, "wfr_test", window, 2, now.Add(3*time.Second))
		if err != nil {
			return err
		}
		if allowed || budget.Used != 2 {
			t.Fatalf("exhausted budget allowed retry: %+v allowed=%v", budget, allowed)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		budget, allowed, err := tx.ReserveRetryBudget(ctx, harnessmodel.RetryBudgetWorkflow, "wfr_test", window, 2, now.Add(2*window))
		if err != nil {
			return err
		}
		if !allowed || budget.Used != 1 || !budget.WindowStart.Equal(now.Add(2*window)) {
			t.Fatalf("budget window did not reset: %+v allowed=%v", budget, allowed)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCircuitBreakerCASRejectsStaleRevision(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(12000, 0).UTC()
	initial := harnessmodel.CircuitBreaker{ServiceKey: "llm", Revision: 1, State: harnessmodel.CircuitOpen, ConsecutiveFailures: 3, FailureThreshold: 3, OpenedAt: now, NextProbeAt: now.Add(time.Minute), UpdatedAt: now}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateCircuitBreaker(ctx, initial) }); err != nil {
		t.Fatal(err)
	}
	probe := initial
	probe.Revision = 2
	probe.State = harnessmodel.CircuitHalfOpen
	probe.ProbeInFlight = true
	probe.UpdatedAt = now.Add(time.Minute)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CompareAndSwapCircuitBreaker(ctx, 1, probe) }); err != nil {
		t.Fatal(err)
	}
	stale := probe
	stale.UpdatedAt = stale.UpdatedAt.Add(time.Second)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CompareAndSwapCircuitBreaker(ctx, 1, stale) }); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("stale circuit CAS=%v want ErrConflict", err)
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		got, err := reader.GetCircuitBreaker(ctx, "llm")
		if err != nil {
			return err
		}
		if got.Revision != 2 || got.State != harnessmodel.CircuitHalfOpen || !got.ProbeInFlight {
			t.Fatalf("unexpected breaker after CAS: %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
