package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/execx"
)

type Item struct {
	Path     string `json:"path"`
	HEAD     string `json:"head,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Bare     bool   `json:"bare,omitempty"`
	Detached bool   `json:"detached,omitempty"`
}

func List(workspace string) ([]Item, error) {
	out, err := execx.Run(30*time.Second, workspace, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var xs []Item
	var cur *Item
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			xs = append(xs, Item{Path: strings.TrimSpace(strings.TrimPrefix(line, "worktree "))})
			cur = &xs[len(xs)-1]
		case cur != nil && strings.HasPrefix(line, "HEAD "):
			cur.HEAD = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
		case cur != nil && strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(line, "branch ")), "refs/heads/")
		case cur != nil && line == "bare":
			cur.Bare = true
		case cur != nil && line == "detached":
			cur.Detached = true
		}
	}
	return xs, nil
}

func Create(workspace, name, base string) (Item, error) {
	name = safe(name)
	if name == "" {
		return Item{}, fmt.Errorf("valid worktree name required")
	}
	if base == "" {
		base = "HEAD"
	}
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return Item{}, err
	}
	top, err := execx.Run(30*time.Second, abs, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return Item{}, err
	}
	top = strings.TrimSpace(top)
	parent := filepath.Join(filepath.Dir(top), "."+filepath.Base(top)+"-agctl-worktrees")
	if err := os.MkdirAll(parent, 0755); err != nil {
		return Item{}, err
	}
	dst := filepath.Join(parent, name)
	branch := "agctl/" + name
	if _, err := execx.Run(2*time.Minute, top, "git", "worktree", "add", "-b", branch, dst, base); err != nil {
		return Item{}, err
	}
	return Item{Path: dst, Branch: branch}, nil
}
func Remove(workspace, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	_, err := execx.Run(2*time.Minute, workspace, "git", args...)
	return err
}
func Prune(workspace string) error {
	_, err := execx.Run(30*time.Second, workspace, "git", "worktree", "prune")
	return err
}
func safe(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, "..") || strings.ContainsAny(s, `/\\:`) {
		return ""
	}
	return s
}
