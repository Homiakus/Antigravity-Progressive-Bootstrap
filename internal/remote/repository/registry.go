package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

type GitInspector interface {
	Root(context.Context, string) (string, error)
	Remote(context.Context, string) (string, error)
	Branch(context.Context, string) (string, error)
}

type ExecGitInspector struct{}

func (ExecGitInspector) Root(ctx context.Context, path string) (string, error) {
	return runGit(ctx, path, "rev-parse", "--show-toplevel")
}

func (ExecGitInspector) Remote(ctx context.Context, path string) (string, error) {
	out, err := runGit(ctx, path, "config", "--get", "remote.origin.url")
	if err != nil {
		return "", nil
	}
	return out, nil
}

func (ExecGitInspector) Branch(ctx context.Context, path string) (string, error) {
	out, err := runGit(ctx, path, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", nil
	}
	return out, nil
}

func runGit(ctx context.Context, path string, args ...string) (string, error) {
	argv := append([]string{"-C", path}, args...)
	out, err := exec.CommandContext(ctx, "git", argv...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

type Options struct {
	AllowedRoots []string
	Git          GitInspector
	IDs          model.IDGenerator
	Now          func() time.Time
}

type Registry struct {
	store        remotestore.RepositoryStore
	allowedRoots []string
	git          GitInspector
	ids          model.IDGenerator
	now          func() time.Time
}

func New(store remotestore.RepositoryStore, opts Options) (*Registry, error) {
	if store == nil {
		return nil, fmt.Errorf("repository store is required")
	}
	git := opts.Git
	if git == nil {
		git = ExecGitInspector{}
	}
	ids := opts.IDs
	if ids == nil {
		generator := model.NewIDGenerator()
		ids = generator
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	roots := make([]string, 0, len(opts.AllowedRoots))
	for _, root := range opts.AllowedRoots {
		canonical, err := canonicalPath(root)
		if err != nil {
			return nil, fmt.Errorf("allowed root %q: %w", root, err)
		}
		roots = append(roots, canonical)
	}
	return &Registry{store: store, allowedRoots: roots, git: git, ids: ids, now: now}, nil
}

func (r *Registry) Add(ctx context.Context, path, name string) (model.Repository, error) {
	canonical, err := canonicalPath(path)
	if err != nil {
		return model.Repository{}, err
	}
	if err := r.ensureAllowed(canonical); err != nil {
		return model.Repository{}, err
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return model.Repository{}, fmt.Errorf("stat repository path: %w", err)
	}
	if !info.IsDir() {
		return model.Repository{}, fmt.Errorf("repository path is not a directory: %s", canonical)
	}
	gitRoot, err := r.git.Root(ctx, canonical)
	if err != nil {
		return model.Repository{}, fmt.Errorf("resolve git root: %w", err)
	}
	gitRoot, err = canonicalPath(gitRoot)
	if err != nil {
		return model.Repository{}, fmt.Errorf("canonicalize git root: %w", err)
	}
	if err := r.ensureAllowed(gitRoot); err != nil {
		return model.Repository{}, err
	}

	now := r.now().UTC()
	existing, getErr := r.store.GetRepositoryByPath(ctx, gitRoot)
	if getErr == nil {
		if strings.TrimSpace(name) != "" {
			existing.Name = strings.TrimSpace(name)
		}
		existing.LastSeenAt = now
		existing.Enabled = true
		existing.GitRemote, _ = r.git.Remote(ctx, gitRoot)
		existing.DefaultBranch, _ = r.git.Branch(ctx, gitRoot)
		if err := r.store.UpsertRepository(ctx, existing); err != nil {
			return model.Repository{}, err
		}
		return existing, nil
	}
	if !errors.Is(getErr, remotestore.ErrNotFound) {
		return model.Repository{}, getErr
	}

	rawID, err := r.ids.New(model.IDRepository)
	if err != nil {
		return model.Repository{}, err
	}
	if strings.TrimSpace(name) == "" {
		name = filepath.Base(gitRoot)
	}
	remote, _ := r.git.Remote(ctx, gitRoot)
	branch, _ := r.git.Branch(ctx, gitRoot)
	record := model.Repository{
		ID:            model.RepositoryID(rawID),
		Name:          strings.TrimSpace(name),
		CanonicalPath: gitRoot,
		GitRoot:       gitRoot,
		GitRemote:     remote,
		DefaultBranch: branch,
		Enabled:       true,
		CreatedAt:     now,
		LastSeenAt:    now,
	}
	if err := record.Validate(); err != nil {
		return model.Repository{}, err
	}
	if err := r.store.UpsertRepository(ctx, record); err != nil {
		return model.Repository{}, err
	}
	return record, nil
}

func (r *Registry) ensureAllowed(path string) error {
	if len(r.allowedRoots) == 0 {
		return nil
	}
	for _, root := range r.allowedRoots {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			continue
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return nil
		}
	}
	return fmt.Errorf("repository path %q is outside configured allowed roots", path)
}

func canonicalPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("absolute path: %w", err)
	}
	cleaned := filepath.Clean(absolute)
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		cleaned = resolved
	}
	if runtime.GOOS == "windows" {
		cleaned = strings.ToLower(cleaned)
	}
	return cleaned, nil
}
