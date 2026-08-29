package demand

import (
	"context"
	"fmt"
	"time"
)

// HistorySource supplies already-authoritatively-classified usage history.
// maxSamplesPerPopulation is a bound for each specificity population used by
// the estimator (exact, no-context, no-repository, no-model, provider-metric),
// not a global bound across their union. Implementations must not infer
// task/repository/context semantics from opaque IDs.
type HistorySource interface {
	LoadDemandHistory(ctx context.Context, query Key, maxSamplesPerPopulation int, notBefore time.Time) ([]Sample, error)
}

// Estimator is a thin orchestration layer around the pure policy. Keeping the
// percentile/fallback math in EstimateAt makes routing logic deterministic and
// keeps persistence/query strategy replaceable.
type Estimator struct {
	Source HistorySource
	Policy Policy
}

func (e Estimator) Estimate(ctx context.Context, query Key, now time.Time) (Estimate, error) {
	if e.Source == nil {
		return Estimate{}, fmt.Errorf("demand history source is required")
	}
	if err := query.Validate(); err != nil {
		return Estimate{}, err
	}
	if now.IsZero() {
		return Estimate{}, fmt.Errorf("demand estimation time is required")
	}
	if err := e.Policy.Validate(); err != nil {
		return Estimate{}, err
	}
	if err := ctx.Err(); err != nil {
		return Estimate{}, err
	}
	samples, err := e.Source.LoadDemandHistory(ctx, query, e.Policy.MaxSamples, now.Add(-e.Policy.MaxAge))
	if err != nil {
		return Estimate{}, fmt.Errorf("load demand history: %w", err)
	}
	populationCount := len(candidates(query))
	maxReturned := e.Policy.MaxSamples * populationCount
	if len(samples) > maxReturned {
		// The source may need a bounded slice for every fallback population. A
		// global MaxSamples cap would starve broad fallback history; the union is
		// therefore bounded by MaxSamples * number of specificity levels.
		return Estimate{}, fmt.Errorf("demand history source returned %d samples, bounded union limit=%d", len(samples), maxReturned)
	}
	return EstimateAt(query, samples, now, e.Policy)
}
