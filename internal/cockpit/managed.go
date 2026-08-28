package cockpit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

type LaunchContext struct {
	InstanceID     string
	BootNonce      string
	BridgeToken    string
	BridgeRegistry string
}

func (c LaunchContext) Validate(instanceID string) error {
	if strings.TrimSpace(instanceID) == "" || strings.TrimSpace(c.InstanceID) == "" {
		return fmt.Errorf("managed Cockpit launch requires instance id")
	}
	if c.InstanceID != instanceID {
		return fmt.Errorf("launch context instance %q does not match target %q", c.InstanceID, instanceID)
	}
	if strings.TrimSpace(c.BootNonce) == "" || strings.TrimSpace(c.BridgeToken) == "" || strings.TrimSpace(c.BridgeRegistry) == "" {
		return fmt.Errorf("managed Cockpit launch requires boot nonce, bridge token and registry path")
	}
	return nil
}

type ManagedClient interface {
	Client
	StartManagedInstance(context.Context, string, LaunchContext) (Instance, error)
}

type EnvRunner interface {
	RunWithEnv(context.Context, string, map[string]string, ...string) (stdout []byte, stderr []byte, err error)
}

func (ExecRunner) RunWithEnv(ctx context.Context, binary string, env map[string]string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = append(os.Environ(), formatEnv(env)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func formatEnv(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out
}

func (c *CLI) StartManagedInstance(ctx context.Context, id string, launch LaunchContext) (Instance, error) {
	if err := launch.Validate(id); err != nil {
		return Instance{}, err
	}
	runner, ok := c.Runner.(EnvRunner)
	if !ok {
		return Instance{}, fmt.Errorf("cockpit runner does not support per-invocation environment")
	}
	env := map[string]string{
		"AGCTL_INSTANCE_ID":     launch.InstanceID,
		"AGCTL_BOOT_NONCE":      launch.BootNonce,
		"AGCTL_BRIDGE_TOKEN":    launch.BridgeToken,
		"AGCTL_BRIDGE_REGISTRY": launch.BridgeRegistry,
	}
	var instance Instance
	if err := c.invokeWithEnv(ctx, runner, env, []string{"instance", "start", "--id", id}, &instance); err != nil {
		return Instance{}, err
	}
	if err := validateInstance(instance); err != nil {
		return Instance{}, err
	}
	return instance, nil
}

func (c *CLI) invokeWithEnv(ctx context.Context, runner EnvRunner, env map[string]string, args []string, out any) error {
	argv := append(append([]string{}, args...), "--json")
	stdout, stderr, err := runner.RunWithEnv(ctx, c.Binary, env, argv...)
	if err != nil {
		message := strings.TrimSpace(string(stderr))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("cockpit-control %s: %s", strings.Join(args, " "), message)
	}
	var response envelope
	decoder := json.NewDecoder(bytes.NewReader(stdout))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return fmt.Errorf("decode cockpit-control response: %w", err)
	}
	if response.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported cockpit-control protocol version %d; want %d", response.ProtocolVersion, ProtocolVersion)
	}
	if !response.OK {
		if response.Error == nil {
			return fmt.Errorf("cockpit-control returned failure without error details")
		}
		return fmt.Errorf("cockpit-control %s: %s", response.Error.Code, response.Error.Message)
	}
	if len(response.Data) == 0 || bytes.Equal(response.Data, []byte("null")) {
		return fmt.Errorf("cockpit-control returned empty data")
	}
	decoder = json.NewDecoder(bytes.NewReader(response.Data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode cockpit-control data: %w", err)
	}
	return nil
}
