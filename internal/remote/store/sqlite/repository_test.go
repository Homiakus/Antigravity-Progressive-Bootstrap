package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	harnesssqlite "github.com/homiakus/agctl/internal/harness/store/sqlite"
	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

func TestRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := harnesssqlite.Open(ctx, filepath.Join(t.TempDir(), "state.db"), harnesssqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := New(db.SQLDB())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0).UTC()
	repository := model.Repository{
		ID:            "rep_1700000000000_00000000000000000000",
		Name:          "repo",
		CanonicalPath: "/work/repo",
		GitRoot:       "/work/repo",
		GitRemote:     "git@example/repo.git",
		DefaultBranch: "main",
		Enabled:       true,
		CreatedAt:     now,
		LastSeenAt:    now,
	}
	if err := store.UpsertRepository(ctx, repository); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetRepository(ctx, repository.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != repository.ID || got.CanonicalPath != repository.CanonicalPath || !got.Enabled {
		t.Fatalf("repository round trip mismatch: %#v", got)
	}
	byPath, err := store.GetRepositoryByPath(ctx, repository.CanonicalPath)
	if err != nil {
		t.Fatal(err)
	}
	if byPath.ID != repository.ID {
		t.Fatalf("path lookup id=%s want=%s", byPath.ID, repository.ID)
	}
	if err := store.SetRepositoryEnabled(ctx, repository.ID, false); err != nil {
		t.Fatal(err)
	}
	enabled, err := store.ListRepositories(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(enabled) != 0 {
		t.Fatalf("enabled repositories=%d want=0", len(enabled))
	}
}

func TestRepositoryCanonicalPathConflict(t *testing.T) {
	ctx := context.Background()
	db, err := harnesssqlite.Open(ctx, filepath.Join(t.TempDir(), "state.db"), harnesssqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := New(db.SQLDB())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0).UTC()
	first := model.Repository{ID: "rep_1700000000000_00000000000000000000", Name: "a", CanonicalPath: "/work/repo", GitRoot: "/work/repo", Enabled: true, CreatedAt: now, LastSeenAt: now}
	second := first
	second.ID = "rep_1700000000000_11111111111111111111"
	if err := store.UpsertRepository(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertRepository(ctx, second); !errors.Is(err, remotestore.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}
