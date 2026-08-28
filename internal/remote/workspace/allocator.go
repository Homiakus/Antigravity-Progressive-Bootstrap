package workspace

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessworkspace "github.com/homiakus/agctl/internal/harness/workspace"
	"github.com/homiakus/agctl/internal/remote/model"
)

type Request struct {
	WorkspaceID model.WorkspaceID
	SessionID   model.RemoteSessionID
	Repository  model.Repository
	Mode        model.IsolationMode
	BaseRef     string
	TTL         time.Duration
}

type Allocation struct {
	ID        model.WorkspaceID
	HarnessID harnessmodel.WorkspaceID
	Path      string
	Branch    string
	Mode      model.IsolationMode
	RepoRoot  string
}

type Allocator interface {
	Allocate(context.Context, Request) (Allocation, error)
	Release(context.Context, Allocation) error
}

type HarnessManager interface {
	Allocate(context.Context, harnessworkspace.AllocateParams) (harnessmodel.WorkspaceRecord, error)
	CreateGitWorktree(context.Context, string, string, string, string) error
	RemoveGitWorktree(context.Context, string, string) error
	Release(context.Context, harnessmodel.WorkspaceID) error
}

type HarnessAllocator struct {
	manager      HarnessManager
	worktreeRoot string
}

func NewHarnessAllocator(manager HarnessManager, worktreeRoot string) (*HarnessAllocator, error) {
	if manager == nil {
		return nil, fmt.Errorf("Harness workspace manager is required")
	}
	if strings.TrimSpace(worktreeRoot) == "" {
		return nil, fmt.Errorf("remote worktree root is required")
	}
	return &HarnessAllocator{manager: manager, worktreeRoot: filepath.Clean(worktreeRoot)}, nil
}

func (a *HarnessAllocator) Allocate(ctx context.Context, request Request) (Allocation, error) {
	if strings.TrimSpace(string(request.WorkspaceID)) == "" || strings.TrimSpace(string(request.SessionID)) == "" {
		return Allocation{}, fmt.Errorf("workspace and session ids are required")
	}
	if !request.Mode.Valid() {
		return Allocation{}, fmt.Errorf("invalid isolation mode %q", request.Mode)
	}
	if strings.TrimSpace(request.Repository.CanonicalPath) == "" || strings.TrimSpace(string(request.Repository.ID)) == "" {
		return Allocation{}, fmt.Errorf("registered repository path and id are required")
	}

	harnessID := harnessmodel.WorkspaceID(request.WorkspaceID)
	owner := harnessmodel.WorkflowRunID("remote/" + string(request.SessionID))
	kind := harnessmodel.WorkspaceSharedRead
	workspacePath := request.Repository.CanonicalPath
	branch := ""
	if request.Mode == model.IsolationExclusiveWrite {
		kind = harnessmodel.WorkspaceExclusive
	}
	if request.Mode == model.IsolationWorktree {
		kind = harnessmodel.WorkspaceGitWorktree
		slug := repoSlug(request.Repository.Name)
		branch = "agctl/remote/" + string(request.SessionID) + "/" + slug
		workspacePath = filepath.Join(a.worktreeRoot, string(request.SessionID), slug)
	}

	record, err := a.manager.Allocate(ctx, harnessworkspace.AllocateParams{
		ID:                 harnessID,
		Kind:               kind,
		BasePath:           workspacePath,
		RepositoryID:       string(request.Repository.ID),
		Branch:             branch,
		OwnerWorkflowRunID: owner,
		TTL:                request.TTL,
		Metadata: map[string]string{
			"remote_session_id": string(request.SessionID),
			"isolation_mode":    string(request.Mode),
		},
	})
	if err != nil {
		return Allocation{}, err
	}

	repoRoot := strings.TrimSpace(request.Repository.GitRoot)
	if repoRoot == "" {
		repoRoot = request.Repository.CanonicalPath
	}
	allocation := Allocation{
		ID:        request.WorkspaceID,
		HarnessID: record.ID,
		Path:      workspacePath,
		Branch:    branch,
		Mode:      request.Mode,
		RepoRoot:  repoRoot,
	}
	if request.Mode != model.IsolationWorktree {
		return allocation, nil
	}

	baseRef := strings.TrimSpace(request.BaseRef)
	if baseRef == "" {
		baseRef = strings.TrimSpace(request.Repository.DefaultBranch)
	}
	if baseRef == "" {
		baseRef = "HEAD"
	}
	if err := a.manager.CreateGitWorktree(ctx, repoRoot, branch, workspacePath, baseRef); err != nil {
		_ = a.manager.Release(context.Background(), record.ID)
		return Allocation{}, fmt.Errorf("create remote git worktree: %w", err)
	}
	return allocation, nil
}

func (a *HarnessAllocator) Release(ctx context.Context, allocation Allocation) error {
	var removeErr error
	if allocation.Mode == model.IsolationWorktree {
		removeErr = a.manager.RemoveGitWorktree(ctx, allocation.RepoRoot, allocation.Path)
	}
	releaseErr := a.manager.Release(ctx, allocation.HarnessID)
	if removeErr != nil && releaseErr != nil {
		return fmt.Errorf("remove worktree: %v; release workspace: %w", removeErr, releaseErr)
	}
	if removeErr != nil {
		return removeErr
	}
	return releaseErr
}

var slugInvalid = regexp.MustCompile(`[^a-z0-9]+`)

func repoSlug(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = slugInvalid.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "repo"
	}
	if len(name) > 40 {
		name = strings.Trim(name[:40], "-")
	}
	return name
}
