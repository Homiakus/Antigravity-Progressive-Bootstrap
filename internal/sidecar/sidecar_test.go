package sidecar

import (
	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/paths"
	"os"
	"path/filepath"
	"testing"
)

func TestScheduleLifecycle(t *testing.T) {
	root := t.TempDir()
	p := paths.Paths{ConfigRoot: filepath.Join(root, "config"), GlobalConfig: filepath.Join(root, "config", "config.json"), SidecarsRoot: filepath.Join(root, "config", "sidecars"), GlobalPluginsRoot: filepath.Join(root, "config", "plugins"), BackupsRoot: filepath.Join(root, "backups")}
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	it, err := CreateSchedule(p, "hourly", "0 * * * *", "agentapi", []string{"new-conversation", "hello"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if it.Builtin != "schedule" {
		t.Fatalf("%+v", it)
	}
	if err := Enable(p, "hourly", "project-1"); err != nil {
		t.Fatal(err)
	}
	xs, err := List(p)
	if err != nil || len(xs) != 1 || !xs[0].Enabled || xs[0].ProjectID != "project-1" {
		t.Fatalf("xs=%+v err=%v", xs, err)
	}
	if err := Disable(p, "hourly"); err != nil {
		t.Fatal(err)
	}
	xs, _ = List(p)
	if xs[0].Enabled {
		t.Fatal("still enabled")
	}
	if err := Remove(p, "hourly"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.SidecarsRoot, "hourly")); !os.IsNotExist(err) {
		t.Fatal("not removed")
	}
}
func TestInvalidSidecar(t *testing.T) {
	root := t.TempDir()
	p := paths.Paths{GlobalConfig: filepath.Join(root, "config.json"), SidecarsRoot: filepath.Join(root, "sidecars"), GlobalPluginsRoot: filepath.Join(root, "plugins"), BackupsRoot: filepath.Join(root, "backups")}
	_ = p.Ensure()
	dir := filepath.Join(root, "src")
	_ = os.MkdirAll(dir, 0755)
	_ = jsonx.WriteAtomic(filepath.Join(dir, "sidecar.json"), Config{Command: "x", Builtin: "schedule"}, "")
	if _, err := InstallDir(p, dir, "bad"); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestScheduleRejectsNonFiveFieldCron(t *testing.T) {
	root := t.TempDir()
	p := paths.Paths{ConfigRoot: filepath.Join(root, "config"), GlobalConfig: filepath.Join(root, "config", "config.json"), SidecarsRoot: filepath.Join(root, "config", "sidecars"), GlobalPluginsRoot: filepath.Join(root, "config", "plugins"), BackupsRoot: filepath.Join(root, "backups")}
	_ = p.Ensure()
	if _, err := CreateSchedule(p, "badcron", "* * * *", "agentapi", []string{"new-conversation", "hello"}, "bad"); err == nil {
		t.Fatal("expected 5-field cron validation error")
	}
}
