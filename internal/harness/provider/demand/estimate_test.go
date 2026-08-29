package demand

import (
	"math"
	"math/rand"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

var testNow = time.Date(2026, 8, 29, 5, 0, 0, 0, time.UTC)

func testKey() Key {
	return Key{
		TaskClass: "implement",
		Provider: harnessmodel.ProviderCodex,
		ModelID: harnessmodel.ProviderModelID("gpt-test"),
		RepositoryID: "repo-a",
		ContextClass: "medium",
		Metric: harnessmodel.QuotaMetricTokens,
	}
}

func samplesFor(key Key, amounts ...float64) []Sample {
	out := make([]Sample, 0, len(amounts))
	for i, amount := range amounts {
		out = append(out, Sample{Key: key, Amount: amount, ObservedAt: testNow.Add(-time.Duration(i) * time.Minute)})
	}
	return out
}

func smallPolicy() Policy {
	p := DefaultPolicy()
	p.MinSamples = 5
	p.TargetSamples = 10
	p.MaxSamples = 20
	return p
}

func TestEstimateUsesEmpiricalP80ForReservation(t *testing.T) {
	got, err := EstimateAt(testKey(), samplesFor(testKey(), 10, 20, 30, 40, 50), testNow, smallPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || got.Source != SourceExact {
		t.Fatalf("unexpected estimate source: %+v", got)
	}
	if got.P50 != 30 || got.P80 != 40 || got.Reservation != 40 {
		t.Fatalf("nearest-rank estimate mismatch: %+v", got)
	}
	if got.Confidence != 0.5 {
		t.Fatalf("unexpected confidence: %v", got.Confidence)
	}
}

func TestP80ResistsSingleExtremeMaximum(t *testing.T) {
	got, err := EstimateAt(testKey(), samplesFor(testKey(), 1, 2, 3, 4, 100), testNow, smallPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if got.P80 != 4 || got.Reservation != 4 {
		t.Fatalf("single maximum incorrectly dominated p80: %+v", got)
	}
	if got.Reservation == got.P50 {
		t.Fatal("mutation sentinel: reservation collapsed to p50")
	}
}

func TestFallbackHierarchyPrefersMostSpecificSufficientPopulation(t *testing.T) {
	query := testKey()
	otherContext := query
	otherContext.ContextClass = "large"
	otherRepo := otherContext
	otherRepo.RepositoryID = "repo-b"
	otherModel := otherRepo
	otherModel.ModelID = harnessmodel.ProviderModelID("other-model")
	otherTask := otherModel
	otherTask.TaskClass = "review"

	cases := []struct {
		name    string
		samples []Sample
		want    Source
	}{
		{"exact", samplesFor(query, 1, 2, 3, 4, 5), SourceExact},
		{"without-context", samplesFor(otherContext, 1, 2, 3, 4, 5), SourceWithoutContext},
		{"without-repository", samplesFor(otherRepo, 1, 2, 3, 4, 5), SourceWithoutRepository},
		{"without-model", samplesFor(otherModel, 1, 2, 3, 4, 5), SourceWithoutModel},
		{"provider-metric", samplesFor(otherTask, 1, 2, 3, 4, 5), SourceProviderMetric},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EstimateAt(query, tc.samples, testNow, smallPolicy())
			if err != nil {
				t.Fatal(err)
			}
			if !got.Available || got.Source != tc.want {
				t.Fatalf("got source %s, want %s: %+v", got.Source, tc.want, got)
			}
		})
	}
}

func TestInsufficientExactHistoryFallsBackWithoutMixingMetricOrProvider(t *testing.T) {
	query := testKey()
	exact := samplesFor(query, 100, 110, 120)
	broader := query
	broader.ContextClass = "large"
	broaderSamples := samplesFor(broader, 10, 20, 30, 40, 50)

	wrongProvider := broader
	wrongProvider.Provider = harnessmodel.ProviderAntigravity
	wrongMetric := broader
	wrongMetric.Metric = harnessmodel.QuotaMetricRequests

	all := append([]Sample{}, exact...)
	all = append(all, broaderSamples...)
	all = append(all, samplesFor(wrongProvider, 900, 900, 900, 900, 900)...)
	all = append(all, samplesFor(wrongMetric, 800, 800, 800, 800, 800)...)

	got, err := EstimateAt(query, all, testNow, smallPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != SourceWithoutContext {
		t.Fatalf("expected context fallback, got %+v", got)
	}
	// Compatible amounts are 10,20,30,40,50,100,110,120. Nearest-rank
	// p80 uses ceil(0.8*8)=7 -> 110. Foreign provider/metric 900/800 values
	// must never enter this population.
	if got.SampleCount != 8 || got.P80 != 110 {
		t.Fatalf("provider/metric isolation failed: %+v", got)
	}
}

func TestNoInventedColdStartDemand(t *testing.T) {
	got, err := EstimateAt(testKey(), nil, testNow, smallPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if got.Available || got.Source != SourceUnavailable || got.Reservation != 0 {
		t.Fatalf("missing history invented demand: %+v", got)
	}

	p := smallPolicy()
	p.ColdStart[harnessmodel.QuotaMetricTokens] = 4096
	got, err = EstimateAt(testKey(), nil, testNow, p)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || got.Source != SourceColdStart || got.Reservation != 4096 || got.Confidence != 0.20 {
		t.Fatalf("explicit cold start not preserved: %+v", got)
	}
}

func TestInvalidFractionAndOpaqueDemandFailClosed(t *testing.T) {
	fraction := testKey()
	fraction.Metric = harnessmodel.QuotaMetricFraction
	p := smallPolicy()
	p.ColdStart[harnessmodel.QuotaMetricFraction] = 1.01
	if _, err := EstimateAt(fraction, nil, testNow, p); err == nil {
		t.Fatal("fractional cold start above one accepted")
	}

	opaque := testKey()
	opaque.Metric = harnessmodel.QuotaMetricOpaque
	if _, err := EstimateAt(opaque, nil, testNow, smallPolicy()); err == nil {
		t.Fatal("opaque demand accepted for arithmetic estimation")
	}
}

func TestStaleAndExcessivelyFutureSamplesDoNotCount(t *testing.T) {
	p := smallPolicy()
	query := testKey()
	valid := samplesFor(query, 10, 20, 30, 40, 50)
	stale := Sample{Key: query, Amount: 999, ObservedAt: testNow.Add(-p.MaxAge - time.Second)}
	future := Sample{Key: query, Amount: 888, ObservedAt: testNow.Add(p.MaxFutureSkew + time.Second)}
	all := append(valid, stale, future)
	got, err := EstimateAt(query, all, testNow, p)
	if err != nil {
		t.Fatal(err)
	}
	if got.SampleCount != len(valid) || got.P80 != 40 {
		t.Fatalf("stale/future evidence entered estimate: %+v", got)
	}
}

func TestBoundedHistoryKeepsNewestSamples(t *testing.T) {
	p := smallPolicy()
	p.TargetSamples = 5
	p.MaxSamples = 5
	query := testKey()
	var all []Sample
	for i := 0; i < 10; i++ {
		all = append(all, Sample{
			Key: query, Amount: float64(100 + i),
			ObservedAt: testNow.Add(-time.Duration(i) * time.Minute),
		})
	}
	got, err := EstimateAt(query, all, testNow, p)
	if err != nil {
		t.Fatal(err)
	}
	if got.SampleCount != 5 || got.OldestAt != testNow.Add(-4*time.Minute) || got.NewestAt != testNow {
		t.Fatalf("history was not bounded by recency: %+v", got)
	}
}

func TestEqualTimestampHistoryKeepsLargerClaimsAtBoundary(t *testing.T) {
	p := smallPolicy()
	p.TargetSamples = 5
	p.MaxSamples = 5
	query := testKey()
	all := []Sample{}
	for i := 1; i <= 10; i++ {
		all = append(all, Sample{Key: query, Amount: float64(i), ObservedAt: testNow})
	}
	got, err := EstimateAt(query, all, testNow, p)
	if err != nil {
		t.Fatal(err)
	}
	if got.P80 != 9 || got.Reservation != 9 {
		t.Fatalf("equal-time truncation underestimated demand: %+v", got)
	}
}

func TestEstimateDeterministicUnderInputPermutation(t *testing.T) {
	query := testKey()
	base := samplesFor(query, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100)
	want, err := EstimateAt(query, base, testNow, smallPolicy())
	if err != nil {
		t.Fatal(err)
	}
	for seed := int64(0); seed < 25; seed++ {
		shuffled := append([]Sample(nil), base...)
		rand.New(rand.NewSource(seed)).Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		got, err := EstimateAt(query, shuffled, testNow, smallPolicy())
		if err != nil {
			t.Fatal(err)
		}
		if got.P50 != want.P50 || got.P80 != want.P80 || got.Reservation != want.Reservation || got.Confidence != want.Confidence {
			t.Fatalf("seed=%d produced non-deterministic estimate: got=%+v want=%+v", seed, got, want)
		}
	}
}

func TestBroaderFallbackReducesConfidence(t *testing.T) {
	query := testKey()
	exact, err := EstimateAt(query, samplesFor(query, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10), testNow, smallPolicy())
	if err != nil {
		t.Fatal(err)
	}
	other := query
	other.ContextClass = "other"
	broader, err := EstimateAt(query, samplesFor(other, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10), testNow, smallPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if exact.Confidence != 1 || broader.Confidence != 0.9 || broader.Confidence >= exact.Confidence {
		t.Fatalf("specificity confidence mutation survived: exact=%v broader=%v", exact.Confidence, broader.Confidence)
	}
}

func TestMalformedSampleFailsClosed(t *testing.T) {
	bad := Sample{Key: testKey(), Amount: math.NaN(), ObservedAt: testNow}
	if _, err := EstimateAt(testKey(), []Sample{bad}, testNow, smallPolicy()); err == nil {
		t.Fatal("NaN demand sample accepted")
	}
	bad.Amount = math.Inf(1)
	if _, err := EstimateAt(testKey(), []Sample{bad}, testNow, smallPolicy()); err == nil {
		t.Fatal("Inf demand sample accepted")
	}
}

func FuzzFractionEstimateStaysBounded(f *testing.F) {
	f.Add(0.1, 0.2, 0.3, 0.4, 0.5)
	f.Add(0.0, 1.0, 0.5, 0.75, 0.25)
	f.Fuzz(func(t *testing.T, a, b, c, d, e float64) {
		values := []float64{a, b, c, d, e}
		for _, value := range values {
			if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
				t.Skip()
			}
		}
		query := testKey()
		query.Metric = harnessmodel.QuotaMetricFraction
		got, err := EstimateAt(query, samplesFor(query, values...), testNow, smallPolicy())
		if err != nil {
			t.Fatal(err)
		}
		if !got.Available || got.Reservation < 0 || got.Reservation > 1 || got.P80 < got.P50 {
			t.Fatalf("fraction estimate escaped invariants: %+v", got)
		}
	})
}

func BenchmarkEstimate1000Samples(b *testing.B) {
	query := testKey()
	samples := make([]Sample, 1000)
	for i := range samples {
		samples[i] = Sample{
			Key: query,
			Amount: float64((i * 7919) % 50000),
			ObservedAt: testNow.Add(-time.Duration(i%1000) * time.Minute),
		}
	}
	p := DefaultPolicy()
	p.MinSamples = 5
	p.TargetSamples = 40
	p.MaxSamples = 256
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := EstimateAt(query, samples, testNow, p); err != nil {
			b.Fatal(err)
		}
	}
}
