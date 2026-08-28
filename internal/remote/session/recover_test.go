package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/cockpit"
	"github.com/homiakus/agctl/internal/remote/model"
)

func (f *fakeStore) GetSession(context.Context, model.RemoteSessionID) (model.RemoteSession, error) { return f.session, nil }
func (f *fakeStore) GetConversation(context.Context, model.ConversationID) (model.Conversation, error) { return f.conversation, nil }
func (f *fakeStore) UpdateSessionStates(_ context.Context, _ model.RemoteSessionID, desired model.SessionDesiredState, observed model.SessionObservedState, at time.Time) error {
	f.session.DesiredState = desired
	f.session.ObservedState = observed
	f.session.UpdatedAt = at
	return nil
}

func TestRecoverRunningInstanceFailsClosedWithoutExplicitRestart(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	account := "a1"
	store := &fakeStore{
		conversation: model.Conversation{ID: "rcv_1700000000000_00000000000000000001", ProviderConversationID: "old"},
		session: model.RemoteSession{ID: "rsi_1700000000000_00000000000000000001", CockpitInstanceID: "i1", CockpitAccountID: account, ConversationID: "rcv_1700000000000_00000000000000000001", WorkspacePath: "/work/repo", DesiredState: model.SessionDesiredReady, ObservedState: model.SessionReady, UpdatedAt: now},
	}
	cockpitClient := &fakeCockpit{instance: cockpit.Instance{ID: "i1", UserDataDir: "/profile", WorkingDir: "/work/repo", BindAccountID: &account, Running: true}}
	service, err := New(Options{Store: store, Cockpit: cockpitClient, Locator: fakeLocator{bridge: &fakeBridge{repo: "/work/repo"}}, Secrets: &fakeSecrets{}, HostID: "host", ProfileRoot: "/profiles", BridgeRegistry: "/state/bridges", Now: func() time.Time { return now }})
	if err != nil { t.Fatal(err) }
	_, err = service.Recover(context.Background(), store.session.ID, false)
	if !errors.Is(err, ErrBridgeCredentialsUnavailable) { t.Fatalf("err=%v", err) }
	if cockpitClient.stopped != 0 || cockpitClient.started != 0 { t.Fatalf("unexpected restart stopped=%d started=%d", cockpitClient.stopped, cockpitClient.started) }
	if store.session.ObservedState != model.SessionNeedsAttention { t.Fatalf("state=%s", store.session.ObservedState) }
}

func TestRecoverRunningInstanceWithExplicitRestartReestablishesBridge(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	account := "a1"
	store := &fakeStore{
		conversation: model.Conversation{ID: "rcv_1700000000000_00000000000000000001", ProviderConversationID: "old"},
		session: model.RemoteSession{ID: "rsi_1700000000000_00000000000000000001", CockpitInstanceID: "i1", CockpitAccountID: account, ConversationID: "rcv_1700000000000_00000000000000000001", WorkspacePath: "/work/repo", DesiredState: model.SessionDesiredReady, ObservedState: model.SessionNeedsAttention, UpdatedAt: now},
	}
	pid := uint32(44)
	cockpitClient := &fakeCockpit{instance: cockpit.Instance{ID: "i1", UserDataDir: "/profile", WorkingDir: "/work/repo", BindAccountID: &account, Running: true, LastPID: &pid}}
	service, err := New(Options{Store: store, Cockpit: cockpitClient, Locator: fakeLocator{bridge: &fakeBridge{repo: "/work/repo"}}, Secrets: &fakeSecrets{}, HostID: "host", ProfileRoot: "/profiles", BridgeRegistry: "/state/bridges", Now: func() time.Time { return now }})
	if err != nil { t.Fatal(err) }
	recovered, err := service.Recover(context.Background(), store.session.ID, true)
	if err != nil { t.Fatal(err) }
	if cockpitClient.stopped != 1 || cockpitClient.started != 1 { t.Fatalf("restart stopped=%d started=%d", cockpitClient.stopped, cockpitClient.started) }
	if recovered.ObservedState != model.SessionReady { t.Fatalf("state=%s", recovered.ObservedState) }
	if _, ok := service.Bridge("i1"); !ok { t.Fatal("expected owned Bridge after recovery") }
}
