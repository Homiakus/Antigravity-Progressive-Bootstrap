package fault

import (
	"fmt"
	"math"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

// RetryAction dictates the subsequent execution strategy after a provider failure.
type RetryAction string

const (
	// ActionRetrySame retries the operation on the same provider account and model with backoff.
	ActionRetrySame RetryAction = "RETRY_SAME"
	// ActionFailover abandons current provider/model and routes to an alternative eligible candidate.
	ActionFailover RetryAction = "FAILOVER"
	// ActionTripCircuitAndFailover records failure in circuit breaker, trips to OPEN if threshold reached, and fails over.
	ActionTripCircuitAndFailover RetryAction = "TRIP_CIRCUIT_AND_FAILOVER"
	// ActionTerminalFail marks the attempt permanently failed without further retries.
	ActionTerminalFail RetryAction = "TERMINAL_FAIL"
)

// Policy configures provider-aware retry and failover behavior.
type Policy struct {
	MaxSameProviderAttempts int           `json:"maxSameProviderAttempts"`
	MaxTotalAttempts        int           `json:"maxTotalAttempts"`
	InitialBackoff          time.Duration `json:"initialBackoff"`
	MaxBackoff              time.Duration `json:"maxBackoff"`
	BackoffFactor           float64       `json:"backoffFactor"`
	CircuitFailureThreshold int           `json:"circuitFailureThreshold"`
	CircuitCooldown         time.Duration `json:"circuitCooldown"`
	Jitter                  float64       `json:"jitter"`
}

// DefaultPolicy returns safe, robust defaults for provider-aware fault tolerance.
func DefaultPolicy() Policy {
	return Policy{
		MaxSameProviderAttempts: 3,
		MaxTotalAttempts:        5,
		InitialBackoff:          500 * time.Millisecond,
		MaxBackoff:              30 * time.Second,
		BackoffFactor:           2.0,
		CircuitFailureThreshold: 3,
		CircuitCooldown:         5 * time.Minute,
		Jitter:                  0.1,
	}
}

// Validate ensures policy configuration contains valid numeric boundaries.
func (p Policy) Validate() error {
	if p.MaxSameProviderAttempts < 1 {
		return fmt.Errorf("maxSameProviderAttempts must be >= 1")
	}
	if p.MaxTotalAttempts < p.MaxSameProviderAttempts {
		return fmt.Errorf("maxTotalAttempts must be >= maxSameProviderAttempts")
	}
	if p.InitialBackoff <= 0 {
		return fmt.Errorf("initialBackoff must be > 0")
	}
	if p.MaxBackoff < p.InitialBackoff {
		return fmt.Errorf("maxBackoff must be >= initialBackoff")
	}
	if p.BackoffFactor < 1.0 {
		return fmt.Errorf("backoffFactor must be >= 1.0")
	}
	if p.CircuitFailureThreshold < 1 {
		return fmt.Errorf("circuitFailureThreshold must be >= 1")
	}
	if p.CircuitCooldown <= 0 {
		return fmt.Errorf("circuitCooldown must be > 0")
	}
	if p.Jitter < 0 || p.Jitter > 1.0 {
		return fmt.Errorf("jitter must be in range [0, 1.0]")
	}
	return nil
}

// DecisionInput contains all context needed to evaluate a provider retry decision.
type DecisionInput struct {
	Fault                Classification
	TotalAttempts        int
	SameProviderAttempts int
	Circuit              *harnessmodel.ProviderCircuitState
	Policy               Policy
	Now                  time.Time
	Random               func() float64
}

// Decision records the operational action and rationale for handling a provider fault.
type Decision struct {
	Action      RetryAction   `json:"action"`
	Delay       time.Duration `json:"delay,omitempty"`
	NotBefore   time.Time     `json:"notBefore,omitempty"`
	Reason      string        `json:"reason"`
	TripCircuit bool          `json:"tripCircuit,omitempty"`
	TripScope   string        `json:"tripScope,omitempty"` // "account" or "model"
}

// Decide computes the optimal retry or failover strategy for a classified provider fault.
func Decide(in DecisionInput) (Decision, error) {
	if in.TotalAttempts < 1 {
		return Decision{}, fmt.Errorf("totalAttempts must be >= 1")
	}
	if in.SameProviderAttempts < 1 {
		return Decision{}, fmt.Errorf("sameProviderAttempts must be >= 1")
	}
	if in.Now.IsZero() {
		return Decision{}, fmt.Errorf("decision timestamp now is required")
	}
	in.Now = in.Now.UTC()

	policy := in.Policy
	if err := policy.Validate(); err != nil {
		policy = DefaultPolicy()
	}

	// 1. Hard non-retryable application violations (e.g. content/safety filter)
	if in.Fault.Kind == FaultContentFilter {
		return Decision{
			Action: ActionTerminalFail,
			Reason: "content policy violation is non-retryable across all providers",
		}, nil
	}

	// 2. Global attempt exhaustion
	if in.TotalAttempts >= policy.MaxTotalAttempts {
		return Decision{
			Action: ActionTerminalFail,
			Reason: fmt.Sprintf("max total attempts (%d) exhausted", policy.MaxTotalAttempts),
		}, nil
	}

	// 3. Account-level authentication failure -> trip account circuit and failover to other account
	if in.Fault.Kind == FaultAuthentication {
		return Decision{
			Action:      ActionTripCircuitAndFailover,
			TripCircuit: true,
			TripScope:   "account",
			Reason:      "account authentication failed; tripping account circuit and failing over",
		}, nil
	}

	// 4. Model not found / unavailable -> trip model circuit and failover
	if in.Fault.Kind == FaultModelUnavailable {
		return Decision{
			Action:      ActionTripCircuitAndFailover,
			TripCircuit: true,
			TripScope:   "model",
			Reason:      "model unavailable on provider; tripping model circuit and failing over",
		}, nil
	}

	// 5. Context limit exceeded -> non-retryable on this model; failover to candidate with larger context window
	if in.Fault.Kind == FaultContextLimitExceeded {
		return Decision{
			Action: ActionFailover,
			Reason: "prompt exceeds model context limit; failover to candidate with higher context capacity",
		}, nil
	}

	// 6. Check if current provider has reached local attempt limit -> failover to alternative provider
	if in.SameProviderAttempts >= policy.MaxSameProviderAttempts {
		return Decision{
			Action: ActionFailover,
			Reason: fmt.Sprintf("max same-provider attempts (%d) reached; failover to alternative provider", policy.MaxSameProviderAttempts),
		}, nil
	}

	// 7. Check circuit breaker state: if half-open probe failed or failure threshold is reached
	failures := 0
	if in.Circuit != nil {
		failures = in.Circuit.ConsecutiveFailures
		if in.Circuit.State == harnessmodel.CircuitHalfOpen {
			return Decision{
				Action:      ActionTripCircuitAndFailover,
				TripCircuit: true,
				TripScope:   "model",
				Reason:      "half-open probe failed; reopening circuit and failing over",
			}, nil
		}
	}
	if failures+1 >= policy.CircuitFailureThreshold {
		return Decision{
			Action:      ActionTripCircuitAndFailover,
			TripCircuit: true,
			TripScope:   "model",
			Reason:      fmt.Sprintf("circuit failure threshold (%d) reached; tripping circuit and failing over", policy.CircuitFailureThreshold),
		}, nil
	}

	// 8. Transient / Rate-limit errors on healthy circuit: compute exponential backoff and retry same provider
	if in.Fault.Retryable {
		delay := computeBackoff(policy, in.SameProviderAttempts, in.Random)
		if in.Fault.RetryAfter > delay {
			delay = in.Fault.RetryAfter
		}
		if delay > policy.MaxBackoff {
			delay = policy.MaxBackoff
		}

		notBefore := in.Now.Add(delay).UTC()
		return Decision{
			Action:    ActionRetrySame,
			Delay:     delay,
			NotBefore: notBefore,
			Reason:    fmt.Sprintf("retryable provider error (%s); retry on same provider after %v", in.Fault.Kind, delay),
		}, nil
	}

	// 9. Fallback: non-retryable unknown error -> failover if total attempts remain, else terminal fail
	return Decision{
		Action: ActionFailover,
		Reason: fmt.Sprintf("unretryable provider error (%s); failover to alternative candidate", in.Fault.Kind),
	}, nil
}

func computeBackoff(p Policy, attempt int, random func() float64) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	multiplier := math.Pow(p.BackoffFactor, float64(attempt-1))
	rawDelay := float64(p.InitialBackoff) * multiplier

	if rawDelay > float64(p.MaxBackoff) {
		rawDelay = float64(p.MaxBackoff)
	}

	// Apply jitter in range [1 - jitter, 1 + jitter]
	if p.Jitter > 0 {
		rnd := 0.5
		if random != nil {
			rnd = random()
		}
		jitterFactor := (1.0 - p.Jitter) + (rnd * 2.0 * p.Jitter)
		rawDelay *= jitterFactor
	}

	delay := time.Duration(rawDelay)
	if delay < p.InitialBackoff/2 {
		delay = p.InitialBackoff / 2
	}
	if delay > p.MaxBackoff {
		delay = p.MaxBackoff
	}
	return delay
}
