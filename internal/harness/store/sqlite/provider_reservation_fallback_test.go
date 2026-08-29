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

// reservationFallbackStore deliberately hides optional store capabilities while
// preserving the mandatory Store/Tx contracts. It proves that the reservation
// broker remains correct for non-SQLite stores that do not implement the
// aggregate optimization.
type reservationFallbackStore struct {
	harnessstore.Store
}

type reservationFallbackTx struct {
	harnessstore.Tx
}

func (s reservationFallbackStore) Update(ctx context.Context, fn func(harnessstore.Tx) error) error {
	return s.Store.Update(ctx, func(tx harnessstore.Tx) error {
		return fn(reservationFallbackTx{Tx: tx})
	})
}

var _ harnessstore.Store = reservationFallbackStore{}
var _ harnessstore.Tx = reservationFallbackTx{}

func TestAtomicProviderReservationCompleteListFallback(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(33_000, 0).UTC()
	seedReservationBroker(t, db, now, []harnessmodel.QuotaWindow{
		tokenWindow("tokens/fallback", "", 100, now, nil),
	})
	createReservationAttempts(t, db, now, "att_fallback_second")

	service := providerreservation.Service{
		Store:  reservationFallbackStore{Store: db},
		Policy: reservationPolicy(),
	}
	if _, err := service.Reserve(ctx, reservationRequest(testProviderAttemptID, "pasn_fallback_first", 60, now)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reserve(ctx, reservationRequest("att_fallback_second", "pasn_fallback_second", 41, now)); !errors.Is(err, providerreservation.ErrInsufficientCapacity) {
		t.Fatalf("fallback second reservation error=%v want insufficient capacity", err)
	}

	if err := db.View(ctx, func(r harnessstore.Reader) error {
		claims, err := r.ListAllActiveProviderReservations(ctx, testProviderAccountID, now)
		if err != nil {
			return err
		}
		if len(claims) != 1 || claims[0].Amount != 60 || claims[0].WindowID != "tokens/fallback" {
			t.Fatalf("fallback path produced unexpected durable claims: %+v", claims)
		}
		if _, err := r.GetProviderAssignment(ctx, "pasn_fallback_second"); !errors.Is(err, harnessstore.ErrNotFound) {
			t.Fatalf("fallback insufficient reservation leaked assignment: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
