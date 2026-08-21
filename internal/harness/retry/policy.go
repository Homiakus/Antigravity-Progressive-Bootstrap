package retry

import (
	"fmt"
	"math"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

var (
	minDurableUnixNanoTime = time.Unix(0, math.MinInt64).UTC()
	maxDurableUnixNanoTime = time.Unix(0, math.MaxInt64).UTC()
)

type Decision struct {
	Retry     bool          `json:"retry"`
	Delay     time.Duration `json:"delay,omitempty"`
	NotBefore time.Time     `json:"notBefore,omitempty"`
	Reason    string        `json:"reason"`
}

type DecisionInput struct {
	Policy         harnessmodel.RetryPolicySpec
	Failure        harnessmodel.Failure
	AttemptNumber  int
	FirstAttemptAt time.Time
	Now            time.Time
	Random         func() float64
}

func Decide(in DecisionInput) (Decision, error) {
	if in.AttemptNumber < 1 {
		return Decision{}, fmt.Errorf("attempt number must be >= 1")
	}
	if !in.Failure.Class.Valid() {
		return Decision{}, fmt.Errorf("invalid failure class %q", in.Failure.Class)
	}
	if in.Now.IsZero() {
		return Decision{}, fmt.Errorf("decision time is required")
	}
	in.Now = in.Now.UTC()
	if !durableUnixNanoTime(in.Now) {
		return Decision{}, fmt.Errorf("decision time is outside durable Unix-nanosecond range")
	}
	if in.Policy.MaxAttempts < 1 {
		return Decision{}, fmt.Errorf("retry policy maxAttempts must be >= 1")
	}

	if hardNonRetryable(in.Failure.Class) {
		return Decision{Reason: "failure class is never automatically retryable"}, nil
	}
	if contains(in.Policy.NonRetryableClasses, in.Failure.Class) {
		return Decision{Reason: "failure class is explicitly non-retryable"}, nil
	}
	if len(in.Policy.RetryableClasses) > 0 {
		if !contains(in.Policy.RetryableClasses, in.Failure.Class) {
			return Decision{Reason: "failure class is not listed as retryable"}, nil
		}
	} else if !defaultRetryable(in.Failure.Class) {
		return Decision{Reason: "failure class is not retryable by default"}, nil
	}
	if in.AttemptNumber >= in.Policy.MaxAttempts {
		return Decision{Reason: "max attempts exhausted"}, nil
	}

	delay, err := ComputeDelay(in.Policy, in.AttemptNumber, in.Random)
	if err != nil {
		return Decision{}, err
	}
	if in.Failure.RetryAfter > delay {
		delay = in.Failure.RetryAfter
	}
	notBefore := in.Now.Add(delay).UTC()
	// Durable scheduler indexes use signed int64 Unix nanoseconds. time.Duration
	// can represent almost 292 years, which is enough to push a current timestamp
	// beyond the UnixNano ceiling (2262-04-11) and wrap the persisted deadline.
	// Clamp the retry deadline to the largest representable durable timestamp;
	// this preserves monotonic ordering and never turns a huge backoff into an
	// immediately-due retry.
	if notBefore.After(maxDurableUnixNanoTime) {
		notBefore = maxDurableUnixNanoTime
		delay = notBefore.Sub(in.Now)
	}
	if !durableUnixNanoTime(notBefore) || notBefore.Before(in.Now) {
		return Decision{}, fmt.Errorf("retry deadline is outside durable Unix-nanosecond range")
	}
	if in.Policy.MaxElapsedTime > 0 && !in.FirstAttemptAt.IsZero() {
		deadline := in.FirstAttemptAt.Add(in.Policy.MaxElapsedTime)
		if !in.Now.Before(deadline) {
			return Decision{Reason: "max elapsed retry time exhausted"}, nil
		}
		if notBefore.After(deadline) {
			return Decision{Reason: "next retry would exceed max elapsed time"}, nil
		}
	}
	return Decision{Retry: true, Delay: delay, NotBefore: notBefore, Reason: "retry permitted"}, nil
}

func durableUnixNanoTime(value time.Time) bool {
	value = value.UTC()
	return !value.Before(minDurableUnixNanoTime) && !value.After(maxDurableUnixNanoTime)
}

func hardNonRetryable(class harnessmodel.ErrorClass) bool {
	switch class {
	case harnessmodel.ErrorApplicationPermanent,
		harnessmodel.ErrorCancelled,
		harnessmodel.ErrorPolicyDenied,
		harnessmodel.ErrorUnschedulable,
		harnessmodel.ErrorUnknownEffect:
		return true
	default:
		return false
	}
}

func defaultRetryable(class harnessmodel.ErrorClass) bool {
	switch class {
	case harnessmodel.ErrorApplicationTransient,
		harnessmodel.ErrorInfraTransient,
		harnessmodel.ErrorRateLimited,
		harnessmodel.ErrorTimeout:
		return true
	default:
		return false
	}
}

func contains(classes []harnessmodel.ErrorClass, class harnessmodel.ErrorClass) bool {
	for _, candidate := range classes {
		if candidate == class {
			return true
		}
	}
	return false
}
