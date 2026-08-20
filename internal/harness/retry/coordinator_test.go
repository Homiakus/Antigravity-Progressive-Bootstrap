package retry_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessretry "github.com/homiakus/agctl/internal/harness/retry"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

func TestCircuitCoordinatorOnlyOneHalfOpenProbeWins(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "state.db"), sqlitestore.Options{BusyTimeout: 2 * time.Second, MaxOpenConns: 8, MaxIdleConns: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Unix(13000, 0).UTC()
	policy := harnessretry.BreakerPolicy{FailureThreshold: 1, Cooldown: time.Second}
	clock := func() time.Time { return now }
	coordinator, err := harnessretry.NewCircuitCoordinator(db, policy, clock)
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := coordinator.Acquire(ctx, "provider")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.RecordFailure(ctx, admitted.Ticket); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)

	const callers = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowedProbes := 0
	otherErrs := []error{}
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			decision, err := coordinator.Acquire(ctx, "provider")
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				otherErrs = append(otherErrs, err)
				return
			}
			if decision.Allow && decision.Probe {
				allowedProbes++
			}
		}()
	}
	wg.Wait()
	if len(otherErrs) != 0 {
		t.Fatalf("unexpected acquire errors: %v", otherErrs)
	}
	if allowedProbes != 1 {
		t.Fatalf("half-open probes allowed=%d want=1", allowedProbes)
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		breaker, err := reader.GetCircuitBreaker(ctx, "provider")
		if err != nil {
			return err
		}
		if breaker.State != harnessmodel.CircuitHalfOpen || !breaker.ProbeInFlight || breaker.Revision < 3 {
			t.Fatalf("unexpected durable half-open state: %+v", breaker)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCircuitCoordinatorSuccessClosesAndFailureReopens(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "state.db"), sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Unix(14000, 0).UTC()
	coordinator, err := harnessretry.NewCircuitCoordinator(db, harnessretry.BreakerPolicy{FailureThreshold: 1, Cooldown: time.Minute}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := coordinator.Acquire(ctx, "github")
	if err != nil {
		t.Fatal(err)
	}
	opened, err := coordinator.RecordFailure(ctx, admitted.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	if opened.State != harnessmodel.CircuitOpen {
		t.Fatalf("failure did not open circuit: %+v", opened)
	}
	now = opened.NextProbeAt
	probe, err := coordinator.Acquire(ctx, "github")
	if err != nil {
		t.Fatal(err)
	}
	if !probe.Allow || !probe.Probe || !probe.Ticket.Probe {
		t.Fatalf("probe not allowed: %+v", probe)
	}
	now = now.Add(time.Second)
	closed, err := coordinator.RecordSuccess(ctx, probe.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	if closed.State != harnessmodel.CircuitClosed || closed.ConsecutiveFailures != 0 || closed.ProbeInFlight {
		t.Fatalf("success did not close circuit: %+v", closed)
	}
}

func TestConcurrentClosedFailuresAreNotLost(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "state.db"), sqlitestore.Options{BusyTimeout: 2 * time.Second, MaxOpenConns: 8, MaxIdleConns: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Unix(15000, 0).UTC()
	const failures = 8
	coordinator, err := harnessretry.NewCircuitCoordinator(db, harnessretry.BreakerPolicy{FailureThreshold: failures, Cooldown: time.Minute}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	tickets := make([]harnessretry.CircuitTicket, failures)
	for i := range tickets {
		decision, err := coordinator.Acquire(ctx, "parallel-provider")
		if err != nil {
			t.Fatal(err)
		}
		if !decision.Allow || decision.Probe {
			t.Fatalf("closed breaker did not admit call %d: %+v", i, decision)
		}
		tickets[i] = decision.Ticket
	}

	var wg sync.WaitGroup
	errCh := make(chan error, failures)
	for _, ticket := range tickets {
		ticket := ticket
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := coordinator.RecordFailure(ctx, ticket)
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("parallel failure recording failed: %v", err)
		}
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		breaker, err := reader.GetCircuitBreaker(ctx, "parallel-provider")
		if err != nil {
			return err
		}
		if breaker.State != harnessmodel.CircuitOpen || breaker.ConsecutiveFailures != failures {
			t.Fatalf("concurrent failures were lost: %+v", breaker)
		}
		if breaker.Revision != uint64(failures+1) {
			t.Fatalf("revision=%d want=%d", breaker.Revision, failures+1)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestStaleSuccessCannotCloseNewlyOpenedCircuit(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "state.db"), sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Unix(16000, 0).UTC()
	coordinator, err := harnessretry.NewCircuitCoordinator(db, harnessretry.BreakerPolicy{FailureThreshold: 1, Cooldown: time.Minute}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	first, err := coordinator.Acquire(ctx, "stale-success")
	if err != nil {
		t.Fatal(err)
	}
	second, err := coordinator.Acquire(ctx, "stale-success")
	if err != nil {
		t.Fatal(err)
	}
	opened, err := coordinator.RecordFailure(ctx, first.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	if opened.State != harnessmodel.CircuitOpen {
		t.Fatalf("failure did not open breaker: %+v", opened)
	}
	got, err := coordinator.RecordSuccess(ctx, second.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != harnessmodel.CircuitOpen || got.Revision != opened.Revision {
		t.Fatalf("stale pre-open success closed or mutated breaker: opened=%+v got=%+v", opened, got)
	}
}

func TestStaleProbeResultIsRejectedAfterProbeFailure(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "state.db"), sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Unix(17000, 0).UTC()
	coordinator, err := harnessretry.NewCircuitCoordinator(db, harnessretry.BreakerPolicy{FailureThreshold: 1, Cooldown: time.Second}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := coordinator.Acquire(ctx, "probe-stale")
	if err != nil {
		t.Fatal(err)
	}
	opened, err := coordinator.RecordFailure(ctx, admitted.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	now = opened.NextProbeAt
	probe, err := coordinator.Acquire(ctx, "probe-stale")
	if err != nil {
		t.Fatal(err)
	}
	if !probe.Probe {
		t.Fatalf("expected half-open probe: %+v", probe)
	}
	if _, err := coordinator.RecordFailure(ctx, probe.Ticket); err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.RecordSuccess(ctx, probe.Ticket); !errors.Is(err, harnessretry.ErrStaleCircuitTicket) {
		t.Fatalf("stale probe success error=%v want ErrStaleCircuitTicket", err)
	}
}
