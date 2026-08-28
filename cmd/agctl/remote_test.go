package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	harnesssqlite "github.com/homiakus/agctl/internal/harness/store/sqlite"
	"github.com/homiakus/agctl/internal/paths"
)

func TestRemoteRepoCLILifecycle(t *testing.T) {
	repoDir := t.TempDir()
	if out, err := exec.Command("git", "-C", repoDir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}
	p := paths.Paths{HarnessDB: filepath.Join(t.TempDir(), "state.db")}
	if err := runRemote(p, []string{"repo", "add", "--name", "fixture", repoDir}); err != nil {
		t.Fatal(err)
	}
	if err := runRemote(p, []string{"repo", "list"}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	db, err := harnesssqlite.Open(ctx, p.HarnessDB, harnesssqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var id string
	if err := db.SQLDB().QueryRowContext(ctx, `SELECT id FROM remote_repositories`).Scan(&id); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if err := runRemote(p, []string{"repo", "disable", "--id", id}); err != nil {
		t.Fatal(err)
	}
	if err := runRemote(p, []string{"repo", "list"}); err != nil {
		t.Fatal(err)
	}
	if err := runRemote(p, []string{"repo", "list", "--all"}); err != nil {
		t.Fatal(err)
	}
	if err := runRemote(p, []string{"repo", "enable", "--id", id}); err != nil {
		t.Fatal(err)
	}
	if err := runRemote(p, []string{"status"}); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteRepoCLIRejectsMalformedRepositoryID(t *testing.T) {
	p := paths.Paths{HarnessDB: filepath.Join(t.TempDir(), "state.db")}
	if err := runRemote(p, []string{"repo", "disable", "--id", "not-an-id"}); err == nil {
		t.Fatal("expected malformed repository id to be rejected")
	}
}
