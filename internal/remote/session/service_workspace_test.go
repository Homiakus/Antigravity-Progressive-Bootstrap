package session

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
	remoteworkspace "github.com/homiakus/agctl/internal/remote/workspace"
)

type fakeWorkspaceAllocator struct {
	allocation remoteworkspace.Allocation
	allocateErr error
	releases   int
	requests   []remoteworkspace.Request
}

func (f *fakeWorkspaceAllocator) Allocate(_ context.Context, request remoteworkspace.Request) (remoteworkspace.Allocation, error) {
	f.requests = append(f.requests, request)
	if f.allocateErr != nil {
		return remoteworkspace.Allocation{}, f.allocateErr
	}
	allocation := f.allocation
	if allocation.ID == "" {
		allocation.ID = request.WorkspaceID
	}
	if allocation.Path == "" {
		allocation.Path = request.Repository.CanonicalPath
	}
	if allocation.Mode == "" {
		allocation.Mode = request.Mode
	}
	return allocation, nil
}

func (f *fakeWorkspaceAllocator) Release(context.Context, remoteworkspace.Allocation) error {
	f.releases++
	return nil
}

func TestProvisionWorktreeUsesAllocatedPathEverywhere(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	repo := model.Repository{ID: "rep_1700000000000_00000000000000000000", Name: "repo", CanonicalPath: "/work/repo", GitRoot: "/work/repo", Enabled: true, CreatedAt: now, LastSeenAt: now}
	worktreePath := "/worktrees/session/repo"
	allocator := &fakeWorkspaceAllocator{allocation: remoteworkspace.Allocation{Path: worktreePath, Mode: model.IsolationWorktree, RepoRoot: repo.GitRoot, Branch: "agctl/remote/session/repo"}}
	store := &fakeStore{repo: repo}
	cockpitClient := &fakeCockpit{}
	bridge := &fakeBridge{repo: worktreePath}
	ids := model.TimeSortableIDGenerator{Now: func() time.Time { return now }, Random: bytes.NewReader(make([]byte, 128))}
	service, err := New(Options{Store: store, Cockpit: cockpitClient, Locator: fakeLocator{bridge: bridge}, WorkspaceAllocator: allocator, IDs: ids, Secrets: &fakeSecrets{}, HostID: "host", ProfileRoot: "/profiles", BridgeRegistry: "/state/bridges", Now: func() time.Time { return now }})
	if err != nil { t.Fatal(err) }
	session, err := service.Provision(context.Background(), Spec{RepositoryID: repo.ID, AccountID: "a1", InstanceStrategy: InstanceDedicated, ConversationStrategy: ConversationNew, IsolationMode: model.IsolationWorktree})
	if err != nil { t.Fatal(err) }
	if session.WorkspacePath != worktreePath { t.Fatalf("session workspace=%q", session.WorkspacePath) }
	if cockpitClient.instance.WorkingDir != worktreePath { t.Fatalf("Cockpit working_dir=%q", cockpitClient.instance.WorkingDir) }
	if allocator.releases != 0 { t.Fatalf("successful allocation released=%d", allocator.releases) }
	if len(allocator.requests) != 1 || allocator.requests[0].Mode != model.IsolationWorktree { t.Fatalf("requests=%+v", allocator.requests) }
}

func TestProvisionFailureReleasesAllocatedWorktree(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	repo := model.Repository{ID: "rep_1700000000000_00000000000000000000", Name: "repo", CanonicalPath: "/work/repo", GitRoot: "/work/repo", Enabled: true, CreatedAt: now, LastSeenAt: now}
	allocator := &fakeWorkspaceAllocator{allocation: remoteworkspace.Allocation{Path: "/worktrees/session/repo", Mode: model.IsolationWorktree}}
	store := &fakeStore{repo: repo}
	cockpitClient := &fakeCockpit{}
	// Bridge exposes the wrong workspace, forcing failure after the IDE has started.
	bridge := &fakeBridge{repo: "/wrong/workspace"}
	ids := model.TimeSortableIDGenerator{Now: func() time.Time { return now }, Random: bytes.NewReader(make([]byte, 128))}
	service, err := New(Options{Store: store, Cockpit: cockpitClient, Locator: fakeLocator{bridge: bridge}, WorkspaceAllocator: allocator, IDs: ids, Secrets: &fakeSecrets{}, HostID: "host", ProfileRoot: "/profiles", BridgeRegistry: "/state/bridges", Now: func() time.Time { return now }})
	if err != nil { t.Fatal(err) }
	_, err = service.Provision(context.Background(), Spec{RepositoryID: repo.ID, AccountID: "a1", InstanceStrategy: InstanceDedicated, ConversationStrategy: ConversationNew, IsolationMode: model.IsolationWorktree})
	if err == nil || !errors.Is(err, ErrWorkspaceMismatch) { t.Fatalf("expected workspace mismatch, got %v", err) }
	if allocator.releases != 1 { t.Fatalf("rollback releases=%d", allocator.releases) }
	if cockpitClient.stopped != 1 { t.Fatalf("started IDE was not rolled back; stops=%d", cockpitClient.stopped) }
}

func TestWorktreeWithoutAllocatorFailsBeforeMutation(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	repo := model.Repository{ID: "rep_1700000000000_00000000000000000000", Name: "repo", CanonicalPath: "/work/repo", GitRoot: "/work/repo", Enabled: true, CreatedAt: now, LastSeenAt: now}
	store := &fakeStore{repo: repo}
	cockpitClient := &fakeCockpit{}
	service, err := New(Options{Store: store, Cockpit: cockpitClient, Locator: fakeLocator{bridge: &fakeBridge{repo: repo.CanonicalPath}}, HostID: "host", ProfileRoot: "/profiles", BridgeRegistry: "/state/bridges"})
	if err != nil { t.Fatal(err) }
	_, err = service.Provision(context.Background(), Spec{RepositoryID: repo.ID, AccountID: "a1", IsolationMode: model.IsolationWorktree})
	if err == nil { t.Fatal("expected missing allocator error") }
	if cockpitClient.created != 0 || cockpitClient.started != 0 { t.Fatalf("mutation occurred: creates=%d starts=%d", cockpitClient.created, cockpitClient.started) }
}
