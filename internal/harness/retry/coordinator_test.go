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
	if _, err := coordinator.RecordFailure(ctx, "provider"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)

	const callers = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowedProbes := 0
	conflicts := 0
	otherErrs := []error{}
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			decision, err := coordinator.Acquire(ctx, "provider")
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if errors.Is(err, harnessstore.ErrConflict) {
					conflicts++
					return
				}
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
		t.Fatalf("half-open probes allowed=%d want=1 (conflicts=%d)", allowedProbes, conflicts)
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		breaker, err := reader.GetCircuitBreaker(ctx, "provider")
		if err != nil {
			return err
		}
		if breaker.State != harnessmodel.CircuitHalfOpen || !breaker.ProbeInFlight {
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
	opened, err := coordinator.RecordFailure(ctx, "github")
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
	if !probe.Allow || !probe.Probe {
		t.Fatalf("probe not allowed: %+v", probe)
	}
	now = now.Add(time.Second)
	closed, err := coordinator.RecordSuccess(ctx, "github")
	if err != nil {
		t.Fatal(err)
	}
	if closed.State != harnessmodel.CircuitClosed || closed.ConsecutiveFailures != 0 || closed.ProbeInFlight {
		t.Fatalf("success did not close circuit: %+v", closed)
	}
}
