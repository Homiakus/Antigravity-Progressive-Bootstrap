package fault

import (
	"errors"
	"testing"
	"time"
)

func BenchmarkFaultClassifyAndDecide1000Operations(b *testing.B) {
	now := time.Unix(2000, 0).UTC()
	policy := DefaultPolicy()

	errSamples := []error{
		errors.New("HTTP 429: rate limit exceeded, retry-after: 2s"),
		errors.New("502 Bad Gateway: connection reset"),
		errors.New("401 Unauthorized: invalid_api_key"),
		errors.New("maximum context length exceeded: requested 50000"),
		errors.New("503 Service Unavailable: overloaded"),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			err := errSamples[j%len(errSamples)]
			c := Classify(err)
			dec, dErr := Decide(DecisionInput{
				Fault:                c,
				TotalAttempts:        1 + (j % 3),
				SameProviderAttempts: 1 + (j % 2),
				Policy:               policy,
				Now:                  now,
				Random:               func() float64 { return 0.5 },
			})
			if dErr != nil || dec.Action == "" {
				b.Fatalf("benchmark failed: %v", dErr)
			}
		}
	}
}
