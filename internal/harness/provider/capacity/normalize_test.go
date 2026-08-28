package capacity

import (
	"math"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestNormalizeKeepsMetricsSeparateAndUsesConservativeBottleneck(t *testing.T) {
	now := time.Date(2026, 8, 28, 15, 30, 0, 0, time.UTC)
	s := baseSnapshot(now)
	s.Windows = []harnessmodel.QuotaWindow{
		{
			ID:         "tokens",
			Metric:     harnessmodel.QuotaMetricTokens,
			Limit:      fp(100),
			Remaining:  fp(60),
			ObservedAt: now,
			Confidence: 1,
		},
		{
			ID:                "requests",
			Metric:            harnessmodel.QuotaMetricRequests,
			RemainingFraction: fp(0.8),
			ObservedAt:        now,
			Confidence:        1,
		},
	}

	got, err := Normalize(s, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != EvidenceQuantified {
		t.Fatalf("state=%s want %s", got.State, EvidenceQuantified)
	}
	assertFloatPtr(t, got.RawHeadroomFraction, 0.6)
	assertFloatPtr(t, got.HeadroomFraction, 0.6)
	if got.BottleneckWindowID != "tokens" {
		t.Fatalf("bottleneck=%q", got.BottleneckWindowID)
	}
	if len(got.Metrics) != 2 {
		t.Fatalf("metrics=%d", len(got.Metrics))
	}
	if got.Metrics[0].Metric != harnessmodel.QuotaMetricRequests || got.Metrics[1].Metric != harnessmodel.QuotaMetricTokens {
		t.Fatalf("metric order=%v", []harnessmodel.QuotaMetricKind{got.Metrics[0].Metric, got.Metrics[1].Metric})
	}
	assertFloatPtr(t, got.Metrics[0].EffectiveFraction, 0.8)
	assertFloatPtr(t, got.Metrics[1].EffectiveFraction, 0.6)
}

func TestNormalizeAppliesConfidenceAndLinearStalenessPenalty(t *testing.T) {
	now := time.Date(2026, 8, 28, 15, 30, 0, 0, time.UTC)
	observed := now.Add(-20 * time.Second)
	s := baseSnapshot(observed)
	s.Windows = []harnessmodel.QuotaWindow{{
		ID:                "quota",
		Metric:            harnessmodel.QuotaMetricFraction,
		RemainingFraction: fp(0.5),
		ObservedAt:        observed,
		Confidence:        0.8,
	}}
	policy := Policy{FreshFor: 10 * time.Second, ExpireAfter: 30 * time.Second, MaxFutureSkew: time.Second}

	got, err := Normalize(s, now, policy)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != EvidencePartial {
		t.Fatalf("state=%s", got.State)
	}
	if len(got.Windows) != 1 {
		t.Fatalf("windows=%d", len(got.Windows))
	}
	w := got.Windows[0]
	assertClose(t, w.Freshness, 0.5)
	assertClose(t, w.EffectiveConfidence, 0.4)
	assertFloatPtr(t, w.EffectiveFraction, 0.2)
	assertFloatPtr(t, got.HeadroomFraction, 0.2)
}

func TestNormalizeExpiresPreResetObservationAfterReset(t *testing.T) {
	reset := time.Date(2026, 8, 28, 15, 30, 0, 0, time.UTC)
	observed := reset.Add(-10 * time.Second)
	now := reset.Add(time.Second)
	s := baseSnapshot(observed)
	s.Windows = []harnessmodel.QuotaWindow{{
		ID:                "quota",
		Metric:            harnessmodel.QuotaMetricFraction,
		RemainingFraction: fp(0.9),
		ResetAt:           tp(reset),
		ObservedAt:        observed,
		Confidence:        1,
	}}

	got, err := Normalize(s, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != EvidenceStale {
		t.Fatalf("state=%s want STALE", got.State)
	}
	if !got.Windows[0].Expired || !got.Windows[0].ExpiredByReset {
		t.Fatalf("window not reset-expired: %+v", got.Windows[0])
	}
	assertFloatPtr(t, got.HeadroomFraction, 0)
}

func TestNormalizeDoesNotExpireObservationTakenAtReset(t *testing.T) {
	reset := time.Date(2026, 8, 28, 15, 30, 0, 0, time.UTC)
	now := reset.Add(time.Second)
	s := baseSnapshot(reset)
	s.Windows = []harnessmodel.QuotaWindow{{
		ID:                "quota",
		Metric:            harnessmodel.QuotaMetricFraction,
		RemainingFraction: fp(0.7),
		ResetAt:           tp(reset),
		ObservedAt:        reset,
		Confidence:        1,
	}}

	got, err := Normalize(s, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if got.Windows[0].ExpiredByReset {
		t.Fatal("observation at reset must be considered post-reset evidence")
	}
	assertFloatPtr(t, got.HeadroomFraction, 0.7)
}

func TestNormalizeZeroHeadroomIsNotExhaustionProof(t *testing.T) {
	now := time.Now().UTC()
	s := baseSnapshot(now)
	s.Windows = []harnessmodel.QuotaWindow{{
		ID:         "requests",
		Metric:     harnessmodel.QuotaMetricRequests,
		Limit:      fp(0),
		Remaining:  fp(0),
		ObservedAt: now,
		Confidence: 1,
	}}

	got, err := Normalize(s, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	assertFloatPtr(t, got.HeadroomFraction, 0)
	if got.ProvenExhausted || got.State == EvidenceExhausted {
		t.Fatalf("zero lower bound must not become exhaustion proof: %+v", got)
	}
}

func TestNormalizePreservesSourceExhaustionProof(t *testing.T) {
	now := time.Now().UTC()
	s := baseSnapshot(now)
	s.Health = harnessmodel.ProviderHealthExhausted

	got, err := Normalize(s, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if !got.ProvenExhausted || got.State != EvidenceExhausted {
		t.Fatalf("got state=%s proven=%v", got.State, got.ProvenExhausted)
	}
}

func TestNormalizeUnavailableHealthOverridesFreshQuota(t *testing.T) {
	now := time.Now().UTC()
	s := baseSnapshot(now)
	s.Health = harnessmodel.ProviderHealthUnavailable
	s.Windows = []harnessmodel.QuotaWindow{{
		ID:                "quota",
		Metric:            harnessmodel.QuotaMetricFraction,
		RemainingFraction: fp(1),
		ObservedAt:        now,
		Confidence:        1,
	}}

	got, err := Normalize(s, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != EvidenceUnavailable {
		t.Fatalf("state=%s", got.State)
	}
}

func TestNormalizeOpaqueAbsoluteValuesStayOpaque(t *testing.T) {
	now := time.Now().UTC()
	s := baseSnapshot(now)
	s.Windows = []harnessmodel.QuotaWindow{{
		ID:         "opaque",
		Metric:     harnessmodel.QuotaMetricOpaque,
		Limit:      fp(10),
		Remaining:  fp(5),
		ObservedAt: now,
		Confidence: 0,
	}}

	got, err := Normalize(s, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if got.Windows[0].RawFraction != nil || got.HeadroomFraction != nil {
		t.Fatal("opaque absolute values must not be converted into fraction")
	}
	if got.State != EvidenceUnknown {
		t.Fatalf("state=%s", got.State)
	}
}

func TestNormalizeAllowsExplicitFractionWithoutChangingMetricKind(t *testing.T) {
	now := time.Now().UTC()
	s := baseSnapshot(now)
	s.Windows = []harnessmodel.QuotaWindow{{
		ID:                "opaque-with-explicit-fraction",
		Metric:            harnessmodel.QuotaMetricOpaque,
		RemainingFraction: fp(0.25),
		ObservedAt:        now,
		Confidence:        1,
	}}

	got, err := Normalize(s, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if got.Windows[0].Metric != harnessmodel.QuotaMetricOpaque {
		t.Fatalf("metric=%s", got.Windows[0].Metric)
	}
	assertFloatPtr(t, got.HeadroomFraction, 0.25)
}

func TestNormalizeRejectsContradictoryFractionAndAbsoluteValues(t *testing.T) {
	now := time.Now().UTC()
	s := baseSnapshot(now)
	s.Windows = []harnessmodel.QuotaWindow{{
		ID:                "conflict",
		Metric:            harnessmodel.QuotaMetricTokens,
		Limit:             fp(100),
		Remaining:         fp(50),
		RemainingFraction: fp(0.4),
		ObservedAt:        now,
		Confidence:        1,
	}}

	if _, err := Normalize(s, now, testPolicy()); err == nil {
		t.Fatal("expected contradiction error")
	}
}

func TestNormalizeRejectsRemainingAboveLimit(t *testing.T) {
	now := time.Now().UTC()
	s := baseSnapshot(now)
	s.Windows = []harnessmodel.QuotaWindow{{
		ID:         "bad",
		Metric:     harnessmodel.QuotaMetricRequests,
		Limit:      fp(10),
		Remaining:  fp(11),
		ObservedAt: now,
		Confidence: 1,
	}}
	if _, err := Normalize(s, now, testPolicy()); err == nil {
		t.Fatal("expected remaining>limit error")
	}
}

func TestNormalizeRejectsNonFiniteTelemetry(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name string
		edit func(*harnessmodel.QuotaWindow)
	}{
		{"nan-confidence", func(w *harnessmodel.QuotaWindow) { w.Confidence = math.NaN() }},
		{"inf-limit", func(w *harnessmodel.QuotaWindow) { w.Limit = fp(math.Inf(1)) }},
		{"nan-remaining", func(w *harnessmodel.QuotaWindow) { w.Remaining = fp(math.NaN()) }},
		{"nan-fraction", func(w *harnessmodel.QuotaWindow) { w.RemainingFraction = fp(math.NaN()) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := harnessmodel.QuotaWindow{ID: "q", Metric: harnessmodel.QuotaMetricFraction, ObservedAt: now, Confidence: 1}
			tc.edit(&w)
			s := baseSnapshot(now)
			s.Windows = []harnessmodel.QuotaWindow{w}
			if _, err := Normalize(s, now, testPolicy()); err == nil {
				t.Fatal("expected non-finite telemetry error")
			}
		})
	}
}

func TestNormalizeRejectsDuplicateNativeWindowIDs(t *testing.T) {
	now := time.Now().UTC()
	s := baseSnapshot(now)
	s.Windows = []harnessmodel.QuotaWindow{
		{ID: "same", Metric: harnessmodel.QuotaMetricFraction, RemainingFraction: fp(0.7), ObservedAt: now, Confidence: 1},
		{ID: "same", Metric: harnessmodel.QuotaMetricTokens, Limit: fp(10), Remaining: fp(5), ObservedAt: now, Confidence: 1},
	}
	if _, err := Normalize(s, now, testPolicy()); err == nil {
		t.Fatal("expected duplicate window id error")
	}
}

func TestNormalizeSortsWindowsDeterministicallyAndPreservesModelAttribution(t *testing.T) {
	now := time.Now().UTC()
	s := baseSnapshot(now)
	s.Windows = []harnessmodel.QuotaWindow{
		{ID: "z", ModelID: "m2", Metric: harnessmodel.QuotaMetricFraction, RemainingFraction: fp(0.9), ObservedAt: now, Confidence: 1},
		{ID: "a", ModelID: "", Metric: harnessmodel.QuotaMetricFraction, RemainingFraction: fp(0.8), ObservedAt: now, Confidence: 1},
		{ID: "m", ModelID: "m1", Metric: harnessmodel.QuotaMetricFraction, RemainingFraction: fp(0.7), ObservedAt: now, Confidence: 1},
	}

	got, err := Normalize(s, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if got.Windows[0].ID != "a" || got.Windows[1].ID != "m" || got.Windows[2].ID != "z" {
		t.Fatalf("order=%q,%q,%q", got.Windows[0].ID, got.Windows[1].ID, got.Windows[2].ID)
	}
	if got.Windows[0].ModelID != "" || got.Windows[1].ModelID != "m1" || got.Windows[2].ModelID != "m2" {
		t.Fatal("normalization changed/inferred model attribution")
	}
}

func TestNormalizeMixedKnownAndUnknownEvidenceIsPartial(t *testing.T) {
	now := time.Now().UTC()
	s := baseSnapshot(now)
	s.Windows = []harnessmodel.QuotaWindow{
		{ID: "known", Metric: harnessmodel.QuotaMetricFraction, RemainingFraction: fp(0.8), ObservedAt: now, Confidence: 1},
		{ID: "unknown", Metric: harnessmodel.QuotaMetricOpaque, ObservedAt: now, Confidence: 0},
	}

	got, err := Normalize(s, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != EvidencePartial || got.QuantifiedWindows != 1 || got.UnknownWindows != 1 {
		t.Fatalf("got state=%s quantified=%d unknown=%d", got.State, got.QuantifiedWindows, got.UnknownWindows)
	}
	assertFloatPtr(t, got.HeadroomFraction, 0.8)
}

func TestNormalizeNoWindowsDistinguishesUnknownFromStale(t *testing.T) {
	now := time.Now().UTC()
	fresh := baseSnapshot(now)
	freshGot, err := Normalize(fresh, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if freshGot.State != EvidenceUnknown {
		t.Fatalf("fresh state=%s", freshGot.State)
	}

	stale := baseSnapshot(now.Add(-2 * time.Minute))
	staleGot, err := Normalize(stale, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if staleGot.State != EvidenceStale {
		t.Fatalf("stale state=%s", staleGot.State)
	}
}

func TestNormalizeFutureClockSkew(t *testing.T) {
	now := time.Now().UTC()
	within := baseSnapshot(now.Add(500 * time.Millisecond))
	within.Windows = []harnessmodel.QuotaWindow{{ID: "q", Metric: harnessmodel.QuotaMetricFraction, RemainingFraction: fp(1), ObservedAt: within.ObservedAt, Confidence: 1}}
	got, err := Normalize(within, now, Policy{FreshFor: time.Second, ExpireAfter: time.Minute, MaxFutureSkew: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if got.Age != 0 || got.Windows[0].Age != 0 {
		t.Fatalf("ages=%s/%s", got.Age, got.Windows[0].Age)
	}

	beyond := baseSnapshot(now.Add(2 * time.Second))
	if _, err := Normalize(beyond, now, Policy{FreshFor: time.Second, ExpireAfter: time.Minute, MaxFutureSkew: time.Second}); err == nil {
		t.Fatal("expected excessive future skew error")
	}
}

func TestNormalizeEarliestFutureReset(t *testing.T) {
	now := time.Now().UTC()
	r1 := now.Add(10 * time.Minute)
	r2 := now.Add(5 * time.Minute)
	s := baseSnapshot(now)
	s.Windows = []harnessmodel.QuotaWindow{
		{ID: "a", Metric: harnessmodel.QuotaMetricFraction, RemainingFraction: fp(0.5), ResetAt: tp(r1), ObservedAt: now, Confidence: 1},
		{ID: "b", Metric: harnessmodel.QuotaMetricFraction, RemainingFraction: fp(0.7), ResetAt: tp(r2), ObservedAt: now, Confidence: 1},
	}
	got, err := Normalize(s, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if got.EarliestResetAt == nil || !got.EarliestResetAt.Equal(r2) {
		t.Fatalf("earliest reset=%v", got.EarliestResetAt)
	}
}

func TestPolicyValidation(t *testing.T) {
	bad := []Policy{
		{FreshFor: -time.Second, ExpireAfter: time.Minute},
		{FreshFor: 0, ExpireAfter: 0},
		{FreshFor: time.Minute, ExpireAfter: time.Minute},
		{FreshFor: time.Minute, ExpireAfter: 2 * time.Minute, MaxFutureSkew: -time.Second},
	}
	for i, p := range bad {
		if err := p.Validate(); err == nil {
			t.Fatalf("case %d expected error", i)
		}
	}
	if err := ConservativePolicy().Validate(); err != nil {
		t.Fatalf("default policy invalid: %v", err)
	}
}

func baseSnapshot(observedAt time.Time) harnessmodel.ProviderCapacitySnapshot {
	return harnessmodel.ProviderCapacitySnapshot{
		AccountID:  "acct",
		Provider:   harnessmodel.ProviderCodex,
		Health:     harnessmodel.ProviderHealthUnknown,
		ObservedAt: observedAt,
	}
}

func testPolicy() Policy {
	return Policy{FreshFor: 10 * time.Second, ExpireAfter: time.Minute, MaxFutureSkew: time.Second}
}

func fp(v float64) *float64 { return &v }
func tp(v time.Time) *time.Time { return &v }

func assertFloatPtr(t *testing.T, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("got nil, want %.12g", want)
	}
	assertClose(t, *got, want)
}

func assertClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %.12g want %.12g", got, want)
	}
}
