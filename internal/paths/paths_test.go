package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectHarnessPathsStayInsideAgctlState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	p, err := Detect()
	if err != nil {
		t.Fatal(err)
	}

	wantStateRoot := filepath.Join(home, ".gemini", "config", "agctl", "state")
	if p.StateRoot != wantStateRoot {
		t.Fatalf("StateRoot = %q, want %q", p.StateRoot, wantStateRoot)
	}
	if p.HarnessDB != filepath.Join(wantStateRoot, "harness", "state.db") {
		t.Fatalf("HarnessDB = %q", p.HarnessDB)
	}
	if p.ArtifactRoot != filepath.Join(wantStateRoot, "harness", "artifacts") {
		t.Fatalf("ArtifactRoot = %q", p.ArtifactRoot)
	}
	if p.HarnessBackupRoot != filepath.Join(home, ".gemini", "config", "agctl", "backups", "harness") {
		t.Fatalf("HarnessBackupRoot = %q", p.HarnessBackupRoot)
	}
	if p.HarnessDB == p.TaskConfig || filepath.Dir(p.HarnessDB) == p.TasksRoot {
		t.Fatal("Harness durable state must not alias legacy task persistence")
	}
}

func TestEnsureCreatesHarnessDirectoriesButNotDatabaseFile(t *testing.T) {
	root := t.TempDir()
	p := Paths{
		StateRoot:         filepath.Join(root, "state"),
		BackupsRoot:       filepath.Join(root, "backups"),
		HarnessDB:         filepath.Join(root, "state", "harness", "state.db"),
		ArtifactRoot:      filepath.Join(root, "state", "harness", "artifacts"),
		HarnessBackupRoot: filepath.Join(root, "backups", "harness"),
	}

	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{filepath.Dir(p.HarnessDB), p.ArtifactRoot, p.HarnessBackupRoot} {
		st, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if !st.IsDir() {
			t.Fatalf("%s is not a directory", dir)
		}
	}
	if _, err := os.Stat(p.HarnessDB); !os.IsNotExist(err) {
		t.Fatalf("Ensure must not create DB file; stat err=%v", err)
	}
}

func TestDetectPlatformsWithEnvOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("AGCTL_PLATFORM", "cursor")

	p, err := Detect()
	if err != nil {
		t.Fatal(err)
	}
	if p.ActivePlatform != PlatformCursor {
		t.Fatalf("ActivePlatform = %q, want %q", p.ActivePlatform, PlatformCursor)
	}

	infos := p.GetPlatformInfos()
	if len(infos) == 0 {
		t.Fatal("Platform infos must not be empty")
	}

	foundActive := false
	for _, info := range infos {
		if info.Platform == PlatformCursor && info.Active {
			foundActive = true
		}
	}
	if !foundActive {
		t.Fatalf("Expected Cursor to be active in PlatformInfos")
	}
}

func TestWorkspaceRuleFilesDiscovery(t *testing.T) {
	ws := t.TempDir()
	ruleFiles := WorkspaceRuleFiles(ws)
	if len(ruleFiles) < 4 {
		t.Fatalf("Expected at least 4 rule files, got %d", len(ruleFiles))
	}
}

