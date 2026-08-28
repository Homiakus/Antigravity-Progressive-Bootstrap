package session

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/cockpit"
	"github.com/homiakus/agctl/internal/remote/model"
)

type staticResolver struct{ decision ResolveDecision }

func (s staticResolver) Resolve(context.Context, ResolveRequest) (ResolveDecision, error) {
	return s.decision, nil
}

func TestProvisionAutoReusesLiveInstanceWithoutRestart(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	repo := model.Repository{ID: "rep_1700000000000_00000000000000000000", Name: "repo", CanonicalPath: "/work/repo", GitRoot: "/work/repo", Enabled: true, CreatedAt: now, LastSeenAt: now}
	store := &fakeStore{repo: repo}
	cockpitClient := &fakeCockpit{}
	bridge := &fakeBridge{repo: repo.CanonicalPath}
	ids := model.TimeSortableIDGenerator{Now: func() time.Time { return now }, Random: bytes.NewReader(make([]byte, 128))}
	service, err := New(Options{Store: store, Cockpit: cockpitClient, Locator: fakeLocator{bridge: bridge}, IDs: ids, HostID: "host", ProfileRoot: "/profiles", BridgeRegistry: "/state/bridges", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Provision(context.Background(), Spec{RepositoryID: repo.ID, AccountID: "a1", InstanceStrategy: InstanceDedicated, ConversationStrategy: ConversationNew, IsolationMode: model.IsolationSharedRead})
	if err != nil {
		t.Fatal(err)
	}
	if cockpitClient.started != 1 {
		t.Fatalf("initial starts=%d", cockpitClient.started)
	}
	service.resolver = staticResolver{decision: ResolveDecision{Action: ResolveReuse, InstanceID: first.CockpitInstanceID}}
	second, err := service.Provision(context.Background(), Spec{RepositoryID: repo.ID, AccountID: "a1", InstanceStrategy: InstanceAuto, ConversationStrategy: ConversationNew, IsolationMode: model.IsolationSharedRead})
	if err != nil {
		t.Fatal(err)
	}
	if cockpitClient.started != 1 {
		t.Fatalf("AUTO reuse restarted live IDE; starts=%d", cockpitClient.started)
	}
	if cockpitClient.created != 1 {
		t.Fatalf("AUTO reuse created extra instance; creates=%d", cockpitClient.created)
	}
	if second.CockpitInstanceID != first.CockpitInstanceID {
		t.Fatalf("first=%s second=%s", first.CockpitInstanceID, second.CockpitInstanceID)
	}
}

func TestProvisionAutoFallsBackToNewInstanceWhenLiveCredentialsAreUnavailable(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	repo := model.Repository{ID: "rep_1700000000000_00000000000000000000", Name: "repo", CanonicalPath: "/work/repo", GitRoot: "/work/repo", Enabled: true, CreatedAt: now, LastSeenAt: now}
	store := &fakeStore{repo: repo}
	account := "a1"
	pid := uint32(77)
	cockpitClient := &fakeCockpit{instance: cockpitInstanceForAuto("existing", repo.CanonicalPath, account, pid)}
	bridge := &fakeBridge{repo: repo.CanonicalPath}
	ids := model.TimeSortableIDGenerator{Now: func() time.Time { return now }, Random: bytes.NewReader(make([]byte, 128))}
	service, err := New(Options{Store: store, Cockpit: cockpitClient, Locator: fakeLocator{bridge: bridge}, Resolver: staticResolver{decision: ResolveDecision{Action: ResolveReuse, InstanceID: model.InstanceID("existing")}}, IDs: ids, HostID: "host", ProfileRoot: "/profiles", BridgeRegistry: "/state/bridges", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Provision(context.Background(), Spec{RepositoryID: repo.ID, AccountID: account, InstanceStrategy: InstanceAuto, ConversationStrategy: ConversationNew, IsolationMode: model.IsolationSharedRead})
	if err != nil {
		t.Fatal(err)
	}
	if cockpitClient.created != 1 {
		t.Fatalf("expected safe isolated fallback create, got %d", cockpitClient.created)
	}
	if cockpitClient.started != 1 {
		t.Fatalf("expected only new instance start, got %d", cockpitClient.started)
	}
}

func cockpitInstanceForAuto(id, workspace, account string, pid uint32) cockpit.Instance {
	return cockpit.Instance{ID: id, UserDataDir: "/profile/" + id, WorkingDir: workspace, BindAccountID: &account, LastPID: &pid, Running: true, Initialized: true}
}
