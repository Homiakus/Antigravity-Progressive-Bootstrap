package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

type Paths struct {
	Home               string
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
	gemini := filepath.Join(home, ".gemini")
	config := filepath.Join(gemini, "config")
	app := filepath.Join(config, "agctl")
	state := filepath.Join(app, "state")
	backups := filepath.Join(app, "backups")
	harnessState := filepath.Join(state, "harness")
	return Paths{
		Home:               home,
		GeminiRoot:         gemini,
		ConfigRoot:         config,
		GlobalSkillsRoot:   filepath.Join(config, "skills"),
		CLISkillsRoot:      filepath.Join(gemini, "antigravity-cli", "skills"),
		CLIPluginsRoot:     filepath.Join(gemini, "antigravity-cli", "plugins"),
		GlobalAgentsRoot:   filepath.Join(config, "agents"),
		GlobalPluginsRoot:  filepath.Join(config, "plugins"),
		GlobalMCP:          filepath.Join(config, "mcp_config.json"),
		GlobalConfig:       filepath.Join(config, "config.json"),
		SidecarsRoot:       filepath.Join(config, "sidecars"),
		SidecarDataRoot:    filepath.Join(gemini, "antigravity", "sidecar_data"),
		GlobalHooks:        filepath.Join(config, "hooks.json"),
		GlobalRule:         filepath.Join(gemini, "GEMINI.md"),
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
