package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	harnesssqlite "github.com/homiakus/agctl/internal/harness/store/sqlite"
	"github.com/homiakus/agctl/internal/remote/model"
)

func TestRemoteSessionStoreRoundTrip(t *testing.T) {
	ctx := context.Background(); now := time.Unix(1700000000,0).UTC()
	db, err := harnesssqlite.Open(ctx, filepath.Join(t.TempDir(),"state.db"), harnesssqlite.Options{}); if err != nil { t.Fatal(err) }; defer db.Close()
	store, _ := New(db.SQLDB())
	repo := model.Repository{ID:"rep_1700000000000_00000000000000000000",Name:"repo",CanonicalPath:"/repo",GitRoot:"/repo",Enabled:true,CreatedAt:now,LastSeenAt:now}; if err := store.UpsertRepository(ctx,repo); err != nil { t.Fatal(err) }
	instance := model.InstanceMirror{ID:"cockpit-i1",Name:"i1",UserDataDir:"/profile",WorkingDir:"/repo",AccountID:"a1",PID:123,DesiredState:model.InstanceDesiredRunning,ObservedState:model.InstanceReady,LastReconciledAt:now}; if err := store.UpsertInstance(ctx,instance); err != nil { t.Fatal(err) }
	conversation := model.Conversation{ID:"rcv_1700000000000_11111111111111111111",ProviderConversationID:"provider-c1",InstanceID:instance.ID,WorkspaceID:"rws_1700000000000_22222222222222222222",Title:"chat",State:model.ConversationActive,MirrorMode:model.MirrorStatus,LastActivityAt:now,CreatedAt:now,UpdatedAt:now}; if err := store.UpsertConversation(ctx,conversation); err != nil { t.Fatal(err) }
	session := model.RemoteSession{ID:"rsi_1700000000000_33333333333333333333",HostID:"host",CockpitInstanceID:instance.ID,CockpitAccountID:"a1",RepositoryID:repo.ID,WorkspaceID:conversation.WorkspaceID,WorkspacePath:"/repo",ConversationID:conversation.ID,DesiredState:model.SessionDesiredReady,ObservedState:model.SessionReady,IsolationMode:model.IsolationExclusiveWrite,CreatedAt:now,UpdatedAt:now}; if err := store.CreateSession(ctx,session); err != nil { t.Fatal(err) }
	got, err := store.GetSession(ctx,session.ID); if err != nil { t.Fatal(err) }; if got.ConversationID != conversation.ID || got.RepositoryID != repo.ID { t.Fatalf("session=%#v",got) }
	if err := store.UpdateSessionStates(ctx,session.ID,model.SessionDesiredPaused,model.SessionPaused,now.Add(time.Second)); err != nil { t.Fatal(err) }
	got, _ = store.GetSession(ctx,session.ID); if got.ObservedState != model.SessionPaused { t.Fatalf("state=%s",got.ObservedState) }
	items, err := store.ListSessionsByInstance(ctx,instance.ID,false); if err != nil || len(items)!=1 { t.Fatalf("items=%#v err=%v",items,err) }
}
