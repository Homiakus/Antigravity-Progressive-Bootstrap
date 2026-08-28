package capacity

import (
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestNormalizeAbsoluteRemainingWithoutLimitIsQuantified(t *testing.T) {
	now := time.Date(2026, 8, 28, 15, 30, 0, 0, time.UTC)
	s := baseSnapshot(now)
	s.Windows = []harnessmodel.QuotaWindow{{
		ID:         "tokens",
		Metric:     harnessmodel.QuotaMetricTokens,
		Remaining:  fp(1200),
		ObservedAt: now,
		Confidence: 1,
	}}

	got, err := Normalize(s, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != EvidenceQuantified {
		t.Fatalf("state=%s want QUANTIFIED", got.State)
	}
	if got.QuantifiedWindows != 1 || got.AbsoluteWindows != 1 || got.FractionalWindows != 0 || got.UnknownWindows != 0 {
		t.Fatalf("counts quantified=%d absolute=%d fractional=%d unknown=%d", got.QuantifiedWindows, got.AbsoluteWindows, got.FractionalWindows, got.UnknownWindows)
	}
	if got.HeadroomFraction != nil || got.RawHeadroomFraction != nil {
		t.Fatal("limitless absolute remaining must not invent a fraction")
	}
	assertFloatPtr(t, got.Windows[0].Remaining, 1200)
	assertFloatPtr(t, got.Windows[0].EffectiveRemaining, 1200)
}

func TestNormalizeAbsoluteRemainingGetsConfidenceAndFreshnessPenalty(t *testing.T) {
	now := time.Date(2026, 8, 28, 15, 30, 0, 0, time.UTC)
	observed := now.Add(-20 * time.Second)
	s := baseSnapshot(observed)
	s.Windows = []harnessmodel.QuotaWindow{{
		ID:         "requests",
		Metric:     harnessmodel.QuotaMetricRequests,
		Remaining:  fp(100),
		ObservedAt: observed,
		Confidence: 0.8,
	}}
	policy := Policy{FreshFor: 10 * time.Second, ExpireAfter: 30 * time.Second, MaxFutureSkew: time.Second}

	got, err := Normalize(s, now, policy)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != EvidencePartial {
		t.Fatalf("state=%s want PARTIAL", got.State)
	}
	assertFloatPtr(t, got.Windows[0].EffectiveRemaining, 40)
	if got.HeadroomFraction != nil {
		t.Fatal("absolute-only telemetry must not invent fractional headroom")
	}
}

func TestNormalizeFractionMetricRemainingIsNotTreatedAsAbsoluteUnits(t *testing.T) {
	now := time.Now().UTC()
	s := baseSnapshot(now)
	s.Windows = []harnessmodel.QuotaWindow{{
		ID:         "fraction-odd-shape",
		Metric:     harnessmodel.QuotaMetricFraction,
		Remaining:  fp(0.5),
		ObservedAt: now,
		Confidence: 1,
	}}

	got, err := Normalize(s, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if got.Windows[0].EffectiveRemaining != nil || got.QuantifiedWindows != 0 || got.State != EvidenceUnknown {
		t.Fatalf("fraction metric remaining was misclassified: %+v", got)
	}
}
