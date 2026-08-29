package sqlite

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	providerreservation "github.com/homiakus/agctl/internal/harness/provider/reservation"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestAtomicProviderReservationConcurrentSameRequestIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(35_000, 0).UTC()
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/replay-concurrent", reservationTestModel, 100, now, nil),
	})
	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	req := reservationRequest(testProviderAttemptID, "pasn_replay_concurrent", 10, now)

	const callers = 50
	var created atomic.Int64
	var replayed atomic.Int64
	var failed atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := service.Reserve(ctx, req)
			if err != nil {
				failed.Add(1)
				t.Logf("concurrent replay failed: %v", err)
				return
			}
			if got.Replayed {
				replayed.Add(1)
			} else {
				created.Add(1)
			}
		}()
	}
	wg.Wait()
	if created.Load() != 1 || replayed.Load() != callers-1 || failed.Load() != 0 {
		t.Fatalf("same-request outcomes created=%d replayed=%d failed=%d", created.Load(), replayed.Load(), failed.Load())
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		assignment, err := r.GetActiveProviderAssignment(ctx, testProviderAttemptID)
		if err != nil {
			return err
		}
		if assignment.ID != req.AssignmentID {
			t.Fatalf("unexpected active assignment: %+v", assignment)
		}
		reservations, err := r.ListProviderReservationsByAssignment(ctx, req.AssignmentID)
		if err != nil {
			return err
		}
		if len(reservations) != 1 || reservations[0].Amount != 10 || reservations[0].State != harnessmodel.ProviderReservationActive {
			t.Fatalf("concurrent replay created duplicate/inconsistent claims: %+v", reservations)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
