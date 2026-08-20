package executor

import (
	"context"
	"errors"
	"testing"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestClassifyFailure(t *testing.T) {
	cases := []struct {
		name   string
		result Result
		err    error
		want   harnessmodel.ErrorClass
	}{
		{name: "success", want: ""},
		{name: "timed out result", result: Result{TimedOut: true}, err: errors.New("process stopped"), want: harnessmodel.ErrorTimeout},
		{name: "deadline", err: context.DeadlineExceeded, want: harnessmodel.ErrorTimeout},
		{name: "cancelled result", result: Result{Cancelled: true}, err: errors.New("process stopped"), want: harnessmodel.ErrorCancelled},
		{name: "cancelled context", err: context.Canceled, want: harnessmodel.ErrorCancelled},
		{name: "ambiguous process failure is safe permanent", err: errors.New("exit status 1"), want: harnessmodel.ErrorApplicationPermanent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyFailure(tc.result, tc.err)
			if got.Class != tc.want {
				t.Fatalf("class=%q want=%q failure=%+v", got.Class, tc.want, got)
			}
			if tc.err == nil && got.Message != "" {
				t.Fatalf("success returned failure message %q", got.Message)
			}
		})
	}
}

func TestTimeoutWinsWhenResultAlsoMarkedCancelled(t *testing.T) {
	got := ClassifyFailure(Result{TimedOut: true, Cancelled: true}, context.Canceled)
	if got.Class != harnessmodel.ErrorTimeout {
		t.Fatalf("timeout/cancel race class=%q want TIMEOUT", got.Class)
	}
}
