package demand

import (
	"context"
	"fmt"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

// StoreHistorySource adapts the durable harness Store to the narrow estimator
// HistorySource without making SQLite or the full Reader interface part of the
// demand package contract.
type StoreHistorySource struct {
	Store harnessstore.Store
}

func (s StoreHistorySource) ListProviderDemandHistory(ctx context.Context, q harnessmodel.ProviderDemandHistoryQuery) ([]harnessmodel.ProviderDemandSample, error) {
	if s.Store == nil {
		return nil, fmt.Errorf("provider demand store is required")
	}
	var out []harnessmodel.ProviderDemandSample
	if err := s.Store.View(ctx, func(r harnessstore.Reader) error {
		dr, ok := r.(harnessstore.ProviderDemandReader)
		if !ok {
			return fmt.Errorf("store reader does not implement provider demand history capability")
		}
		var err error
		out, err = dr.ListProviderDemandHistory(ctx, q)
		return err
	}); err != nil {
		return nil, err
	}
	return out, nil
}
