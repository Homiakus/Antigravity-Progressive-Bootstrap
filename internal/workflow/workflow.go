package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/paths"
)

type Item struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
}

var Embedded = map[string]string{
	"verified-goal": `---
description: Execute the requested task until verified completion with subagent-assisted review.
---

# Verified goal

Use Antigravity native /goal semantics for the user's task: continue without intermediate user input until the requested outcome is materially complete.

Before completion:
1. inspect the repository and infer a Definition of Done;
2. implement the full requested outcome;
3. run relevant tests/lint/build/browser checks;
4. diagnose and fix failures;
5. use an independent test/review subagent for substantial changes;
6. perform final requirement coverage and regression review;
7. satisfy the agctl verified completion gate with actual evidence.
`,
	"deep-review": `---
description: Parallel architecture, security, test and code-review pass using native Antigravity subagents.
---

# Deep review

Delegate independent review lanes when the task is substantial:
- architect: boundaries, contracts, failure modes;
- security-reviewer: trust boundaries, secrets, destructive effects;
- test-engineer: missing tests and failure paths;
- code-reviewer: correctness and regression risk.

Consolidate duplicate findings, prioritize by impact, implement requested fixes when the user's task includes remediation, and verify the result.
`,
	"release-gate": `---
description: Production release gate for verification, security, migration and rollback readiness.
---

# Release gate

Verify the repository is release-ready:
- tests, static analysis and build pass;
- migrations/config changes are safe and documented;
- no known critical security regression;
- user-visible or API changes are covered;
- rollback/recovery is understood;
- final diff contains no unrelated changes.

Do not claim release-ready without evidence.
`,
}

func Root(workspace string) string { return paths.WorkspaceWorkflows(workspace) }

func InstallEmbedded(workspace string, names ...string) error {
	if strings.TrimSpace(workspace) == "" {
		return fmt.Errorf("workspace is required")
	}
	if len(names) == 0 {
		for name := range Embedded {
			names = append(names, name)
		}
	}
	root := Root(workspace)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	for _, name := range names {
		body, ok := Embedded[name]
		if !ok {
			return fmt.Errorf("unknown embedded workflow %q", name)
		}
		if err := jsonx.WriteTextAtomic(filepath.Join(root, name+".md"), body, ""); err != nil {
			return err
		}
	}
	return nil
}

func List(workspace string) ([]Item, error) {
	if strings.TrimSpace(workspace) == "" {
		return nil, fmt.Errorf("workspace is required")
	}
	root := Root(workspace)
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Item
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		path := filepath.Join(root, e.Name())
		out = append(out, Item{Name: strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())), Path: path, Description: description(path)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func Remove(workspace, name string) error {
	if strings.ContainsAny(name, `/\\`) || strings.Contains(name, "..") || strings.TrimSpace(name) == "" {
		return fmt.Errorf("invalid workflow name")
	}
	return os.Remove(filepath.Join(Root(workspace), name+".md"))
}

func description(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	in := false
	for _, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(raw)
		if line == "---" {
			if !in {
				in = true
				continue
			}
			break
		}
		if in && strings.HasPrefix(line, "description:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "description:")), "\"'")
		}
	}
	return ""
}
