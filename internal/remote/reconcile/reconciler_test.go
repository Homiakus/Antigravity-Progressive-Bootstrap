package reconcile

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/cockpit"
	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

type fakeStore struct {
	instances     []model.InstanceMirror
	sessions      map[model.InstanceID][]model.RemoteSession
	conversations map[model.ConversationID]model.Conversation
	updated       map[model.RemoteSessionID]model.SessionObservedState
	lastInstance  model.InstanceMirror
}

func (f *fakeStore) ListInstances(context.Context) ([]model.InstanceMirror, error) {
	return append([]model.InstanceMirror(nil), f.instances...), nil
}
func (f *fakeStore) UpsertInstance(_ context.Context, instance model.InstanceMirror) error {
	f.lastInstance = instance
	return nil
}
func (f *fakeStore) ListSessionsByInstance(_ context.Context, id model.InstanceID, _ bool) ([]model.RemoteSession, error) {
	return append([]model.RemoteSession(nil), f.sessions[id]...), nil
}
func (f *fakeStore) GetConversation(_ context.Context, id model.ConversationID) (model.Conversation, error) {
	value, ok := f.conversations[id]
	if !ok {
		return model.Conversation{}, remotestore.ErrNotFound
	}
	return value, nil
}
func (f *fakeStore) UpdateSessionStates(_ context.Context, id model.RemoteSessionID, _ model.SessionDesiredState, observed model.SessionObservedState, _ time.Time) error {
	if f.updated == nil {
		f.updated = map[model.RemoteSessionID]model.SessionObservedState{}
	}
	f.updated[id] = observed
	return nil
}

type fakeCockpit struct{ instances []cockpit.Instance }
func (f fakeCockpit) ListInstances(context.Context) ([]cockpit.Instance, error) { return append([]cockpit.Instance(nil), f.instances...), nil }

type fakeBridge struct {
	observation BridgeObservation
	err         error
}
func (f fakeBridge) Observe(context.Context, string) (BridgeObservation, error) { return f.observation, f.err }

func fixture() (*fakeStore, cockpit.Instance, BridgeObservation) {
	pid := uint32(42)
	account := "acct-a"
	instanceID := model.InstanceID("inst-a")
	conversationID := model.ConversationID("conv-a")
	store := &fakeStore{
		instances: []model.InstanceMirror{{ID: instanceID, WorkingDir: "/repo/a", AccountID: account, PID: 42, DesiredState: model.InstanceDesiredRunning, ObservedState: model.InstanceReady}},
		sessions: map[model.InstanceID][]model.RemoteSession{instanceID: {{ID: model.RemoteSessionID("sess-a"), CockpitInstanceID: instanceID, WorkspacePath: "/repo/a", ConversationID: conversationID, DesiredState: model.SessionDesiredReady, ObservedState: model.SessionRunning}}},
		conversations: map[model.ConversationID]model.Conversation{conversationID: {ID: conversationID, InstanceID: instanceID, ProviderConversationID: "provider-a", State: model.ConversationActive, MirrorMode: model.MirrorRemoteOnly}},
	}
	actual := cockpit.Instance{ID: string(instanceID), WorkingDir: "/repo/a", BindAccountID: &account, LastPID: &pid, Running: true}
	bridge := BridgeObservation{Authenticated: true, PID: 42, BootNonce: "boot-a", WorkspaceFolders: []string{"/repo/a"}, Conversations: map[string]ConversationObservation{"provider-a": {Busy: false}}}
	return store, actual, bridge
}

func TestReconcileHealthyInstanceBecomesReady(t *testing.T) {
	store, actual, bridge := fixture()
	r, err := New(Options{Store: store, Cockpit: fakeCockpit{instances: []cockpit.Instance{actual}}, Bridge: fakeBridge{observation: bridge}, Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil { t.Fatal(err) }
	report, err := r.ReconcileAll(context.Background())
	if err != nil { t.Fatal(err) }
	if len(report.Drifts) != 0 { t.Fatalf("unexpected drifts: %+v", report.Drifts) }
	if store.lastInstance.ObservedState != model.InstanceReady { t.Fatalf("instance state=%s", store.lastInstance.ObservedState) }
	if got := store.updated[model.RemoteSessionID("sess-a")]; got != model.SessionReady { t.Fatalf("session state=%s", got) }
}

func TestReconcileAccountMismatchNeedsAttention(t *testing.T) {
	store, actual, bridge := fixture()
	other := "acct-b"
	actual.BindAccountID = &other
	r, _ := New(Options{Store: store, Cockpit: fakeCockpit{instances: []cockpit.Instance{actual}}, Bridge: fakeBridge{observation: bridge}})
	report, err := r.ReconcileAll(context.Background())
	if err != nil { t.Fatal(err) }
	if len(report.Drifts) != 1 || report.Drifts[0].Kind != DriftAccountMismatch { t.Fatalf("drifts=%+v", report.Drifts) }
	if got := store.updated[model.RemoteSessionID("sess-a")]; got != model.SessionNeedsAttention { t.Fatalf("session state=%s", got) }
	if store.lastInstance.ObservedState != model.InstanceDegraded { t.Fatalf("instance state=%s", store.lastInstance.ObservedState) }
}

func TestReconcileMissingBridgeDegradesWithoutRepair(t *testing.T) {
	store, actual, _ := fixture()
	r, _ := New(Options{Store: store, Cockpit: fakeCockpit{instances: []cockpit.Instance{actual}}, Bridge: fakeBridge{err: errors.New("bridge auth unavailable")}})
	report, err := r.ReconcileAll(context.Background())
	if err != nil { t.Fatal(err) }
	if len(report.Drifts) != 1 || report.Drifts[0].Kind != DriftBridgeUnavailable { t.Fatalf("drifts=%+v", report.Drifts) }
	if got := store.updated[model.RemoteSessionID("sess-a")]; got != model.SessionDegraded { t.Fatalf("session state=%s", got) }
}

func TestReconcileMissingConversationDoesNotCreateReplacement(t *testing.T) {
	store, actual, bridge := fixture()
	bridge.Conversations = map[string]ConversationObservation{}
	r, _ := New(Options{Store: store, Cockpit: fakeCockpit{instances: []cockpit.Instance{actual}}, Bridge: fakeBridge{observation: bridge}})
	report, err := r.ReconcileAll(context.Background())
	if err != nil { t.Fatal(err) }
	if len(report.Drifts) != 1 || report.Drifts[0].Kind != DriftConversationMissing { t.Fatalf("drifts=%+v", report.Drifts) }
	if got := store.updated[model.RemoteSessionID("sess-a")]; got != model.SessionNeedsAttention { t.Fatalf("session state=%s", got) }
}
