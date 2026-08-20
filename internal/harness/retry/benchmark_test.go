package retry

import (
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func BenchmarkRetryDecisionStorm100K(b *testing.B) {
	policy := harnessmodel.RetryPolicySpec{
		MaxAttempts:   5,
		InitialDelay:  100 * time.Millisecond,
		BackoffFactor: 2,
		MaxDelay:      30 * time.Second,
		Jitter:        0.2,
		RetryableClasses: []harnessmodel.ErrorClass{
			harnessmodel.ErrorApplicationTransient,
			harnessmodel.ErrorInfraTransient,
			harnessmodel.ErrorRateLimited,
			harnessmodel.ErrorTimeout,
		},
	}
	failure := harnessmodel.Failure{Class: harnessmodel.ErrorInfraTransient}
	first := time.Unix(1_700_000_000, 0).UTC()
	now := first.Add(time.Second)
	random := func() float64 { return 0.5 }
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		for i := 0; i < 100_000; i++ {
			decision, err := Decide(DecisionInput{
				Policy: policy,
				Failure: failure,
				AttemptNumber: 1 + i%4,
				FirstAttemptAt: first,
				Now: now,
				Random: random,
			})
			if err != nil || !decision.Retry {
				b.Fatalf("unexpected retry decision at %d: decision=%+v err=%v", i, decision, err)
			}
		}
	}
}
