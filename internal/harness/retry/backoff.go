package retry

import (
	"fmt"
	"math"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func ComputeDelay(policy harnessmodel.RetryPolicySpec, failedAttemptNumber int, random func() float64) (time.Duration, error) {
	if failedAttemptNumber < 1 {
		return 0, fmt.Errorf("failed attempt number must be >= 1")
	}
	if policy.InitialDelay < 0 || policy.MaxDelay < 0 {
		return 0, fmt.Errorf("retry delays must be non-negative")
	}
	if policy.BackoffFactor != 0 && policy.BackoffFactor < 1 {
		return 0, fmt.Errorf("backoff factor must be >= 1 when set")
	}
	if policy.Jitter < 0 || policy.Jitter > 1 {
		return 0, fmt.Errorf("jitter must be in [0,1]")
	}

	factor := policy.BackoffFactor
	if factor == 0 {
		factor = 2
	}
	delay := float64(policy.InitialDelay)
	if failedAttemptNumber > 1 && delay > 0 {
		power := math.Pow(factor, float64(failedAttemptNumber-1))
		delay *= power
	}
	if delay > float64(math.MaxInt64) {
		delay = float64(math.MaxInt64)
	}
	base := time.Duration(delay)
	if policy.MaxDelay > 0 && base > policy.MaxDelay {
		base = policy.MaxDelay
	}
	if base <= 0 || policy.Jitter == 0 {
		return base, nil
	}

	r := 0.5
	if random != nil {
		r = random()
	}
	if r < 0 || r > 1 || math.IsNaN(r) {
		return 0, fmt.Errorf("random jitter source must return value in [0,1]")
	}
	multiplier := (1 - policy.Jitter) + (2 * policy.Jitter * r)
	jittered := float64(base) * multiplier
	if jittered < 0 {
		jittered = 0
	}
	if jittered > float64(math.MaxInt64) {
		jittered = float64(math.MaxInt64)
	}
	result := time.Duration(jittered)
	if policy.MaxDelay > 0 && result > policy.MaxDelay {
		result = policy.MaxDelay
	}
	return result, nil
}
