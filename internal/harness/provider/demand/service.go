package demand

import (
	"context"
	"fmt"
	"time"
)

// HistorySource supplies already-authoritatively-classified usage history.
// Implementations may read SQLite, a replay fixture, or another durable source,
// but must not infer task/repository/context semantics from opaque IDs.
type HistorySource interface {
	LoadDemandHistory(ctx context.Context, query Key, maxSamples int, notBefore time.Time) ([]Sample, error)
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
	if len(samples) > e.Policy.MaxSamples {
		// The source contract is bounded. Reject rather than silently trusting a
		// buggy/unbounded implementation because callers rely on this cost bound.
		return Estimate{}, fmt.Errorf("demand history source returned %d samples, limit=%d", len(samples), e.Policy.MaxSamples)
	}
	return EstimateAt(query, samples, now, e.Policy)
}
