package capability

import (
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/sidecar"
	"path/filepath"
	"testing"
)

func TestBuildIncludesSidecars(t *testing.T) {
	root := t.TempDir()
	p := paths.Paths{Home: root, ConfigRoot: filepath.Join(root, ".gemini", "config")}
	p.GlobalSkillsRoot = filepath.Join(p.ConfigRoot, "skills")
	p.GlobalAgentsRoot = filepath.Join(p.ConfigRoot, "agents")
	p.GlobalPluginsRoot = filepath.Join(p.ConfigRoot, "plugins")
	p.CLIPluginsRoot = filepath.Join(root, ".gemini", "antigravity-cli", "plugins")
	p.GlobalMCP = filepath.Join(p.ConfigRoot, "mcp_config.json")
	p.GlobalConfig = filepath.Join(p.ConfigRoot, "config.json")
	p.SidecarsRoot = filepath.Join(p.ConfigRoot, "sidecars")
	p.AppRoot = filepath.Join(p.ConfigRoot, "agctl")
	p.BackupsRoot = filepath.Join(p.AppRoot, "backups")
	p.CapabilityDB = filepath.Join(p.AppRoot, "capabilities.json")
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	if _, err := sidecar.CreateSchedule(p, "hourly", "0 * * * *", "agentapi", []string{"new-conversation", "health check"}, "health check"); err != nil {
		t.Fatal(err)
	}
	if err := sidecar.Enable(p, "hourly", ""); err != nil {
		t.Fatal(err)
	}
	reg, err := Build(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range reg.Capabilities {
		if c.Kind == "sidecar" && c.ID == "hourly" && c.Enabled {
			found = true
		}
	}
	if !found {
		t.Fatalf("enabled sidecar not present in capability registry: %#v", reg.Capabilities)
	}
}
