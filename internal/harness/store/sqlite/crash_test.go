package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/harness/events"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

const crashHelperEnv = "AGCTL_HARNESS_CRASH_HELPER"

func TestCrashAtomicitySubprocess(t *testing.T) {
	if os.Getenv(crashHelperEnv) != "" {
		return
	}
	cases := []struct {
		point         string
		wantCommitted bool
	}{
		{point: "before_update", wantCommitted: false},
		{point: "between_update_event", wantCommitted: false},
		{point: "between_event_commit", wantCommitted: false},
		{point: "after_commit", wantCommitted: true},
	}
	for _, tc := range cases {
		t.Run(tc.point, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.db")
			now := time.Unix(1000, 0).UTC()
			db, err := Open(context.Background(), path, Options{})
			if err != nil {
				t.Fatal(err)
			}
			seedRun(t, db, now)
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command(os.Args[0], "-test.run=^TestCrashHelper$")
			cmd.Env = append(os.Environ(), crashHelperEnv+"=1", "AGCTL_HARNESS_CRASH_DB="+path, "AGCTL_HARNESS_CRASH_POINT="+tc.point)
			err = cmd.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 91 {
				t.Fatalf("crash helper exit=%v, want code 91", err)
			}

			reopened, err := Open(context.Background(), path, Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if err := reopened.View(context.Background(), func(r harnessstore.Reader) error {
				run, err := r.GetWorkflowRun(context.Background(), "wfr_test")
				if err != nil {
					return err
				}
				evs, err := r.ListEvents(context.Background(), "wfr_test", 0, 10)
				if err != nil {
					return err
				}
				if tc.wantCommitted {
					if run.State != harnessmodel.WorkflowRunning || len(evs) != 1 || evs[0].WorkflowSeq != 1 {
						t.Fatalf("committed transaction not recovered atomically: run=%+v events=%+v", run, evs)
					}
				} else if run.State != harnessmodel.WorkflowCreated || len(evs) != 0 {
					t.Fatalf("partial transaction survived crash: run=%+v events=%+v", run, evs)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCrashHelper(t *testing.T) {
	if os.Getenv(crashHelperEnv) == "" {
		t.Skip("subprocess helper")
	}
	path := os.Getenv("AGCTL_HARNESS_CRASH_DB")
	point := os.Getenv("AGCTL_HARNESS_CRASH_POINT")
	ctx := context.Background()
	db, err := Open(ctx, path, Options{})
	if err != nil {
		os.Exit(92)
	}
	now := time.Unix(1001, 0).UTC()
	if point == "before_update" {
		os.Exit(91)
	}
	err = db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.UpdateWorkflowRunState(ctx, "wfr_test", harnessmodel.WorkflowRunning, now); err != nil {
			return err
		}
		if point == "between_update_event" {
			os.Exit(91)
		}
		if _, err := tx.AppendEvent(ctx, events.Event{ID: "evt_crash", WorkflowRunID: "wfr_test", Type: "WorkflowRunning", Timestamp: now, EntityType: "workflow_run", EntityID: "wfr_test", PayloadVersion: 1, Payload: json.RawMessage(`{"state":"RUNNING"}`)}, &events.OutboxMessage{Topic: "workflow.events"}); err != nil {
			return err
		}
		if point == "between_event_commit" {
			os.Exit(91)
		}
		return nil
	})
	if err != nil {
		os.Exit(93)
	}
	if point == "after_commit" {
		os.Exit(91)
	}
	os.Exit(94)
}
