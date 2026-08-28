package account

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/antigravityide"
	"github.com/homiakus/agctl/internal/cockpit"
	"github.com/homiakus/agctl/internal/remote/model"
)

type switchStore struct {
	instance      model.InstanceMirror
	sessions      map[model.RemoteSessionID]model.RemoteSession
	conversations map[model.ConversationID]model.Conversation
}

func (s *switchStore) GetInstance(context.Context, model.InstanceID) (model.InstanceMirror, error) { return s.instance, nil }
func (s *switchStore) UpsertInstance(_ context.Context, instance model.InstanceMirror) error { s.instance = instance; return nil }
func (s *switchStore) ListSessionsByInstance(_ context.Context, _ model.InstanceID, _ bool) ([]model.RemoteSession, error) {
	out := make([]model.RemoteSession, 0, len(s.sessions)); for _, session := range s.sessions { out = append(out, session) }; return out, nil
}
func (s *switchStore) GetSession(_ context.Context, id model.RemoteSessionID) (model.RemoteSession, error) { return s.sessions[id], nil }
func (s *switchStore) GetConversation(_ context.Context, id model.ConversationID) (model.Conversation, error) { return s.conversations[id], nil }
func (s *switchStore) UpsertConversation(_ context.Context, conversation model.Conversation) error { s.conversations[conversation.ID] = conversation; return nil }
func (s *switchStore) UpdateSessionStates(_ context.Context, id model.RemoteSessionID, desired model.SessionDesiredState, observed model.SessionObservedState, updated time.Time) error {
	session := s.sessions[id]; session.DesiredState = desired; session.ObservedState = observed; session.UpdatedAt = updated; s.sessions[id] = session; return nil
}
func (s *switchStore) UpdateSessionAccount(_ context.Context, id model.RemoteSessionID, account string, updated time.Time) error {
	session := s.sessions[id]; session.CockpitAccountID = account; session.UpdatedAt = updated; s.sessions[id] = session; return nil
}

type switchCockpit struct {
	accounts  []cockpit.Account
	instance  cockpit.Instance
	binds     int
}
func (s *switchCockpit) ListAccounts(context.Context) ([]cockpit.Account, error) { return append([]cockpit.Account(nil), s.accounts...), nil }
func (s *switchCockpit) ListInstances(context.Context) ([]cockpit.Instance, error) { return []cockpit.Instance{s.instance}, nil }
func (s *switchCockpit) BindAccount(_ context.Context, id, account string) (cockpit.Instance, error) {
	s.binds++; s.instance.ID = id; s.instance.BindAccountID = &account; s.instance.Running = false; s.instance.LastPID = nil; return s.instance, nil
}

type switchRuntime struct {
	bridge antigravityide.LocatedBridge
	stops  int
	starts int
}
func (s *switchRuntime) Start(_ context.Context, instance cockpit.Instance) (cockpit.Instance, antigravityide.LocatedBridge, error) {
	s.starts++; pid := uint32(88); instance.Running = true; instance.LastPID = &pid; return instance, s.bridge, nil
}
func (s *switchRuntime) Stop(_ context.Context, id string) (cockpit.Instance, error) { s.stops++; return cockpit.Instance{ID: id}, nil }
func (s *switchRuntime) Bridge(string) (antigravityide.LocatedBridge, bool) { return s.bridge, s.bridge.Client != nil }
func (s *switchRuntime) Forget(string) {}

type switchBridgeClient struct {
	workspace string
	created   int
	sent      []string
}
func (s *switchBridgeClient) Health(context.Context) (antigravityide.Health, error) { return antigravityide.Health{Status: "ok", InstanceID: "i1"}, nil }
func (s *switchBridgeClient) Capabilities(context.Context) (antigravityide.Capabilities, error) { return antigravityide.Capabilities{}, nil }
func (s *switchBridgeClient) Context(context.Context) (antigravityide.Context, error) { return antigravityide.Context{WorkspaceFolders: []string{s.workspace}}, nil }
func (s *switchBridgeClient) ListConversations(context.Context) ([]antigravityide.Conversation, error) { return nil, nil }
func (s *switchBridgeClient) CreateConversation(context.Context) (antigravityide.Conversation, error) { s.created++; return antigravityide.Conversation{ID: "p-new", Title: "continued"}, nil }
func (s *switchBridgeClient) FocusConversation(context.Context, string) error { return nil }
func (s *switchBridgeClient) SendMessage(_ context.Context, _ string, text string) error { s.sent = append(s.sent, text); return nil }
func (s *switchBridgeClient) OpenWorkspace(context.Context, string) (antigravityide.OpenWorkspaceResult, error) { return antigravityide.OpenWorkspaceResult{}, nil }

type switchCheckpointer struct{}
func (switchCheckpointer) Checkpoint(context.Context, model.RemoteSession, model.Conversation, antigravityide.Client) (Checkpoint, error) { return Checkpoint{Summary: "checkpoint-state"}, nil }

func testSwitchFixture(active bool) (*Service, *switchStore, *switchCockpit, *switchRuntime, *switchBridgeClient) {
	account := "a1"
	pid := uint32(77)
	store := &switchStore{instance: model.InstanceMirror{ID: "i1", AccountID: account, UserDataDir: "/profile", WorkingDir: "/repo", DesiredState: model.InstanceDesiredRunning, ObservedState: model.InstanceReady}, sessions: map[model.RemoteSessionID]model.RemoteSession{}, conversations: map[model.ConversationID]model.Conversation{}}
	if active {
		store.sessions["s1"] = model.RemoteSession{ID: "s1", CockpitInstanceID: "i1", CockpitAccountID: account, RepositoryID: "r1", WorkspaceID: "w1", WorkspacePath: "/repo", ConversationID: "c1", DesiredState: model.SessionDesiredReady, ObservedState: model.SessionRunning, IsolationMode: model.IsolationSharedRead}
		store.conversations["c1"] = model.Conversation{ID: "c1", ProviderConversationID: "p-old", InstanceID: "i1", WorkspaceID: "w1", State: model.ConversationActive, MirrorMode: model.MirrorStatus, CreatedAt: time.Now(), UpdatedAt: time.Now(), LastActivityAt: time.Now()}
	}
	cockpitClient := &switchCockpit{accounts: []cockpit.Account{{ID: "a1"}, {ID: "a2"}}, instance: cockpit.Instance{ID: "i1", UserDataDir: "/profile", WorkingDir: "/repo", BindAccountID: &account, LastPID: &pid, Running: true}}
	bridgeClient := &switchBridgeClient{workspace: "/repo"}
	runtimeManager := &switchRuntime{bridge: antigravityide.LocatedBridge{Registration: antigravityide.Registration{InstanceID: "i1", BootNonce: "boot-new", PID: 88}, Client: bridgeClient}}
	service, _ := New(store, cockpitClient)
	service.WithRuntime(runtimeManager)
	return service, store, cockpitClient, runtimeManager, bridgeClient
}

func TestColdSwitchWithoutSessionsStopsBindsAndRestarts(t *testing.T) {
	service, store, cockpitClient, runtimeManager, _ := testSwitchFixture(false)
	result, err := service.Switch(context.Background(), "i1", "a2", SwitchOptions{})
	if err != nil { t.Fatal(err) }
	if result.Plan.NoOp { t.Fatal("unexpected no-op") }
	if runtimeManager.stops != 1 || runtimeManager.starts != 1 || cockpitClient.binds != 1 { t.Fatalf("stops=%d starts=%d binds=%d", runtimeManager.stops, runtimeManager.starts, cockpitClient.binds) }
	if store.instance.AccountID != "a2" || store.instance.ObservedState != model.InstanceReady { t.Fatalf("mirror=%+v", store.instance) }
}

func TestActiveSessionBlocksSwitchWithoutHandoff(t *testing.T) {
	service, _, cockpitClient, runtimeManager, _ := testSwitchFixture(true)
	_, err := service.Switch(context.Background(), "i1", "a2", SwitchOptions{})
	if err == nil { t.Fatal("expected active-session guard") }
	if cockpitClient.binds != 0 || runtimeManager.stops != 0 { t.Fatalf("mutation before guard: binds=%d stops=%d", cockpitClient.binds, runtimeManager.stops) }
}

func TestHandoffSwitchCreatesContinuationAndUpdatesSessionAccount(t *testing.T) {
	service, store, cockpitClient, runtimeManager, bridgeClient := testSwitchFixture(true)
	result, err := service.Switch(context.Background(), "i1", "a2", SwitchOptions{AllowHandoff: true, Checkpointer: switchCheckpointer{}})
	if err != nil { t.Fatal(err) }
	if cockpitClient.binds != 1 || runtimeManager.stops != 1 || runtimeManager.starts != 1 { t.Fatalf("binds=%d stops=%d starts=%d", cockpitClient.binds, runtimeManager.stops, runtimeManager.starts) }
	if len(result.Continuations) != 1 || result.Continuations[0].NewProviderConversation != "p-new" { t.Fatalf("continuations=%+v", result.Continuations) }
	if store.sessions["s1"].CockpitAccountID != "a2" { t.Fatalf("session=%+v", store.sessions["s1"]) }
	if store.conversations["c1"].ProviderConversationID != "p-new" { t.Fatalf("conversation=%+v", store.conversations["c1"]) }
	if len(bridgeClient.sent) != 1 || !strings.Contains(bridgeClient.sent[0], "checkpoint-state") { t.Fatalf("sent=%v", bridgeClient.sent) }
}
