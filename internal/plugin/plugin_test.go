package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
)

func TestInstallDirPreservesBundleAndCreatesLock(t *testing.T) {
	root := t.TempDir()
	p := paths.Paths{Home: root, ConfigRoot: filepath.Join(root, "config"), GlobalPluginsRoot: filepath.Join(root, "config", "plugins"), AppRoot: filepath.Join(root, "app")}
	p.LocksRoot = filepath.Join(p.AppRoot, "locks")
	p.BackupsRoot = filepath.Join(p.AppRoot, "backups")
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(src, "skills", "x"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := jsonx.WriteAtomic(filepath.Join(src, "plugin.json"), model.PluginManifest{Name: "demo"}, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "skills", "x", "SKILL.md"), []byte("---\nname: x\ndescription: x\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	it, err := InstallDir(p, "", src)
	if err != nil {
		t.Fatal(err)
	}
	if !it.Valid || it.Name != "demo" {
		t.Fatalf("item=%+v", it)
	}
	locks, _ := os.ReadDir(p.LocksRoot)
	if len(locks) == 0 {
		t.Fatal("lock not created")
	}
}
