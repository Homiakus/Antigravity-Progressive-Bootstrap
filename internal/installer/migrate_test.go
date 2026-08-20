package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/paths"
)

func migratePaths(t *testing.T) paths.Paths {
	root := t.TempDir()
	p := paths.Paths{
		Home:       root,
		GeminiRoot: filepath.Join(root, ".gemini"),
		ConfigRoot: filepath.Join(root, ".gemini", "config"),
	}
	p.GlobalSkillsRoot = filepath.Join(p.ConfigRoot, "skills")
	p.CLISkillsRoot = filepath.Join(p.GeminiRoot, "antigravity-cli", "skills")
	p.GlobalMCP = filepath.Join(p.ConfigRoot, "mcp_config.json")
	p.GlobalHooks = filepath.Join(p.ConfigRoot, "hooks.json")
	p.GlobalRule = filepath.Join(p.GeminiRoot, "GEMINI.md")
	p.CLISettings = filepath.Join(p.GeminiRoot, "antigravity-cli", "settings.json")
	p.AppRoot = filepath.Join(p.ConfigRoot, "agctl")
	p.BinRoot = filepath.Join(p.AppRoot, "bin")
	p.BackupsRoot = filepath.Join(p.AppRoot, "backups")
	p.StateRoot = filepath.Join(p.AppRoot, "state")
	p.RouterConfig = filepath.Join(p.AppRoot, "router.json")
	p.LoopConfig = filepath.Join(p.AppRoot, "loop.json")
	p.InstalledManifest = filepath.Join(p.AppRoot, "manifest.json")
	p.LogRoot = filepath.Join(p.AppRoot, "logs")
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMigrateLegacyRemovesOnlyManagedPowerShellEntries(t *testing.T) {
	p := migratePaths(t)
	hooks := map[string]any{
		"adaptive-tool-router-pre-invocation": map[string]any{"enabled": true},
		"autonomous-completion-loop":          map[string]any{"enabled": true},
		"autonomous-completion-no-prompt":     map[string]any{"enabled": true},
		"my-user-hook":                        map[string]any{"enabled": true, "x": "preserve"},
		"future-field":                        map[string]any{"nested": 42.0},
	}
	if err := jsonx.WriteAtomic(p.GlobalHooks, hooks, ""); err != nil {
		t.Fatal(err)
	}
	rule := strings.Join([]string{
		"# My global rule",
		"keep this line",
		"<!-- ANTIGRAVITY-BOOTSTRAP:ADAPTIVE-TOOL-ROUTER:BEGIN -->",
		"old managed content",
		"<!-- ANTIGRAVITY-BOOTSTRAP:ADAPTIVE-TOOL-ROUTER:END -->",
		"keep after block",
		"",
	}, "\n")
	if err := os.WriteFile(p.GlobalRule, []byte(rule), 0o644); err != nil {
		t.Fatal(err)
	}

	notes, err := MigrateLegacy(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) < 4 {
		t.Fatalf("expected migration notes, got %#v", notes)
	}

	gotHooks, err := jsonx.ReadMap(p.GlobalHooks)
	if err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []string{"adaptive-tool-router-pre-invocation", "autonomous-completion-loop", "autonomous-completion-no-prompt"} {
		if _, ok := gotHooks[legacy]; ok {
			t.Fatalf("legacy hook still present: %s", legacy)
		}
	}
	if _, ok := gotHooks["my-user-hook"]; !ok {
		t.Fatal("user hook was removed")
	}
	if _, ok := gotHooks["future-field"]; !ok {
		t.Fatal("unknown field was removed")
	}

	gotRule, err := os.ReadFile(p.GlobalRule)
	if err != nil {
		t.Fatal(err)
	}
	s := string(gotRule)
	if strings.Contains(s, "ANTIGRAVITY-BOOTSTRAP:ADAPTIVE-TOOL-ROUTER") || strings.Contains(s, "old managed content") {
		t.Fatalf("legacy rule block still present: %s", s)
	}
	if !strings.Contains(s, "keep this line") || !strings.Contains(s, "keep after block") {
		t.Fatalf("user rule content was lost: %s", s)
	}

	backups, err := jsonx.ListBackups(p.BackupsRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) < 2 {
		t.Fatalf("expected backups for hooks and rule, got %d", len(backups))
	}
}
