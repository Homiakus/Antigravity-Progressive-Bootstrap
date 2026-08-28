package session

import (
	"context"
	"testing"

	"github.com/homiakus/agctl/internal/remote/model"
)

type resolverStore struct {
	instances []model.InstanceMirror
	sessions  map[model.InstanceID][]model.RemoteSession
}

func (f resolverStore) ListInstances(context.Context) ([]model.InstanceMirror, error) {
	return append([]model.InstanceMirror(nil), f.instances...), nil
}
func (f resolverStore) ListSessionsByInstance(_ context.Context, id model.InstanceID, _ bool) ([]model.RemoteSession, error) {
	return append([]model.RemoteSession(nil), f.sessions[id]...), nil
}

func readyInstance(id, account, workspace string) model.InstanceMirror {
	return model.InstanceMirror{ID: model.InstanceID(id), AccountID: account, WorkingDir: workspace, DesiredState: model.InstanceDesiredRunning, ObservedState: model.InstanceReady}
}

func TestResolverPrefersExactAccountWorkspace(t *testing.T) {
	store := resolverStore{instances: []model.InstanceMirror{
		readyInstance("inst-b", "acct-a", "/repo/b"),
		readyInstance("inst-a", "acct-a", "/repo/a"),
	}}
	resolver, _ := NewResolver(store)
	decision, err := resolver.Resolve(context.Background(), ResolveRequest{AccountID: "acct-a", WorkspacePath: "/repo/a"})
	if err != nil { t.Fatal(err) }
	if decision.Action != ResolveReuse || decision.InstanceID != model.InstanceID("inst-a") { t.Fatalf("decision=%+v", decision) }
}

func TestResolverRejectsCrossAccountReuse(t *testing.T) {
	store := resolverStore{instances: []model.InstanceMirror{readyInstance("inst-a", "acct-b", "/repo/a")}}
	resolver, _ := NewResolver(store)
	decision, err := resolver.Resolve(context.Background(), ResolveRequest{AccountID: "acct-a", WorkspacePath: "/repo/a", AllowWorkspaceReplacement: true})
	if err != nil { t.Fatal(err) }
	if decision.Action != ResolveCreateNew { t.Fatalf("decision=%+v", decision) }
}

func TestResolverDoesNotReplacePinnedWorkspace(t *testing.T) {
	instance := readyInstance("inst-a", "acct-a", "/repo/old")
	store := resolverStore{
		instances: []model.InstanceMirror{instance},
		sessions: map[model.InstanceID][]model.RemoteSession{instance.ID: {{ID: model.RemoteSessionID("sess-a"), DesiredState: model.SessionDesiredReady, ObservedState: model.SessionReady}}},
	}
	resolver, _ := NewResolver(store)
	decision, err := resolver.Resolve(context.Background(), ResolveRequest{AccountID: "acct-a", WorkspacePath: "/repo/new", AllowWorkspaceReplacement: true})
	if err != nil { t.Fatal(err) }
	if decision.Action != ResolveCreateNew { t.Fatalf("decision=%+v", decision) }
}

func TestResolverCanReuseIdleWorkspaceWhenPolicyAllows(t *testing.T) {
	instance := readyInstance("inst-a", "acct-a", "/repo/old")
	store := resolverStore{instances: []model.InstanceMirror{instance}, sessions: map[model.InstanceID][]model.RemoteSession{instance.ID: {{ID: model.RemoteSessionID("closed"), DesiredState: model.SessionDesiredClosed, ObservedState: model.SessionClosed}}}}
	resolver, _ := NewResolver(store)
	decision, err := resolver.Resolve(context.Background(), ResolveRequest{AccountID: "acct-a", WorkspacePath: "/repo/new", AllowWorkspaceReplacement: true})
	if err != nil { t.Fatal(err) }
	if decision.Action != ResolveReuse || decision.InstanceID != instance.ID { t.Fatalf("decision=%+v", decision) }
}

func TestResolverSupportsMultipleInstancesForSameAccount(t *testing.T) {
	store := resolverStore{instances: []model.InstanceMirror{
		readyInstance("inst-a", "acct-a", "/repo/a"),
		readyInstance("inst-b", "acct-a", "/repo/b"),
		readyInstance("inst-c", "acct-b", "/repo/c"),
	}}
	resolver, _ := NewResolver(store)
	decision, err := resolver.Resolve(context.Background(), ResolveRequest{AccountID: "acct-a", WorkspacePath: "/repo/b"})
	if err != nil { t.Fatal(err) }
	if decision.InstanceID != model.InstanceID("inst-b") { t.Fatalf("decision=%+v", decision) }
}
