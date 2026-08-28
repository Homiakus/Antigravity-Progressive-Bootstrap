package daemon

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/antigravityide"
	"github.com/homiakus/agctl/internal/remote/mirror"
	"github.com/homiakus/agctl/internal/remote/model"
)

type fakePoller struct{}
func(fakePoller)Run(ctx context.Context)error{<-ctx.Done();return ctx.Err()}
type fakeBatch struct{calls int}
func(f *fakeBatch)RunOnce(context.Context,int)(int,error){f.calls++;return 1,nil}
type fakeIngestor struct{calls int}
func(f *fakeIngestor)PollSession(context.Context,mirror.EventClient,model.RemoteSessionID)(int,error){f.calls++;return 1,nil}
type fakeStore struct{instance model.InstanceMirror;session model.RemoteSession}
func(f fakeStore)ListInstances(context.Context)([]model.InstanceMirror,error){return []model.InstanceMirror{f.instance},nil}
func(f fakeStore)ListSessionsByInstance(context.Context,model.InstanceID,bool)([]model.RemoteSession,error){return []model.RemoteSession{f.session},nil}
type fakeResolver struct{bridge antigravityide.LocatedBridge}
func(f fakeResolver)Bridge(string)(antigravityide.LocatedBridge,bool){return f.bridge,true}
type fakeClient struct{}
func(fakeClient)Health(context.Context)(antigravityide.Health,error){return antigravityide.Health{Status:"ok"},nil}
func(fakeClient)Capabilities(context.Context)(antigravityide.Capabilities,error){return antigravityide.Capabilities{ProtocolVersion:1,AgentEvents:true},nil}
func(fakeClient)Context(context.Context)(antigravityide.Context,error){return antigravityide.Context{},nil}
func(fakeClient)ListConversations(context.Context)([]antigravityide.Conversation,error){return nil,nil}
func(fakeClient)CreateConversation(context.Context)(antigravityide.Conversation,error){return antigravityide.Conversation{},nil}
func(fakeClient)FocusConversation(context.Context,string)error{return nil}
func(fakeClient)SendMessage(context.Context,string,string)error{return nil}
func(fakeClient)OpenWorkspace(context.Context,string)(antigravityide.OpenWorkspaceResult,error){return antigravityide.OpenWorkspaceResult{},nil}
func(fakeClient)Events(context.Context,string,uint64)([]antigravityide.BridgeEvent,error){return []antigravityide.BridgeEvent{{Seq:1,Type:"agent_delta",SourceEventID:"e",StreamKey:"s",Timestamp:time.Unix(1,0).UTC(),Payload:json.RawMessage(`{"conversationId":"p","stepIndex":1,"text":"x","final":false}`)}},nil}

func TestCycleRunsCommandsMirrorAndDelivery(t *testing.T){
	commands:=&fakeBatch{};delivery:=&fakeBatch{};ingestor:=&fakeIngestor{}
	instance:=model.InstanceMirror{ID:"instance-a"}
	session:=model.RemoteSession{ID:"rsi_1700000000000_00000000000000000001",CockpitInstanceID:"instance-a",ObservedState:model.SessionReady,DesiredState:model.SessionDesiredReady}
	s,err:=New(Options{Telegram:fakePoller{},Commands:commands,Delivery:delivery,Ingestor:ingestor,Store:fakeStore{instance:instance,session:session},Bridges:fakeResolver{bridge:antigravityide.LocatedBridge{Client:fakeClient{}}}})
	if err!=nil{t.Fatal(err)}
	if err:=s.cycle(context.Background());err!=nil{t.Fatal(err)}
	if commands.calls!=1||delivery.calls!=1||ingestor.calls!=1{t.Fatalf("calls commands=%d delivery=%d ingest=%d",commands.calls,delivery.calls,ingestor.calls)}
}
