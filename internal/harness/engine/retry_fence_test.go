package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	harnesslease "github.com/homiakus/agctl/internal/harness/lease"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestFencedRetryRejectsUnfencedAndStaleOwners(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-retry", clock.current)

	run, err := eng.StartWorkflow(ctx, retryDefinition("wfd_retry_fenced", transientPolicy()))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-retry", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, "worker-retry", claim.Lease.Epoch)
	if err != nil {
		t.Fatal(err)
	}
	failure := harnessmodel.Failure{Class: harnessmodel.ErrorInfraTransient, Message: "temporary provider failure", ServiceKey: "provider-a"}

	if _, err := eng.CompleteAttemptFailureWithRetry(ctx, attempt.ID, failure); err == nil {
		t.Fatal("unfenced retry completion unexpectedly accepted for leased attempt")
	}
	if _, err := eng.CompleteAttemptFailureWithRetryFenced(ctx, attempt.ID, "worker-retry", claim.Lease.Epoch+1, failure); !errors.Is(err, harnesslease.ErrStaleFence) {
		t.Fatalf("stale retry fence error=%v want ErrStaleFence", err)
	}

	result, err := eng.CompleteAttemptFailureWithRetryFenced(ctx, attempt.ID, "worker-retry", claim.Lease.Epoch, failure)
	if err != nil {
		t.Fatal(err)
	}
	if result.Terminal || result.RetrySchedule == nil || result.Completion.Attempt.State != harnessmodel.AttemptFailed || result.Completion.NodeRun.State != harnessmodel.NodeRetryWait {
		t.Fatalf("unexpected fenced retry result: %+v", result)
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		_, err := reader.GetCurrentLease(ctx, attempt.ID)
		return err
	}); !errors.Is(err, harnessstore.ErrNotFound) {
		t.Fatalf("retry completion left active lease: %v", err)
	}

	duplicate, err := eng.CompleteAttemptFailureWithRetryFenced(ctx, attempt.ID, "worker-retry", claim.Lease.Epoch, failure)
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate.Completion.Idempotent || duplicate.RetrySchedule == nil || duplicate.RetrySchedule.FailedAttemptID != attempt.ID {
		t.Fatalf("fenced duplicate retry was not idempotent: %+v", duplicate)
	}
	if _, err := eng.CompleteAttemptFailureWithRetryFenced(ctx, attempt.ID, "worker-retry", claim.Lease.Epoch+1, failure); !errors.Is(err, harnesslease.ErrStaleFence) {
		t.Fatalf("stale owner accepted after retry was committed: %v", err)
	}
}

func TestRetryDuplicateDoesNotAppendEventsAndSequenceRemainsContiguous(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	run, err := eng.StartWorkflow(ctx, retryDefinition("wfd_retry_events", transientPolicy()))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	attempt, err := eng.StartAttempt(ctx, node.ID)
	if err != nil {
		t.Fatal(err)
	}
	failure := harnessmodel.Failure{Class: harnessmodel.ErrorInfraTransient, Message: "temporary", ServiceKey: "provider-events"}

	first, err := eng.CompleteAttemptFailureWithRetry(ctx, attempt.ID, failure)
	if err != nil {
		t.Fatal(err)
	}
	if first.RetrySchedule == nil {
		t.Fatal("retry schedule missing")
	}
	beforeDuplicate := readWorkflowEvents(t, ctx, db, run.ID)
	if _, err := eng.CompleteAttemptFailureWithRetry(ctx, attempt.ID, failure); err != nil {
		t.Fatal(err)
	}
	afterDuplicate := readWorkflowEvents(t, ctx, db, run.ID)
	if len(afterDuplicate) != len(beforeDuplicate) {
		t.Fatalf("idempotent duplicate appended events: before=%d after=%d", len(beforeDuplicate), len(afterDuplicate))
	}

	clock.current = first.RetrySchedule.NotBefore
	if _, err := eng.ReleaseDueRetries(ctx, 10); err != nil {
		t.Fatal(err)
	}
	events := readWorkflowEvents(t, ctx, db, run.ID)
	for i, event := range events {
		want := int64(i + 1)
		if event.WorkflowSeq != want {
			t.Fatalf("event sequence gap at index=%d got=%d want=%d type=%s", i, event.WorkflowSeq, want, event.Type)
		}
	}
	if len(events) == 0 || events[len(events)-1].Type != "RetryReady" {
		t.Fatalf("RetryReady is not final release event: %+v", events)
	}
}

func readWorkflowEvents(t *testing.T, ctx context.Context, db harnessstore.Store, runID harnessmodel.WorkflowRunID) []interfaceEvent {
	t.Helper()
	var out []interfaceEvent
	err := db.View(ctx, func(reader harnessstore.Reader) error {
		events, err := reader.ListEvents(ctx, runID, 0, 1000)
		if err != nil {
			return err
		}
		out = make([]interfaceEvent, len(events))
		for i, event := range events {
			out[i] = interfaceEvent{WorkflowSeq: event.WorkflowSeq, Type: event.Type}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

type interfaceEvent struct {
	WorkflowSeq int64
	Type        string
}
