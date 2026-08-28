package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/antigravityide"
	"github.com/homiakus/agctl/internal/cockpit"
	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

var (
	ErrAccountMismatch   = errors.New("remote session: Cockpit account mismatch")
	ErrWorkspaceMismatch = errors.New("remote session: IDE workspace mismatch")
	ErrCapability        = errors.New("remote session: required IDE capability missing")
)

type InstanceStrategy string
const (
	InstanceDedicated InstanceStrategy = "DEDICATED"
	InstanceExisting  InstanceStrategy = "EXISTING"
)

type ConversationStrategy string
const (
	ConversationNew      ConversationStrategy = "NEW"
	ConversationExisting ConversationStrategy = "EXISTING"
)

type Spec struct {
	RepositoryID            model.RepositoryID
	AccountID               string
	InstanceStrategy        InstanceStrategy
	InstanceID              string
	ConversationStrategy    ConversationStrategy
	ProviderConversationID  string
	IsolationMode           model.IsolationMode
}

type Store interface {
	GetRepository(context.Context, model.RepositoryID) (model.Repository, error)
	UpsertInstance(context.Context, model.InstanceMirror) error
	UpsertConversation(context.Context, model.Conversation) error
	CreateSession(context.Context, model.RemoteSession) error
}

type SecretSource interface { NewSecret(int) (string,error) }
type CryptoSecretSource struct{}
func (CryptoSecretSource) NewSecret(bytes int) (string,error) { if bytes<=0 { return "",fmt.Errorf("secret byte count must be positive") }; b:=make([]byte,bytes); if _,err:=rand.Read(b);err!=nil{return "",err}; return base64.RawURLEncoding.EncodeToString(b),nil }

type Options struct {
	Store          Store
	Cockpit        cockpit.ManagedClient
	Locator        antigravityide.Locator
	IDs            model.IDGenerator
	Secrets        SecretSource
	HostID         model.HostID
	ProfileRoot    string
	BridgeRegistry string
	Now            func() time.Time
}

type Service struct { store Store; cockpit cockpit.ManagedClient; locator antigravityide.Locator; ids model.IDGenerator; secrets SecretSource; hostID model.HostID; profileRoot,bridgeRegistry string; now func()time.Time }

func New(opts Options) (*Service,error) {
	if opts.Store==nil || opts.Cockpit==nil || opts.Locator==nil { return nil,fmt.Errorf("remote session store, Cockpit client and Bridge locator are required") }
	if strings.TrimSpace(string(opts.HostID))=="" || strings.TrimSpace(opts.ProfileRoot)=="" || strings.TrimSpace(opts.BridgeRegistry)=="" { return nil,fmt.Errorf("remote session host id, profile root and bridge registry are required") }
	ids:=opts.IDs; if ids==nil { g:=model.NewIDGenerator(); ids=g }
	secrets:=opts.Secrets; if secrets==nil { secrets=CryptoSecretSource{} }
	now:=opts.Now; if now==nil { now=time.Now }
	return &Service{store:opts.Store,cockpit:opts.Cockpit,locator:opts.Locator,ids:ids,secrets:secrets,hostID:opts.HostID,profileRoot:opts.ProfileRoot,bridgeRegistry:opts.BridgeRegistry,now:now},nil
}

func (s *Service) Provision(ctx context.Context, spec Spec) (model.RemoteSession,error) {
	if spec.InstanceStrategy=="" { spec.InstanceStrategy=InstanceDedicated }; if spec.ConversationStrategy=="" { spec.ConversationStrategy=ConversationNew }; if spec.IsolationMode=="" { spec.IsolationMode=model.IsolationExclusiveWrite }
	if spec.InstanceStrategy!=InstanceDedicated && spec.InstanceStrategy!=InstanceExisting { return model.RemoteSession{},fmt.Errorf("invalid instance strategy %q",spec.InstanceStrategy) }
	if spec.ConversationStrategy!=ConversationNew && spec.ConversationStrategy!=ConversationExisting { return model.RemoteSession{},fmt.Errorf("invalid conversation strategy %q",spec.ConversationStrategy) }
	if !spec.IsolationMode.Valid() || spec.IsolationMode==model.IsolationWorktree { return model.RemoteSession{},fmt.Errorf("R9 supports shared-read or exclusive-write isolation only") }
	if strings.TrimSpace(spec.AccountID)=="" { return model.RemoteSession{},fmt.Errorf("account id is required") }
	repo,err:=s.store.GetRepository(ctx,spec.RepositoryID); if err!=nil{return model.RemoteSession{},err}; if !repo.Enabled{return model.RemoteSession{},fmt.Errorf("repository %s is disabled",repo.ID)}
	sessionID,err:=s.newID(model.IDRemoteSession); if err!=nil{return model.RemoteSession{},err}; workspaceID,err:=s.newID(model.IDWorkspace); if err!=nil{return model.RemoteSession{},err}; conversationID,err:=s.newID(model.IDConversation); if err!=nil{return model.RemoteSession{},err}

	instance,created,err:=s.resolveInstance(ctx,spec,repo,model.RemoteSessionID(sessionID)); if err!=nil{return model.RemoteSession{},err}
	started:=false
	rollback:=func(){ if created && started { stopCtx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel(); _,_=s.cockpit.StopInstance(stopCtx,instance.ID) } }
	now:=s.now().UTC(); mirror:=instanceMirror(instance,spec.AccountID,model.InstanceDesiredRunning,model.InstancePreparing,now); if err:=s.store.UpsertInstance(ctx,mirror);err!=nil{return model.RemoteSession{},err}
	bootNonce,err:=s.secrets.NewSecret(16); if err!=nil{return model.RemoteSession{},err}; bridgeToken,err:=s.secrets.NewSecret(32); if err!=nil{return model.RemoteSession{},err}
	instance,err=s.cockpit.StartManagedInstance(ctx,instance.ID,cockpit.LaunchContext{InstanceID:instance.ID,BootNonce:bootNonce,BridgeToken:bridgeToken,BridgeRegistry:s.bridgeRegistry}); if err!=nil{return model.RemoteSession{},err}; started=true
	mirror=instanceMirror(instance,spec.AccountID,model.InstanceDesiredRunning,model.InstanceProcessRunning,s.now().UTC()); if err:=s.store.UpsertInstance(ctx,mirror);err!=nil{rollback();return model.RemoteSession{},err}
	located,err:=s.locator.Wait(ctx,instance.ID,bridgeToken); if err!=nil{rollback();return model.RemoteSession{},err}
	health,err:=located.Client.Health(ctx); if err!=nil || (health.InstanceID!="" && health.InstanceID!=instance.ID) { rollback(); if err!=nil{return model.RemoteSession{},err};return model.RemoteSession{},fmt.Errorf("bridge instance identity mismatch") }
	caps,err:=located.Client.Capabilities(ctx); if err!=nil{rollback();return model.RemoteSession{},err}; if !caps.ConversationList || !caps.ConversationFocus || !caps.ConversationSend || (spec.ConversationStrategy==ConversationNew && !caps.ConversationCreate) { rollback();return model.RemoteSession{},fmt.Errorf("conversation control: %w",ErrCapability) }
	ideContext,err:=located.Client.Context(ctx); if err!=nil{rollback();return model.RemoteSession{},err}; if !containsPath(ideContext.WorkspaceFolders,repo.CanonicalPath) { rollback();return model.RemoteSession{},fmt.Errorf("expected %s in IDE workspace: %w",repo.CanonicalPath,ErrWorkspaceMismatch) }
	providerConversation,err:=s.resolveConversation(ctx,located.Client,spec); if err!=nil{rollback();return model.RemoteSession{},err}
	mirrorMode:=model.MirrorStatus; if caps.AgentEvents { mirrorMode=model.MirrorRemoteOnly; if caps.MessageHistory { mirrorMode=model.MirrorFull } }
	now=s.now().UTC(); conversation:=model.Conversation{ID:model.ConversationID(conversationID),ProviderConversationID:providerConversation.ID,InstanceID:model.InstanceID(instance.ID),WorkspaceID:model.WorkspaceID(workspaceID),Title:providerConversation.Title,State:model.ConversationActive,MirrorMode:mirrorMode,LastActivityAt:now,CreatedAt:now,UpdatedAt:now}; if err:=s.store.UpsertConversation(ctx,conversation);err!=nil{rollback();return model.RemoteSession{},err}
	session:=model.RemoteSession{ID:model.RemoteSessionID(sessionID),HostID:s.hostID,CockpitInstanceID:model.InstanceID(instance.ID),CockpitAccountID:spec.AccountID,RepositoryID:repo.ID,WorkspaceID:model.WorkspaceID(workspaceID),WorkspacePath:repo.CanonicalPath,ConversationID:conversation.ID,DesiredState:model.SessionDesiredReady,ObservedState:model.SessionReady,IsolationMode:spec.IsolationMode,CreatedAt:now,UpdatedAt:now}; if err:=s.store.CreateSession(ctx,session);err!=nil{rollback();return model.RemoteSession{},err}
	mirror.ObservedState=model.InstanceReady; mirror.BridgeID=located.Registration.BootNonce; mirror.LastReconciledAt=s.now().UTC(); _=s.store.UpsertInstance(ctx,mirror)
	return session,nil
}

func (s *Service) resolveInstance(ctx context.Context,spec Spec,repo model.Repository,sessionID model.RemoteSessionID)(cockpit.Instance,bool,error){
	if spec.InstanceStrategy==InstanceExisting { if strings.TrimSpace(spec.InstanceID)==""{return cockpit.Instance{},false,fmt.Errorf("existing instance id is required")}; items,err:=s.cockpit.ListInstances(ctx);if err!=nil{return cockpit.Instance{},false,err};for _,item:=range items{if item.ID==spec.InstanceID{if item.BindAccountID!=nil&&*item.BindAccountID!=spec.AccountID{return cockpit.Instance{},false,fmt.Errorf("instance %s bound to account %s, requested %s: %w",item.ID,*item.BindAccountID,spec.AccountID,ErrAccountMismatch)};return item,false,nil}};return cockpit.Instance{},false,fmt.Errorf("Cockpit instance %s not found",spec.InstanceID) }
	profile:=filepath.Join(s.profileRoot,string(sessionID)); instance,err:=s.cockpit.CreateInstance(ctx,cockpit.CreateInstanceSpec{Name:repo.Name+"-"+shortID(string(sessionID)),UserDataDir:profile,WorkingDir:repo.CanonicalPath,BindAccountID:spec.AccountID,InitMode:"copy"}); return instance,true,err
}

func (s *Service) resolveConversation(ctx context.Context,client antigravityide.Client,spec Spec)(antigravityide.Conversation,error){
	if spec.ConversationStrategy==ConversationNew { conversation,err:=client.CreateConversation(ctx);if err!=nil{return antigravityide.Conversation{},err};if err:=client.FocusConversation(ctx,conversation.ID);err!=nil{return antigravityide.Conversation{},err};return conversation,nil }
	if strings.TrimSpace(spec.ProviderConversationID)==""{return antigravityide.Conversation{},fmt.Errorf("existing provider conversation id is required")};items,err:=client.ListConversations(ctx);if err!=nil{return antigravityide.Conversation{},err};for _,item:=range items{if item.ID==spec.ProviderConversationID{if err:=client.FocusConversation(ctx,item.ID);err!=nil{return antigravityide.Conversation{},err};return item,nil}};return antigravityide.Conversation{},fmt.Errorf("Antigravity conversation %s not found",spec.ProviderConversationID)
}

func (s *Service) newID(kind model.IDKind)(string,error){return s.ids.New(kind)}
func shortID(value string)string{if len(value)<=8{return value};return value[len(value)-8:]}
func instanceMirror(instance cockpit.Instance,account string,desired model.InstanceDesiredState,observed model.InstanceObservedState,now time.Time)model.InstanceMirror{pid:=0;if instance.LastPID!=nil{pid=int(*instance.LastPID)};return model.InstanceMirror{ID:model.InstanceID(instance.ID),Name:instance.Name,UserDataDir:instance.UserDataDir,WorkingDir:instance.WorkingDir,AccountID:account,PID:pid,DesiredState:desired,ObservedState:observed,LastReconciledAt:now}}
func containsPath(items []string,want string)bool{for _,item:=range items{if samePath(item,want){return true}};return false}
func samePath(a,b string)bool{a=filepath.Clean(a);b=filepath.Clean(b);if runtime.GOOS=="windows"{return strings.EqualFold(a,b)};return a==b}

var _ = remotestore.ErrNotFound
