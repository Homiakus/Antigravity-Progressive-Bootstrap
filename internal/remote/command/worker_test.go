package command

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/antigravityide"
	"github.com/homiakus/agctl/internal/remote/model"
)

type fakeStore struct {
	command model.RemoteCommand
	session model.RemoteSession
	conversation model.Conversation
	states []model.CommandState
}
func (f *fakeStore) ListPendingRemoteCommands(context.Context,int)([]model.RemoteCommand,error){return []model.RemoteCommand{f.command},nil}
func (f *fakeStore) UpdateRemoteCommandState(_ context.Context,_ model.RemoteCommandID,state model.CommandState,_ string,_ time.Time) error { f.states=append(f.states,state); return nil }
func (f *fakeStore) GetSession(context.Context,model.RemoteSessionID)(model.RemoteSession,error){return f.session,nil}
func (f *fakeStore) GetConversation(context.Context,model.ConversationID)(model.Conversation,error){return f.conversation,nil}

type fakeResolver struct{ bridge antigravityide.LocatedBridge; available bool }
func (f fakeResolver) Bridge(string)(antigravityide.LocatedBridge,bool){return f.bridge,f.available}

type fakeClient struct{ focused,sentID,text string }
func (f *fakeClient) Health(context.Context)(antigravityide.Health,error){return antigravityide.Health{Status:"ok"},nil}
func (f *fakeClient) Capabilities(context.Context)(antigravityide.Capabilities,error){return antigravityide.Capabilities{ProtocolVersion:1,ConversationList:true,ConversationFocus:true,ConversationSend:true},nil}
func (f *fakeClient) Context(context.Context)(antigravityide.Context,error){return antigravityide.Context{},nil}
func (f *fakeClient) ListConversations(context.Context)([]antigravityide.Conversation,error){return []antigravityide.Conversation{{ID:"provider-42"}},nil}
func (f *fakeClient) CreateConversation(context.Context)(antigravityide.Conversation,error){return antigravityide.Conversation{},nil}
func (f *fakeClient) FocusConversation(_ context.Context,id string) error { f.focused=id; return nil }
func (f *fakeClient) SendMessage(_ context.Context,id,text string) error { f.sentID=id; f.text=text; return nil }
func (f *fakeClient) OpenWorkspace(context.Context,string)(antigravityide.OpenWorkspaceResult,error){return antigravityide.OpenWorkspaceResult{},nil}

func commandFixture(now time.Time)(*fakeStore,json.RawMessage){
	payload,_:=json.Marshal(map[string]string{"text":"continue plan"})
	store:=&fakeStore{
		command:model.RemoteCommand{ID:"rcm_1700000000000_00000000000000000001",SessionID:"rsi_1700000000000_00000000000000000001",Kind:ConversationSend,Payload:payload,State:model.CommandPending,RequestedAt:now},
		session:model.RemoteSession{ID:"rsi_1700000000000_00000000000000000001",CockpitInstanceID:"instance-a",ConversationID:"rcv_1700000000000_00000000000000000001"},
		conversation:model.Conversation{ID:"rcv_1700000000000_00000000000000000001",ProviderConversationID:"provider-42"},
	}
	return store,payload
}

func TestWorkerSendsToExactProviderConversation(t *testing.T){
	now:=time.Unix(1,0).UTC();store,_:=commandFixture(now);client:=&fakeClient{}
	worker:=&Worker{Store:store,Bridges:fakeResolver{bridge:antigravityide.LocatedBridge{Client:client},available:true},Now:func()time.Time{return now}}
	processed,err:=worker.RunOnce(context.Background(),10)
	if err!=nil{t.Fatal(err)}
	if processed!=1{t.Fatalf("processed=%d",processed)}
	if client.focused!="provider-42"||client.sentID!="provider-42"||client.text!="continue plan"{t.Fatalf("focused=%q sent=%q text=%q",client.focused,client.sentID,client.text)}
	if len(store.states)!=2||store.states[0]!=model.CommandRunning||store.states[1]!=model.CommandSucceeded{t.Fatalf("states=%v",store.states)}
}

func TestWorkerLeavesCommandPendingWithoutOwnedBridge(t *testing.T){
	now:=time.Unix(1,0).UTC();store,_:=commandFixture(now)
	worker:=&Worker{Store:store,Bridges:fakeResolver{available:false},Now:func()time.Time{return now}}
	processed,err:=worker.RunOnce(context.Background(),10)
	if err!=nil{t.Fatal(err)}
	if processed!=0{t.Fatalf("processed=%d",processed)}
	if len(store.states)!=0{t.Fatalf("command should remain pending, states=%v",store.states)}
}
