package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/homiakus/agctl/internal/paths"
)

func TestDetectAndInitGo(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/x\n\ngo 1.23\n"), 0644); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	p := paths.Paths{Home: home, ConfigRoot: filepath.Join(home, "config"), GlobalAgentsRoot: filepath.Join(home, "config", "agents"), GlobalMCP: filepath.Join(home, "config", "mcp_config.json"), AppRoot: filepath.Join(home, "app")}
	p.BackupsRoot = filepath.Join(p.AppRoot, "backups")
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	d, err := Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Profiles) == 0 || d.Profiles[0] != "go" {
		t.Fatalf("d=%+v", d)
	}
	if _, err := Init(p, root, []string{"go"}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"AGENTS.md", filepath.Join(".agents", "rules", "agctl-project.md"), filepath.Join(".agents", "workflows", "verified-goal.md"), filepath.Join(".agents", "agents", "architect", "agent.md")} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Fatalf("missing %s: %v", f, err)
		}
	}
	rule, err := os.ReadFile(filepath.Join(root, ".agents", "rules", "agctl-project.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rule), "activation:") {
		t.Fatal("agctl must not invent undocumented rule activation frontmatter")
	}
}
