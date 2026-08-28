package demand

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

var ErrInsufficientHistory = errors.New("provider demand: insufficient history")

type MatchLevel string

const (
	MatchExact         MatchLevel = "EXACT"
	MatchRepository    MatchLevel = "REPOSITORY"
	MatchTask          MatchLevel = "TASK"
	MatchModelBaseline MatchLevel = "MODEL_BASELINE"
)

type Policy struct {
	MaxAge             time.Duration
	MaxFutureSkew      time.Duration
	MinExactSamples    int
	MinFallbackSamples int
	MaxSamples         int
}

func ConservativePolicy() Policy {
	return Policy{
		MaxAge:             30 * 24 * time.Hour,
		MaxFutureSkew:      5 * time.Second,
		MinExactSamples:    5,
		MinFallbackSamples: 8,
		MaxSamples:         128,
	}
}

func (p Policy) Validate() error {
	if p.MaxAge <= 0 {
		return fmt.Errorf("provider demand maxAge must be positive")
	}
	if p.MaxFutureSkew < 0 {
		return fmt.Errorf("provider demand maxFutureSkew must be non-negative")
	}
	if p.MinExactSamples <= 0 || p.MinFallbackSamples <= 0 {
		return fmt.Errorf("provider demand minimum sample counts must be positive")
	}
	if p.MaxSamples < p.MinExactSamples || p.MaxSamples < p.MinFallbackSamples || p.MaxSamples > 10000 {
		return fmt.Errorf("provider demand maxSamples must cover minimums and be at most 10000")
	}
	return nil
}

type Request struct {
	Provider        harnessmodel.ProviderKind
	ModelID         harnessmodel.ProviderModelID
	Metric          harnessmodel.QuotaMetricKind
	TaskClass       string
	RepositoryClass string
	ContextClass    string
	Now             time.Time
}

func (r Request) Validate() error {
	if r.Now.IsZero() {
		return fmt.Errorf("provider demand estimate now is required")
	}
	probe := harnessmodel.ProviderDemandHistoryQuery{
		Provider: r.Provider, ModelID: r.ModelID, Metric: r.Metric,
		TaskClass: r.TaskClass, RepositoryClass: r.RepositoryClass, ContextClass: r.ContextClass,
		Since: r.Now, Limit: 1,
	}
	return probe.Validate()
}

type Estimate struct {
	Provider          harnessmodel.ProviderKind      `json:"provider"`
	ModelID           harnessmodel.ProviderModelID   `json:"modelId"`
	Metric            harnessmodel.QuotaMetricKind   `json:"metric"`
	MatchLevel        MatchLevel                     `json:"matchLevel"`
	SamplesUsed       int                            `json:"samplesUsed"`
	P50               float64                        `json:"p50"`
	P80               float64                        `json:"p80"`
	ReservationAmount float64                        `json:"reservationAmount"`
	OldestObservedAt  time.Time                      `json:"oldestObservedAt"`
	NewestObservedAt  time.Time                      `json:"newestObservedAt"`
}

type HistorySource interface {
	ListProviderDemandHistory(context.Context, harnessmodel.ProviderDemandHistoryQuery) ([]harnessmodel.ProviderDemandSample, error)
}

type Estimator struct {
	Source HistorySource
	Policy Policy
}

func (e Estimator) Estimate(ctx context.Context, req Request) (Estimate, error) {
	if e.Source == nil {
		return Estimate{}, fmt.Errorf("provider demand history source is required")
	}
	if err := e.Policy.Validate(); err != nil {
		return Estimate{}, err
	}
	if err := req.Validate(); err != nil {
		return Estimate{}, err
	}

	levels := []struct {
		level MatchLevel
		min   int
		task  string
		repo  string
		ctx   string
	}{
		{MatchExact, e.Policy.MinExactSamples, req.TaskClass, req.RepositoryClass, req.ContextClass},
		{MatchRepository, e.Policy.MinFallbackSamples, req.TaskClass, req.RepositoryClass, ""},
		{MatchTask, e.Policy.MinFallbackSamples, req.TaskClass, "", ""},
		{MatchModelBaseline, e.Policy.MinFallbackSamples, "", "", ""},
	}

	for _, level := range levels {
		q := harnessmodel.ProviderDemandHistoryQuery{
			Provider: req.Provider,
			ModelID: req.ModelID,
			Metric: req.Metric,
			TaskClass: level.task,
			RepositoryClass: level.repo,
			ContextClass: level.ctx,
			Since: req.Now.Add(-e.Policy.MaxAge),
			Limit: e.Policy.MaxSamples,
		}
		samples, err := e.Source.ListProviderDemandHistory(ctx, q)
		if err != nil {
			return Estimate{}, fmt.Errorf("load %s provider demand history: %w", level.level, err)
		}
		valid, err := validateHistory(samples, q, req.Now, e.Policy.MaxFutureSkew)
		if err != nil {
			return Estimate{}, fmt.Errorf("validate %s provider demand history: %w", level.level, err)
		}
		if len(valid) < level.min {
			continue
		}
		return summarize(req, level.level, valid), nil
	}
	return Estimate{}, fmt.Errorf("%w for provider=%s model=%s metric=%s", ErrInsufficientHistory, req.Provider, req.ModelID, req.Metric)
}

func validateHistory(samples []harnessmodel.ProviderDemandSample, q harnessmodel.ProviderDemandHistoryQuery, now time.Time, maxFutureSkew time.Duration) ([]harnessmodel.ProviderDemandSample, error) {
	out := make([]harnessmodel.ProviderDemandSample, 0, len(samples))
	seen := make(map[string]struct{}, len(samples))
	for _, sample := range samples {
		if err := sample.Validate(); err != nil {
			return nil, err
		}
		if _, ok := seen[sample.UsageKey]; ok {
			return nil, fmt.Errorf("duplicate provider demand usage key %q", sample.UsageKey)
		}
		seen[sample.UsageKey] = struct{}{}
		if sample.Provider != q.Provider || sample.ModelID != q.ModelID || sample.Metric != q.Metric {
			return nil, fmt.Errorf("history source returned incompatible provider/model/metric for %q", sample.UsageKey)
		}
		if sample.ObservedAt.Before(q.Since) {
			return nil, fmt.Errorf("history source returned sample %q older than requested horizon", sample.UsageKey)
		}
		if sample.ObservedAt.After(now.Add(maxFutureSkew)) {
			return nil, fmt.Errorf("history sample %q exceeds allowed future skew", sample.UsageKey)
		}
		if q.TaskClass != "" && sample.TaskClass != q.TaskClass {
			return nil, fmt.Errorf("history source returned wrong task class for %q", sample.UsageKey)
		}
		if q.RepositoryClass != "" && sample.RepositoryClass != q.RepositoryClass {
			return nil, fmt.Errorf("history source returned wrong repository class for %q", sample.UsageKey)
		}
		if q.ContextClass != "" && sample.ContextClass != q.ContextClass {
			return nil, fmt.Errorf("history source returned wrong context class for %q", sample.UsageKey)
		}
		out = append(out, sample)
	}
	return out, nil
}

func summarize(req Request, level MatchLevel, samples []harnessmodel.ProviderDemandSample) Estimate {
	values := make([]float64, len(samples))
	oldest := samples[0].ObservedAt
	newest := samples[0].ObservedAt
	for i, sample := range samples {
		values[i] = sample.Amount
		if sample.ObservedAt.Before(oldest) {
			oldest = sample.ObservedAt
		}
		if sample.ObservedAt.After(newest) {
			newest = sample.ObservedAt
		}
	}
	sort.Float64s(values)
	p50 := nearestRank(values, 0.50)
	p80 := nearestRank(values, 0.80)
	return Estimate{
		Provider: req.Provider,
		ModelID: req.ModelID,
		Metric: req.Metric,
		MatchLevel: level,
		SamplesUsed: len(samples),
		P50: p50,
		P80: p80,
		ReservationAmount: p80,
		OldestObservedAt: oldest.UTC(),
		NewestObservedAt: newest.UTC(),
	}
}

func nearestRank(values []float64, q float64) float64 {
	if len(values) == 0 || q <= 0 || q > 1 || math.IsNaN(q) {
		panic("invalid nearest-rank input")
	}
	idx := int(math.Ceil(q*float64(len(values)))) - 1
	return values[idx]
}
