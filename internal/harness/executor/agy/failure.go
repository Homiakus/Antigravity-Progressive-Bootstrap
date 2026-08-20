package agy

import (
	"context"
	"errors"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

// ClassifyFailure converts observable AGY/process facts into the Harness error
// taxonomy. It deliberately defaults ambiguous failures to permanent rather
// than creating unsafe retries. Provider/network adapters may override this
// with a more specific transient Failure when they have stronger evidence.
func ClassifyFailure(result RunResult, err error) harnessmodel.Failure {
	if err == nil {
		return harnessmodel.Failure{}
	}
	failure := harnessmodel.Failure{Class: harnessmodel.ErrorApplicationPermanent, Message: err.Error()}
	if result.Process.TimedOut || errors.Is(err, context.DeadlineExceeded) {
		failure.Class = harnessmodel.ErrorTimeout
		return failure
	}
	if result.Process.Cancelled || errors.Is(err, context.Canceled) {
		failure.Class = harnessmodel.ErrorCancelled
		return failure
	}
	if result.Protocol.PermissionDenied || errors.Is(err, ErrPermissionDenied) {
		failure.Class = harnessmodel.ErrorPolicyDenied
		return failure
	}
	if errors.Is(err, ErrMissingResult) || result.Protocol.MalformedLines > 0 || result.Protocol.OversizedLines > 0 {
		failure.Class = harnessmodel.ErrorProtocol
		return failure
	}
	if errors.Is(err, ErrResultFailed) {
		failure.Class = harnessmodel.ErrorApplicationPermanent
		return failure
	}
	return failure
}
