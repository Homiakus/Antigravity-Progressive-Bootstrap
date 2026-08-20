package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

// TestCrashRecoveryCommittedState verifies the durable boundary available to
// Stage 1 without relying on in-process memory: a committed transaction must be
// readable after the database is fully closed and reopened.
func TestCrashRecoveryCommittedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	ctx := context.Background()
	now := time.Unix(800, 0).UTC()
	db, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	seedRun(t, db, now)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.UpdateWorkflowRunState(ctx, "wfr_test", harnessmodel.WorkflowRunning, now.Add(time.Second))
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.View(ctx, func(r harnessstore.Reader) error {
		run, err := r.GetWorkflowRun(ctx, "wfr_test")
		if err != nil {
			return err
		}
		if run.State != harnessmodel.WorkflowRunning {
			t.Fatalf("committed state did not survive reopen: %s", run.State)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database missing after reopen: %v", err)
	}
}
