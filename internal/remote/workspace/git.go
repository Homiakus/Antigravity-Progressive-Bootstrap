package workspace

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type ExecGitInspector struct{}

func (ExecGitInspector) Status(ctx context.Context, repoRoot string) (GitStatus, error) {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return GitStatus{}, fmt.Errorf("repository root is required")
	}
	statusOut, err := exec.CommandContext(ctx, "git", "-C", repoRoot, "status", "--porcelain", "--untracked-files=normal").CombinedOutput()
	if err != nil {
		return GitStatus{}, fmt.Errorf("git status: %s: %w", strings.TrimSpace(string(statusOut)), err)
	}
	headOut, err := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		return GitStatus{}, fmt.Errorf("git rev-parse HEAD: %s: %w", strings.TrimSpace(string(headOut)), err)
	}
	return GitStatus{Dirty: strings.TrimSpace(string(statusOut)) != "", Head: strings.TrimSpace(string(headOut))}, nil
}
