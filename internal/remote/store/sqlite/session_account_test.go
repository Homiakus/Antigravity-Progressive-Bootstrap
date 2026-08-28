package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	harnesssqlite "github.com/homiakus/agctl/internal/harness/store/sqlite"
	"github.com/homiakus/agctl/internal/remote/model"
)

func TestUpdateSessionAccountPersistsBinding(t *testing.T) {
	ctx := context.Background()
	db, err := harnesssqlite.Open(ctx, filepath.Join(t.TempDir(), "state.db"), harnesssqlite.Options{})
	if err != nil { t.Fatal(err) }
	defer db.Close()
	store, err := New(db.SQLDB())
	if err != nil { t.Fatal(err) }
	now := time.Unix(1700000000, 0).UTC()
	repo := model.Repository{ID: "rep_1700000000000_00000000000000000000", Name: "r", CanonicalPath: "/r", GitRoot: "/r", Enabled: true, CreatedAt: now, LastSeenAt: now}
	if err := store.UpsertRepository(ctx, repo); err != nil { t.Fatal(err) }
	instance := model.InstanceMirror{ID: "i1", UserDataDir: "/p", WorkingDir: "/r", AccountID: "a1", DesiredState: model.InstanceDesiredRunning, ObservedState: model.InstanceReady, LastReconciledAt: now}
	if err := store.UpsertInstance(ctx, instance); err != nil { t.Fatal(err) }
	conv := model.Conversation{ID: "rcv_1700000000000_00000000000000000000", ProviderConversationID: "p1", InstanceID: "i1", WorkspaceID: "rws_1700000000000_00000000000000000000", State: model.ConversationActive, MirrorMode: model.MirrorStatus, LastActivityAt: now, CreatedAt: now, UpdatedAt: now}
	if err := store.UpsertConversation(ctx, conv); err != nil { t.Fatal(err) }
	session := model.RemoteSession{ID: "rsi_1700000000000_00000000000000000000", HostID: "host", CockpitInstanceID: "i1", CockpitAccountID: "a1", RepositoryID: repo.ID, WorkspaceID: conv.WorkspaceID, WorkspacePath: "/r", ConversationID: conv.ID, DesiredState: model.SessionDesiredReady, ObservedState: model.SessionReady, IsolationMode: model.IsolationSharedRead, CreatedAt: now, UpdatedAt: now}
	if err := store.CreateSession(ctx, session); err != nil { t.Fatal(err) }
	if err := store.UpdateSessionAccount(ctx, session.ID, "a2", now.Add(time.Second)); err != nil { t.Fatal(err) }
	got, err := store.GetSession(ctx, session.ID)
	if err != nil { t.Fatal(err) }
	if got.CockpitAccountID != "a2" { t.Fatalf("account=%q", got.CockpitAccountID) }
}
