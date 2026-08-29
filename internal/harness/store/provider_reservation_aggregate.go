package store

import (
	"context"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

// ActiveProviderReservationTotal is a complete aggregate for one durable
// reservation dimension within a provider-native window. It is deliberately a
// correctness projection, not a paged diagnostic view.
type ActiveProviderReservationTotal struct {
	WindowID string
	ModelID  harnessmodel.ProviderModelID
	Metric   harnessmodel.QuotaMetricKind
	Amount   float64
	Count    int64
}

// ActiveProviderReservationAggregator is an optional Reader capability used by
// correctness-sensitive feasibility paths. Implementations MUST aggregate over
// the complete ACTIVE, unexpired set for the requested account/window at
// activeAt. Callers must fail closed on contradictory returned dimensions.
//
// Store.Reader deliberately does not require this optimization: callers can
// fall back to ListAllActiveProviderReservations without weakening correctness.
type ActiveProviderReservationAggregator interface {
	ListActiveProviderReservationTotalsByWindow(
		ctx context.Context,
		accountID harnessmodel.ProviderAccountID,
		windowID string,
		activeAt time.Time,
	) ([]ActiveProviderReservationTotal, error)
}
