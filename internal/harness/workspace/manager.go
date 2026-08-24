package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

var (
	ErrWorkspaceConflict = errors.New("harness workspace: exclusive workspace conflict")
	ErrWorkspaceNotFound = errors.New("harness workspace: workspace not found")
)

type AllocateParams struct {
	ID                 harnessmodel.WorkspaceID
	Kind               harnessmodel.WorkspaceKind
	BasePath           string
	RepositoryID       string
	Branch             string
	OwnerWorkflowRunID harnessmodel.WorkflowRunID
	OwnerNodeRunID     harnessmodel.NodeRunID
	OwnerAttemptID     harnessmodel.AttemptID
	TTL                time.Duration
	Metadata           map[string]string
}

type Options struct {
	Now func() time.Time
}

type Manager struct {
	store harnessstore.Store
	now   func() time.Time
}

func NewManager(store harnessstore.Store, opts Options) (*Manager, error) {
	if store == nil {
		return nil, fmt.Errorf("harness store is required")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Manager{store: store, now: now}, nil
}

func (m *Manager) Allocate(ctx context.Context, params AllocateParams) (harnessmodel.WorkspaceRecord, error) {
	if params.ID == "" {
		return harnessmodel.WorkspaceRecord{}, fmt.Errorf("workspace id is required")
	}
	if !params.Kind.Valid() {
		return harnessmodel.WorkspaceRecord{}, fmt.Errorf("invalid workspace kind %q", params.Kind)
	}
	if strings.TrimSpace(params.BasePath) == "" {
		return harnessmodel.WorkspaceRecord{}, fmt.Errorf("workspace base_path is required")
	}

	now := m.now().UTC()
	ttl := params.TTL
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	expiresAt := now.Add(ttl)

	record := harnessmodel.WorkspaceRecord{
		ID:                 params.ID,
		Kind:               params.Kind,
		State:              harnessmodel.WorkspaceActive,
		BasePath:           params.BasePath,
		RepositoryID:       params.RepositoryID,
		Branch:             params.Branch,
		OwnerWorkflowRunID: params.OwnerWorkflowRunID,
		OwnerNodeRunID:     params.OwnerNodeRunID,
		OwnerAttemptID:     params.OwnerAttemptID,
		CreatedAt:          now,
		ExpiresAt:          expiresAt,
		Metadata:           params.Metadata,
	}

	err := m.store.Update(ctx, func(tx harnessstore.Tx) error {
		// Conflict check for exclusive / persistent writes
		if params.Kind == harnessmodel.WorkspaceExclusive || params.Kind == harnessmodel.WorkspacePersistent {
			if params.RepositoryID != "" {
				existing, err := tx.ListWorkspacesByRepo(ctx, params.RepositoryID)
				if err != nil {
					return err
				}
				for _, ws := range existing {
					if ws.State == harnessmodel.WorkspaceActive && ws.ID != params.ID && ws.OwnerWorkflowRunID != params.OwnerWorkflowRunID {
						if ws.Kind == harnessmodel.WorkspaceExclusive || ws.Kind == harnessmodel.WorkspacePersistent {
							return fmt.Errorf("repository %s is already locked by workspace %s (run %s): %w",
								params.RepositoryID, ws.ID, ws.OwnerWorkflowRunID, ErrWorkspaceConflict)
						}
					}
				}
			}
		}

		return tx.CreateWorkspace(ctx, record)
	})
	if err != nil {
		return harnessmodel.WorkspaceRecord{}, err
	}
	return record, nil
}

func (m *Manager) Release(ctx context.Context, id harnessmodel.WorkspaceID) error {
	now := m.now().UTC()
	return m.store.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.UpdateWorkspaceState(ctx, id, harnessmodel.WorkspaceReleased, now)
	})
}

func (m *Manager) Get(ctx context.Context, id harnessmodel.WorkspaceID) (harnessmodel.WorkspaceRecord, error) {
	var ws harnessmodel.WorkspaceRecord
	err := m.store.View(ctx, func(r harnessstore.Reader) error {
		var readErr error
		ws, readErr = r.GetWorkspace(ctx, id)
		return readErr
	})
	if err != nil {
		if errors.Is(err, harnessstore.ErrNotFound) {
			return harnessmodel.WorkspaceRecord{}, ErrWorkspaceNotFound
		}
		return harnessmodel.WorkspaceRecord{}, err
	}
	return ws, nil
}

// CreateGitWorktree creates an isolated git worktree for concurrent agent isolation.
func (m *Manager) CreateGitWorktree(ctx context.Context, repoDir, branch, targetDir, baseRef string) error {
	if repoDir == "" || branch == "" || targetDir == "" {
		return fmt.Errorf("repoDir, branch and targetDir are required")
	}
	if baseRef == "" {
		baseRef = "HEAD"
	}
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return fmt.Errorf("create parent dir for worktree: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "worktree", "add", "-b", branch, targetDir, baseRef)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// RemoveGitWorktree safely tears down a git worktree.
func (m *Manager) RemoveGitWorktree(ctx context.Context, repoDir, targetDir string) error {
	if repoDir == "" || targetDir == "" {
		return fmt.Errorf("repoDir and targetDir are required")
	}
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "worktree", "remove", "--force", targetDir)
	_ = cmd.Run()
	_ = os.RemoveAll(targetDir)
	return nil
}

// Reconcile scans active workspaces on disk and marks missing/damaged ones as CORRUPTED.
func (m *Manager) Reconcile(ctx context.Context) (reconciledCount int, corruptedCount int, err error) {
	var active []harnessmodel.WorkspaceRecord
	err = m.store.View(ctx, func(r harnessstore.Reader) error {
		var listErr error
		active, listErr = r.ListActiveWorkspaces(ctx)
		return listErr
	})
	if err != nil {
		return 0, 0, fmt.Errorf("list active workspaces for reconciliation: %w", err)
	}

	now := m.now().UTC()
	for _, ws := range active {
		reconciledCount++
		// Check if directory exists on disk
		if _, statErr := os.Stat(ws.BasePath); statErr != nil {
			// Mark as corrupted
			_ = m.store.Update(ctx, func(tx harnessstore.Tx) error {
				return tx.UpdateWorkspaceState(ctx, ws.ID, harnessmodel.WorkspaceCorrupted, now)
			})
			corruptedCount++
		}
	}
	return reconciledCount, corruptedCount, nil
}

// GC cleans up released workspaces older than gracePeriod.
func (m *Manager) GC(ctx context.Context, gracePeriod time.Duration) (deletedCount int, err error) {
	now := m.now().UTC()
	threshold := now.Add(-gracePeriod)

	var active []harnessmodel.WorkspaceRecord
	err = m.store.View(ctx, func(r harnessstore.Reader) error {
		var listErr error
		active, listErr = r.ListActiveWorkspaces(ctx)
		return listErr
	})
	if err != nil {
		return 0, err
	}

	for _, ws := range active {
		if ws.ExpiresAt.Before(threshold) {
			_ = m.store.Update(ctx, func(tx harnessstore.Tx) error {
				return tx.DeleteWorkspace(ctx, ws.ID)
			})
			deletedCount++
		}
	}
	return deletedCount, nil
}
