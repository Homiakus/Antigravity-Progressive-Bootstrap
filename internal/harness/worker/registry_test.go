package worker

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

type fixedIDs struct{}

func (fixedIDs) New(kind harnessmodel.IDKind) (string, error) {
	return string(kind) + "_0000000000001_00000000000000000001", nil
}

func newRegistryFixture(t *testing.T, now *time.Time) (*Registry, *sqlitestore.DB) {
	t.Helper()
	db, err := sqlitestore.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"), sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	registry, err := NewRegistry(db, Options{IDs: fixedIDs{}, Now: func() time.Time { return *now }})
	if err != nil {
		t.Fatal(err)
	}
	return registry, db
}

func TestRegisterDefaultsAndGeneratedIdentity(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1000, 0).UTC()
	registry, _ := newRegistryFixture(t, &now)
	worker, err := registry.Register(ctx, harnessmodel.Worker{Name: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if worker.ID == "" || worker.State != harnessmodel.WorkerActive || worker.Trust != harnessmodel.WorkerTrustedLocal {
		t.Fatalf("unexpected worker defaults: %+v", worker)
	}
	if !worker.CreatedAt.Equal(now) || !worker.LastSeenAt.Equal(now) {
		t.Fatalf("unexpected worker timestamps: %+v", worker)
	}
	loaded, err := registry.Get(ctx, worker.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != worker.ID || !loaded.CreatedAt.Equal(worker.CreatedAt) || !loaded.LastSeenAt.Equal(worker.LastSeenAt) {
		t.Fatalf("durable worker differs from returned worker: returned=%+v loaded=%+v", worker, loaded)
	}
}

func TestReregisterPreservesCreatedAtAndRefreshesCapabilities(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(2000, 0).UTC()
	registry, _ := newRegistryFixture(t, &now)
	first, err := registry.Register(ctx, harnessmodel.Worker{ID: "worker-stable", Name: "first", Capabilities: []string{"go.build"}})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(10 * time.Minute)
	second, err := registry.Register(ctx, harnessmodel.Worker{ID: first.ID, Name: "second", Trust: harnessmodel.WorkerTrustedRemote, Capabilities: []string{"go.build", "docker"}})
	if err != nil {
		t.Fatal(err)
	}
	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("worker identity creation time changed: first=%s second=%s", first.CreatedAt, second.CreatedAt)
	}
	if !second.LastSeenAt.Equal(now) || second.Name != "second" || second.Trust != harnessmodel.WorkerTrustedRemote || len(second.Capabilities) != 2 {
		t.Fatalf("worker refresh failed: %+v", second)
	}
	loaded, err := registry.Get(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.CreatedAt.Equal(first.CreatedAt) || !loaded.LastSeenAt.Equal(now) || loaded.Name != "second" || len(loaded.Capabilities) != 2 {
		t.Fatalf("durable re-registration mismatch: %+v", loaded)
	}
}

func TestHeartbeatUpdatesLivenessOnly(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(3000, 0).UTC()
	registry, _ := newRegistryFixture(t, &now)
	worker, err := registry.Register(ctx, harnessmodel.Worker{ID: "worker-heartbeat", Name: "heartbeat", State: harnessmodel.WorkerDraining, Trust: harnessmodel.WorkerUntrustedRemote})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Second)
	if err := registry.Heartbeat(ctx, worker.ID); err != nil {
		t.Fatal(err)
	}
	loaded, err := registry.Get(ctx, worker.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.LastSeenAt.Equal(now) || !loaded.CreatedAt.Equal(worker.CreatedAt) || loaded.State != harnessmodel.WorkerDraining || loaded.Trust != harnessmodel.WorkerUntrustedRemote {
		t.Fatalf("heartbeat changed worker identity/configuration: before=%+v after=%+v", worker, loaded)
	}
}

func TestRegisterRejectsInvalidStateAndTrust(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(4000, 0).UTC()
	registry, _ := newRegistryFixture(t, &now)
	if _, err := registry.Register(ctx, harnessmodel.Worker{ID: "bad-state", State: "BOGUS"}); err == nil {
		t.Fatal("invalid worker state accepted")
	}
	if _, err := registry.Register(ctx, harnessmodel.Worker{ID: "bad-trust", Trust: "BOGUS"}); err == nil {
		t.Fatal("invalid worker trust accepted")
	}
}
