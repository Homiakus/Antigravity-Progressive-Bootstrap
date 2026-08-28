package demand

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

type fakeHistorySource struct {
	samples []harnessmodel.ProviderDemandSample
	queries []harnessmodel.ProviderDemandHistoryQuery
}

func (f *fakeHistorySource) ListProviderDemandHistory(_ context.Context, q harnessmodel.ProviderDemandHistoryQuery) ([]harnessmodel.ProviderDemandSample, error) {
	f.queries = append(f.queries, q)
	out := make([]harnessmodel.ProviderDemandSample, 0)
	for _, s := range f.samples {
		if s.Provider != q.Provider || s.ModelID != q.ModelID || s.Metric != q.Metric || s.ObservedAt.Before(q.Since) {
			continue
		}
		if q.TaskClass != "" && s.TaskClass != q.TaskClass {
			continue
		}
		if q.RepositoryClass != "" && s.RepositoryClass != q.RepositoryClass {
			continue
		}
		if q.ContextClass != "" && s.ContextClass != q.ContextClass {
			continue
		}
		out = append(out, s)
		if len(out) == q.Limit {
			break
		}
	}
	return out, nil
}

func demandSample(key string, amount float64, when time.Time, task, repo, ctx string) harnessmodel.ProviderDemandSample {
	return harnessmodel.ProviderDemandSample{
		UsageKey: key, AccountID: "acct", Provider: harnessmodel.ProviderCodex, ModelID: "model-a",
		Metric: harnessmodel.QuotaMetricTokens, Amount: amount, TaskClass: task, RepositoryClass: repo, ContextClass: ctx, ObservedAt: when,
	}
}

func TestEstimatorUsesNearestRankP50P80ForExactHistory(t *testing.T) {
	now := time.Unix(10000, 0).UTC()
	source := &fakeHistorySource{}
	for i := 1; i <= 10; i++ {
		source.samples = append(source.samples, demandSample(fmt.Sprintf("u-%02d", i), float64(i), now.Add(-time.Duration(i)*time.Hour), "code", "medium", "warm"))
	}
	est := Estimator{Source: source, Policy: ConservativePolicy()}
	got, err := est.Estimate(context.Background(), Request{
		Provider: harnessmodel.ProviderCodex, ModelID: "model-a", Metric: harnessmodel.QuotaMetricTokens,
		TaskClass: "code", RepositoryClass: "medium", ContextClass: "warm", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.MatchLevel != MatchExact || got.SamplesUsed != 10 || got.P50 != 5 || got.P80 != 8 || got.ReservationAmount != 8 {
		t.Fatalf("unexpected estimate: %+v", got)
	}
	if len(source.queries) != 1 {
		t.Fatalf("queries=%d want=1", len(source.queries))
	}
}

func TestEstimatorFallsBackWithoutCrossingModelOrMetric(t *testing.T) {
	now := time.Unix(20000, 0).UTC()
	source := &fakeHistorySource{}
	// Four exact samples are below the exact threshold. Eight task+repo samples
	// are sufficient once context is dropped.
	for i := 0; i < 4; i++ {
		source.samples = append(source.samples, demandSample(fmt.Sprintf("exact-%d", i), 10+float64(i), now.Add(-time.Duration(i+1)*time.Hour), "code", "large", "hot"))
	}
	for i := 0; i < 4; i++ {
		source.samples = append(source.samples, demandSample(fmt.Sprintf("otherctx-%d", i), 20+float64(i), now.Add(-time.Duration(i+5)*time.Hour), "code", "large", "cold"))
	}
	// These would distort the estimate if provider/model/metric filters were relaxed.
	wrongModel := demandSample("wrong-model", 9999, now.Add(-time.Hour), "code", "large", "hot")
	wrongModel.ModelID = "model-b"
	source.samples = append(source.samples, wrongModel)
	wrongMetric := demandSample("wrong-metric", 9999, now.Add(-time.Hour), "code", "large", "hot")
	wrongMetric.Metric = harnessmodel.QuotaMetricRequests
	source.samples = append(source.samples, wrongMetric)

	est := Estimator{Source: source, Policy: ConservativePolicy()}
	got, err := est.Estimate(context.Background(), Request{
		Provider: harnessmodel.ProviderCodex, ModelID: "model-a", Metric: harnessmodel.QuotaMetricTokens,
		TaskClass: "code", RepositoryClass: "large", ContextClass: "hot", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.MatchLevel != MatchRepository || got.SamplesUsed != 8 || got.P80 != 22 {
		t.Fatalf("unexpected repository fallback: %+v", got)
	}
	if len(source.queries) != 2 {
		t.Fatalf("queries=%d want=2", len(source.queries))
	}
	for _, q := range source.queries {
		if q.Provider != harnessmodel.ProviderCodex || q.ModelID != "model-a" || q.Metric != harnessmodel.QuotaMetricTokens {
			t.Fatalf("hard dimension relaxed: %+v", q)
		}
	}
}

func TestEstimatorFailsClosedOnInsufficientHistoryAndOpaqueMetric(t *testing.T) {
	now := time.Unix(30000, 0).UTC()
	source := &fakeHistorySource{samples: []harnessmodel.ProviderDemandSample{
		demandSample("only-one", 7, now.Add(-time.Hour), "code", "small", "cold"),
	}}
	est := Estimator{Source: source, Policy: ConservativePolicy()}
	_, err := est.Estimate(context.Background(), Request{
		Provider: harnessmodel.ProviderCodex, ModelID: "model-a", Metric: harnessmodel.QuotaMetricTokens,
		TaskClass: "code", RepositoryClass: "small", ContextClass: "cold", Now: now,
	})
	if !errors.Is(err, ErrInsufficientHistory) {
		t.Fatalf("error=%v want ErrInsufficientHistory", err)
	}
	_, err = est.Estimate(context.Background(), Request{
		Provider: harnessmodel.ProviderCodex, ModelID: "model-a", Metric: harnessmodel.QuotaMetricOpaque,
		TaskClass: "code", RepositoryClass: "small", ContextClass: "cold", Now: now,
	})
	if err == nil {
		t.Fatal("OPAQUE request unexpectedly accepted")
	}
}

func TestEstimatorRejectsFutureAndDuplicateHistory(t *testing.T) {
	now := time.Unix(40000, 0).UTC()
	policy := ConservativePolicy()
	policy.MinExactSamples = 1
	policy.MinFallbackSamples = 1

	future := demandSample("future", 1, now.Add(policy.MaxFutureSkew+time.Second), "code", "small", "warm")
	est := Estimator{Source: &fakeHistorySource{samples: []harnessmodel.ProviderDemandSample{future}}, Policy: policy}
	if _, err := est.Estimate(context.Background(), Request{Provider: harnessmodel.ProviderCodex, ModelID: "model-a", Metric: harnessmodel.QuotaMetricTokens, TaskClass: "code", RepositoryClass: "small", ContextClass: "warm", Now: now}); err == nil {
		t.Fatal("future-skewed history unexpectedly accepted")
	}

	s := demandSample("dup", 1, now.Add(-time.Hour), "code", "small", "warm")
	est.Source = duplicateSource{s: s}
	if _, err := est.Estimate(context.Background(), Request{Provider: harnessmodel.ProviderCodex, ModelID: "model-a", Metric: harnessmodel.QuotaMetricTokens, TaskClass: "code", RepositoryClass: "small", ContextClass: "warm", Now: now}); err == nil {
		t.Fatal("duplicate history unexpectedly accepted")
	}
}

type duplicateSource struct{ s harnessmodel.ProviderDemandSample }

func (d duplicateSource) ListProviderDemandHistory(context.Context, harnessmodel.ProviderDemandHistoryQuery) ([]harnessmodel.ProviderDemandSample, error) {
	return []harnessmodel.ProviderDemandSample{d.s, d.s}, nil
}

func TestEstimatorPreservesFractionUnits(t *testing.T) {
	now := time.Unix(50000, 0).UTC()
	policy := ConservativePolicy()
	policy.MinExactSamples = 3
	samples := []harnessmodel.ProviderDemandSample{}
	for i, v := range []float64{0.1, 0.25, 0.8} {
		s := demandSample(fmt.Sprintf("f-%d", i), v, now.Add(-time.Duration(i+1)*time.Minute), "review", "small", "warm")
		s.Metric = harnessmodel.QuotaMetricFraction
		samples = append(samples, s)
	}
	got, err := (Estimator{Source: &fakeHistorySource{samples: samples}, Policy: policy}).Estimate(context.Background(), Request{
		Provider: harnessmodel.ProviderCodex, ModelID: "model-a", Metric: harnessmodel.QuotaMetricFraction,
		TaskClass: "review", RepositoryClass: "small", ContextClass: "warm", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.P80 != 0.8 || got.ReservationAmount != 0.8 || got.Metric != harnessmodel.QuotaMetricFraction {
		t.Fatalf("fraction units changed: %+v", got)
	}
}
