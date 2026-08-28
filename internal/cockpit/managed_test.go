package cockpit

import (
	"context"
	"strings"
	"testing"
)

type managedFakeRunner struct {
	fakeRunner
	env map[string]string
}

func (f *managedFakeRunner) RunWithEnv(_ context.Context, binary string, env map[string]string, args ...string) ([]byte, []byte, error) {
	f.args = append([]string{binary}, args...)
	f.env = env
	return f.stdout, f.stderr, f.err
}

func TestManagedLaunchKeepsBridgeTokenOutOfArgv(t *testing.T) {
	runner := &managedFakeRunner{}
	runner.stdout = []byte(`{"protocolVersion":1,"ok":true,"data":{"id":"i1","userDataDir":"C:/profile","workingDir":"C:/repo","lastPid":123,"running":true,"initialized":true}}`)
	client, _ := NewCLI("cockpit-control-test", runner)
	launch := LaunchContext{InstanceID: "i1", BootNonce: "boot-1", BridgeToken: "top-secret-token", BridgeRegistry: "C:/state/bridges"}
	instance, err := client.StartManagedInstance(context.Background(), "i1", launch)
	if err != nil {
		t.Fatal(err)
	}
	if instance.ID != "i1" || !instance.Running {
		t.Fatalf("instance=%#v", instance)
	}
	joined := strings.Join(runner.args, " ")
	if strings.Contains(joined, launch.BridgeToken) || strings.Contains(joined, launch.BootNonce) {
		t.Fatalf("launch secret/context leaked to argv: %q", joined)
	}
	if runner.env["AGCTL_BRIDGE_TOKEN"] != launch.BridgeToken || runner.env["AGCTL_INSTANCE_ID"] != "i1" {
		t.Fatalf("managed env=%v", runner.env)
	}
}

func TestManagedLaunchRejectsCrossInstanceContext(t *testing.T) {
	runner := &managedFakeRunner{}
	client, _ := NewCLI("cockpit-control-test", runner)
	_, err := client.StartManagedInstance(context.Background(), "i1", LaunchContext{InstanceID: "i2", BootNonce: "b", BridgeToken: "t", BridgeRegistry: "r"})
	if err == nil {
		t.Fatal("expected cross-instance launch context to fail")
	}
}
