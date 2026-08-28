package workspace

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessworkspace "github.com/homiakus/agctl/internal/harness/workspace"
	"github.com/homiakus/agctl/internal/remote/model"
)

var ErrDirtyRepository = errors.New("remote workspace: repository has uncommitted changes")

type Request struct {
	WorkspaceID    model.WorkspaceID
	SessionID      model.RemoteSessionID
	Repository     model.Repository
	Mode           model.IsolationMode
	BaseRef        string
	TTL            time.Duration
	AllowDirtyBase bool
}

type Allocation struct {
	ID        model.WorkspaceID
	HarnessID harnessmodel.WorkspaceID
	Path      string
	Branch    string
	BaseRef   string
	Mode      model.IsolationMode
	RepoRoot  string
}

type Handoff struct {
	WorkspaceID  model.WorkspaceID `json:"workspaceId"`
	RepoRoot     string            `json:"repoRoot"`
	WorktreePath string            `json:"worktreePath"`
	SourceBranch string            `json:"sourceBranch"`
	BaseRef      string            `json:"baseRef"`
	TargetBranch string            `json:"targetBranch"`
	Strategy     string            `json:"strategy"`
}

type Allocator interface {
	Allocate(context.Context, Request) (Allocation, error)
	Release(context.Context, Allocation) error
}

type GitStatus struct {
	Dirty bool
	Head  string
}

type GitInspector interface {
	Status(context.Context, string) (GitStatus, error)
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
	git          GitInspector
}

func NewHarnessAllocator(manager HarnessManager, worktreeRoot string) (*HarnessAllocator, error) {
	if manager == nil {
		return nil, fmt.Errorf("Harness workspace manager is required")
	}
	if strings.TrimSpace(worktreeRoot) == "" {
		return nil, fmt.Errorf("remote worktree root is required")
	}
	return &HarnessAllocator{manager: manager, worktreeRoot: filepath.Clean(worktreeRoot), git: ExecGitInspector{}}, nil
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

	repoRoot := strings.TrimSpace(request.Repository.GitRoot)
	if repoRoot == "" {
		repoRoot = request.Repository.CanonicalPath
	}
	baseRef := strings.TrimSpace(request.BaseRef)
	if baseRef == "" {
		baseRef = strings.TrimSpace(request.Repository.DefaultBranch)
	}
	if baseRef == "" {
		baseRef = "HEAD"
	}
	if request.Mode == model.IsolationWorktree && !request.AllowDirtyBase {
		status, err := a.git.Status(ctx, repoRoot)
		if err != nil {
			return Allocation{}, fmt.Errorf("inspect repository before worktree allocation: %w", err)
		}
		if status.Dirty {
			return Allocation{}, fmt.Errorf("%s: %w", repoRoot, ErrDirtyRepository)
		}
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
			"base_ref":          baseRef,
		},
	})
	if err != nil {
		return Allocation{}, err
	}

	allocation := Allocation{
		ID:        request.WorkspaceID,
		HarnessID: record.ID,
		Path:      workspacePath,
		Branch:    branch,
		BaseRef:   baseRef,
		Mode:      request.Mode,
		RepoRoot:  repoRoot,
	}
	if request.Mode != model.IsolationWorktree {
		return allocation, nil
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

// BuildHandoff never merges automatically. It creates an explicit integration
// descriptor that can be passed to a controlled Harness verification/merge
// workflow after the remote write session has finished.
func BuildHandoff(allocation Allocation, targetBranch string) (Handoff, error) {
	if allocation.Mode != model.IsolationWorktree || strings.TrimSpace(allocation.Branch) == "" || strings.TrimSpace(allocation.Path) == "" {
		return Handoff{}, fmt.Errorf("merge handoff requires a git worktree allocation")
	}
	targetBranch = strings.TrimSpace(targetBranch)
	if targetBranch == "" {
		targetBranch = allocation.BaseRef
	}
	if targetBranch == "" {
		targetBranch = "main"
	}
	return Handoff{
		WorkspaceID:  allocation.ID,
		RepoRoot:     allocation.RepoRoot,
		WorktreePath: allocation.Path,
		SourceBranch: allocation.Branch,
		BaseRef:      allocation.BaseRef,
		TargetBranch: targetBranch,
		Strategy:     "VERIFY_THEN_MERGE_VIA_HARNESS",
	}, nil
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
