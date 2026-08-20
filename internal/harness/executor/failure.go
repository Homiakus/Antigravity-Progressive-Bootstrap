package executor

import (
	"context"
	"errors"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

// ClassifyFailure converts process-level execution facts into the Harness error
// taxonomy. Timeout and cancellation are authoritative. Other process failures
// default to APPLICATION_PERMANENT because a generic executor cannot safely
// infer whether rerunning an arbitrary command is transient or side-effect-free;
// specialized adapters may refine that classification with protocol knowledge.
func ClassifyFailure(result Result, err error) harnessmodel.Failure {
	if err == nil {
		return harnessmodel.Failure{}
	}
	failure := harnessmodel.Failure{Message: err.Error()}
	switch {
	case result.TimedOut || errors.Is(err, context.DeadlineExceeded):
		failure.Class = harnessmodel.ErrorTimeout
	case result.Cancelled || errors.Is(err, context.Canceled):
		failure.Class = harnessmodel.ErrorCancelled
	default:
		failure.Class = harnessmodel.ErrorApplicationPermanent
	}
	return failure
}
