package project

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/agents"
	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/mcp"
	"github.com/homiakus/agctl/internal/paths"
)

type Profile struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Signals     []string `json:"signals"`
	MCP         []string `json:"mcp"`
	Agents      []string `json:"agents"`
	Checks      []string `json:"checks"`
}

type Detection struct {
	Workspace string   `json:"workspace"`
	Profiles  []string `json:"profiles"`
	Signals   []string `json:"signals"`
}

var profiles = []Profile{
	{Name: "go", Description: "Go services/libraries", Signals: []string{"go.mod"}, MCP: []string{"context7", "github"}, Agents: []string{"architect", "implementer", "test-engineer", "code-reviewer"}, Checks: []string{"gofmt", "go test ./...", "go vet ./...", "staticcheck ./..."}},
	{Name: "web", Description: "Web/frontend/full-stack", Signals: []string{"package.json", "vite.config", "next.config", "src"}, MCP: []string{"context7", "playwright", "chrome-devtools", "github"}, Agents: []string{"architect", "implementer", "test-engineer", "code-reviewer"}, Checks: []string{"typecheck", "lint", "unit tests", "build", "Playwright/E2E"}},
	{Name: "infra", Description: "Infrastructure as code", Signals: []string{"terraform", ".tf", "Dockerfile", "k8s", "helm"}, MCP: []string{"context7", "github"}, Agents: []string{"architect", "security-reviewer", "test-engineer"}, Checks: []string{"format", "validate", "plan", "security review"}},
	{Name: "observability", Description: "Monitoring/telemetry", Signals: []string{"grafana", "sentry", "otel", "opentelemetry", "prometheus"}, MCP: []string{"github"}, Agents: []string{"architect", "researcher", "test-engineer"}, Checks: []string{"config validation", "integration checks"}},
	{Name: "ai", Description: "AI/agent/MCP project", Signals: []string{"mcp_config.json", "modelcontextprotocol", "gemini", "openai", "anthropic", "llm", "agents-sdk"}, MCP: []string{"context7", "github"}, Agents: []string{"architect", "researcher", "security-reviewer", "test-engineer"}, Checks: []string{"protocol tests", "evals", "security review"}},
}

func Profiles() []Profile { return append([]Profile(nil), profiles...) }
func FindProfile(name string) (Profile, bool) {
	for _, p := range profiles {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

func Detect(workspace string) (Detection, error) {
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return Detection{}, err
	}
	d := Detection{Workspace: abs}
	scores := map[string]int{}
	signals := map[string]bool{}
	_ = filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		rel, _ := filepath.Rel(abs, path)
		rel = filepath.ToSlash(rel)
		if rel != "." {
			first := strings.Split(rel, "/")[0]
			switch strings.ToLower(first) {
			case ".git", ".agents", ".agent", "node_modules", "vendor", "dist", "build", ".next", ".cache":
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if strings.Count(rel, "/") > 3 {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		base := strings.ToLower(info.Name())
		for _, p := range profiles {
			for _, sig := range p.Signals {
				s := strings.ToLower(sig)
				if base == s || strings.Contains(base, s) || strings.Contains(strings.ToLower(rel), s) {
					scores[p.Name]++
					signals[rel] = true
				}
			}
		}
		return nil
	})
	type pair struct {
		name  string
		score int
	}
	var ps []pair
	for n, s := range scores {
		if s > 0 {
			ps = append(ps, pair{n, s})
		}
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].score > ps[j].score })
	for _, x := range ps {
		d.Profiles = append(d.Profiles, x.name)
	}
	for s := range signals {
		d.Signals = append(d.Signals, s)
	}
	sort.Strings(d.Signals)
	if len(d.Profiles) == 0 {
		d.Profiles = []string{"general"}
	}
	return d, nil
}

func Init(p paths.Paths, workspace string, selected []string) (Detection, error) {
	d, err := Detect(workspace)
	if err != nil {
		return d, err
	}
	if len(selected) == 0 {
		selected = d.Profiles
	}
	abs := d.Workspace
	root := paths.WorkspaceRoot(abs)
	for _, dir := range []string{root, paths.WorkspaceSkills(abs), paths.WorkspaceRules(abs), paths.WorkspaceWorkflows(abs), paths.WorkspaceAgents(abs), paths.WorkspacePlugins(abs)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return d, err
		}
	}
	_ = agents.InstallEmbedded(p, abs)
	merged := mergeProfiles(selected)
	if err := writeAgentsMD(abs, merged); err != nil {
		return d, err
	}
	if err := writeRule(abs, merged); err != nil {
		return d, err
	}
	if err := writeWorkflows(abs, merged); err != nil {
		return d, err
	}
	for _, id := range merged.MCP {
		if id == "github" {
			continue
		}
		_ = mcp.Install(p, abs, id)
	}
	lock := map[string]any{"version": "3.2.1", "initializedAt": time.Now().Format(time.RFC3339Nano), "profiles": selected, "detected": d, "checks": merged.Checks}
	_ = jsonx.WriteAtomic(filepath.Join(root, "agctl-project.json"), lock, p.BackupsRoot)
	return d, nil
}

func mergeProfiles(names []string) Profile {
	out := Profile{Name: "composite"}
	seenM, seenA, seenC := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, n := range names {
		p, ok := FindProfile(n)
		if !ok {
			continue
		}
		out.Description += p.Description + "; "
		for _, x := range p.MCP {
			if !seenM[x] {
				seenM[x] = true
				out.MCP = append(out.MCP, x)
			}
		}
		for _, x := range p.Agents {
			if !seenA[x] {
				seenA[x] = true
				out.Agents = append(out.Agents, x)
			}
		}
		for _, x := range p.Checks {
			if !seenC[x] {
				seenC[x] = true
				out.Checks = append(out.Checks, x)
			}
		}
	}
	sort.Strings(out.MCP)
	sort.Strings(out.Agents)
	return out
}

func writeAgentsMD(root string, p Profile) error {
	path := filepath.Join(root, "AGENTS.md")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	var b strings.Builder
	b.WriteString("# AGENTS.md\n\n")
	b.WriteString("This repository is prepared for autonomous Antigravity development by agctl 3.2.1.\n\n")
	b.WriteString("## Operating contract\n\n")
	b.WriteString("- Inspect existing architecture and conventions before editing.\n")
	b.WriteString("- Keep changes scoped to the delegated outcome.\n")
	b.WriteString("- Prefer reversible, testable implementation choices when requirements leave room for engineering judgment.\n")
	b.WriteString("- Do not stop at a plan or partial patch when implementation was requested.\n")
	b.WriteString("- Use native Antigravity subagents for separable research, testing, review, or security work when the task is substantial.\n")
	b.WriteString("- Use `/goal` semantics for delegated until-done implementation work.\n")
	b.WriteString("- Preserve secrets and never commit credentials.\n\n")
	b.WriteString("## Verification\n\nUse the strongest relevant local checks. Suggested checks for the detected project profile:\n")
	for _, x := range p.Checks {
		b.WriteString("\n- `" + x + "`")
	}
	b.WriteString("\n\nOnly report completion after applicable verification has actually run.\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writeRule(root string, p Profile) error {
	path := filepath.Join(paths.WorkspaceRules(root), "agctl-project.md")
	body := fmt.Sprintf(`# Project control-plane policy

Use specialized custom agents and subagents only when they reduce context bloat or allow independent parallel verification. Avoid multiple agents writing the same files concurrently.

When implementation is delegated, keep working until the requested result is implemented and verified. Use native /goal behavior where available. Treat test/build/lint failures as work to diagnose, not as a reason to stop.

Recommended specialist roles: %s.
`, strings.Join(p.Agents, ", "))
	return os.WriteFile(path, []byte(body), 0o644)
}

func writeWorkflows(root string, _ Profile) error {
	wf := paths.WorkspaceWorkflows(root)
	flows := map[string]string{
		"verified-goal.md": `---
title: Verified Goal
description: Run a delegated engineering task until it is implemented and independently verified.
---

1. Treat the user's remaining instruction as a /goal-style until-complete task.
2. Inspect the codebase and infer a concrete Definition of Done.
3. Delegate independent research/test/review work to subagents when useful.
4. Implement the task; do not stop after planning.
5. Run targeted verification and fix failures.
6. Run requirement coverage and regression review.
7. Finish only when the requested outcome is materially complete and verified.
`,
		"deep-review.md": `---
title: Deep Review
description: Parallel architecture, security, test and code review of the current change.
---

1. Inspect the current diff and affected architecture.
2. Invoke independent code-reviewer, test-engineer and security-reviewer subagents when appropriate.
3. Consolidate findings by severity and evidence.
4. Fix actionable high-confidence defects if the user delegated implementation.
5. Re-run verification and report residual risks only.
`,
	}
	for name, content := range flows {
		if err := os.WriteFile(filepath.Join(wf, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func Fingerprint(workspace string) (string, error) {
	h := sha256.New()
	_ = filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(workspace, path)
		if strings.HasPrefix(filepath.ToSlash(rel), ".git/") {
			return nil
		}
		h.Write([]byte(filepath.ToSlash(rel)))
		h.Write([]byte{0})
		return nil
	})
	return hex.EncodeToString(h.Sum(nil)), nil
}

func Summary(d Detection) string {
	return fmt.Sprintf("workspace=%s profiles=%s signals=%d", d.Workspace, strings.Join(d.Profiles, ","), len(d.Signals))
}
