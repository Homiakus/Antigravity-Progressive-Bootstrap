package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/provenance"
)

func TestCLISkillContentKeepsResourceBase(t *testing.T) {
	base := filepath.Join(t.TempDir(), "skill")
	got := string(cliSkillContent([]byte("---\nname: x\ndescription: y\n---\n\n# X\n"), base))
	if !strings.HasPrefix(got, "---\n") {
		t.Fatal("YAML frontmatter must remain first")
	}
	if !strings.Contains(got, filepath.Clean(base)) || !strings.Contains(got, "references/") {
		t.Fatalf("missing CLI resource base note: %s", got)
	}
}

func TestSkillPackLockCoversResourcesAndDetectsTamper(t *testing.T) {
	root := t.TempDir()
	p := paths.Paths{
		GlobalSkillsRoot: filepath.Join(root, "skills"),
		LocksRoot:        filepath.Join(root, "locks"),
		BackupsRoot:      filepath.Join(root, "backups"),
	}
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	skillRoot := filepath.Join(p.GlobalSkillsRoot, "demo")
	if err := os.MkdirAll(filepath.Join(skillRoot, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte("skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(skillRoot, "scripts", "check.sh")
	if err := os.WriteFile(script, []byte("echo ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	pack := Pack{ID: "demo-pack", Repo: "https://example.invalid/demo.git"}
	if err := writePackLock(p, pack, "abc123", []Item{{Name: "demo", Path: skillRoot}}); err != nil {
		t.Fatal(err)
	}
	lock, err := jsonx.Read(filepath.Join(p.LocksRoot, "skill-pack-demo-pack.json"), model.ProvenanceLock{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lock.Files["demo/scripts/check.sh"]; !ok {
		t.Fatalf("resource script missing from provenance lock: %#v", lock.Files)
	}
	if v := provenance.Verify(lock); !v.OK {
		t.Fatalf("fresh skill pack should verify: %+v", v)
	}
	if err := os.WriteFile(script, []byte("echo tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := provenance.Verify(lock); v.OK {
		t.Fatal("resource tampering must be detected")
	}
}
