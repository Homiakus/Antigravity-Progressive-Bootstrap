package demand

import (
	"context"
	"fmt"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func BenchmarkEstimatorExact128(b *testing.B) {
	now := time.Unix(80000, 0).UTC()
	samples := make([]harnessmodel.ProviderDemandSample, 128)
	for i := range samples {
		samples[i] = demandSample(fmt.Sprintf("bench-%03d", i), float64((i%31)+1), now.Add(-time.Duration(i)*time.Minute), "code", "large", "warm")
	}
	source := &fakeHistorySource{samples: samples}
	estimator := Estimator{Source: source, Policy: ConservativePolicy()}
	req := Request{Provider: harnessmodel.ProviderCodex, ModelID: "model-a", Metric: harnessmodel.QuotaMetricTokens, TaskClass: "code", RepositoryClass: "large", ContextClass: "warm", Now: now}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source.queries = source.queries[:0]
		if _, err := estimator.Estimate(context.Background(), req); err != nil {
			b.Fatal(err)
		}
	}
}
