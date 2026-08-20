package retry

import (
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

type BreakerPolicy struct {
	FailureThreshold int
	Cooldown         time.Duration
}

type BreakerDecision struct {
	Breaker harnessmodel.CircuitBreaker
	Allow   bool
	Probe   bool
	Reason  string
}

// Allow checks whether a service call may start. OPEN transitions to HALF_OPEN
// only after the durable probe deadline, and exactly one caller may own the
// half-open probe through ProbeInFlight.
func Allow(b harnessmodel.CircuitBreaker, now time.Time) (BreakerDecision, error) {
	if b.ServiceKey == "" {
		return BreakerDecision{}, fmt.Errorf("circuit breaker service key is required")
	}
	if now.IsZero() {
		return BreakerDecision{}, fmt.Errorf("circuit breaker decision time is required")
	}
	if b.State == "" {
		b.State = harnessmodel.CircuitClosed
	}
	switch b.State {
	case harnessmodel.CircuitClosed:
		return BreakerDecision{Breaker: b, Allow: true, Reason: "circuit closed"}, nil
	case harnessmodel.CircuitOpen:
		if b.NextProbeAt.IsZero() || now.Before(b.NextProbeAt) {
			return BreakerDecision{Breaker: b, Reason: "circuit open"}, nil
		}
		b.State = harnessmodel.CircuitHalfOpen
		b.ProbeInFlight = true
		b.UpdatedAt = now.UTC()
		return BreakerDecision{Breaker: b, Allow: true, Probe: true, Reason: "half-open probe acquired"}, nil
	case harnessmodel.CircuitHalfOpen:
		if b.ProbeInFlight {
			return BreakerDecision{Breaker: b, Reason: "half-open probe already in flight"}, nil
		}
		b.ProbeInFlight = true
		b.UpdatedAt = now.UTC()
		return BreakerDecision{Breaker: b, Allow: true, Probe: true, Reason: "half-open probe acquired"}, nil
	default:
		return BreakerDecision{}, fmt.Errorf("invalid circuit state %q", b.State)
	}
}

func RecordSuccess(b harnessmodel.CircuitBreaker, now time.Time) (harnessmodel.CircuitBreaker, error) {
	if b.ServiceKey == "" || now.IsZero() {
		return harnessmodel.CircuitBreaker{}, fmt.Errorf("service key and time are required")
	}
	b.State = harnessmodel.CircuitClosed
	b.ConsecutiveFailures = 0
	b.OpenedAt = time.Time{}
	b.NextProbeAt = time.Time{}
	b.ProbeInFlight = false
	b.UpdatedAt = now.UTC()
	return b, nil
}

func RecordFailure(b harnessmodel.CircuitBreaker, policy BreakerPolicy, now time.Time) (harnessmodel.CircuitBreaker, error) {
	if b.ServiceKey == "" || now.IsZero() {
		return harnessmodel.CircuitBreaker{}, fmt.Errorf("service key and time are required")
	}
	if policy.FailureThreshold < 1 {
		return harnessmodel.CircuitBreaker{}, fmt.Errorf("failure threshold must be >= 1")
	}
	if policy.Cooldown <= 0 {
		return harnessmodel.CircuitBreaker{}, fmt.Errorf("circuit cooldown must be > 0")
	}
	if b.State == "" {
		b.State = harnessmodel.CircuitClosed
	}
	b.FailureThreshold = policy.FailureThreshold
	b.ConsecutiveFailures++
	b.ProbeInFlight = false
	b.UpdatedAt = now.UTC()

	if b.State == harnessmodel.CircuitHalfOpen || b.State == harnessmodel.CircuitOpen || b.ConsecutiveFailures >= policy.FailureThreshold {
		b.State = harnessmodel.CircuitOpen
		b.OpenedAt = now.UTC()
		b.NextProbeAt = now.Add(policy.Cooldown).UTC()
	}
	return b, nil
}
