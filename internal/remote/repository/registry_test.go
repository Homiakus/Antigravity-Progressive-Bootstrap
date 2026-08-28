package repository

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

type memoryStore struct {
	byID map[model.RepositoryID]model.Repository
}

func newMemoryStore() *memoryStore {
	return &memoryStore{byID: map[model.RepositoryID]model.Repository{}}
}
func (s *memoryStore) UpsertRepository(_ context.Context, r model.Repository) error {
	s.byID[r.ID] = r
	return nil
}
func (s *memoryStore) GetRepository(_ context.Context, id model.RepositoryID) (model.Repository, error) {
	r, ok := s.byID[id]
	if !ok {
		return model.Repository{}, remotestore.ErrNotFound
	}
	return r, nil
}
func (s *memoryStore) GetRepositoryByPath(_ context.Context, p string) (model.Repository, error) {
	for _, r := range s.byID {
		if r.CanonicalPath == p {
			return r, nil
		}
	}
	return model.Repository{}, remotestore.ErrNotFound
}
func (s *memoryStore) ListRepositories(_ context.Context, enabled bool) ([]model.Repository, error) {
	var out []model.Repository
	for _, r := range s.byID {
		if !enabled || r.Enabled {
			out = append(out, r)
		}
	}
	return out, nil
}
func (s *memoryStore) SetRepositoryEnabled(_ context.Context, id model.RepositoryID, enabled bool) error {
	r, ok := s.byID[id]
	if !ok {
		return remotestore.ErrNotFound
	}
	r.Enabled = enabled
	s.byID[id] = r
	return nil
}

type fakeGit struct{ root string }

func (g fakeGit) Root(context.Context, string) (string, error) { return g.root, nil }
func (fakeGit) Remote(context.Context, string) (string, error) {
	return "git@example/repo.git", nil
}
func (fakeGit) Branch(context.Context, string) (string, error) { return "main", nil }

func TestRegistryRejectsRepositoryOutsideAllowedRoots(t *testing.T) {
	allowed := t.TempDir()
	outside := t.TempDir()
	registry, err := New(newMemoryStore(), Options{AllowedRoots: []string{allowed}, Git: fakeGit{root: outside}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Add(context.Background(), outside, "repo"); err == nil {
		t.Fatal("expected path outside allowed root to be rejected")
	}
}

func TestRegistryAddIsIdempotentByCanonicalPath(t *testing.T) {
	root := t.TempDir()
	now := time.Unix(1700000000, 0).UTC()
	generator := model.TimeSortableIDGenerator{Now: func() time.Time { return now }, Random: bytes.NewReader(make([]byte, 64))}
	store := newMemoryStore()
	registry, err := New(store, Options{AllowedRoots: []string{filepath.Dir(root)}, Git: fakeGit{root: root}, IDs: generator, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	first, err := registry.Add(context.Background(), root, "first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := registry.Add(context.Background(), root, "renamed")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("id changed on idempotent add: %s != %s", first.ID, second.ID)
	}
	if second.Name != "renamed" {
		t.Fatalf("name=%q want renamed", second.Name)
	}
}

func TestMemoryStoreMissingRepositoryUsesSentinel(t *testing.T) {
	_, err := newMemoryStore().GetRepository(context.Background(), "missing")
	if !errors.Is(err, remotestore.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}
