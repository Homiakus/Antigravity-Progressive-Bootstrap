package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/antigravityide"
	"github.com/homiakus/agctl/internal/cockpit"
)

type fakeSecrets struct{ n int }
func (f *fakeSecrets) NewSecret(int) (string, error) { f.n++; if f.n == 1 { return "boot", nil }; return "token", nil }

type fakeCockpit struct {
	cockpit.ManagedClient
	started int
	stopped int
	launch  cockpit.LaunchContext
}
func (f *fakeCockpit) StartManagedInstance(_ context.Context, id string, launch cockpit.LaunchContext) (cockpit.Instance, error) {
	f.started++
	f.launch = launch
	pid := uint32(99)
	return cockpit.Instance{ID: id, Running: true, LastPID: &pid}, nil
}
func (f *fakeCockpit) StopInstance(_ context.Context, id string) (cockpit.Instance, error) {
	f.stopped++
	return cockpit.Instance{ID: id, Running: false}, nil
}

type fakeClient struct{ health antigravityide.Health }
func (f fakeClient) Health(context.Context) (antigravityide.Health, error) { return f.health, nil }
func (f fakeClient) Capabilities(context.Context) (antigravityide.Capabilities, error) { return antigravityide.Capabilities{}, nil }
func (f fakeClient) Context(context.Context) (antigravityide.Context, error) { return antigravityide.Context{}, nil }
func (f fakeClient) ListConversations(context.Context) ([]antigravityide.Conversation, error) { return nil, nil }
func (f fakeClient) CreateConversation(context.Context) (antigravityide.Conversation, error) { return antigravityide.Conversation{}, nil }
func (f fakeClient) FocusConversation(context.Context, string) error { return nil }
func (f fakeClient) SendMessage(context.Context, string, string) error { return nil }
func (f fakeClient) OpenWorkspace(context.Context, string) (antigravityide.OpenWorkspaceResult, error) { return antigravityide.OpenWorkspaceResult{}, nil }

type fakeLocator struct{ client antigravityide.Client }
func (f fakeLocator) Wait(_ context.Context, instanceID, token string) (antigravityide.LocatedBridge, error) {
	if token != "token" { return antigravityide.LocatedBridge{}, fmt.Errorf("unexpected token") }
	return antigravityide.LocatedBridge{Registration: antigravityide.Registration{ProtocolVersion: 1, InstanceID: instanceID, BootNonce: "boot", PID: 99, Port: 1, StartedAt: time.Now()}, Client: f.client}, nil
}

func TestStartCreatesManagedBridgeAndCachesIt(t *testing.T) {
	cockpitClient := &fakeCockpit{}
	manager, err := New(Options{Cockpit: cockpitClient, Locator: fakeLocator{client: fakeClient{health: antigravityide.Health{Status: "ok", InstanceID: "i1", BootNonce: "boot"}}}, Secrets: &fakeSecrets{}, BridgeRegistry: "/registry"})
	if err != nil { t.Fatal(err) }
	started, bridge, err := manager.Start(context.Background(), cockpit.Instance{ID: "i1"})
	if err != nil { t.Fatal(err) }
	if !started.Running || bridge.Registration.InstanceID != "i1" { t.Fatalf("started=%+v bridge=%+v", started, bridge.Registration) }
	if cockpitClient.started != 1 || cockpitClient.launch.BridgeToken != "token" { t.Fatalf("starts=%d launch=%+v", cockpitClient.started, cockpitClient.launch) }
	if _, ok := manager.Bridge("i1"); !ok { t.Fatal("Bridge was not cached") }
}

func TestRunningInstanceWithoutCredentialsFailsClosed(t *testing.T) {
	manager, _ := New(Options{Cockpit: &fakeCockpit{}, Locator: fakeLocator{client: fakeClient{}}, Secrets: &fakeSecrets{}, BridgeRegistry: "/registry"})
	_, _, err := manager.Start(context.Background(), cockpit.Instance{ID: "i1", Running: true})
	if err == nil { t.Fatal("expected missing credentials error") }
}

func TestStopForgetsBridge(t *testing.T) {
	cockpitClient := &fakeCockpit{}
	manager, _ := New(Options{Cockpit: cockpitClient, Locator: fakeLocator{client: fakeClient{health: antigravityide.Health{Status: "ok", InstanceID: "i1"}}}, Secrets: &fakeSecrets{}, BridgeRegistry: "/registry"})
	_, _, err := manager.Start(context.Background(), cockpit.Instance{ID: "i1"})
	if err != nil { t.Fatal(err) }
	if _, err := manager.Stop(context.Background(), "i1"); err != nil { t.Fatal(err) }
	if cockpitClient.stopped != 1 { t.Fatalf("stops=%d", cockpitClient.stopped) }
	if _, ok := manager.Bridge("i1"); ok { t.Fatal("stale Bridge remained after stop") }
}
