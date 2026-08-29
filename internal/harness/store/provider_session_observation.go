package store

import (
	"context"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

// ObservedProviderSessionReader is an optional correctness capability for
// stores that can return the authoritative observation timestamp together with
// each provider session. ProviderSessionSnapshot.LastUsedAt is provider usage
// recency and MUST NOT be substituted for observation freshness.
//
// Session brokers may fall back to Reader.ListProviderSessions for other Store
// implementations, but a session without ObservedAt is not reusable.
type ObservedProviderSessionReader interface {
	ListObservedProviderSessions(context.Context, harnessmodel.ProviderAccountID) ([]harnessmodel.ProviderSessionSnapshot, error)
}
