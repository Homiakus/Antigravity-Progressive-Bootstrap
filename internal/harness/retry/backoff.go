package retry

import (
	"fmt"
	"math"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

const maxDuration = time.Duration(1<<63 - 1)

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
	capDelay := maxDuration
	if policy.MaxDelay > 0 {
		capDelay = policy.MaxDelay
	}
	base := policy.InitialDelay
	for i := 1; i < failedAttemptNumber && base > 0 && base < capDelay; i++ {
		next := float64(base) * factor
		// Keep conversion back to time.Duration strictly inside int64 range.
		if math.IsInf(next, 1) || next >= float64(capDelay)-4096 {
			base = capDelay
			break
		}
		base = time.Duration(next)
	}
	if base > capDelay {
		base = capDelay
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
	if jittered <= 0 {
		return 0, nil
	}
	if math.IsInf(jittered, 1) || jittered >= float64(capDelay)-4096 {
		return capDelay, nil
	}
	result := time.Duration(jittered)
	if result > capDelay {
		result = capDelay
	}
	return result, nil
}
