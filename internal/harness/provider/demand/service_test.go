package demand

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type fakeHistorySource struct {
	samples   []Sample
	err       error
	query     Key
	limit     int
	notBefore time.Time
	calls     int
}

func (f *fakeHistorySource) LoadDemandHistory(_ context.Context, query Key, maxSamples int, notBefore time.Time) ([]Sample, error) {
	f.calls++
	f.query = query
	f.limit = maxSamples
	f.notBefore = notBefore
	return append([]Sample(nil), f.samples...), f.err
}

func TestEstimatorPassesBoundedHistoryContract(t *testing.T) {
	p := smallPolicy()
	source := &fakeHistorySource{samples: samplesFor(testKey(), 10, 20, 30, 40, 50)}
	got, err := (Estimator{Source: source, Policy: p}).Estimate(context.Background(), testKey(), testNow)
	if err != nil {
		t.Fatal(err)
	}
	if source.calls != 1 || source.query != testKey() || source.limit != p.MaxSamples || !source.notBefore.Equal(testNow.Add(-p.MaxAge)) {
		t.Fatalf("history source contract mismatch: %+v", source)
	}
	if !got.Available || got.Reservation != 40 {
		t.Fatalf("unexpected estimate: %+v", got)
	}
}

func TestEstimatorAllowsBoundedUnionAcrossFallbackPopulations(t *testing.T) {
	p := smallPolicy()
	// More than one population's MaxSamples is legitimate: a source can return
	// distinct slices needed for exact and broader fallbacks. The total remains
	// bounded by MaxSamples * number of specificity populations.
	count := p.MaxSamples + 1
	boundedUnion := make([]Sample, count)
	for i := range boundedUnion {
		boundedUnion[i] = Sample{Key: testKey(), Amount: float64(i), ObservedAt: testNow}
	}
	if _, err := (Estimator{Source: &fakeHistorySource{samples: boundedUnion}, Policy: p}).Estimate(context.Background(), testKey(), testNow); err != nil {
		t.Fatalf("bounded fallback union rejected: %v", err)
	}
}

func TestEstimatorRejectsHistoryBeyondAllFallbackBounds(t *testing.T) {
	p := smallPolicy()
	maxReturned := p.MaxSamples * len(candidates(testKey()))
	tooMany := make([]Sample, maxReturned+1)
	for i := range tooMany {
		tooMany[i] = Sample{Key: testKey(), Amount: float64(i), ObservedAt: testNow}
	}
	_, err := (Estimator{Source: &fakeHistorySource{samples: tooMany}, Policy: p}).Estimate(context.Background(), testKey(), testNow)
	if err == nil {
		t.Fatal("history source exceeding bounded fallback union was accepted")
	}
}

func TestEstimatorPropagatesHistoryFailureAndCancellation(t *testing.T) {
	p := smallPolicy()
	source := &fakeHistorySource{err: fmt.Errorf("read failure")}
	if _, err := (Estimator{Source: source, Policy: p}).Estimate(context.Background(), testKey(), testNow); err == nil {
		t.Fatal("history failure was hidden")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	source = &fakeHistorySource{}
	if _, err := (Estimator{Source: source, Policy: p}).Estimate(ctx, testKey(), testNow); err == nil {
		t.Fatal("cancelled estimation proceeded")
	}
	if source.calls != 0 {
		t.Fatal("cancelled context reached history source")
	}
}

func TestEstimatorRequiresSource(t *testing.T) {
	if _, err := (Estimator{Policy: smallPolicy()}).Estimate(context.Background(), testKey(), testNow); err == nil {
		t.Fatal("nil history source accepted")
	}
}
