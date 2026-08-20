package router

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/sidecar"
)

const (
	ModeSilent      = "silent"
	ModeBalanced    = "balanced"
	ModeTransparent = "transparent"
	ModeMaximum     = "maximum"
)

func DefaultConfig() model.RouterConfig {
	return model.RouterConfig{Enabled: true, Mode: ModeBalanced}
}

func Load(p paths.Paths) (model.RouterConfig, error) {
	cfg, err := jsonx.Read(p.RouterConfig, DefaultConfig())
	if err != nil {
		return cfg, err
	}
	if cfg.Mode == "" {
		cfg.Mode = ModeBalanced
	}
	return cfg, nil
}

func Save(p paths.Paths, cfg model.RouterConfig) error {
	if !validMode(cfg.Mode) {
		return fmt.Errorf("invalid router mode %q", cfg.Mode)
	}
	return jsonx.WriteAtomic(p.RouterConfig, cfg, p.BackupsRoot)
}

func Enable(p paths.Paths, mode string) error {
	if !validMode(mode) {
		return fmt.Errorf("invalid router mode %q", mode)
	}
	return Save(p, model.RouterConfig{Enabled: true, Mode: mode})
}

func Disable(p paths.Paths) error {
	cfg, _ := Load(p)
	cfg.Enabled = false
	return Save(p, cfg)
}

func validMode(s string) bool {
	switch s {
	case ModeSilent, ModeBalanced, ModeTransparent, ModeMaximum:
		return true
	default:
		return false
	}
}

type Inventory struct {
	Skills    []string `json:"skills"`
	MCP       []string `json:"mcp"`
	Agents    []string `json:"agents"`
	Plugins   []string `json:"plugins"`
	Workflows []string `json:"workflows"`
	Sidecars  []string `json:"sidecars"`
}

func Discover(p paths.Paths, workspaces []string) Inventory {
	skillSet := map[string]struct{}{}
	mcpSet := map[string]struct{}{}
	agentSet := map[string]struct{}{}
	pluginSet := map[string]struct{}{}
	workflowSet := map[string]struct{}{}
	sidecarSet := map[string]struct{}{}

	addSkills := func(root string) {
		entries, err := os.ReadDir(root)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				if _, err := os.Stat(filepath.Join(root, e.Name(), "SKILL.md")); err == nil {
					skillSet[e.Name()] = struct{}{}
				}
				continue
			}
			if strings.EqualFold(filepath.Ext(e.Name()), ".md") {
				skillSet[strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))] = struct{}{}
			}
		}
	}
	addMCP := func(file string) {
		cfg, err := jsonx.Read(file, model.MCPConfig{MCPServers: map[string]model.MCPServer{}})
		if err != nil {
			return
		}
		for k := range cfg.MCPServers {
			mcpSet[k] = struct{}{}
		}
	}
	addAgents := func(root string) {
		entries, err := os.ReadDir(root)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				if _, err := os.Stat(filepath.Join(root, e.Name(), "agent.md")); err == nil {
					agentSet[e.Name()] = struct{}{}
				}
				continue
			}
			if strings.EqualFold(filepath.Ext(e.Name()), ".md") {
				agentSet[strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))] = struct{}{}
			}
		}
	}
	addPlugins := func(root string) {
		entries, err := os.ReadDir(root)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			pluginRoot := filepath.Join(root, e.Name())
			if _, err := os.Stat(filepath.Join(pluginRoot, "plugin.json")); err == nil {
				pluginSet[e.Name()] = struct{}{}
				addAgents(filepath.Join(pluginRoot, "agents"))
			}
		}
	}
	addWorkflows := func(root string) {
		entries, err := os.ReadDir(root)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				workflowSet[strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))] = struct{}{}
			}
		}
	}

	addSkills(p.GlobalSkillsRoot)
	addSkills(p.CLISkillsRoot)
	addAgents(p.GlobalAgentsRoot)
	addPlugins(p.GlobalPluginsRoot)
	addPlugins(p.CLIPluginsRoot)
	addMCP(p.GlobalMCP)
	if sidecars, err := sidecar.List(p); err == nil {
		for _, sc := range sidecars {
			sidecarSet[sc.ID] = struct{}{}
		}
	}
	for _, w := range workspaces {
		if strings.TrimSpace(w) == "" {
			continue
		}
		addSkills(paths.WorkspaceSkills(w))
		addMCP(paths.WorkspaceMCP(w))
		addAgents(paths.WorkspaceAgents(w))
		addPlugins(paths.WorkspacePlugins(w))
		addWorkflows(paths.WorkspaceWorkflows(w))
	}

	inv := Inventory{}
	for k := range skillSet {
		inv.Skills = append(inv.Skills, k)
	}
	for k := range mcpSet {
		inv.MCP = append(inv.MCP, k)
	}
	for k := range agentSet {
		inv.Agents = append(inv.Agents, k)
	}
	for k := range pluginSet {
		inv.Plugins = append(inv.Plugins, k)
	}
	for k := range workflowSet {
		inv.Workflows = append(inv.Workflows, k)
	}
	for k := range sidecarSet {
		inv.Sidecars = append(inv.Sidecars, k)
	}
	sort.Strings(inv.Skills)
	sort.Strings(inv.MCP)
	sort.Strings(inv.Agents)
	sort.Strings(inv.Plugins)
	sort.Strings(inv.Workflows)
	sort.Strings(inv.Sidecars)
	return inv
}

func Injection(cfg model.RouterConfig, inv Inventory) string {
	if !cfg.Enabled {
		return ""
	}
	skills := "(none discovered)"
	if len(inv.Skills) > 0 {
		skills = strings.Join(inv.Skills, ", ")
	}
	mcps := "(none configured)"
	if len(inv.MCP) > 0 {
		mcps = strings.Join(inv.MCP, ", ")
	}
	agents := "(none discovered)"
	if len(inv.Agents) > 0 {
		agents = strings.Join(inv.Agents, ", ")
	}
	plugins := "(none discovered)"
	if len(inv.Plugins) > 0 {
		plugins = strings.Join(inv.Plugins, ", ")
	}
	workflows := "(none discovered)"
	if len(inv.Workflows) > 0 {
		workflows = strings.Join(inv.Workflows, ", ")
	}
	sidecars := "(none discovered)"
	if len(inv.Sidecars) > 0 {
		sidecars = strings.Join(inv.Sidecars, ", ")
	}

	visibility := "Perform routing internally."
	switch cfg.Mode {
	case ModeTransparent:
		visibility = "Your first visible assistant text for this task MUST be exactly one concise line in the form `Route: skills=[...]; MCP=[...]; verification=[...]`, then continue immediately. Do not omit it even if lists are empty."
	case ModeMaximum:
		visibility = "For every non-trivial task, print one concise Route line before substantive prose and proactively combine complementary capabilities when they materially improve quality."
	case ModeBalanced:
		visibility = "For non-trivial tasks where capability selection materially matters, print one concise Route line before substantive prose; keep trivial routing internal."
	case ModeSilent:
		visibility = "Keep routing decisions internal unless the user asks."
	}

	return fmt.Sprintf(`ADAPTIVE TOOL ROUTER

Before substantive work, classify the actual task and select the SMALLEST SUFFICIENT set of skills, MCP servers, native tools, verification methods, and subagents.

Discovered skills:
%s

Configured MCP servers:
%s

Available custom agents/subagents:
%s

Installed plugins:
%s

Workspace workflows:
%s

Available sidecars/background services:
%s

Routing preferences:
- writing/editing/proofreading/publication -> editorial-quality-director; no-ai-slop only as a subordinate anti-slop pass when useful;
- delegated implementation/debug/refactor -> prefer native /goal semantics when the current Antigravity surface supports it; keep agctl verified completion gate as evidence enforcement; use autonomous-completion-loop as compatibility/fallback control;
- substantial separable work -> use native subagents with independent architect/test/security/review roles rather than bloating the main context;
- current/versioned library or API docs -> Context7 when configured and needed;
- browser user flows/E2E -> Playwright when configured and needed;
- console/network/runtime/performance diagnosis -> Chrome DevTools when configured and needed;
- Go semantic diagnostics/refactoring -> gopls MCP when configured and needed;
- remote GitHub state/actions -> GitHub MCP when configured and needed;
- local file/code work -> native workspace/editor/terminal tools first.

Do not use a capability merely because it is installed. If a chosen capability fails or is irrelevant, re-route automatically. Routing never overrides explicit constraints, permissions, credentials, or safety boundaries.

Visibility mode: %s
%s`, skills, mcps, agents, plugins, workflows, sidecars, cfg.Mode, visibility)
}
