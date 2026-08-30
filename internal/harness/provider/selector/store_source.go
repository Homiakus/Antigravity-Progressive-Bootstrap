package selector

import (
	"context"
	"errors"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/provider/demand"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

// StoreSource coordinates candidate state reading within one atomic Store.View
// read transaction, preventing torn state across accounts, models, capacities,
// sessions, and circuits.
type StoreSource struct {
	Store           harnessstore.Store
	DemandEstimator *demand.Estimator
}

// Select retrieves all provider candidates from the store and computes a deterministic Decision.
func (s StoreSource) Select(ctx context.Context, req Request, now time.Time, policy Policy) (Decision, error) {
	if s.Store == nil {
		return Decision{}, fmt.Errorf("selector store source requires a non-nil Store")
	}
	if err := req.Validate(); err != nil {
		return Decision{}, fmt.Errorf("selector request invalid: %w", err)
	}

	var candidates []Candidate
	err := s.Store.View(ctx, func(view harnessstore.Reader) error {
		accounts, err := view.ListProviderAccounts(ctx, "", "")
		if err != nil {
			return fmt.Errorf("list provider accounts: %w", err)
		}

		for _, account := range accounts {
			models, err := view.ListProviderModels(ctx, account.ID)
			if err != nil {
				return fmt.Errorf("list provider models for %s: %w", account.ID, err)
			}

			var capacitySnap *harnessmodel.ProviderCapacitySnapshot
			snap, err := view.GetLatestProviderCapacity(ctx, account.ID)
			if err == nil {
				capacitySnap = &snap
			} else if !errors.Is(err, harnessstore.ErrNotFound) {
				return fmt.Errorf("get latest capacity for %s: %w", account.ID, err)
			}

			sessions, err := view.ListProviderSessions(ctx, account.ID)
			if err != nil {
				return fmt.Errorf("list provider sessions for %s: %w", account.ID, err)
			}

			for _, model := range models {
				var circuitState *harnessmodel.ProviderCircuitState
				cState, err := view.GetProviderCircuitState(ctx, account.ID, model.ID)
				if err == nil {
					circuitState = &cState
				} else if !errors.Is(err, harnessstore.ErrNotFound) {
					return fmt.Errorf("get circuit state for %s/%s: %w", account.ID, model.ID, err)
				}

				var estimates []demand.Estimate
				if s.DemandEstimator != nil && capacitySnap != nil {
					for _, w := range capacitySnap.Windows {
						if (w.ModelID == "" || w.ModelID == model.ID) && w.Metric != harnessmodel.QuotaMetricOpaque {
							key := demand.Key{
								TaskClass:    req.TaskClass,
								Provider:     account.Provider,
								ModelID:      model.ID,
								RepositoryID: req.RepositoryID,
								ContextClass: req.ContextClass,
								Metric:       w.Metric,
							}
							if est, err := s.DemandEstimator.Estimate(ctx, key, now); err == nil {
								estimates = append(estimates, est)
							}
						}
					}
				}

				candidates = append(candidates, Candidate{
					Account:         account,
					Model:           model,
					Capacity:        capacitySnap,
					Sessions:        sessions,
					Circuit:         circuitState,
					DemandEstimates: estimates,
				})
			}
		}
		return nil
	})
	if err != nil {
		return Decision{}, err
	}

	return Evaluate(ctx, req, candidates, now, policy)
}
