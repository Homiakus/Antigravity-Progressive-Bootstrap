package session

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/antigravityide"
	"github.com/homiakus/agctl/internal/cockpit"
	"github.com/homiakus/agctl/internal/remote/model"
)

type fakeStore struct{ repo model.Repository; instance model.InstanceMirror; conversation model.Conversation; session model.RemoteSession }
func (f *fakeStore)GetRepository(context.Context,model.RepositoryID)(model.Repository,error){return f.repo,nil}
func (f *fakeStore)UpsertInstance(_ context.Context,v model.InstanceMirror)error{f.instance=v;return nil}
func (f *fakeStore)UpsertConversation(_ context.Context,v model.Conversation)error{f.conversation=v;return nil}
func (f *fakeStore)CreateSession(_ context.Context,v model.RemoteSession)error{f.session=v;return nil}

type fakeCockpit struct{ instance cockpit.Instance; launch cockpit.LaunchContext; created int; started int; stopped int }
func (f *fakeCockpit)Protocol(context.Context)(cockpit.ProtocolInfo,error){return cockpit.ProtocolInfo{ProtocolVersion:1},nil}
func (f *fakeCockpit)ListAccounts(context.Context)([]cockpit.Account,error){return nil,nil}
func (f *fakeCockpit)ListInstances(context.Context)([]cockpit.Instance,error){return []cockpit.Instance{f.instance},nil}
func (f *fakeCockpit)CreateInstance(_ context.Context,s cockpit.CreateInstanceSpec)(cockpit.Instance,error){f.created++; account:=s.BindAccountID; f.instance=cockpit.Instance{ID:"i1",Name:s.Name,UserDataDir:s.UserDataDir,WorkingDir:s.WorkingDir,BindAccountID:&account,Initialized:true};return f.instance,nil}
func (f *fakeCockpit)UpdateInstance(context.Context,string,cockpit.InstancePatch)(cockpit.Instance,error){return f.instance,nil}
func (f *fakeCockpit)StartInstance(context.Context,string)(cockpit.Instance,error){return f.instance,nil}
func (f *fakeCockpit)StartManagedInstance(_ context.Context,_ string,l cockpit.LaunchContext)(cockpit.Instance,error){f.started++;f.launch=l;pid:=uint32(123);f.instance.Running=true;f.instance.LastPID=&pid;return f.instance,nil}
func (f *fakeCockpit)StopInstance(context.Context,string)(cockpit.Instance,error){f.stopped++;f.instance.Running=false;return f.instance,nil}
func (f *fakeCockpit)FocusInstance(context.Context,string)error{return nil}
func (f *fakeCockpit)BindAccount(context.Context,string,string)(cockpit.Instance,error){return f.instance,nil}

type fakeBridge struct{ repo string; focused string }
func (f *fakeBridge)Health(context.Context)(antigravityide.Health,error){return antigravityide.Health{Status:"ok",InstanceID:"i1",BootNonce:"boot"},nil}
func (f *fakeBridge)Capabilities(context.Context)(antigravityide.Capabilities,error){return antigravityide.Capabilities{ProtocolVersion:1,ConversationList:true,ConversationCreate:true,ConversationFocus:true,ConversationSend:true},nil}
func (f *fakeBridge)Context(context.Context)(antigravityide.Context,error){return antigravityide.Context{InstanceID:"i1",WorkspaceFolders:[]string{f.repo}},nil}
func (f *fakeBridge)ListConversations(context.Context)([]antigravityide.Conversation,error){return []antigravityide.Conversation{{ID:"old",Title:"old"}},nil}
func (f *fakeBridge)CreateConversation(context.Context)(antigravityide.Conversation,error){return antigravityide.Conversation{ID:"provider-new",Title:"new"},nil}
func (f *fakeBridge)FocusConversation(_ context.Context,id string)error{f.focused=id;return nil}
func (f *fakeBridge)SendMessage(context.Context,string,string)error{return nil}
func (f *fakeBridge)OpenWorkspace(context.Context,string)(antigravityide.OpenWorkspaceResult,error){return antigravityide.OpenWorkspaceResult{},fmt.Errorf("unexpected workspace open")}

type fakeLocator struct{ bridge *fakeBridge }
func (f fakeLocator)Wait(_ context.Context,instanceID,token string)(antigravityide.LocatedBridge,error){if instanceID!="i1"||token==""{return antigravityide.LocatedBridge{},fmt.Errorf("bad lookup")};return antigravityide.LocatedBridge{Registration:antigravityide.Registration{ProtocolVersion:1,InstanceID:"i1",BootNonce:"boot",PID:123,Port:1,StartedAt:time.Now()},Client:f.bridge},nil}

type fakeSecrets struct{ n int }
func (f *fakeSecrets)NewSecret(int)(string,error){f.n++;return fmt.Sprintf("secret-%d",f.n),nil}

func TestProvisionDedicatedSessionEndToEnd(t *testing.T){
	now:=time.Unix(1700000000,0).UTC();repo:=model.Repository{ID:"rep_1700000000000_00000000000000000000",Name:"repo",CanonicalPath:"/work/repo",GitRoot:"/work/repo",Enabled:true,CreatedAt:now,LastSeenAt:now};store:=&fakeStore{repo:repo};cockpitClient:=&fakeCockpit{};bridge:=&fakeBridge{repo:repo.CanonicalPath};ids:=model.TimeSortableIDGenerator{Now:func()time.Time{return now},Random:bytes.NewReader(make([]byte,64))};secrets:=&fakeSecrets{}
	service,err:=New(Options{Store:store,Cockpit:cockpitClient,Locator:fakeLocator{bridge:bridge},IDs:ids,Secrets:secrets,HostID:"host",ProfileRoot:"/profiles",BridgeRegistry:"/state/bridges",Now:func()time.Time{return now}});if err!=nil{t.Fatal(err)}
	session,err:=service.Provision(context.Background(),Spec{RepositoryID:repo.ID,AccountID:"a1",InstanceStrategy:InstanceDedicated,ConversationStrategy:ConversationNew,IsolationMode:model.IsolationExclusiveWrite});if err!=nil{t.Fatal(err)}
	if session.ObservedState!=model.SessionReady||cockpitClient.created!=1||cockpitClient.started!=1{t.Fatalf("session=%#v cockpit=%#v",session,cockpitClient)}
	if cockpitClient.launch.BridgeToken!="secret-2"||cockpitClient.launch.BootNonce!="secret-1"{t.Fatalf("launch=%#v",cockpitClient.launch)}
	if store.conversation.ProviderConversationID!="provider-new"||bridge.focused!="provider-new"{t.Fatalf("conversation=%#v focused=%s",store.conversation,bridge.focused)}
}

func TestProvisionExistingInstanceFailsClosedOnAccountMismatch(t *testing.T){
	now:=time.Unix(1700000000,0).UTC();repo:=model.Repository{ID:"rep_1700000000000_00000000000000000000",Name:"repo",CanonicalPath:"/work/repo",GitRoot:"/work/repo",Enabled:true,CreatedAt:now,LastSeenAt:now};wrong:="other";store:=&fakeStore{repo:repo};cockpitClient:=&fakeCockpit{instance:cockpit.Instance{ID:"i1",UserDataDir:"/profile",WorkingDir:repo.CanonicalPath,BindAccountID:&wrong}};service,_:=New(Options{Store:store,Cockpit:cockpitClient,Locator:fakeLocator{bridge:&fakeBridge{repo:repo.CanonicalPath}},HostID:"host",ProfileRoot:"/profiles",BridgeRegistry:"/state/bridges"})
	_,err:=service.Provision(context.Background(),Spec{RepositoryID:repo.ID,AccountID:"wanted",InstanceStrategy:InstanceExisting,InstanceID:"i1",ConversationStrategy:ConversationNew,IsolationMode:model.IsolationExclusiveWrite});if err==nil{t.Fatal("expected account mismatch")}
}
