package demand

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

// Source identifies how much historical specificity supported an estimate.
// Lower-specificity sources are intentionally less confident, but they never
// change the metric unit or cross provider boundaries.
type Source string

const (
	SourceExact             Source = "EXACT"
	SourceWithoutContext    Source = "WITHOUT_CONTEXT"
	SourceWithoutRepository Source = "WITHOUT_REPOSITORY"
	SourceWithoutModel      Source = "WITHOUT_MODEL"
	SourceProviderMetric    Source = "PROVIDER_METRIC"
	SourceColdStart         Source = "COLD_START"
	SourceUnavailable       Source = "UNAVAILABLE"
)

// Key describes one demand population. TaskClass is caller-defined (for
// example implement/test/review), RepositoryID is a stable repository identity,
// and ContextClass is an opaque caller taxonomy. Metric always stays native.
type Key struct {
	TaskClass    string
	Provider     harnessmodel.ProviderKind
	ModelID      harnessmodel.ProviderModelID
	RepositoryID string
	ContextClass string
	Metric       harnessmodel.QuotaMetricKind
}

func (k Key) Validate() error {
	if strings.TrimSpace(k.TaskClass) == "" {
		return fmt.Errorf("demand task class is required")
	}
	if !k.Provider.Valid() {
		return fmt.Errorf("invalid demand provider %q", k.Provider)
	}
	if !k.Metric.Valid() || k.Metric == harnessmodel.QuotaMetricOpaque {
		return fmt.Errorf("demand metric %q is not estimable", k.Metric)
	}
	if len(k.TaskClass) > 128 || len(k.RepositoryID) > 256 || len(k.ContextClass) > 128 || len(k.ModelID) > 256 {
		return fmt.Errorf("demand classification field exceeds size limit")
	}
	return nil
}

// Sample is a classified immutable provider-usage observation. Amount is in the
// exact native unit declared by Key.Metric.
type Sample struct {
	Key        Key
	Amount     float64
	ObservedAt time.Time
}

func (s Sample) Validate() error {
	if err := s.Key.Validate(); err != nil {
		return err
	}
	if math.IsNaN(s.Amount) || math.IsInf(s.Amount, 0) || s.Amount < 0 {
		return fmt.Errorf("demand sample amount must be finite and non-negative")
	}
	if s.Key.Metric == harnessmodel.QuotaMetricFraction && s.Amount > 1 {
		return fmt.Errorf("fractional demand sample must be within [0,1]")
	}
	if s.ObservedAt.IsZero() {
		return fmt.Errorf("demand sample observedAt is required")
	}
	return nil
}

// Policy bounds history and defines explicit operator-supplied cold starts.
// ColdStart deliberately has no built-in token/request/cost values: inventing a
// native-unit demand would violate unit-preserving routing semantics.
type Policy struct {
	MinSamples    int
	TargetSamples int
	MaxSamples    int
	MaxAge        time.Duration
	MaxFutureSkew time.Duration
	ColdStart     map[harnessmodel.QuotaMetricKind]float64
}

func DefaultPolicy() Policy {
	return Policy{
		MinSamples:    5,
		TargetSamples: 40,
		MaxSamples:    256,
		MaxAge:        30 * 24 * time.Hour,
		MaxFutureSkew: 5 * time.Minute,
		ColdStart:     map[harnessmodel.QuotaMetricKind]float64{},
	}
}

func (p Policy) Validate() error {
	if p.MinSamples < 1 {
		return fmt.Errorf("demand min samples must be positive")
	}
	if p.TargetSamples < p.MinSamples {
		return fmt.Errorf("demand target samples must be >= min samples")
	}
	if p.MaxSamples < p.TargetSamples {
		return fmt.Errorf("demand max samples must be >= target samples")
	}
	if p.MaxAge <= 0 || p.MaxFutureSkew < 0 {
		return fmt.Errorf("invalid demand age policy")
	}
	for metric, value := range p.ColdStart {
		if !metric.Valid() || metric == harnessmodel.QuotaMetricOpaque {
			return fmt.Errorf("cold-start metric %q is not estimable", metric)
		}
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return fmt.Errorf("cold-start demand for %s must be finite and non-negative", metric)
		}
		if metric == harnessmodel.QuotaMetricFraction && value > 1 {
			return fmt.Errorf("fractional cold-start demand must be within [0,1]")
		}
	}
	return nil
}

// Estimate is a deterministic conservative view over one compatible native
// metric. Reservation is intentionally p80 rather than the mean/p50.
type Estimate struct {
	Key         Key
	Available   bool
	Source      Source
	SampleCount int
	P50         float64
	P80         float64
	Reservation float64
	Confidence  float64
	OldestAt    time.Time
	NewestAt    time.Time
}

// EstimateAt derives demand without shared mutable state. It is safe to invoke
// concurrently as long as callers do not mutate the supplied sample slice.
func EstimateAt(query Key, samples []Sample, now time.Time, policy Policy) (Estimate, error) {
	if err := query.Validate(); err != nil {
		return Estimate{}, err
	}
	if now.IsZero() {
		return Estimate{}, fmt.Errorf("demand estimation time is required")
	}
	if err := policy.Validate(); err != nil {
		return Estimate{}, err
	}

	valid := make([]Sample, 0, len(samples))
	for i, sample := range samples {
		if err := sample.Validate(); err != nil {
			return Estimate{}, fmt.Errorf("demand sample %d: %w", i, err)
		}
		if sample.Key.Provider != query.Provider || sample.Key.Metric != query.Metric {
			continue
		}
		if sample.ObservedAt.After(now.Add(policy.MaxFutureSkew)) {
			continue
		}
		if now.Sub(sample.ObservedAt) > policy.MaxAge {
			continue
		}
		valid = append(valid, sample)
	}

	for _, candidate := range candidates(query) {
		matched := matching(valid, candidate.key)
		if len(matched) < policy.MinSamples {
			continue
		}
		if len(matched) > policy.MaxSamples {
			sort.SliceStable(matched, func(i, j int) bool {
				if matched[i].ObservedAt.Equal(matched[j].ObservedAt) {
					// If a provider reports several samples with the same timestamp,
					// retain the larger claims at the bounded-history boundary. Keeping
					// smaller equal-time values would systematically underestimate p80.
					return matched[i].Amount > matched[j].Amount
				}
				return matched[i].ObservedAt.After(matched[j].ObservedAt)
			})
			matched = matched[:policy.MaxSamples]
		}
		return summarize(query, candidate.source, matched, policy), nil
	}

	if value, ok := policy.ColdStart[query.Metric]; ok {
		return Estimate{
			Key: query, Available: true, Source: SourceColdStart,
			P50: value, P80: value, Reservation: value, Confidence: sourceConfidence(SourceColdStart),
		}, nil
	}
	return Estimate{Key: query, Source: SourceUnavailable}, nil
}

type candidate struct {
	key    Key
	source Source
}

func candidates(query Key) []candidate {
	out := []candidate{{key: query, source: SourceExact}}
	withoutContext := query
	withoutContext.ContextClass = ""
	out = appendCandidate(out, withoutContext, SourceWithoutContext)
	withoutRepository := withoutContext
	withoutRepository.RepositoryID = ""
	out = appendCandidate(out, withoutRepository, SourceWithoutRepository)
	withoutModel := withoutRepository
	withoutModel.ModelID = ""
	out = appendCandidate(out, withoutModel, SourceWithoutModel)
	providerMetric := withoutModel
	providerMetric.TaskClass = ""
	out = appendCandidate(out, providerMetric, SourceProviderMetric)
	return out
}

func appendCandidate(in []candidate, key Key, source Source) []candidate {
	last := in[len(in)-1]
	if equalKey(last.key, key) {
		return in
	}
	return append(in, candidate{key: key, source: source})
}

func equalKey(a, b Key) bool {
	return a.TaskClass == b.TaskClass && a.Provider == b.Provider && a.ModelID == b.ModelID &&
		a.RepositoryID == b.RepositoryID && a.ContextClass == b.ContextClass && a.Metric == b.Metric
}

func matching(samples []Sample, key Key) []Sample {
	out := make([]Sample, 0, len(samples))
	for _, sample := range samples {
		if sample.Key.Provider != key.Provider || sample.Key.Metric != key.Metric {
			continue
		}
		if key.TaskClass != "" && sample.Key.TaskClass != key.TaskClass {
			continue
		}
		if key.ModelID != "" && sample.Key.ModelID != key.ModelID {
			continue
		}
		if key.RepositoryID != "" && sample.Key.RepositoryID != key.RepositoryID {
			continue
		}
		if key.ContextClass != "" && sample.Key.ContextClass != key.ContextClass {
			continue
		}
		out = append(out, sample)
	}
	return out
}

func summarize(query Key, source Source, samples []Sample, policy Policy) Estimate {
	amounts := make([]float64, len(samples))
	oldest := samples[0].ObservedAt
	newest := samples[0].ObservedAt
	for i, sample := range samples {
		amounts[i] = sample.Amount
		if sample.ObservedAt.Before(oldest) {
			oldest = sample.ObservedAt
		}
		if sample.ObservedAt.After(newest) {
			newest = sample.ObservedAt
		}
	}
	sort.Float64s(amounts)
	p50 := nearestRank(amounts, 0.50)
	p80 := nearestRank(amounts, 0.80)
	confidence := math.Min(1, float64(len(samples))/float64(policy.TargetSamples)) * sourceConfidence(source)
	return Estimate{
		Key: query, Available: true, Source: source, SampleCount: len(samples),
		P50: p50, P80: p80, Reservation: p80, Confidence: confidence,
		OldestAt: oldest, NewestAt: newest,
	}
}

func nearestRank(sorted []float64, percentile float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(percentile * float64(len(sorted))))
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

func sourceConfidence(source Source) float64 {
	switch source {
	case SourceExact:
		return 1.00
	case SourceWithoutContext:
		return 0.90
	case SourceWithoutRepository:
		return 0.80
	case SourceWithoutModel:
		return 0.70
	case SourceProviderMetric:
		return 0.60
	case SourceColdStart:
		return 0.20
	default:
		return 0
	}
}
