package capability

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/homiakus/agctl/internal/paths"
)

func TestBuildSuppressesCLIMirrorButKeepsCLIOnlySkill(t *testing.T) {
	root := t.TempDir()
	p := paths.Paths{
		GlobalSkillsRoot: filepath.Join(root, "config", "skills"),
		CLISkillsRoot:    filepath.Join(root, "cli", "skills"),
		CapabilityDB:     filepath.Join(root, "agctl", "capabilities.json"),
		BackupsRoot:      filepath.Join(root, "backups"),
	}
	for _, dir := range []string{p.GlobalSkillsRoot, p.CLISkillsRoot, filepath.Dir(p.CapabilityDB), p.BackupsRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	canonicalDir := filepath.Join(p.GlobalSkillsRoot, "shared-skill")
	if err := os.MkdirAll(canonicalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	canonical := "---\nname: shared-skill\ndescription: canonical copy\n---\n"
	if err := os.WriteFile(filepath.Join(canonicalDir, "SKILL.md"), []byte(canonical), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.CLISkillsRoot, "shared-skill.md"), []byte(canonical), 0o644); err != nil {
		t.Fatal(err)
	}
	cliOnly := "---\nname: cli-only\ndescription: CLI only copy\n---\n"
	if err := os.WriteFile(filepath.Join(p.CLISkillsRoot, "cli-only.md"), []byte(cliOnly), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := Build(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	scopes := map[string]string{}
	for _, c := range reg.Capabilities {
		if c.Kind == "skill" {
			counts[c.ID]++
			scopes[c.ID] = c.Scope
		}
	}
	if counts["shared-skill"] != 1 || scopes["shared-skill"] != "global" {
		t.Fatalf("shared-skill should appear once from canonical IDE root; count=%d scope=%q", counts["shared-skill"], scopes["shared-skill"])
	}
	if counts["cli-only"] != 1 || scopes["cli-only"] != "cli-global" {
		t.Fatalf("cli-only skill should remain discoverable; count=%d scope=%q", counts["cli-only"], scopes["cli-only"])
	}
}
