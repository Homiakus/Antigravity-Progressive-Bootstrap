package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

func TestWorkspaceAllocationAndExclusiveConflict(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	db, err := sqlitestore.Open(ctx, filepath.Join(tempDir, "state.db"), sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Unix(170_000, 0).UTC()
	mgr, err := NewManager(db, Options{
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	wsPath1 := filepath.Join(tempDir, "ws1")
	_ = os.MkdirAll(wsPath1, 0o755)

	// 1. Allocate exclusive workspace for run 1
	rec1, err := mgr.Allocate(ctx, AllocateParams{
		ID:                 "ws_run1_main",
		Kind:               harnessmodel.WorkspaceExclusive,
		BasePath:           wsPath1,
		RepositoryID:       "repo_core",
		OwnerWorkflowRunID: "wfr_1",
		TTL:                1 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec1.State != harnessmodel.WorkspaceActive {
		t.Fatalf("expected state ACTIVE, got %s", rec1.State)
	}

	// 2. Second workflow run attempts exclusive workspace on SAME repository -> should fail with conflict!
	wsPath2 := filepath.Join(tempDir, "ws2")
	_ = os.MkdirAll(wsPath2, 0o755)

	_, err = mgr.Allocate(ctx, AllocateParams{
		ID:                 "ws_run2_main",
		Kind:               harnessmodel.WorkspaceExclusive,
		BasePath:           wsPath2,
		RepositoryID:       "repo_core",
		OwnerWorkflowRunID: "wfr_2",
		TTL:                1 * time.Hour,
	})
	if err == nil || !errors.Is(err, ErrWorkspaceConflict) {
		t.Fatalf("expected ErrWorkspaceConflict, got %v", err)
	}

	// 3. Second workflow run allocates isolated GIT_WORKTREE on same repository -> allowed!
	recWorktree, err := mgr.Allocate(ctx, AllocateParams{
		ID:                 "ws_run2_worktree",
		Kind:               harnessmodel.WorkspaceGitWorktree,
		BasePath:           wsPath2,
		RepositoryID:       "repo_core",
		Branch:             "feat/agent2",
		OwnerWorkflowRunID: "wfr_2",
		TTL:                1 * time.Hour,
	})
	if err != nil {
		t.Fatalf("expected worktree allocation to succeed, got %v", err)
	}
	if recWorktree.State != harnessmodel.WorkspaceActive {
		t.Fatalf("expected state ACTIVE, got %s", recWorktree.State)
	}

	// 4. Release run 1 exclusive workspace
	if err := mgr.Release(ctx, rec1.ID); err != nil {
		t.Fatal(err)
	}

	gotWs1, err := mgr.Get(ctx, rec1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotWs1.State != harnessmodel.WorkspaceReleased {
		t.Fatalf("expected state RELEASED, got %s", gotWs1.State)
	}

	// 5. Now run 2 can allocate exclusive workspace on repo_core
	_, err = mgr.Allocate(ctx, AllocateParams{
		ID:                 "ws_run2_exclusive_now",
		Kind:               harnessmodel.WorkspaceExclusive,
		BasePath:           wsPath2,
		RepositoryID:       "repo_core",
		OwnerWorkflowRunID: "wfr_2",
		TTL:                1 * time.Hour,
	})
	if err != nil {
		t.Fatalf("expected exclusive allocation after release to succeed, got %v", err)
	}
}

func TestWorkspaceReconciliationDetectsCorruptedWorkspace(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	db, err := sqlitestore.Open(ctx, filepath.Join(tempDir, "state.db"), sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Unix(170_000, 0).UTC()
	mgr, err := NewManager(db, Options{
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	wsDir := filepath.Join(tempDir, "missing_dir")
	// Note: directory does not exist on disk!

	rec, err := mgr.Allocate(ctx, AllocateParams{
		ID:                 "ws_missing",
		Kind:               harnessmodel.WorkspaceEphemeral,
		BasePath:           wsDir,
		OwnerWorkflowRunID: "wfr_recon",
		TTL:                1 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	reconciled, corrupted, err := mgr.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if reconciled != 1 || corrupted != 1 {
		t.Fatalf("expected 1 reconciled, 1 corrupted, got %d / %d", reconciled, corrupted)
	}

	updated, err := mgr.Get(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != harnessmodel.WorkspaceCorrupted {
		t.Fatalf("expected state CORRUPTED, got %s", updated.State)
	}
}
