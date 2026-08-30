package fault

import (
	"errors"
	"testing"
	"time"
)

func TestMutationsKillDefects(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	policy := DefaultPolicy()

	t.Run("mutant: content filter permitted to retry", func(t *testing.T) {
		fault := Classify(errors.New("safety violation: blocked prompt"))
		dec, err := Decide(DecisionInput{
			Fault:                fault,
			TotalAttempts:        1,
			SameProviderAttempts: 1,
			Policy:               policy,
			Now:                  now,
		})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Action != ActionTerminalFail {
			t.Fatalf("mutant survival: content filter was permitted to retry (action=%s)", dec.Action)
		}
	})

	t.Run("mutant: retry-after duration ignored in backoff calculation", func(t *testing.T) {
		fault := Classify(errors.New("429 rate limit exceeded, retry-after: 15s"))
		if fault.RetryAfter != 15*time.Second {
			t.Fatalf("mutant survival: retry-after was not parsed (got %v)", fault.RetryAfter)
		}
		dec, err := Decide(DecisionInput{
			Fault:                fault,
			TotalAttempts:        1,
			SameProviderAttempts: 1,
			Policy:               policy,
			Now:                  now,
			Random:               func() float64 { return 0.5 },
		})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Delay < 15*time.Second {
			t.Fatalf("mutant survival: retry-after was not respected in backoff delay (got %v, want >= 15s)", dec.Delay)
		}
	})

	t.Run("mutant: endless same provider retries permitted", func(t *testing.T) {
		fault := Classify(errors.New("500 internal server error"))
		dec, err := Decide(DecisionInput{
			Fault:                fault,
			TotalAttempts:        3,
			SameProviderAttempts: 3, // equal to MaxSameProviderAttempts (3)
			Policy:               policy,
			Now:                  now,
		})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Action == ActionRetrySame {
			t.Fatal("mutant survival: same provider attempt limit was ignored")
		}
		if dec.Action != ActionFailover {
			t.Fatalf("mutant survival: expected FAILOVER, got %s", dec.Action)
		}
	})

	t.Run("mutant: total attempts exceeded without terminal failure", func(t *testing.T) {
		fault := Classify(errors.New("503 overloaded"))
		dec, err := Decide(DecisionInput{
			Fault:                fault,
			TotalAttempts:        5, // equal to MaxTotalAttempts (5)
			SameProviderAttempts: 1,
			Policy:               policy,
			Now:                  now,
		})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Action != ActionTerminalFail {
			t.Fatalf("mutant survival: max total attempts exceeded without TERMINAL_FAIL (action=%s)", dec.Action)
		}
	})
}
