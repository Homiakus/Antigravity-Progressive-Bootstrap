package process

import (
	"context"
	"errors"
	"testing"
	"time"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
)

func TestStartTimeoutClassIsDistinct(t *testing.T) {
	executor := New(Options{})
	prepared := helperPrepared(t, executor, "start-timeout", "sleep", "3s")
	prepared.Request.Timeouts.Start = time.Nanosecond
	result, err := executor.Execute(context.Background(), prepared, nil)
	if err == nil {
		t.Fatal("nanosecond start budget unexpectedly succeeded")
	}
	if !errors.Is(err, errStartTimeout) || !result.TimedOut || result.TimeoutClass != "START_TIMEOUT" {
		t.Fatalf("unexpected start-timeout result: %+v err=%v", result, err)
	}
	status, reconcileErr := executor.Reconcile(context.Background(), prepared.Request.ID)
	if reconcileErr != nil {
		t.Fatal(reconcileErr)
	}
	if status.State != harnessexecutor.RuntimeUnknown {
		t.Fatalf("start-timeout execution leaked live runtime state: %+v", status)
	}
}

func TestContextCancellationMarksResultAndTerminatesProcess(t *testing.T) {
	executor := New(Options{})
	prepared := helperPrepared(t, executor, "context-cancel", "sleep", "5s")
	ctx, cancel := context.WithCancel(context.Background())
	type outcome struct {
		result harnessexecutor.Result
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := executor.Execute(ctx, prepared, nil)
		done <- outcome{result: result, err: err}
	}()
	waitRuntimeState(t, executor, prepared.Request.ID, harnessexecutor.RuntimeRunning)
	cancel()
	select {
	case got := <-done:
		if !errors.Is(got.err, context.Canceled) || !got.result.Cancelled {
			t.Fatalf("unexpected context cancellation outcome: result=%+v err=%v", got.result, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("context-cancelled process did not terminate")
	}
}
