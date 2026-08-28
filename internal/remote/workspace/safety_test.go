package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/homiakus/agctl/internal/remote/model"
)

type fakeGitInspector struct {
	status GitStatus
	err    error
}

func (f fakeGitInspector) Status(context.Context, string) (GitStatus, error) { return f.status, f.err }

func TestDirtyRepositoryBlocksWorktreeBeforeLease(t *testing.T) {
	manager := &fakeManager{}
	allocator, _ := NewHarnessAllocator(manager, "/worktrees")
	allocator.git = fakeGitInspector{status: GitStatus{Dirty: true, Head: "abc"}}
	_, err := allocator.Allocate(context.Background(), Request{
		WorkspaceID: model.WorkspaceID("rws_1700000000000_00000000000000000001"),
		SessionID:   model.RemoteSessionID("rsi_1700000000000_00000000000000000001"),
		Repository:  testRepo(),
		Mode:        model.IsolationWorktree,
	})
	if err == nil || !errors.Is(err, ErrDirtyRepository) { t.Fatalf("expected dirty repository error, got %v", err) }
	if len(manager.allocParams) != 0 { t.Fatalf("lease created before dirty check: %+v", manager.allocParams) }
}

func TestExplicitDirtyOverrideStillUsesIsolatedWorktree(t *testing.T) {
	manager := &fakeManager{}
	allocator, _ := NewHarnessAllocator(manager, "/worktrees")
	allocator.git = fakeGitInspector{status: GitStatus{Dirty: true}}
	_, err := allocator.Allocate(context.Background(), Request{
		WorkspaceID:    model.WorkspaceID("rws_1700000000000_00000000000000000001"),
		SessionID:      model.RemoteSessionID("rsi_1700000000000_00000000000000000001"),
		Repository:     testRepo(),
		Mode:           model.IsolationWorktree,
		AllowDirtyBase: true,
	})
	if err != nil { t.Fatal(err) }
	if manager.createdPath == testRepo().CanonicalPath { t.Fatal("dirty override reused main physical tree") }
}

func TestBuildHandoffIsExplicitAndNonMerging(t *testing.T) {
	allocation := Allocation{
		ID:       model.WorkspaceID("rws_1"),
		Path:     "/worktrees/s1/repo",
		Branch:   "agctl/remote/s1/repo",
		BaseRef:  "main",
		Mode:     model.IsolationWorktree,
		RepoRoot: "/repo",
	}
	handoff, err := BuildHandoff(allocation, "main")
	if err != nil { t.Fatal(err) }
	if handoff.SourceBranch != allocation.Branch || handoff.TargetBranch != "main" { t.Fatalf("handoff=%+v", handoff) }
	if handoff.Strategy != "VERIFY_THEN_MERGE_VIA_HARNESS" { t.Fatalf("strategy=%q", handoff.Strategy) }
}
