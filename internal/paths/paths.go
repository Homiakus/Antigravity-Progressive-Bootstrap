package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Platform string

const (
	PlatformAntigravity Platform = "antigravity"
	PlatformDSH         Platform = "dsh"
	PlatformCursor      Platform = "cursor"
	PlatformClaude      Platform = "claude"
	PlatformCline       Platform = "cline"
	PlatformUniversal   Platform = "universal"
)

type PlatformInfo struct {
	Platform    Platform `json:"platform"`
	Label       string   `json:"label"`
	ConfigDir   string   `json:"configDir"`
	RulePath    string   `json:"rulePath"`
	MCPPath     string   `json:"mcpPath"`
	Active      bool     `json:"active"`
	Description string   `json:"description"`
}

type Paths struct {
	Home               string
	ActivePlatform     Platform
	DetectedPlatforms  []Platform
	GeminiRoot         string
	ConfigRoot         string
	GlobalSkillsRoot   string
	CLISkillsRoot      string
	CLIPluginsRoot     string
	GlobalAgentsRoot   string
	GlobalPluginsRoot  string
	GlobalMCP          string
	GlobalConfig       string
	SidecarsRoot       string
	SidecarDataRoot    string
	GlobalHooks        string
	GlobalRule         string
	CLISettings        string
	AppRoot            string
	BinRoot            string
	BackupsRoot        string
	StateRoot          string
	HarnessDB          string
	ArtifactRoot       string
	HarnessBackupRoot  string
	RouterConfig       string
	LoopConfig         string
	OrchestratorConfig string
	CapabilityDB       string
	RiskPolicy         string
	ProfilesRoot       string
	LocksRoot          string
	RegistryCacheRoot  string
	TasksRoot          string
	TaskConfig         string
	PlansRoot          string
	SecurityRoot       string
	ReplanRoot         string
	ReplanConfig       string
	ReplanInbox        string
	ReplanArchive      string
	TelemetryRoot      string
	InstalledManifest  string
	LogRoot            string
}

func Detect() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("detect user home: %w", err)
	}

	activePlatform := detectActivePlatform(home)
	allPlatforms := detectAllPlatforms(home)

	customConfig := os.Getenv("AGCTL_CONFIG_ROOT")
	if customConfig == "" {
		customConfig = os.Getenv("AGCTL_CONFIG_DIR")
	}

	gemini := filepath.Join(home, ".gemini")
	config := filepath.Join(gemini, "config")
	if customConfig != "" {
		config = customConfig
	}

	app := filepath.Join(config, "agctl")
	state := filepath.Join(app, "state")
	if envState := os.Getenv("AGCTL_STATE_ROOT"); envState != "" {
		state = envState
	}
	backups := filepath.Join(app, "backups")
	harnessState := filepath.Join(state, "harness")

	globalRule := filepath.Join(gemini, "GEMINI.md")
	if activePlatform == PlatformClaude {
		globalRule = filepath.Join(home, "CLAUDE.md")
	} else if activePlatform == PlatformDSH || activePlatform == PlatformUniversal {
		if _, err := os.Stat(filepath.Join(gemini, "GEMINI.md")); os.IsNotExist(err) {
			globalRule = filepath.Join(home, "AGENTS.md")
		}
	}

	globalMCP := filepath.Join(config, "mcp_config.json")
	if _, err := os.Stat(globalMCP); os.IsNotExist(err) {
		cursorMCP := filepath.Join(home, ".cursor", "mcp.json")
		claudeMCP := filepath.Join(home, ".claude", "mcp_config.json")
		if _, err := os.Stat(cursorMCP); err == nil && activePlatform == PlatformCursor {
			globalMCP = cursorMCP
		} else if _, err := os.Stat(claudeMCP); err == nil && activePlatform == PlatformClaude {
			globalMCP = claudeMCP
		}
	}

	return Paths{
		Home:               home,
		ActivePlatform:     activePlatform,
		DetectedPlatforms:  allPlatforms,
		GeminiRoot:         gemini,
		ConfigRoot:         config,
		GlobalSkillsRoot:   filepath.Join(config, "skills"),
		CLISkillsRoot:      filepath.Join(gemini, "antigravity-cli", "skills"),
		CLIPluginsRoot:     filepath.Join(gemini, "antigravity-cli", "plugins"),
		GlobalAgentsRoot:   filepath.Join(config, "agents"),
		GlobalPluginsRoot:  filepath.Join(config, "plugins"),
		GlobalMCP:          globalMCP,
		GlobalConfig:       filepath.Join(config, "config.json"),
		SidecarsRoot:       filepath.Join(config, "sidecars"),
		SidecarDataRoot:    filepath.Join(gemini, "antigravity", "sidecar_data"),
		GlobalHooks:        filepath.Join(config, "hooks.json"),
		GlobalRule:         globalRule,
		CLISettings:        filepath.Join(gemini, "antigravity-cli", "settings.json"),
		AppRoot:            app,
		BinRoot:            filepath.Join(app, "bin"),
		BackupsRoot:        backups,
		StateRoot:          state,
		HarnessDB:          filepath.Join(harnessState, "state.db"),
		ArtifactRoot:       filepath.Join(harnessState, "artifacts"),
		HarnessBackupRoot:  filepath.Join(backups, "harness"),
		RouterConfig:       filepath.Join(app, "router.json"),
		LoopConfig:         filepath.Join(app, "loop.json"),
		OrchestratorConfig: filepath.Join(app, "orchestrator.json"),
		CapabilityDB:       filepath.Join(app, "capabilities.json"),
		RiskPolicy:         filepath.Join(app, "risk-policy.json"),
		ProfilesRoot:       filepath.Join(app, "profiles"),
		LocksRoot:          filepath.Join(app, "locks"),
		RegistryCacheRoot:  filepath.Join(app, "registry-cache"),
		TasksRoot:          filepath.Join(app, "tasks"),
		TaskConfig:         filepath.Join(app, "task-supervisor.json"),
		PlansRoot:          filepath.Join(app, "plans"),
		SecurityRoot:       filepath.Join(app, "security"),
		ReplanRoot:         filepath.Join(app, "replan"),
		ReplanConfig:       filepath.Join(app, "replan", "config.json"),
		ReplanInbox:        filepath.Join(app, "replan", "inbox"),
		ReplanArchive:      filepath.Join(app, "replan", "archive"),
		TelemetryRoot:      filepath.Join(app, "telemetry"),
		InstalledManifest:  filepath.Join(app, "manifest.json"),
		LogRoot:            filepath.Join(app, "logs"),
	}, nil
}

func detectActivePlatform(home string) Platform {
	if env := os.Getenv("AGCTL_PLATFORM"); env != "" {
		switch strings.ToLower(env) {
		case "antigravity", "agy", "gemini":
			return PlatformAntigravity
		case "dsh", "deepseek":
			return PlatformDSH
		case "cursor":
			return PlatformCursor
		case "claude", "anthropic":
			return PlatformClaude
		case "cline", "roo", "roocode":
			return PlatformCline
		case "universal", "generic":
			return PlatformUniversal
		}
	}

	if os.Getenv("ANTIGRAVITY_APP_DATA") != "" || os.Getenv("AGY_VERSION") != "" {
		return PlatformAntigravity
	}
	if os.Getenv("DSH_ROOT") != "" || os.Getenv("CORDIS_PLUGIN_DIR") != "" {
		return PlatformDSH
	}
	if os.Getenv("CURSOR_AGENT") != "" || os.Getenv("CURSOR_VERSION") != "" {
		return PlatformCursor
	}
	if os.Getenv("CLAUDE_CONFIG_DIR") != "" || os.Getenv("CLAUDE_CODE") != "" {
		return PlatformClaude
	}
	if os.Getenv("CLINE_DIR") != "" || os.Getenv("ROO_CODE") != "" {
		return PlatformCline
	}

	geminiDir := filepath.Join(home, ".gemini")
	if _, err := os.Stat(geminiDir); err == nil {
		return PlatformAntigravity
	}
	cursorDir := filepath.Join(home, ".cursor")
	if _, err := os.Stat(cursorDir); err == nil {
		return PlatformCursor
	}
	claudeDir := filepath.Join(home, ".claude")
	if _, err := os.Stat(claudeDir); err == nil {
		return PlatformClaude
	}

	return PlatformUniversal
}

func detectAllPlatforms(home string) []Platform {
	var platforms []Platform
	geminiDir := filepath.Join(home, ".gemini")
	if _, err := os.Stat(geminiDir); err == nil {
		platforms = append(platforms, PlatformAntigravity)
	}
	cursorDir := filepath.Join(home, ".cursor")
	if _, err := os.Stat(cursorDir); err == nil {
		platforms = append(platforms, PlatformCursor)
	}
	claudeDir := filepath.Join(home, ".claude")
	if _, err := os.Stat(claudeDir); err == nil {
		platforms = append(platforms, PlatformClaude)
	}
	clineDir := filepath.Join(home, ".cline")
	if _, err := os.Stat(clineDir); err == nil {
		platforms = append(platforms, PlatformCline)
	}
	if os.Getenv("DSH_ROOT") != "" || os.Getenv("CORDIS_PLUGIN_DIR") != "" {
		platforms = append(platforms, PlatformDSH)
	}
	if len(platforms) == 0 {
		platforms = append(platforms, PlatformUniversal)
	}
	return platforms
}

func (p Paths) Ensure() error {
	dirs := []string{
		p.ConfigRoot, p.GlobalSkillsRoot, p.CLISkillsRoot, p.CLIPluginsRoot, p.GlobalAgentsRoot, p.GlobalPluginsRoot, p.SidecarsRoot, p.SidecarDataRoot,
		p.AppRoot, p.BinRoot, p.BackupsRoot, p.StateRoot, p.LogRoot, p.ProfilesRoot, p.LocksRoot,
		p.ArtifactRoot, p.HarnessBackupRoot, filepath.Dir(p.HarnessDB),
		p.RegistryCacheRoot, p.TasksRoot, p.PlansRoot, p.SecurityRoot, p.ReplanRoot, p.ReplanInbox, p.ReplanArchive, p.TelemetryRoot,
	}
	for _, d := range dirs {
		if d == "" || d == "." {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

func (p Paths) GetPlatformInfos() []PlatformInfo {
	home := p.Home
	list := []PlatformInfo{
		{
			Platform:    PlatformAntigravity,
			Label:       "Antigravity IDE / AGY CLI",
			ConfigDir:   filepath.Join(home, ".gemini", "config"),
			RulePath:    filepath.Join(home, ".gemini", "GEMINI.md"),
			MCPPath:     filepath.Join(home, ".gemini", "config", "mcp_config.json"),
			Description: "Native Google Antigravity reasoning, hooks, subagent orchestrator and skills",
		},
		{
			Platform:    PlatformDSH,
			Label:       "DeepSeek Harness (DSH)",
			ConfigDir:   filepath.Join(home, ".gemini", "config", "agctl"),
			RulePath:    filepath.Join(home, "AGENTS.md"),
			MCPPath:     filepath.Join(home, ".gemini", "config", "mcp_config.json"),
			Description: "DeepSeek Harness Cordis plugin tree, durable jobs, pwsh sidecar execution",
		},
		{
			Platform:    PlatformCursor,
			Label:       "Cursor Agent",
			ConfigDir:   filepath.Join(home, ".cursor"),
			RulePath:    filepath.Join(home, ".cursorrules"),
			MCPPath:     filepath.Join(home, ".cursor", "mcp.json"),
			Description: "Cursor AI editor agent, .cursorrules and project-level rules",
		},
		{
			Platform:    PlatformClaude,
			Label:       "Claude Code",
			ConfigDir:   filepath.Join(home, ".claude"),
			RulePath:    filepath.Join(home, "CLAUDE.md"),
			MCPPath:     filepath.Join(home, ".claude", "mcp_config.json"),
			Description: "Anthropic Claude Code CLI agent and CLAUDE.md project memory",
		},
		{
			Platform:    PlatformCline,
			Label:       "Roo Code / Cline",
			ConfigDir:   filepath.Join(home, ".cline"),
			RulePath:    filepath.Join(home, ".roomodes"),
			MCPPath:     filepath.Join(home, ".cline", "mcp_settings.json"),
			Description: "Roo Code and Cline multi-mode autonomous agents",
		},
		{
			Platform:    PlatformUniversal,
			Label:       "Universal / Generic Agent",
			ConfigDir:   filepath.Join(home, ".gemini", "config", "agctl"),
			RulePath:    filepath.Join(home, "AGENTS.md"),
			MCPPath:     filepath.Join(home, ".agents", "mcp_config.json"),
			Description: "Generic autonomous agent CLI (Codex CLI, OpenCode, Aider, etc.)",
		},
	}

	for i := range list {
		list[i].Active = (list[i].Platform == p.ActivePlatform)
	}
	return list
}

func WorkspaceRoot(workspace string) string { return filepath.Join(workspace, ".agents") }
func WorkspaceMCP(workspace string) string {
	return filepath.Join(WorkspaceRoot(workspace), "mcp_config.json")
}
func WorkspaceSkills(workspace string) string {
	return filepath.Join(WorkspaceRoot(workspace), "skills")
}
func WorkspaceRules(workspace string) string { return filepath.Join(WorkspaceRoot(workspace), "rules") }
func WorkspaceWorkflows(workspace string) string {
	return filepath.Join(WorkspaceRoot(workspace), "workflows")
}
func WorkspaceAgents(workspace string) string {
	return filepath.Join(WorkspaceRoot(workspace), "agents")
}
func WorkspacePlugins(workspace string) string {
	return filepath.Join(WorkspaceRoot(workspace), "plugins")
}
func WorkspaceHooks(workspace string) string {
	return filepath.Join(WorkspaceRoot(workspace), "hooks.json")
}

func WorkspaceRuleFiles(workspace string) []string {
	return []string{
		filepath.Join(workspace, "AGENTS.md"),
		filepath.Join(workspace, "GEMINI.md"),
		filepath.Join(workspace, "CLAUDE.md"),
		filepath.Join(workspace, ".cursorrules"),
		filepath.Join(WorkspaceRules(workspace), "agctl-project.md"),
	}
}

func WorkspaceMCPFiles(workspace string) []string {
	return []string{
		WorkspaceMCP(workspace),
		filepath.Join(workspace, ".cursor", "mcp.json"),
		filepath.Join(workspace, "mcp.json"),
	}
}

