package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessworkspace "github.com/homiakus/agctl/internal/harness/workspace"
	"github.com/homiakus/agctl/internal/remote/model"
)

type fakeManager struct {
	allocParams  []harnessworkspace.AllocateParams
	createdRepo  string
	createdBranch string
	createdPath  string
	createdBase  string
	createErr    error
	removedRepo  string
	removedPath  string
	released     []harnessmodel.WorkspaceID
}

func (f *fakeManager) Allocate(_ context.Context, params harnessworkspace.AllocateParams) (harnessmodel.WorkspaceRecord, error) {
	f.allocParams = append(f.allocParams, params)
	return harnessmodel.WorkspaceRecord{ID: params.ID, Kind: params.Kind, BasePath: params.BasePath, RepositoryID: params.RepositoryID, Branch: params.Branch, OwnerWorkflowRunID: params.OwnerWorkflowRunID}, nil
}
func (f *fakeManager) CreateGitWorktree(_ context.Context, repo, branch, target, base string) error {
	f.createdRepo, f.createdBranch, f.createdPath, f.createdBase = repo, branch, target, base
	return f.createErr
}
func (f *fakeManager) RemoveGitWorktree(_ context.Context, repo, target string) error {
	f.removedRepo, f.removedPath = repo, target
	return nil
}
func (f *fakeManager) Release(_ context.Context, id harnessmodel.WorkspaceID) error {
	f.released = append(f.released, id)
	return nil
}

func testRepo() model.Repository {
	return model.Repository{ID: model.RepositoryID("rep_1700000000000_00000000000000000000"), Name: "My Repo", CanonicalPath: "/src/repo", GitRoot: "/src/repo", DefaultBranch: "main", Enabled: true}
}

func TestExclusiveAllocationsHaveSessionScopedOwners(t *testing.T) {
	manager := &fakeManager{}
	allocator, err := NewHarnessAllocator(manager, "/worktrees")
	if err != nil { t.Fatal(err) }
	for _, sid := range []model.RemoteSessionID{"rsi_1700000000000_00000000000000000001", "rsi_1700000000000_00000000000000000002"} {
		_, err := allocator.Allocate(context.Background(), Request{WorkspaceID: model.WorkspaceID("rws_1700000000000_00000000000000000000"), SessionID: sid, Repository: testRepo(), Mode: model.IsolationExclusiveWrite})
		if err != nil { t.Fatal(err) }
	}
	if len(manager.allocParams) != 2 { t.Fatalf("allocations=%d", len(manager.allocParams)) }
	if manager.allocParams[0].OwnerWorkflowRunID == manager.allocParams[1].OwnerWorkflowRunID { t.Fatalf("owners unexpectedly equal: %q", manager.allocParams[0].OwnerWorkflowRunID) }
	if manager.allocParams[0].Kind != harnessmodel.WorkspaceExclusive { t.Fatalf("kind=%s", manager.allocParams[0].Kind) }
}

func TestWorktreeAllocationUsesDeterministicBranchAndPath(t *testing.T) {
	manager := &fakeManager{}
	root := filepath.Join("tmp", "remote-worktrees")
	allocator, err := NewHarnessAllocator(manager, root)
	if err != nil { t.Fatal(err) }
	sid := model.RemoteSessionID("rsi_1700000000000_00000000000000000001")
	wid := model.WorkspaceID("rws_1700000000000_00000000000000000001")
	allocation, err := allocator.Allocate(context.Background(), Request{WorkspaceID: wid, SessionID: sid, Repository: testRepo(), Mode: model.IsolationWorktree})
	if err != nil { t.Fatal(err) }
	wantBranch := "agctl/remote/" + string(sid) + "/my-repo"
	if allocation.Branch != wantBranch || manager.createdBranch != wantBranch { t.Fatalf("branch=%q created=%q", allocation.Branch, manager.createdBranch) }
	wantPath := filepath.Join(root, string(sid), "my-repo")
	if allocation.Path != wantPath || manager.createdPath != wantPath { t.Fatalf("path=%q created=%q", allocation.Path, manager.createdPath) }
	if manager.createdBase != "main" { t.Fatalf("base=%q", manager.createdBase) }
}

func TestWorktreeCreationFailureReleasesHarnessLease(t *testing.T) {
	manager := &fakeManager{createErr: errors.New("git failed")}
	allocator, _ := NewHarnessAllocator(manager, "/worktrees")
	wid := model.WorkspaceID("rws_1700000000000_00000000000000000001")
	_, err := allocator.Allocate(context.Background(), Request{WorkspaceID: wid, SessionID: model.RemoteSessionID("rsi_1700000000000_00000000000000000001"), Repository: testRepo(), Mode: model.IsolationWorktree})
	if err == nil { t.Fatal("expected worktree failure") }
	if len(manager.released) != 1 || manager.released[0] != harnessmodel.WorkspaceID(wid) { t.Fatalf("released=%v", manager.released) }
}

func TestReleaseWorktreeRemovesPhysicalTreeThenLease(t *testing.T) {
	manager := &fakeManager{}
	allocator, _ := NewHarnessAllocator(manager, "/worktrees")
	allocation := Allocation{HarnessID: "rws_test", Mode: model.IsolationWorktree, RepoRoot: "/repo", Path: "/worktree"}
	if err := allocator.Release(context.Background(), allocation); err != nil { t.Fatal(err) }
	if manager.removedRepo != "/repo" || manager.removedPath != "/worktree" { t.Fatalf("removed=%q %q", manager.removedRepo, manager.removedPath) }
	if len(manager.released) != 1 || manager.released[0] != "rws_test" { t.Fatalf("released=%v", manager.released) }
}
