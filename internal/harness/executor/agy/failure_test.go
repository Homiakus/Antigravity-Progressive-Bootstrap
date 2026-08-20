package agy

import (
	"context"
	"errors"
	"fmt"
	"testing"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestClassifyFailureTaxonomy(t *testing.T) {
	cases := []struct {
		name   string
		result RunResult
		err    error
		want   harnessmodel.ErrorClass
	}{
		{name: "success", want: ""},
		{name: "process timeout", result: RunResult{Process: harnessexecutor.Result{TimedOut: true}}, err: errors.New("timeout"), want: harnessmodel.ErrorTimeout},
		{name: "deadline exceeded", err: context.DeadlineExceeded, want: harnessmodel.ErrorTimeout},
		{name: "process cancelled", result: RunResult{Process: harnessexecutor.Result{Cancelled: true}}, err: errors.New("cancelled"), want: harnessmodel.ErrorCancelled},
		{name: "context cancelled", err: context.Canceled, want: harnessmodel.ErrorCancelled},
		{name: "permission outcome", result: RunResult{Protocol: Outcome{PermissionDenied: true}}, err: errors.New("tool denied"), want: harnessmodel.ErrorPolicyDenied},
		{name: "wrapped permission", err: fmt.Errorf("agy: %w", ErrPermissionDenied), want: harnessmodel.ErrorPolicyDenied},
		{name: "missing result", err: fmt.Errorf("protocol: %w", ErrMissingResult), want: harnessmodel.ErrorProtocol},
		{name: "malformed stream", result: RunResult{Protocol: Outcome{MalformedLines: 1}}, err: errors.New("bad stream"), want: harnessmodel.ErrorProtocol},
		{name: "oversized stream", result: RunResult{Protocol: Outcome{OversizedLines: 1}}, err: errors.New("bad stream"), want: harnessmodel.ErrorProtocol},
		{name: "terminal result failed", err: fmt.Errorf("remote: %w", ErrResultFailed), want: harnessmodel.ErrorApplicationPermanent},
		{name: "ambiguous error defaults safe", err: errors.New("something unexpected"), want: harnessmodel.ErrorApplicationPermanent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyFailure(tc.result, tc.err)
			if got.Class != tc.want {
				t.Fatalf("class=%q want=%q failure=%+v", got.Class, tc.want, got)
			}
			if tc.err == nil {
				if got.Message != "" {
					t.Fatalf("successful classification returned message %q", got.Message)
				}
				return
			}
			if got.Message == "" {
				t.Fatal("classified failure lost error message")
			}
		})
	}
}

func TestTimeoutClassificationWinsOverCancellationFlags(t *testing.T) {
	result := RunResult{Process: harnessexecutor.Result{TimedOut: true, Cancelled: true}}
	got := ClassifyFailure(result, context.Canceled)
	if got.Class != harnessmodel.ErrorTimeout {
		t.Fatalf("timeout/cancel race classified as %q want TIMEOUT", got.Class)
	}
}
