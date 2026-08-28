package capacity

import (
	"fmt"
	"math"
	"sort"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

const fractionEpsilon = 1e-9

// EvidenceState describes how much routing-safe capacity evidence is available.
// It is deliberately separate from ProviderHealth: a provider can be HEALTHY
// while its quota telemetry is UNKNOWN/PARTIAL, and an effective fraction of
// zero does not by itself prove provider exhaustion.
type EvidenceState string

const (
	EvidenceUnknown     EvidenceState = "UNKNOWN"
	EvidencePartial     EvidenceState = "PARTIAL"
	EvidenceQuantified  EvidenceState = "QUANTIFIED"
	EvidenceStale       EvidenceState = "STALE"
	EvidenceExhausted   EvidenceState = "EXHAUSTED"
	EvidenceUnavailable EvidenceState = "UNAVAILABLE"
)

// Policy controls only observation freshness. It does not encode provider
// quota semantics or routing thresholds.
type Policy struct {
	FreshFor      time.Duration
	ExpireAfter   time.Duration
	MaxFutureSkew time.Duration
}

// ConservativePolicy is an initial operational default. Callers may override
// it when an ingestion cadence is known. ResetAt always expires pre-reset data
// immediately, independent of these durations.
func ConservativePolicy() Policy {
	return Policy{
		FreshFor:      30 * time.Second,
		ExpireAfter:   5 * time.Minute,
		MaxFutureSkew: 5 * time.Second,
	}
}

func (p Policy) Validate() error {
	if p.FreshFor < 0 {
		return fmt.Errorf("capacity policy freshFor must be non-negative")
	}
	if p.ExpireAfter <= 0 {
		return fmt.Errorf("capacity policy expireAfter must be positive")
	}
	if p.ExpireAfter <= p.FreshFor {
		return fmt.Errorf("capacity policy expireAfter must be greater than freshFor")
	}
	if p.MaxFutureSkew < 0 {
		return fmt.Errorf("capacity policy maxFutureSkew must be non-negative")
	}
	return nil
}

// Window is the normalized, explainable view of one native quota window.
// Absolute units remain untouched; RawFraction is derived only when the source
// explicitly provides a fraction or both limit and remaining make the ratio
// mathematically justified.
type Window struct {
	ID                  string
	ModelID             harnessmodel.ProviderModelID
	Metric              harnessmodel.QuotaMetricKind
	Limit               *float64
	Remaining           *float64
	RawFraction          *float64
	EffectiveFraction    *float64
	ResetAt             *time.Time
	ObservedAt           time.Time
	Age                  time.Duration
	Freshness            float64
	Confidence           float64
	EffectiveConfidence  float64
	Expired              bool
	ExpiredByReset       bool
}

// MetricHeadroom keeps unlike provider metrics separate. Fractions are
// dimensionless lower bounds and therefore can be compared, but TOKENS,
// REQUESTS, COST, FRACTION and OPAQUE are never summed or converted into one
// another.
type MetricHeadroom struct {
	Metric              harnessmodel.QuotaMetricKind
	Windows             int
	QuantifiedWindows   int
	UnknownWindows      int
	ExpiredWindows      int
	RawFraction         *float64
	EffectiveFraction   *float64
	BottleneckWindowID  string
}

// Summary is a provider-neutral capacity view suitable for later reservation
// and selector layers. HeadroomFraction is the minimum effective fraction among
// all quantified windows: a conservative lower bound, not an exhaustion proof.
type Summary struct {
	AccountID            harnessmodel.ProviderAccountID
	Provider             harnessmodel.ProviderKind
	SourceHealth         harnessmodel.ProviderHealth
	ObservedAt           time.Time
	Age                  time.Duration
	SnapshotFreshness    float64
	ActiveRuns           int
	State                EvidenceState
	Windows              []Window
	Metrics              []MetricHeadroom
	QuantifiedWindows    int
	UnknownWindows       int
	ExpiredWindows       int
	UncertainWindows     int
	RawHeadroomFraction  *float64
	HeadroomFraction     *float64
	BottleneckWindowID   string
	EarliestResetAt      *time.Time
	ProvenExhausted      bool
}

// Normalize converts an immutable provider capacity snapshot into conservative,
// explainable effective headroom at time now.
func Normalize(snapshot harnessmodel.ProviderCapacitySnapshot, now time.Time, policy Policy) (Summary, error) {
	if err := policy.Validate(); err != nil {
		return Summary{}, err
	}
	if now.IsZero() {
		return Summary{}, fmt.Errorf("capacity normalization now is required")
	}
	if err := snapshot.Validate(); err != nil {
		return Summary{}, fmt.Errorf("validate provider capacity snapshot: %w", err)
	}
	if err := validateFiniteSnapshot(snapshot); err != nil {
		return Summary{}, err
	}

	snapshotAge, err := observationAge(snapshot.ObservedAt, now, policy.MaxFutureSkew)
	if err != nil {
		return Summary{}, fmt.Errorf("provider capacity observedAt: %w", err)
	}
	result := Summary{
		AccountID:         snapshot.AccountID,
		Provider:          snapshot.Provider,
		SourceHealth:      snapshot.Health,
		ObservedAt:        snapshot.ObservedAt.UTC(),
		Age:               snapshotAge,
		SnapshotFreshness: freshness(snapshotAge, policy),
		ActiveRuns:        snapshot.ActiveRuns,
	}

	windows := append([]harnessmodel.QuotaWindow(nil), snapshot.Windows...)
	sort.SliceStable(windows, func(i, j int) bool {
		if windows[i].ID != windows[j].ID {
			return windows[i].ID < windows[j].ID
		}
		if windows[i].ModelID != windows[j].ModelID {
			return windows[i].ModelID < windows[j].ModelID
		}
		return windows[i].Metric < windows[j].Metric
	})

	seen := make(map[string]struct{}, len(windows))
	metricIndex := make(map[harnessmodel.QuotaMetricKind]int)
	for _, native := range windows {
		if _, ok := seen[native.ID]; ok {
			return Summary{}, fmt.Errorf("duplicate quota window id %q", native.ID)
		}
		seen[native.ID] = struct{}{}

		normalized, err := normalizeWindow(native, now, policy)
		if err != nil {
			return Summary{}, fmt.Errorf("normalize quota window %q: %w", native.ID, err)
		}
		result.Windows = append(result.Windows, normalized)
		if normalized.Expired {
			result.ExpiredWindows++
		}
		if normalized.RawFraction != nil {
			result.QuantifiedWindows++
			result.RawHeadroomFraction = minFraction(result.RawHeadroomFraction, *normalized.RawFraction)
			if normalized.EffectiveFraction != nil {
				if result.HeadroomFraction == nil || *normalized.EffectiveFraction < *result.HeadroomFraction {
					v := *normalized.EffectiveFraction
					result.HeadroomFraction = &v
					result.BottleneckWindowID = normalized.ID
				}
			}
		} else {
			result.UnknownWindows++
		}
		if normalized.Expired || normalized.EffectiveConfidence < 1-fractionEpsilon {
			result.UncertainWindows++
		}
		if normalized.ResetAt != nil && normalized.ResetAt.After(now) {
			result.EarliestResetAt = earlierTime(result.EarliestResetAt, *normalized.ResetAt)
		}

		idx, ok := metricIndex[normalized.Metric]
		if !ok {
			idx = len(result.Metrics)
			metricIndex[normalized.Metric] = idx
			result.Metrics = append(result.Metrics, MetricHeadroom{Metric: normalized.Metric})
		}
		m := &result.Metrics[idx]
		m.Windows++
		if normalized.Expired {
			m.ExpiredWindows++
		}
		if normalized.RawFraction == nil {
			m.UnknownWindows++
		} else {
			m.QuantifiedWindows++
			m.RawFraction = minFraction(m.RawFraction, *normalized.RawFraction)
			if normalized.EffectiveFraction != nil && (m.EffectiveFraction == nil || *normalized.EffectiveFraction < *m.EffectiveFraction) {
				v := *normalized.EffectiveFraction
				m.EffectiveFraction = &v
				m.BottleneckWindowID = normalized.ID
			}
		}
	}

	sort.Slice(result.Metrics, func(i, j int) bool { return result.Metrics[i].Metric < result.Metrics[j].Metric })
	result.ProvenExhausted = snapshot.Health == harnessmodel.ProviderHealthExhausted
	result.State = classify(result)
	return result, nil
}

func normalizeWindow(native harnessmodel.QuotaWindow, now time.Time, policy Policy) (Window, error) {
	if err := validateFiniteWindow(native); err != nil {
		return Window{}, err
	}
	age, err := observationAge(native.ObservedAt, now, policy.MaxFutureSkew)
	if err != nil {
		return Window{}, fmt.Errorf("observedAt: %w", err)
	}

	rawFraction, err := deriveFraction(native)
	if err != nil {
		return Window{}, err
	}
	fresh := freshness(age, policy)
	expiredByReset := native.ResetAt != nil && !now.Before(*native.ResetAt) && native.ObservedAt.Before(*native.ResetAt)
	expired := fresh <= 0 || expiredByReset
	if expired {
		fresh = 0
	}
	effectiveConfidence := native.Confidence * fresh

	out := Window{
		ID:                 native.ID,
		ModelID:            native.ModelID,
		Metric:             native.Metric,
		Limit:              cloneFloat(native.Limit),
		Remaining:          cloneFloat(native.Remaining),
		RawFraction:        cloneFloat(rawFraction),
		ResetAt:            cloneTime(native.ResetAt),
		ObservedAt:         native.ObservedAt.UTC(),
		Age:                age,
		Freshness:          fresh,
		Confidence:         native.Confidence,
		EffectiveConfidence: effectiveConfidence,
		Expired:            expired,
		ExpiredByReset:     expiredByReset,
	}
	if rawFraction != nil {
		v := *rawFraction * effectiveConfidence
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		out.EffectiveFraction = &v
	}
	return out, nil
}

func deriveFraction(w harnessmodel.QuotaWindow) (*float64, error) {
	var derived *float64
	if w.Limit != nil && w.Remaining != nil {
		if *w.Remaining > *w.Limit {
			return nil, fmt.Errorf("remaining %.12g exceeds limit %.12g", *w.Remaining, *w.Limit)
		}
		var value float64
		if *w.Limit == 0 {
			if *w.Remaining != 0 {
				return nil, fmt.Errorf("non-zero remaining with zero limit")
			}
			value = 0
		} else {
			value = *w.Remaining / *w.Limit
		}
		derived = &value
	}

	if w.RemainingFraction != nil {
		if derived != nil && math.Abs(*derived-*w.RemainingFraction) > fractionEpsilon {
			return nil, fmt.Errorf("remainingFraction %.12g conflicts with remaining/limit %.12g", *w.RemainingFraction, *derived)
		}
		v := *w.RemainingFraction
		return &v, nil
	}
	if w.Metric == harnessmodel.QuotaMetricOpaque {
		return nil, nil
	}
	return derived, nil
}

func classify(s Summary) EvidenceState {
	switch s.SourceHealth {
	case harnessmodel.ProviderHealthUnavailable:
		return EvidenceUnavailable
	case harnessmodel.ProviderHealthExhausted:
		return EvidenceExhausted
	}
	if len(s.Windows) == 0 {
		if s.SnapshotFreshness <= 0 {
			return EvidenceStale
		}
		return EvidenceUnknown
	}
	if s.QuantifiedWindows == 0 {
		if s.ExpiredWindows == len(s.Windows) || s.SnapshotFreshness <= 0 {
			return EvidenceStale
		}
		return EvidenceUnknown
	}
	if s.SnapshotFreshness <= 0 || s.ExpiredWindows == len(s.Windows) {
		return EvidenceStale
	}
	if s.UnknownWindows > 0 || s.UncertainWindows > 0 || s.ExpiredWindows > 0 || s.SnapshotFreshness < 1-fractionEpsilon {
		return EvidencePartial
	}
	return EvidenceQuantified
}

func freshness(age time.Duration, p Policy) float64 {
	if age <= p.FreshFor {
		return 1
	}
	if age >= p.ExpireAfter {
		return 0
	}
	span := p.ExpireAfter - p.FreshFor
	return 1 - float64(age-p.FreshFor)/float64(span)
}

func observationAge(observedAt, now time.Time, maxFutureSkew time.Duration) (time.Duration, error) {
	if observedAt.IsZero() {
		return 0, fmt.Errorf("timestamp is required")
	}
	if observedAt.After(now) {
		skew := observedAt.Sub(now)
		if skew > maxFutureSkew {
			return 0, fmt.Errorf("timestamp is %s in the future; max skew is %s", skew, maxFutureSkew)
		}
		return 0, nil
	}
	return now.Sub(observedAt), nil
}

func validateFiniteSnapshot(s harnessmodel.ProviderCapacitySnapshot) error {
	for i, w := range s.Windows {
		if err := validateFiniteWindow(w); err != nil {
			return fmt.Errorf("quota window %d: %w", i, err)
		}
	}
	return nil
}

func validateFiniteWindow(w harnessmodel.QuotaWindow) error {
	if !finite(w.Confidence) {
		return fmt.Errorf("confidence must be finite")
	}
	for name, value := range map[string]*float64{
		"limit": w.Limit,
		"remaining": w.Remaining,
		"remainingFraction": w.RemainingFraction,
	} {
		if value != nil && !finite(*value) {
			return fmt.Errorf("%s must be finite", name)
		}
	}
	return nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func minFraction(current *float64, candidate float64) *float64 {
	if current == nil || candidate < *current {
		v := candidate
		return &v
	}
	return current
}

func earlierTime(current *time.Time, candidate time.Time) *time.Time {
	candidate = candidate.UTC()
	if current == nil || candidate.Before(*current) {
		return &candidate
	}
	return current
}

func cloneFloat(v *float64) *float64 {
	if v == nil {
		return nil
	}
	copy := *v
	return &copy
}

func cloneTime(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	copy := v.UTC()
	return &copy
}
