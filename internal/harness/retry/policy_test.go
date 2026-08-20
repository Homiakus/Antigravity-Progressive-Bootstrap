package retry

import (
	"strings"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func basePolicy() harnessmodel.RetryPolicySpec {
	return harnessmodel.RetryPolicySpec{
		MaxAttempts:   4,
		InitialDelay:  time.Second,
		BackoffFactor: 2,
		MaxDelay:      10 * time.Second,
	}
}

func TestHardNonRetryableClassesNeverRetry(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	classes := []harnessmodel.ErrorClass{
		harnessmodel.ErrorApplicationPermanent,
		harnessmodel.ErrorCancelled,
		harnessmodel.ErrorPolicyDenied,
		harnessmodel.ErrorUnschedulable,
		harnessmodel.ErrorUnknownEffect,
	}
	for _, class := range classes {
		t.Run(string(class), func(t *testing.T) {
			p := basePolicy()
			// Even an explicit retryable list cannot make these automatic retries.
			p.RetryableClasses = []harnessmodel.ErrorClass{class}
			decision, err := Decide(DecisionInput{Policy: p, Failure: harnessmodel.Failure{Class: class}, AttemptNumber: 1, Now: now})
			if err != nil {
				t.Fatal(err)
			}
			if decision.Retry {
				t.Fatalf("hard non-retryable class %s was retried: %+v", class, decision)
			}
		})
	}
}

func TestDefaultTransientClassesRetry(t *testing.T) {
	now := time.Unix(2000, 0).UTC()
	for _, class := range []harnessmodel.ErrorClass{
		harnessmodel.ErrorApplicationTransient,
		harnessmodel.ErrorInfraTransient,
		harnessmodel.ErrorRateLimited,
		harnessmodel.ErrorTimeout,
	} {
		decision, err := Decide(DecisionInput{Policy: basePolicy(), Failure: harnessmodel.Failure{Class: class}, AttemptNumber: 1, Now: now})
		if err != nil {
			t.Fatal(err)
		}
		if !decision.Retry || decision.Delay != time.Second || !decision.NotBefore.Equal(now.Add(time.Second)) {
			t.Fatalf("class %s unexpected decision: %+v", class, decision)
		}
	}
}

func TestProtocolErrorRequiresExplicitRetryPolicy(t *testing.T) {
	now := time.Unix(3000, 0).UTC()
	p := basePolicy()
	decision, err := Decide(DecisionInput{Policy: p, Failure: harnessmodel.Failure{Class: harnessmodel.ErrorProtocol}, AttemptNumber: 1, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Retry {
		t.Fatal("protocol error retried by default")
	}
	p.RetryableClasses = []harnessmodel.ErrorClass{harnessmodel.ErrorProtocol}
	decision, err = Decide(DecisionInput{Policy: p, Failure: harnessmodel.Failure{Class: harnessmodel.ErrorProtocol}, AttemptNumber: 1, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Retry {
		t.Fatalf("explicit protocol retry was rejected: %+v", decision)
	}
}

func TestRetryAfterIsLowerBound(t *testing.T) {
	now := time.Unix(4000, 0).UTC()
	decision, err := Decide(DecisionInput{
		Policy: basePolicy(), Failure: harnessmodel.Failure{Class: harnessmodel.ErrorRateLimited, RetryAfter: 17 * time.Second},
		AttemptNumber: 1, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Retry || decision.Delay != 17*time.Second || !decision.NotBefore.Equal(now.Add(17*time.Second)) {
		t.Fatalf("Retry-After was not respected: %+v", decision)
	}
}

func TestMaxAttemptsAndElapsedTime(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	p := basePolicy()
	p.MaxAttempts = 2
	decision, err := Decide(DecisionInput{Policy: p, Failure: harnessmodel.Failure{Class: harnessmodel.ErrorInfraTransient}, AttemptNumber: 2, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Retry || !strings.Contains(decision.Reason, "max attempts") {
		t.Fatalf("max attempts not enforced: %+v", decision)
	}

	p = basePolicy()
	p.MaxElapsedTime = 5 * time.Second
	decision, err = Decide(DecisionInput{
		Policy: p, Failure: harnessmodel.Failure{Class: harnessmodel.ErrorInfraTransient}, AttemptNumber: 2,
		FirstAttemptAt: now.Add(-4 * time.Second), Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Retry || !strings.Contains(decision.Reason, "exceed max elapsed") {
		t.Fatalf("next retry crossed max elapsed deadline: %+v", decision)
	}
}

func TestBackoffCapsAndJitterIsDeterministic(t *testing.T) {
	p := basePolicy()
	p.Jitter = 0
	for attempt, want := range map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 3: 4 * time.Second, 4: 8 * time.Second, 8: 10 * time.Second} {
		got, err := ComputeDelay(p, attempt, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("attempt %d delay=%s want=%s", attempt, got, want)
		}
	}
	p.Jitter = 0.5
	low, err := ComputeDelay(p, 1, func() float64 { return 0 })
	if err != nil {
		t.Fatal(err)
	}
	high, err := ComputeDelay(p, 1, func() float64 { return 1 })
	if err != nil {
		t.Fatal(err)
	}
	if low != 500*time.Millisecond || high != 1500*time.Millisecond {
		t.Fatalf("unexpected deterministic jitter low=%s high=%s", low, high)
	}
}

func TestBackoffCannotOverflowDuration(t *testing.T) {
	p := basePolicy()
	p.InitialDelay = time.Hour
	p.MaxDelay = 0
	p.BackoffFactor = 10
	p.Jitter = 1
	got, err := ComputeDelay(p, 10000, func() float64 { return 1 })
	if err != nil {
		t.Fatal(err)
	}
	if got <= 0 {
		t.Fatalf("overflow produced non-positive duration: %s", got)
	}
}
