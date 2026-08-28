package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	harnesssqlite "github.com/homiakus/agctl/internal/harness/store/sqlite"
	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

func openRemoteTestStore(t *testing.T)(context.Context,*Store,func()){
	t.Helper();ctx:=context.Background();db,err:=harnesssqlite.Open(ctx,filepath.Join(t.TempDir(),"state.db"),harnesssqlite.Options{});if err!=nil{t.Fatal(err)};store,err:=New(db.SQLDB());if err!=nil{t.Fatal(err)};return ctx,store,func(){_ = db.Close()}
}

func TestTelegramCursorNeverRewinds(t *testing.T){ctx,s,done:=openRemoteTestStore(t);defer done();now:=time.Unix(1000,0).UTC();if err:=s.AdvanceTelegramCursor(ctx,model.TelegramCursor{BotKey:"primary",NextUpdateID:11,UpdatedAt:now});err!=nil{t.Fatal(err)};if err:=s.AdvanceTelegramCursor(ctx,model.TelegramCursor{BotKey:"primary",NextUpdateID:7,UpdatedAt:now.Add(time.Second)});err!=nil{t.Fatal(err)};got,err:=s.GetTelegramCursor(ctx,"primary");if err!=nil{t.Fatal(err)};if got.NextUpdateID!=11{t.Fatalf("cursor=%d",got.NextUpdateID)}}

func TestTelegramPairingSingleUseAndChatBound(t *testing.T){ctx,s,done:=openRemoteTestStore(t);defer done();now:=time.Unix(1000,0).UTC();pair:=model.TelegramPairing{TokenHash:"hash",Role:model.TelegramRoleOwner,IntendedChatID:99,CreatedAt:now,ExpiresAt:now.Add(time.Minute)};if err:=s.CreateTelegramPairing(ctx,pair);err!=nil{t.Fatal(err)};if _,err:=s.ConsumeTelegramPairing(ctx,"hash",42,98,now.Add(time.Second));err==nil{t.Fatal("expected chat mismatch")};p,err:=s.ConsumeTelegramPairing(ctx,"hash",42,99,now.Add(2*time.Second));if err!=nil{t.Fatal(err)};if p.UserID!=42||p.Role!=model.TelegramRoleOwner{t.Fatalf("principal=%#v",p)};if _,err:=s.ConsumeTelegramPairing(ctx,"hash",43,99,now.Add(3*time.Second));!errors.Is(err,remotestore.ErrConflict){t.Fatalf("reuse err=%v",err)}}

func TestCallbackReplayReservedOnce(t *testing.T){ctx,s,done:=openRemoteTestStore(t);defer done();now:=time.Unix(1000,0).UTC();first,err:=s.ReserveTelegramCallback(ctx,"cb-1",42,99,now);if err!=nil||!first{t.Fatalf("first=%v err=%v",first,err)};second,err:=s.ReserveTelegramCallback(ctx,"cb-1",42,99,now);if err!=nil||second{t.Fatalf("second=%v err=%v",second,err)}}

func TestRemoteCommandAdmissionIsIdempotent(t *testing.T){ctx,s,done:=openRemoteTestStore(t);defer done();seedRemoteGraph(t,ctx,s);now:=time.Unix(1000,0).UTC();cmd:=model.RemoteCommand{ID:"rcm_1700000000000_00000000000000000001",Source:"telegram",SourceMessageID:"update:77",SessionID:"rsi_1700000000000_00000000000000000001",Kind:"conversation.send",Payload:json.RawMessage(`{"text":"hi"}`),State:model.CommandPending,RequestedAt:now};got,created,err:=s.AdmitRemoteCommand(ctx,cmd);if err!=nil||!created{t.Fatalf("created=%v got=%#v err=%v",created,got,err)};duplicate:=cmd;duplicate.ID="rcm_1700000000000_00000000000000000002";got,created,err=s.AdmitRemoteCommand(ctx,duplicate);if err!=nil||created{t.Fatalf("duplicate created=%v err=%v",created,err)};if got.ID!=cmd.ID{t.Fatalf("dedupe returned %s",got.ID)}}

func TestRemoteEventSequenceAndSourceDedupe(t *testing.T){ctx,s,done:=openRemoteTestStore(t);defer done();seedRemoteGraph(t,ctx,s);now:=time.Unix(1000,0).UTC();first:=model.RemoteEvent{ID:"rev_1700000000000_00000000000000000001",SessionID:"rsi_1700000000000_00000000000000000001",Source:model.EventSourceIDE,Type:model.EventAgentMessage,SourceEventID:"ide-1",Payload:json.RawMessage(`{}`),Timestamp:now};got,created,err:=s.AppendRemoteEvent(ctx,first);if err!=nil||!created||got.Seq!=1{t.Fatalf("first=%#v created=%v err=%v",got,created,err)};dup:=first;dup.ID="rev_1700000000000_00000000000000000002";got,created,err=s.AppendRemoteEvent(ctx,dup);if err!=nil||created||got.Seq!=1{t.Fatalf("dup=%#v created=%v err=%v",got,created,err)};second:=first;second.ID="rev_1700000000000_00000000000000000003";second.SourceEventID="ide-2";got,created,err=s.AppendRemoteEvent(ctx,second);if err!=nil||!created||got.Seq!=2{t.Fatalf("second=%#v created=%v err=%v",got,created,err)}}

func seedRemoteGraph(t *testing.T,ctx context.Context,s *Store){t.Helper();now:=time.Unix(900,0).UTC();repo:=model.Repository{ID:"rep_1700000000000_00000000000000000001",Name:"repo",CanonicalPath:"/repo",GitRoot:"/repo",Enabled:true,CreatedAt:now,LastSeenAt:now};if err:=s.UpsertRepository(ctx,repo);err!=nil{t.Fatal(err)};inst:=model.InstanceMirror{ID:"i1",UserDataDir:"/profile",WorkingDir:"/repo",AccountID:"a1",DesiredState:model.InstanceDesiredRunning,ObservedState:model.InstanceReady,LastReconciledAt:now};if err:=s.UpsertInstance(ctx,inst);err!=nil{t.Fatal(err)};conv:=model.Conversation{ID:"rcv_1700000000000_00000000000000000001",ProviderConversationID:"p1",InstanceID:"i1",WorkspaceID:"rws_1700000000000_00000000000000000001",State:model.ConversationActive,MirrorMode:model.MirrorStatus,LastActivityAt:now,CreatedAt:now,UpdatedAt:now};if err:=s.UpsertConversation(ctx,conv);err!=nil{t.Fatal(err)};session:=model.RemoteSession{ID:"rsi_1700000000000_00000000000000000001",HostID:"host",CockpitInstanceID:"i1",CockpitAccountID:"a1",RepositoryID:repo.ID,WorkspaceID:"rws_1700000000000_00000000000000000001",WorkspacePath:"/repo",ConversationID:conv.ID,DesiredState:model.SessionDesiredReady,ObservedState:model.SessionReady,IsolationMode:model.IsolationExclusiveWrite,CreatedAt:now,UpdatedAt:now};if err:=s.CreateSession(ctx,session);err!=nil{t.Fatal(err)}}
