package installer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/agents"
	"github.com/homiakus/agctl/internal/capability"
	"github.com/homiakus/agctl/internal/execx"
	"github.com/homiakus/agctl/internal/hooks"
	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/loop"
	"github.com/homiakus/agctl/internal/mcp"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/permissions"
	"github.com/homiakus/agctl/internal/replan"
	"github.com/homiakus/agctl/internal/risk"
	"github.com/homiakus/agctl/internal/router"
	"github.com/homiakus/agctl/internal/skills"
	"github.com/homiakus/agctl/internal/tasks"
)

const ruleBegin = "<!-- AGCTL:ADAPTIVE-CONTROL-PLANE:BEGIN -->"
const ruleEnd = "<!-- AGCTL:ADAPTIVE-CONTROL-PLANE:END -->"

func InstalledBinaryPath(p paths.Paths) string {
	name := "agctl"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(p.BinRoot, name)
}

func InstallSelf(p paths.Paths) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, _ = filepath.Abs(exe)
	dst := InstalledBinaryPath(p)
	if sameFilePath(exe, dst) {
		return dst, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	src, err := os.Open(exe)
	if err != nil {
		return "", err
	}
	defer src.Close()
	tmp := dst + ".new"
	out, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return "", err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(tmp, 0o755)
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(dst)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(dst, 0o755)
	}
	return dst, nil
}

func sameFilePath(a, b string) bool {
	aa, _ := filepath.Abs(a)
	bb, _ := filepath.Abs(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(aa, bb)
	}
	return aa == bb
}

func GlobalRule(mode string) string {
	visibility := "For non-trivial tasks, show one concise Route line when capability selection matters."
	switch mode {
	case router.ModeSilent:
		visibility = "Keep capability routing internal unless explicitly requested."
	case router.ModeTransparent:
		visibility = "For every task, the first visible assistant text must be one concise `Route: skills=[...]; MCP=[...]; verification=[...]` line."
	case router.ModeMaximum:
		visibility = "For every non-trivial task, show a Route line and proactively compose complementary capabilities when useful."
	}
	return fmt.Sprintf(`%s
# agctl global control-plane rule

Before non-trivial work, use the adaptive capability route: choose the smallest sufficient set of relevant skills, MCP/native tools, subagents and verification methods. Do not use tools merely because they are installed.

For delegated implementation/debug/refactor tasks, follow the autonomous completion contract when enabled: continue through implement -> verify -> diagnose -> fix -> re-verify -> requirement coverage -> final regression until verified Definition of Done. Do not stop at a plan or progress report.

For editing/rewrite/proofreading/publication, use editorial-quality-director and treat no-ai-slop as a subordinate anti-slop pass rather than the whole editing process.

%s

Capability routing and autonomous execution do not override explicit user constraints, unavailable credentials, organization policy, or hard platform guardrails.
%s
`, ruleBegin, visibility, ruleEnd)
}

func InstallGlobalRule(p paths.Paths, mode string) error {
	block := GlobalRule(mode)
	targets := []string{p.GlobalRule}

	// Also sync to other platform global rule files if they exist
	for _, extra := range []string{
		filepath.Join(p.Home, "AGENTS.md"),
		filepath.Join(p.Home, "CLAUDE.md"),
		filepath.Join(p.Home, ".cursorrules"),
		filepath.Join(p.GeminiRoot, "GEMINI.md"),
	} {
		if extra != p.GlobalRule {
			if _, err := os.Stat(extra); err == nil {
				targets = append(targets, extra)
			}
		}
	}

	for _, target := range targets {
		old := ""
		if b, err := os.ReadFile(target); err == nil {
			old = string(b)
		}
		start := strings.Index(old, ruleBegin)
		end := strings.Index(old, ruleEnd)
		var next string
		if start >= 0 && end >= start {
			end += len(ruleEnd)
			next = old[:start] + block + old[end:]
		} else if strings.TrimSpace(old) == "" {
			next = block
		} else {
			next = strings.TrimRight(old, "\r\n") + "\n\n" + block
		}
		if err := jsonx.WriteTextAtomic(target, next, p.BackupsRoot); err != nil {
			return err
		}
	}
	return nil
}


type Prerequisite struct {
	ID      string
	Command string
	Label   string
}

var Prerequisites = []Prerequisite{
	{ID: "agy", Command: "agy", Label: "Antigravity CLI"},
	{ID: "git", Command: "git", Label: "Git"},
	{ID: "node", Command: "node", Label: "Node.js"},
	{ID: "npx", Command: "npx", Label: "npx"},
	{ID: "uv", Command: "uv", Label: "uv"},
	{ID: "uvx", Command: "uvx", Label: "uvx"},
	{ID: "gh", Command: "gh", Label: "GitHub CLI"},
	{ID: "docker", Command: "docker", Label: "Docker"},
	{ID: "go", Command: "go", Label: "Go"},
	{ID: "gopls", Command: "gopls", Label: "gopls"},
}

func MissingPrerequisites() []Prerequisite {
	var out []Prerequisite
	for _, p := range Prerequisites {
		if !execx.Exists(p.Command) {
			out = append(out, p)
		}
	}
	return out
}

func RefreshProcessPath() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	out, err := execx.Run(15*time.Second, "", "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", `[Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User')`)
	if err != nil {
		return err
	}
	pathValue := strings.TrimSpace(out)
	if pathValue == "" {
		return fmt.Errorf("refreshed PATH is empty")
	}
	return os.Setenv("PATH", pathValue)
}

func InstallPrerequisite(id string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("automatic prerequisite installation is currently implemented for Windows only")
	}
	switch id {
	case "agy":
		if err := execx.RunInteractive("", "powershell.exe", "-NoLogo", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "irm https://antigravity.google/cli/install.ps1 | iex"); err != nil {
			return err
		}
		return RefreshProcessPath()
	case "git":
		return winget("Git.Git")
	case "node", "npx":
		return winget("OpenJS.NodeJS.LTS")
	case "gh":
		return winget("GitHub.cli")
	case "docker":
		return winget("Docker.DockerDesktop")
	case "uv", "uvx":
		return winget("astral-sh.uv")
	case "go":
		return winget("GoLang.Go")
	case "gopls":
		if !execx.Exists("go") {
			return fmt.Errorf("install Go first")
		}
		return execx.RunInteractive("", "go", "install", "golang.org/x/tools/gopls@latest")
	default:
		return fmt.Errorf("unknown prerequisite %q", id)
	}
}

func winget(id string) error {
	if !execx.Exists("winget") {
		return fmt.Errorf("winget is not available")
	}
	if err := execx.RunInteractive("", "winget", "install", "--id", id, "-e", "--accept-package-agreements", "--accept-source-agreements"); err != nil {
		return err
	}
	return RefreshProcessPath()
}

func MigrateLegacy(p paths.Paths) ([]string, error) {
	var notes []string
	// Remove only hook keys that were owned by the PowerShell bootstrap.
	hooksRaw, err := jsonx.ReadMap(p.GlobalHooks)
	if err != nil {
		return notes, err
	}
	changedHooks := false
	for _, name := range []string{
		"adaptive-tool-router-pre-invocation",
		"autonomous-completion-loop",
		"autonomous-completion-no-prompt",
	} {
		if _, ok := hooksRaw[name]; ok {
			delete(hooksRaw, name)
			changedHooks = true
			notes = append(notes, "removed legacy PowerShell hook: "+name)
		}
	}
	if changedHooks {
		if err := jsonx.WriteAtomic(p.GlobalHooks, hooksRaw, p.BackupsRoot); err != nil {
			return notes, err
		}
	}

	// Remove the old bootstrap-managed router block while preserving all other user rules.
	if b, err := os.ReadFile(p.GlobalRule); err == nil {
		old := string(b)
		re := regexp.MustCompile(`(?s)<!-- ANTIGRAVITY-BOOTSTRAP:ADAPTIVE-TOOL-ROUTER:BEGIN -->.*?<!-- ANTIGRAVITY-BOOTSTRAP:ADAPTIVE-TOOL-ROUTER:END -->\s*`)
		next := re.ReplaceAllString(old, "")
		if next != old {
			if err := jsonx.WriteTextAtomic(p.GlobalRule, strings.TrimSpace(next)+"\n", p.BackupsRoot); err != nil {
				return notes, err
			}
			notes = append(notes, "removed legacy PowerShell managed GEMINI.md router block")
		}
	}
	return notes, nil
}

type Report struct {
	InstalledBinary string
	SkillPackCounts map[string]int
	MCPWarnings     []string
	Notes           []string
}

func Recommended(p paths.Paths, installPrereqs bool) (Report, error) {
	if err := p.Ensure(); err != nil {
		return Report{}, err
	}
	if err := risk.EnsurePolicy(p); err != nil {
		return Report{}, err
	}
	legacyNotes, err := MigrateLegacy(p)
	if err != nil {
		return Report{}, err
	}
	if installPrereqs {
		for _, pr := range MissingPrerequisites() {
			if pr.ID == "docker" || pr.ID == "go" || pr.ID == "gopls" || pr.ID == "uv" || pr.ID == "uvx" || pr.ID == "gh" {
				continue
			}
			_ = InstallPrerequisite(pr.ID)
		}
	}
	bin, err := InstallSelf(p)
	if err != nil {
		return Report{}, err
	}
	if err := skills.InstallEmbedded(p); err != nil {
		return Report{}, err
	}
	if err := agents.InstallEmbedded(p, ""); err != nil {
		return Report{}, err
	}
	if err := agents.Enable(p, agents.ModeBalanced); err != nil {
		return Report{}, err
	}
	counts := map[string]int{"embedded": 4}
	if execx.Exists("git") {
		for _, pack := range skills.Packs {
			if !pack.Recommended {
				continue
			}
			items, e := skills.SyncPack(p, pack.ID)
			if e != nil {
				counts[pack.ID] = -1
			} else {
				counts[pack.ID] = len(items)
			}
		}
	}
	mcpWarnings := []string{}
	for _, e := range mcp.InstallRecommended(p, "", false) {
		mcpWarnings = append(mcpWarnings, e.Error())
	}
	if err := router.Enable(p, router.ModeBalanced); err != nil {
		return Report{}, err
	}
	if err := InstallGlobalRule(p, router.ModeBalanced); err != nil {
		return Report{}, err
	}
	if err := permissions.Apply(p, "autonomous"); err != nil {
		return Report{}, err
	}
	// Recommended installs capability but keeps completion loop opt-in.
	cfg := loop.DefaultConfig()
	cfg.Enabled = false
	_ = loop.Save(p, cfg)
	if err := hooks.Install(p, bin); err != nil {
		return Report{}, err
	}
	_ = hooks.DisableLoopHooks(p)
	if _, err := os.Stat(p.TaskConfig); os.IsNotExist(err) {
		_ = tasks.SaveConfig(p, tasks.DefaultConfig())
	}
	if _, err := os.Stat(p.ReplanConfig); os.IsNotExist(err) {
		_ = replan.SaveConfig(p, replan.DefaultConfig())
	}
	_, _ = capability.Build(p, nil)
	notes := append([]string{}, legacyNotes...)
	notes = append(notes, "Native subagent profiles, Capability Registry, and bounded adaptive DAG replanning installed.")
	notes = append(notes, "Autonomous Completion Loop installed but disabled; enable deep/until-done when desired.")
	notes = append(notes, "Reference Memory MCP is experimental and is not part of the stable default.")
	return Report{InstalledBinary: bin, SkillPackCounts: counts, MCPWarnings: mcpWarnings, Notes: notes}, nil
}

func Full(p paths.Paths, installPrereqs bool) (Report, error) {
	if installPrereqs {
		for _, pr := range MissingPrerequisites() {
			// Docker is large and can require reboot; do not silently install it.
			if pr.ID == "docker" {
				continue
			}
			_ = InstallPrerequisite(pr.ID)
		}
	}
	r, err := Recommended(p, false)
	if err != nil {
		return r, err
	}
	warnings := append([]string{}, r.MCPWarnings...)
	for _, e := range mcp.InstallRecommended(p, "", false) {
		warnings = append(warnings, e.Error())
	}
	if err := agents.Enable(p, agents.ModeParallel); err != nil {
		return r, err
	}
	if _, err := loop.EnableProfile(p, "deep"); err != nil {
		return r, err
	}
	if err := hooks.EnableLoopHooks(p); err != nil {
		return r, err
	}
	r.MCPWarnings = warnings
	_, _ = capability.Build(p, nil)
	r.Notes = append(r.Notes, "Parallel native subagent orchestration and adaptive DAG replanning enabled.")
	r.Notes = append(r.Notes, "Deep Verified Autonomous Completion Loop enabled (50 execution attempts, guarded no-prompt tools).")
	return r, nil
}

func SetRouterMode(p paths.Paths, mode string) error {
	if err := router.Enable(p, mode); err != nil {
		return err
	}
	if err := InstallGlobalRule(p, mode); err != nil {
		return err
	}
	bin, err := InstallSelf(p)
	if err != nil {
		return err
	}
	return hooks.Install(p, bin)
}

func EnableLoop(p paths.Paths, profile string) error {
	if _, err := loop.EnableProfile(p, profile); err != nil {
		return err
	}
	bin, err := InstallSelf(p)
	if err != nil {
		return err
	}
	if err := hooks.Install(p, bin); err != nil {
		return err
	}
	return hooks.EnableLoopHooks(p)
}

func DisableLoop(p paths.Paths) error {
	if err := loop.Disable(p); err != nil {
		return err
	}
	return hooks.DisableLoopHooks(p)
}

func MigrateV2(p paths.Paths) ([]string, error) {
	var notes []string
	if err := p.Ensure(); err != nil {
		return notes, err
	}
	legacy, err := MigrateLegacy(p)
	if err != nil {
		return notes, err
	}
	notes = append(notes, legacy...)
	if err := risk.EnsurePolicy(p); err != nil {
		return notes, err
	}
	if err := skills.InstallEmbedded(p); err != nil {
		return notes, err
	}
	if err := agents.InstallEmbedded(p, ""); err != nil {
		return notes, err
	}
	if cfg, err := agents.LoadConfig(p); err == nil {
		if !cfg.Enabled {
			_ = agents.Enable(p, agents.ModeBalanced)
		}
	} else {
		_ = agents.Enable(p, agents.ModeBalanced)
	}
	if _, err := os.Stat(p.TaskConfig); os.IsNotExist(err) {
		_ = tasks.SaveConfig(p, tasks.DefaultConfig())
	}
	bin, err := InstallSelf(p)
	if err != nil {
		return notes, err
	}
	if err := hooks.Install(p, bin); err != nil {
		return notes, err
	}
	if _, err := capability.Build(p, nil); err != nil {
		notes = append(notes, "capability registry warning: "+err.Error())
	}
	if names, _ := mcp.Names(p, ""); containsString(names, "memory") {
		notes = append(notes, "existing Memory MCP kept for compatibility; it is experimental and no longer part of stable defaults")
	}
	notes = append(notes, "installed 3.2 custom agents/orchestrator, semantic risk policy, adaptive replanning, task supervisor, capability registry and refreshed Go hooks")
	return notes, nil
}

func MigrateV31(p paths.Paths) ([]string, error) {
	var notes []string
	if err := p.Ensure(); err != nil {
		return notes, err
	}
	if _, err := os.Stat(p.ReplanConfig); os.IsNotExist(err) {
		if err := replan.SaveConfig(p, replan.DefaultConfig()); err != nil {
			return notes, err
		}
		notes = append(notes, "created bounded adaptive DAG replanning configuration")
	}
	if _, err := os.Stat(p.TaskConfig); os.IsNotExist(err) {
		if err := tasks.SaveConfig(p, tasks.DefaultConfig()); err != nil {
			return notes, err
		}
		notes = append(notes, "created resource-aware task supervisor configuration")
	}
	if err := TouchManifest(p, "3.2.1"); err != nil {
		return notes, err
	}
	notes = append(notes, "3.1 configuration is compatible; no destructive migration was required")
	return notes, nil
}

func MigrateV32(p paths.Paths) ([]string, error) {
	var notes []string
	if err := p.Ensure(); err != nil {
		return notes, err
	}
	bin, err := InstallSelf(p)
	if err != nil {
		return notes, err
	}
	if err := skills.InstallEmbedded(p); err != nil {
		return notes, err
	}
	if err := agents.InstallEmbedded(p, ""); err != nil {
		return notes, err
	}
	if err := hooks.Install(p, bin); err != nil {
		return notes, err
	}
	if _, err := capability.Build(p, nil); err != nil {
		notes = append(notes, "capability registry warning: "+err.Error())
	}
	if err := TouchManifest(p, "3.2.1"); err != nil {
		return notes, err
	}
	notes = append(notes,
		"refreshed managed hooks/CLI plugin for current Antigravity schemas",
		"refreshed embedded custom agents with documented frontmatter",
		"rebuilt capability registry with flat/folder/plugin agent discovery",
		"kept existing MCP/skills/plugins/tasks/replan configuration; no destructive migration required",
		"gopls MCP remains available as explicit opt-in because its upstream MCP mode is experimental",
	)
	return notes, nil
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func TouchManifest(p paths.Paths, version string) error {
	m := map[string]any{"version": version, "installedAt": time.Now().Format(time.RFC3339Nano), "binary": InstalledBinaryPath(p)}
	return jsonx.WriteAtomic(p.InstalledManifest, m, p.BackupsRoot)
}
