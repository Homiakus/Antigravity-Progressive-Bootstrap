package demand

import (
	"context"
	"fmt"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type RecordResult struct {
	UsageCreated      bool `json:"usageCreated"`
	DimensionsCreated bool `json:"dimensionsCreated"`
}

// Recorder atomically records one canonical settled provider usage sample and
// its estimator dimensions. The usage must already reference a SETTLED provider
// reservation and carry authoritative model attribution; intermediate/raw usage
// updates remain outside estimator training history. Replays are safe at both
// layers, and any semantic conflict rolls back the transaction.
type Recorder struct {
	Store harnessstore.Store
}

func (r Recorder) Record(ctx context.Context, usage harnessmodel.ProviderUsageSample, dimensions harnessmodel.ProviderDemandDimensions) (RecordResult, error) {
	if r.Store == nil {
		return RecordResult{}, fmt.Errorf("provider demand store is required")
	}
	if usage.Key != dimensions.UsageKey {
		return RecordResult{}, fmt.Errorf("provider demand usage/dimensions key mismatch")
	}
	if err := usage.Validate(); err != nil {
		return RecordResult{}, err
	}
	if err := dimensions.Validate(); err != nil {
		return RecordResult{}, err
	}
	var result RecordResult
	if err := r.Store.Update(ctx, func(tx harnessstore.Tx) error {
		_, created, err := tx.PutProviderUsageSample(ctx, usage)
		if err != nil {
			return err
		}
		result.UsageCreated = created
		dtx, ok := tx.(harnessstore.ProviderDemandTx)
		if !ok {
			return fmt.Errorf("store transaction does not implement provider demand history capability")
		}
		_, created, err = dtx.PutProviderDemandDimensions(ctx, dimensions)
		if err != nil {
			return err
		}
		result.DimensionsCreated = created
		return nil
	}); err != nil {
		return RecordResult{}, err
	}
	return result, nil
}
