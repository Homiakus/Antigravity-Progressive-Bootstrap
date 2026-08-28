package cockpit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Runner interface {
	Run(context.Context, string, ...string) (stdout []byte, stderr []byte, err error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, binary string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

type CLI struct {
	Binary string
	Runner Runner
}

type envelope struct {
	ProtocolVersion int             `json:"protocolVersion"`
	OK              bool            `json:"ok"`
	Data            json.RawMessage `json:"data"`
	Error           *protocolError  `json:"error,omitempty"`
}

type protocolError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func NewCLI(binary string, runner Runner) (*CLI, error) {
	binary = strings.TrimSpace(binary)
	if binary == "" {
		binary = "cockpit-control"
	}
	if runner == nil {
		runner = ExecRunner{}
	}
	return &CLI{Binary: binary, Runner: runner}, nil
}

func (c *CLI) Protocol(ctx context.Context) (ProtocolInfo, error) {
	var info ProtocolInfo
	if err := c.invoke(ctx, []string{"protocol"}, &info); err != nil {
		return ProtocolInfo{}, err
	}
	if info.ProtocolVersion != ProtocolVersion {
		return ProtocolInfo{}, fmt.Errorf("cockpit protocol payload version=%d want=%d", info.ProtocolVersion, ProtocolVersion)
	}
	return info, nil
}

func (c *CLI) ListAccounts(ctx context.Context) ([]Account, error) {
	var accounts []Account
	if err := c.invoke(ctx, []string{"accounts", "list"}, &accounts); err != nil {
		return nil, err
	}
	for _, account := range accounts {
		if strings.TrimSpace(account.ID) == "" {
			return nil, fmt.Errorf("cockpit returned account with empty id")
		}
	}
	return accounts, nil
}

func (c *CLI) ListInstances(ctx context.Context) ([]Instance, error) {
	var instances []Instance
	if err := c.invoke(ctx, []string{"instances", "list"}, &instances); err != nil {
		return nil, err
	}
	for _, instance := range instances {
		if err := validateInstance(instance); err != nil {
			return nil, err
		}
	}
	return instances, nil
}

func (c *CLI) CreateInstance(ctx context.Context, spec CreateInstanceSpec) (Instance, error) {
	if strings.TrimSpace(spec.Name) == "" || strings.TrimSpace(spec.UserDataDir) == "" {
		return Instance{}, fmt.Errorf("cockpit instance name and user-data-dir are required")
	}
	args := []string{"instance", "create", "--name", spec.Name, "--user-data-dir", spec.UserDataDir}
	args = appendOptional(args, "--working-dir", spec.WorkingDir)
	args = appendOptional(args, "--extra-args", spec.ExtraArgs)
	args = appendOptional(args, "--account-id", spec.BindAccountID)
	args = appendOptional(args, "--copy-source-instance-id", spec.CopySourceInstanceID)
	args = appendOptional(args, "--init-mode", spec.InitMode)
	return c.instanceCommand(ctx, args)
}

func (c *CLI) UpdateInstance(ctx context.Context, id string, patch InstancePatch) (Instance, error) {
	if strings.TrimSpace(id) == "" {
		return Instance{}, fmt.Errorf("cockpit instance id is required")
	}
	args := []string{"instance", "update", "--id", id}
	if patch.Name != nil {
		args = append(args, "--name", *patch.Name)
	}
	if patch.WorkingDir != nil {
		args = append(args, "--working-dir", *patch.WorkingDir)
	}
	if patch.ExtraArgs != nil {
		args = append(args, "--extra-args", *patch.ExtraArgs)
	}
	if patch.UnbindAccount {
		args = append(args, "--unbind-account")
	} else if patch.BindAccountID != nil {
		args = append(args, "--account-id", *patch.BindAccountID)
	}
	return c.instanceCommand(ctx, args)
}

func (c *CLI) StartInstance(ctx context.Context, id string) (Instance, error) {
	return c.instanceByIDCommand(ctx, "start", id)
}

func (c *CLI) StopInstance(ctx context.Context, id string) (Instance, error) {
	return c.instanceByIDCommand(ctx, "stop", id)
}

func (c *CLI) FocusInstance(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("cockpit instance id is required")
	}
	var result struct{}
	return c.invoke(ctx, []string{"instance", "focus", "--id", id}, &result)
}

func (c *CLI) BindAccount(ctx context.Context, instanceID, accountID string) (Instance, error) {
	if strings.TrimSpace(instanceID) == "" || strings.TrimSpace(accountID) == "" {
		return Instance{}, fmt.Errorf("cockpit instance and account ids are required")
	}
	return c.instanceCommand(ctx, []string{"instance", "bind-account", "--id", instanceID, "--account-id", accountID})
}

func (c *CLI) instanceByIDCommand(ctx context.Context, action, id string) (Instance, error) {
	if strings.TrimSpace(id) == "" {
		return Instance{}, fmt.Errorf("cockpit instance id is required")
	}
	return c.instanceCommand(ctx, []string{"instance", action, "--id", id})
}

func (c *CLI) instanceCommand(ctx context.Context, args []string) (Instance, error) {
	var instance Instance
	if err := c.invoke(ctx, args, &instance); err != nil {
		return Instance{}, err
	}
	if err := validateInstance(instance); err != nil {
		return Instance{}, err
	}
	return instance, nil
}

func validateInstance(instance Instance) error {
	if strings.TrimSpace(instance.ID) == "" {
		return fmt.Errorf("cockpit returned instance with empty id")
	}
	if strings.TrimSpace(instance.UserDataDir) == "" {
		return fmt.Errorf("cockpit instance %s has empty userDataDir", instance.ID)
	}
	return nil
}

func (c *CLI) invoke(ctx context.Context, args []string, out any) error {
	args = append(append([]string{}, args...), "--json")
	stdout, stderr, err := c.Runner.Run(ctx, c.Binary, args...)
	if err != nil {
		message := strings.TrimSpace(string(stderr))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("cockpit-control %s: %s", strings.Join(args[:len(args)-1], " "), message)
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
			return errors.New("cockpit-control returned failure without error details")
		}
		return fmt.Errorf("cockpit-control %s: %s", response.Error.Code, response.Error.Message)
	}
	if out == nil {
		return nil
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

func appendOptional(args []string, flag, value string) []string {
	if strings.TrimSpace(value) != "" {
		return append(args, flag, value)
	}
	return args
}

// boolString is retained for protocol extensions that require explicit bool
// flags without relying on shell-specific formatting.
func boolString(value bool) string { return strconv.FormatBool(value) }
