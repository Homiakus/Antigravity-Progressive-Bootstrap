package account

import (
	"context"
	"errors"
	"testing"

	"github.com/homiakus/agctl/internal/cockpit"
	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

type fakeStore struct {
	instance model.InstanceMirror
	sessions []model.RemoteSession
	err      error
}
func (f fakeStore) GetInstance(context.Context, model.InstanceID) (model.InstanceMirror, error) {
	if f.err != nil { return model.InstanceMirror{}, f.err }
	return f.instance, nil
}
func (f fakeStore) ListSessionsByInstance(context.Context, model.InstanceID, bool) ([]model.RemoteSession, error) {
	return append([]model.RemoteSession(nil), f.sessions...), nil
}

type fakeCockpit struct {
	accounts  []cockpit.Account
	instances []cockpit.Instance
}
func (f fakeCockpit) ListAccounts(context.Context) ([]cockpit.Account, error) { return append([]cockpit.Account(nil), f.accounts...), nil }
func (f fakeCockpit) ListInstances(context.Context) ([]cockpit.Instance, error) { return append([]cockpit.Instance(nil), f.instances...), nil }

func TestPlanSwitchCalculatesBlastRadius(t *testing.T) {
	current := "a1"
	instanceID := model.InstanceID("i1")
	store := fakeStore{instance: model.InstanceMirror{ID: instanceID, AccountID: current}, sessions: []model.RemoteSession{
		{ID: "s2", RepositoryID: "r2", ConversationID: "c2", WorkspacePath: "/r2", DesiredState: model.SessionDesiredReady, ObservedState: model.SessionRunning},
		{ID: "closed", DesiredState: model.SessionDesiredClosed, ObservedState: model.SessionClosed},
		{ID: "s1", RepositoryID: "r1", ConversationID: "c1", WorkspacePath: "/r1", DesiredState: model.SessionDesiredReady, ObservedState: model.SessionReady},
	}}
	client := fakeCockpit{accounts: []cockpit.Account{{ID: "a1"}, {ID: "a2", Email: "two@example.com"}}, instances: []cockpit.Instance{{ID: "i1", BindAccountID: &current, Running: true}}}
	service, _ := New(store, client)
	plan, err := service.PlanSwitch(context.Background(), instanceID, "a2")
	if err != nil { t.Fatal(err) }
	if !plan.RequiresColdRestart || !plan.RequiresCheckpoint || !plan.RequiresHandoff { t.Fatalf("plan=%+v", plan) }
	if len(plan.Impacts) != 2 || plan.Impacts[0].SessionID != "s1" || plan.Impacts[1].SessionID != "s2" { t.Fatalf("impacts=%+v", plan.Impacts) }
	if err := plan.ValidateExecution(false); !errors.Is(err, ErrActiveSessions) { t.Fatalf("guard err=%v", err) }
	if err := plan.ValidateExecution(true); err != nil { t.Fatalf("handoff allowed err=%v", err) }
}

func TestPlanSwitchRejectsDisabledAccount(t *testing.T) {
	current := "a1"
	service, _ := New(fakeStore{instance: model.InstanceMirror{ID: "i1", AccountID: current}}, fakeCockpit{accounts: []cockpit.Account{{ID: "a2", Disabled: true}}, instances: []cockpit.Instance{{ID: "i1", BindAccountID: &current}}})
	_, err := service.PlanSwitch(context.Background(), "i1", "a2")
	if !errors.Is(err, ErrAccountDisabled) { t.Fatalf("err=%v", err) }
}

func TestPlanSwitchFailsClosedOnPersistedAccountDrift(t *testing.T) {
	actual := "a2"
	service, _ := New(fakeStore{instance: model.InstanceMirror{ID: "i1", AccountID: "a1"}}, fakeCockpit{accounts: []cockpit.Account{{ID: "a3"}}, instances: []cockpit.Instance{{ID: "i1", BindAccountID: &actual}}})
	plan, err := service.PlanSwitch(context.Background(), "i1", "a3")
	if err != nil { t.Fatal(err) }
	if !plan.PersistedAccountMismatch { t.Fatalf("plan=%+v", plan) }
	if err := plan.ValidateExecution(true); !errors.Is(err, ErrObservedMismatch) { t.Fatalf("guard err=%v", err) }
}

func TestPlanSwitchToleratesMissingMirrorForDiscovery(t *testing.T) {
	current := "a1"
	service, _ := New(fakeStore{err: remotestore.ErrNotFound}, fakeCockpit{accounts: []cockpit.Account{{ID: "a2"}}, instances: []cockpit.Instance{{ID: "i1", BindAccountID: &current}}})
	plan, err := service.PlanSwitch(context.Background(), "i1", "a2")
	if err != nil { t.Fatal(err) }
	if plan.PersistedAccountMismatch { t.Fatal("missing mirror must not be treated as mismatch") }
}
