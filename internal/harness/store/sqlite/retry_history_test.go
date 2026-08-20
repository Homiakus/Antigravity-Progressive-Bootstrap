package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestRetryHistorySurvivesActiveDeletionAndMultipleAttempts(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	db, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(21_000, 0).UTC()
	first := seedRetryFixture(t, db, now)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.CreateRetrySchedule(ctx, first)
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.DeleteRetrySchedule(ctx, first.NodeRunID)
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		if _, err := reader.GetRetrySchedule(ctx, first.NodeRunID); !errors.Is(err, harnessstore.ErrNotFound) {
			t.Fatalf("deleted active retry still visible: %v", err)
		}
		history, err := reader.GetRetryScheduleByAttempt(ctx, first.FailedAttemptID)
		if err != nil {
			return err
		}
		if history.FailedAttemptID != first.FailedAttemptID || !history.NotBefore.Equal(first.NotBefore) {
			t.Fatalf("first retry history changed: %+v", history)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	second := harnessmodel.RetrySchedule{
		NodeRunID: first.NodeRunID, WorkflowRunID: first.WorkflowRunID,
		FailedAttemptID: "att_retry_2", AttemptNumber: 2,
		FailureClass: harnessmodel.ErrorRateLimited, PolicyRef: first.PolicyRef,
		ServiceKey: first.ServiceKey, ScheduledAt: now.Add(time.Minute), NotBefore: now.Add(2 * time.Minute),
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		attempt, err := tx.CreateNextAttempt(ctx, harnessmodel.Attempt{
			ID: second.FailedAttemptID, NodeRunID: second.NodeRunID,
			State: harnessmodel.AttemptCreated, CreatedAt: second.ScheduledAt.Add(-time.Second),
		})
		if err != nil {
			return err
		}
		if attempt.Number != 2 {
			t.Fatalf("second attempt number=%d want=2", attempt.Number)
		}
		attempt.State = harnessmodel.AttemptFailed
		attempt.StartedAt = second.ScheduledAt.Add(-time.Second)
		attempt.FinishedAt = second.ScheduledAt
		attempt.ErrorClass = string(second.FailureClass)
		if err := tx.CompareAndSwapAttempt(ctx, harnessmodel.AttemptCreated, attempt); err != nil {
			return err
		}
		return tx.CreateRetrySchedule(ctx, second)
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		active, err := reader.GetRetrySchedule(ctx, first.NodeRunID)
		if err != nil {
			return err
		}
		if active.FailedAttemptID != second.FailedAttemptID || active.AttemptNumber != 2 {
			t.Fatalf("second retry is not active after reopen: %+v", active)
		}
		for _, want := range []harnessmodel.RetrySchedule{first, second} {
			got, err := reader.GetRetryScheduleByAttempt(ctx, want.FailedAttemptID)
			if err != nil {
				return err
			}
			if got.FailedAttemptID != want.FailedAttemptID || got.AttemptNumber != want.AttemptNumber || !got.NotBefore.Equal(want.NotBefore) {
				t.Fatalf("retry history mismatch want=%+v got=%+v", want, got)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
