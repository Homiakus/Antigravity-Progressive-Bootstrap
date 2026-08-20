package planner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/homiakus/agctl/internal/paths"
)

func TestCreateBuildsAcyclicGoPlan(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := paths.Paths{PlansRoot: filepath.Join(root, "plans"), CapabilityDB: filepath.Join(root, "caps.json"), BackupsRoot: filepath.Join(root, "backups"), GlobalSkillsRoot: filepath.Join(root, "skills"), GlobalAgentsRoot: filepath.Join(root, "agents"), GlobalPluginsRoot: filepath.Join(root, "plugins"), GlobalMCP: filepath.Join(root, "mcp.json")}
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	pl, err := Create(p, "Проведи архитектурный аудит Go сервиса, исправь ошибки и проверь тестами", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Nodes) < 4 {
		t.Fatalf("expected multi-stage plan, got %d nodes", len(pl.Nodes))
	}
	foundGo := false
	for _, profile := range pl.Profiles {
		if profile == "go" {
			foundGo = true
		}
	}
	if !foundGo {
		t.Fatalf("expected go profile, got %v", pl.Profiles)
	}
	if err := validate(pl); err != nil {
		t.Fatal(err)
	}
}

func TestReadOnlyAuditSkipsImplement(t *testing.T) {
	root := t.TempDir()
	p := paths.Paths{PlansRoot: filepath.Join(root, "plans"), CapabilityDB: filepath.Join(root, "caps.json"), GlobalSkillsRoot: filepath.Join(root, "skills"), GlobalAgentsRoot: filepath.Join(root, "agents"), GlobalPluginsRoot: filepath.Join(root, "plugins"), GlobalMCP: filepath.Join(root, "mcp.json")}
	_ = p.Ensure()
	pl, err := Create(p, "Только анализ, ничего не меняй: проведи аудит архитектуры", root)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range pl.Nodes {
		if n.ID == "implement" {
			t.Fatal("read-only plan must not contain implement node")
		}
	}
}
