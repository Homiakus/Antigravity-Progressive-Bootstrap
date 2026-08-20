package retry_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessretry "github.com/homiakus/agctl/internal/harness/retry"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

func TestCircuitRevisionAndStaleTicketSafetySurviveRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	now := time.Unix(20_000, 0).UTC()
	policy := harnessretry.BreakerPolicy{FailureThreshold: 1, Cooldown: time.Minute}

	db, err := sqlitestore.Open(ctx, path, sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	coordinator, err := harnessretry.NewCircuitCoordinator(db, policy, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	first, err := coordinator.Acquire(ctx, "restart-provider")
	if err != nil {
		t.Fatal(err)
	}
	lateSuccess, err := coordinator.Acquire(ctx, "restart-provider")
	if err != nil {
		t.Fatal(err)
	}
	opened, err := coordinator.RecordFailure(ctx, first.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	if opened.State != harnessmodel.CircuitOpen || opened.Revision <= first.Ticket.Revision {
		t.Fatalf("breaker did not open with advanced revision: first=%+v opened=%+v", first, opened)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = sqlitestore.Open(ctx, path, sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	coordinator, err = harnessretry.NewCircuitCoordinator(db, policy, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	// A success from a call admitted before the crash/open transition is stale
	// evidence. Restart must not erase the durable revision fence.
	stillOpen, err := coordinator.RecordSuccess(ctx, lateSuccess.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	if stillOpen.State != harnessmodel.CircuitOpen || stillOpen.Revision != opened.Revision {
		t.Fatalf("stale pre-restart success mutated OPEN breaker: opened=%+v got=%+v", opened, stillOpen)
	}

	now = opened.NextProbeAt
	probe, err := coordinator.Acquire(ctx, "restart-provider")
	if err != nil {
		t.Fatal(err)
	}
	if !probe.Allow || !probe.Probe || !probe.Ticket.Probe || probe.Ticket.Revision <= opened.Revision {
		t.Fatalf("restart did not preserve/advance half-open ownership: opened=%+v probe=%+v", opened, probe)
	}
	closed, err := coordinator.RecordSuccess(ctx, probe.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	if closed.State != harnessmodel.CircuitClosed || closed.Revision <= probe.Ticket.Revision || closed.ProbeInFlight {
		t.Fatalf("probe success did not durably close after restart: %+v", closed)
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		got, err := reader.GetCircuitBreaker(ctx, "restart-provider")
		if err != nil {
			return err
		}
		if got.State != harnessmodel.CircuitClosed || got.Revision != closed.Revision {
			t.Fatalf("reopened store lost circuit state: %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
