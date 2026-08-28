package capacity

import (
	"context"
	"math"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/provider/antigravity"
	"github.com/homiakus/agctl/internal/harness/provider/codex"
)

func TestNormalizeAntigravityAdapterOutput(t *testing.T) {
	now := time.Date(2026, 8, 28, 15, 30, 0, 0, time.UTC)
	account := providerAccount(harnessmodel.ProviderAntigravity, now)
	payload := []byte(`{
		"product":"antigravity",
		"model":{"id":"agy-model","display_name":"AGY"},
		"conversation_id":"session-1",
		"context_window":{"context_window_size":100000,"used_percentage":20},
		"quota":{
			"bucket-b":{"remaining_fraction":0.8,"reset_time":"2026-08-28T16:00:00Z"},
			"bucket-a":{"remaining_fraction":0.3,"reset_time":"2026-08-28T15:45:00Z"}
		}
	}`)
	obs, err := antigravity.ParseStatusLine(account, payload, now)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Normalize(obs.Capacity, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	assertFloatPtr(t, got.HeadroomFraction, 0.3)
	if got.BottleneckWindowID != "bucket-a" {
		t.Fatalf("bottleneck=%q", got.BottleneckWindowID)
	}
	if got.State != EvidenceQuantified || got.ProvenExhausted {
		t.Fatalf("state=%s proven=%v", got.State, got.ProvenExhausted)
	}
}

func TestNormalizeCodexAdapterMultipleWindowsConservatively(t *testing.T) {
	now := time.Date(2026, 8, 28, 15, 30, 0, 0, time.UTC)
	account := providerAccount(harnessmodel.ProviderCodex, now)
	adapter, err := codex.NewAdapter(account)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{
		"rateLimits":{
			"limitId":"codex",
			"primary":{"usedPercent":20,"windowDurationMins":300,"resetsAt":1787932800},
			"secondary":{"usedPercent":60,"windowDurationMins":10080,"resetsAt":1788537600}
		},
		"rateLimitsByLimitId":null,
		"rateLimitResetCredits":null
	}`)
	if err := adapter.ApplyRateLimitsRead(payload, now); err != nil {
		t.Fatal(err)
	}
	capacitySnapshot, err := adapter.Capacity(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	got, err := Normalize(capacitySnapshot, now, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	assertFloatPtr(t, got.HeadroomFraction, 0.4)
	if got.BottleneckWindowID != "codex/secondary" {
		t.Fatalf("bottleneck=%q", got.BottleneckWindowID)
	}
	if len(got.Windows) != 2 || got.Windows[0].ID != "codex/primary" || got.Windows[1].ID != "codex/secondary" {
		t.Fatalf("windows=%+v", got.Windows)
	}
}

func FuzzNormalizeFractionBounds(f *testing.F) {
	f.Add(100.0, 50.0, 0.5, 1.0, int64(0))
	f.Add(1.0, 0.0, 0.0, 0.8, int64(15))
	f.Add(0.0, 0.0, 0.0, 1.0, int64(70))

	f.Fuzz(func(t *testing.T, limit, remaining, explicit, confidence float64, ageSeconds int64) {
		if !finite(limit) || !finite(remaining) || !finite(explicit) || !finite(confidence) {
			return
		}
		if limit < 0 || remaining < 0 || explicit < 0 || explicit > 1 || confidence < 0 || confidence > 1 {
			return
		}
		if ageSeconds < -1 || ageSeconds > 600 {
			return
		}
		now := time.Date(2026, 8, 28, 15, 30, 0, 0, time.UTC)
		observed := now.Add(-time.Duration(ageSeconds) * time.Second)
		s := baseSnapshot(observed)
		s.Windows = []harnessmodel.QuotaWindow{{
			ID:                "fuzz",
			Metric:            harnessmodel.QuotaMetricTokens,
			Limit:             fp(limit),
			Remaining:         fp(remaining),
			RemainingFraction: fp(explicit),
			ObservedAt:        observed,
			Confidence:        confidence,
		}}
		got, err := Normalize(s, now, testPolicy())
		if err != nil {
			return
		}
		if got.ProvenExhausted || got.State == EvidenceExhausted {
			t.Fatalf("unknown source health became exhausted: %+v", got)
		}
		for _, value := range []*float64{got.RawHeadroomFraction, got.HeadroomFraction, got.Windows[0].RawFraction, got.Windows[0].EffectiveFraction} {
			if value == nil {
				continue
			}
			if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 || *value > 1 {
				t.Fatalf("invalid normalized fraction %v", *value)
			}
		}
	})
}

func BenchmarkNormalize100Windows(b *testing.B) {
	now := time.Date(2026, 8, 28, 15, 30, 0, 0, time.UTC)
	s := baseSnapshot(now)
	for i := 0; i < 100; i++ {
		fraction := float64(100-i) / 100
		s.Windows = append(s.Windows, harnessmodel.QuotaWindow{
			ID:                "window-" + threeDigits(i),
			Metric:            harnessmodel.QuotaMetricFraction,
			RemainingFraction: fp(fraction),
			ObservedAt:        now,
			Confidence:        1,
		})
	}
	policy := testPolicy()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Normalize(s, now, policy); err != nil {
			b.Fatal(err)
		}
	}
}

func providerAccount(kind harnessmodel.ProviderKind, now time.Time) harnessmodel.ProviderAccount {
	return harnessmodel.ProviderAccount{
		ID:        "acct",
		Provider:  kind,
		Name:      "test",
		State:     harnessmodel.ProviderAccountActive,
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now,
	}
}

func threeDigits(v int) string {
	return string([]byte{'0' + byte(v/100)%10, '0' + byte(v/10)%10, '0' + byte(v)%10})
}
