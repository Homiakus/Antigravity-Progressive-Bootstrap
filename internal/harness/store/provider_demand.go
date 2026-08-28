package store

import (
	"context"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

// ProviderDemandReader is a narrow optional capability for historical provider
// demand. It is intentionally separate from the core Reader so estimator users
// do not widen the durable harness Store contract merely to consume statistics.
type ProviderDemandReader interface {
	GetProviderDemandDimensions(context.Context, string) (harnessmodel.ProviderDemandDimensions, error)
	ListProviderDemandHistory(context.Context, harnessmodel.ProviderDemandHistoryQuery) ([]harnessmodel.ProviderDemandSample, error)
}

// ProviderDemandTx extends the read capability with idempotent binding of one
// immutable usage event to its estimator dimensions.
type ProviderDemandTx interface {
	ProviderDemandReader
	PutProviderDemandDimensions(context.Context, harnessmodel.ProviderDemandDimensions) (harnessmodel.ProviderDemandDimensions, bool, error)
}
