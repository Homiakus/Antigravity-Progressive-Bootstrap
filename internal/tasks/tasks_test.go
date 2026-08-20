package tasks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
)

func testPaths(t *testing.T) paths.Paths {
	t.Helper()
	root := t.TempDir()
	p := paths.Paths{Home: root, AppRoot: filepath.Join(root, "app")}
	p.BackupsRoot = filepath.Join(p.AppRoot, "backups")
	p.TasksRoot = filepath.Join(p.AppRoot, "tasks")
	p.TaskConfig = filepath.Join(p.AppRoot, "task-supervisor.json")
	p.TelemetryRoot = filepath.Join(p.AppRoot, "telemetry")
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAddListRetryCancel(t *testing.T) {
	p := testPaths(t)
	workspace := t.TempDir()
	rec, err := Add(p, "do something", workspace, 5, false, "", []string{"test"})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != StatusQueued {
		t.Fatalf("status=%s", rec.Status)
	}
	xs, err := List(p)
	if err != nil || len(xs) != 1 {
		t.Fatalf("list=%v err=%v", xs, err)
	}
	if err := Cancel(p, rec.ID); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusCancelled {
		t.Fatalf("cancel status=%s", got.Status)
	}
	got, err = Retry(p, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusQueued {
		t.Fatalf("retry status=%s", got.Status)
	}
	if err := Remove(p, rec.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(taskPath(p, rec.ID)); !os.IsNotExist(err) {
		t.Fatalf("task file still exists")
	}
}

func TestConfigRoundTrip(t *testing.T) {
	p := testPaths(t)
	cfg := DefaultConfig()
	cfg.MaxParallel = 3
	if err := SaveConfig(p, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.MaxParallel != 3 {
		t.Fatalf("max=%d", got.MaxParallel)
	}
}

func TestSelectBatchRespectsResourcesAndExclusiveWorkspace(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxParallel = 4
	cfg.CPUWeight = 100
	cfg.BuildSlots = 1
	cfg.BrowserSlots = 1
	ready := []model.TaskRecord{
		{ID: "a", Workspace: "w1", Resources: model.ResourceRequest{CPUWeight: 50, BuildSlots: 1, ExclusiveWorkspace: true}},
		{ID: "b", Workspace: "w1", Resources: model.ResourceRequest{CPUWeight: 10, ReadOnly: true}},
		{ID: "c", Workspace: "w2", Resources: model.ResourceRequest{CPUWeight: 40, BrowserSlots: 1, ReadOnly: true}},
		{ID: "d", Workspace: "w3", Resources: model.ResourceRequest{CPUWeight: 40, BrowserSlots: 1, ReadOnly: true}},
	}
	batch := selectBatch(ready, cfg)
	ids := map[string]bool{}
	for _, rec := range batch {
		ids[rec.ID] = true
	}
	if !ids["a"] || !ids["c"] {
		t.Fatalf("expected a+c, got %#v", ids)
	}
	if ids["b"] {
		t.Fatal("exclusive workspace task must exclude other same-workspace tasks")
	}
	if ids["d"] {
		t.Fatal("browser slot budget should allow only one browser task")
	}
}

func TestRunPendingBlocksFailedDependencies(t *testing.T) {
	p := testPaths(t)
	workspace := t.TempDir()
	fake := writeFakeAGYWithSpec(t, fakeSpec{
		MatchFail:  "FAIL_PARENT",
		FailExit:   7,
		FailStderr: "parent failed",
		Stdout:     `{"event":"result","result":{"status":"SUCCESS","response":"ok"}}`,
	})
	t.Setenv("AGCTL_AGY_COMMAND", fake)
	parent, err := AddAdvanced(p, Spec{Prompt: "FAIL_PARENT", Workspace: workspace})
	if err != nil {
		t.Fatal(err)
	}
	child, err := AddAdvanced(p, Spec{Prompt: "child", Workspace: workspace, Dependencies: []string{parent.ID}})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = RunPending(p)
	gotParent, _ := Load(p, parent.ID)
	gotChild, _ := Load(p, child.ID)
	if gotParent.Status != StatusFailed {
		t.Fatalf("parent=%s", gotParent.Status)
	}
	if gotChild.Status != StatusBlocked {
		t.Fatalf("child=%s err=%s", gotChild.Status, gotChild.Error)
	}
}

func TestClaimTaskIsExclusive(t *testing.T) {
	p := testPaths(t)
	if err := os.MkdirAll(p.TasksRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	release, err := claimTask(p, "x")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := claimTask(p, "x"); err == nil {
		t.Fatal("second claim should fail")
	}
}

func TestSelectBatchAccountsForAlreadyRunningResources(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxParallel = 2
	cfg.CPUWeight = 100
	cfg.BuildSlots = 1
	running := []model.TaskRecord{{ID: "running", Workspace: "w0", Resources: model.ResourceRequest{CPUWeight: 70, BuildSlots: 1}}}
	ready := []model.TaskRecord{{ID: "build", Workspace: "w1", Resources: model.ResourceRequest{CPUWeight: 20, BuildSlots: 1}}, {ID: "light", Workspace: "w2", Resources: model.ResourceRequest{CPUWeight: 20}}}
	batch := selectBatchWithRunning(ready, cfg, running)
	if len(batch) != 1 || batch[0].ID != "light" {
		t.Fatalf("expected only light task, got %#v", batch)
	}
}

type fakeSpec struct {
	DumpArgsFile string `json:"dumpArgsFile"`
	MatchFail    string `json:"matchFail"`
	FailStderr   string `json:"failStderr"`
	FailExit     int    `json:"failExit"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	ExitCode     int    `json:"exitCode"`
}

var (
	fakeBinaryOnce sync.Once
	fakeBinaryPath string
	fakeBinaryErr  error
)

func getFakeBinary() (string, error) {
	fakeBinaryOnce.Do(func() {
		tmpDir, err := os.MkdirTemp("", "agctl-test-fake-*")
		if err != nil {
			fakeBinaryErr = err
			return
		}
		src := filepath.Join(tmpDir, "main.go")
		code := `package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Spec struct {
	DumpArgsFile string ` + "`json:\"dumpArgsFile\"`" + `
	MatchFail    string ` + "`json:\"matchFail\"`" + `
	FailStderr   string ` + "`json:\"failStderr\"`" + `
	FailExit     int    ` + "`json:\"failExit\"`" + `
	Stdout       string ` + "`json:\"stdout\"`" + `
	Stderr       string ` + "`json:\"stderr\"`" + `
	ExitCode     int    ` + "`json:\"exitCode\"`" + `
}

func main() {
	specPath := os.Getenv("AGCTL_TEST_FAKE_SPEC")
	var spec Spec
	if specPath != "" {
		b, err := os.ReadFile(specPath)
		if err == nil {
			_ = json.Unmarshal(b, &spec)
		}
	}
	allArgs := strings.Join(os.Args[1:], " ")
	if spec.DumpArgsFile != "" {
		_ = os.WriteFile(spec.DumpArgsFile, []byte(allArgs), 0644)
	}
	if spec.MatchFail != "" && strings.Contains(allArgs, spec.MatchFail) {
		if spec.FailStderr != "" {
			_, _ = fmt.Fprintln(os.Stderr, spec.FailStderr)
		}
		exitCode := spec.FailExit
		if exitCode == 0 {
			exitCode = 1
		}
		os.Exit(exitCode)
	}
	if spec.Stderr != "" {
		_, _ = fmt.Fprintln(os.Stderr, spec.Stderr)
	}
	if spec.Stdout != "" {
		_, _ = fmt.Fprintln(os.Stdout, spec.Stdout)
	}
	os.Exit(spec.ExitCode)
}
`
		if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
			fakeBinaryErr = err
			return
		}
		binPath := filepath.Join(tmpDir, "fake-agy.exe")
		if runtime.GOOS != "windows" {
			binPath = filepath.Join(tmpDir, "fake-agy")
		}
		cmd := exec.Command("go", "build", "-o", binPath, src)
		if out, err := cmd.CombinedOutput(); err != nil {
			fakeBinaryErr = fmt.Errorf("build fake agy: %w: %s", err, string(out))
			return
		}
		fakeBinaryPath = binPath
	})
	return fakeBinaryPath, fakeBinaryErr
}

func writeFakeAGYWithSpec(t *testing.T, spec fakeSpec) string {
	t.Helper()
	bin, err := getFakeBinary()
	if err != nil {
		t.Fatal(err)
	}
	specData, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	specFile := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(specFile, specData, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGCTL_TEST_FAKE_SPEC", specFile)
	return bin
}

func runOneWithSpec(t *testing.T, spec fakeSpec, agent string, maxMinutes int) model.TaskRecord {
	t.Helper()
	p := testPaths(t)
	cfg := DefaultConfig()
	cfg.MaxTaskMinutes = maxMinutes
	if err := SaveConfig(p, cfg); err != nil {
		t.Fatal(err)
	}
	fake := writeFakeAGYWithSpec(t, spec)
	t.Setenv("AGCTL_AGY_COMMAND", fake)
	rec, err := AddAdvanced(p, Spec{Prompt: "verify headless semantics", Workspace: t.TempDir(), Agent: agent})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Run(p, rec.ID)
	return got
}

func TestHeadlessSoftDeniedExitZeroIsFailure(t *testing.T) {
	got := runOneWithSpec(t, fakeSpec{
		Stdout:   `{"event":"result","result":{"status":"SUCCESS","response":"partial"}}`,
		Stderr:   "tool run_command was soft-denied because permission approval is unavailable",
		ExitCode: 0,
	}, "", 1)
	if got.Status != StatusFailed || !strings.Contains(strings.ToLower(got.Error), "soft-denied") {
		t.Fatalf("got=%+v", got)
	}
}

func TestHeadlessTerminalErrorExitZeroIsFailure(t *testing.T) {
	got := runOneWithSpec(t, fakeSpec{
		Stdout:   `{"event":"result","result":{"status":"ERROR","error":"model/tool failure"}}`,
		ExitCode: 0,
	}, "", 1)
	if got.Status != StatusFailed || !strings.Contains(got.Error, "model/tool failure") {
		t.Fatalf("got=%+v", got)
	}
}

func TestHeadlessMissingTerminalResultIsFailure(t *testing.T) {
	got := runOneWithSpec(t, fakeSpec{
		Stdout:   `{"event":"step_update","step_update":{"step_type":"agent_response","state":"DONE"}}`,
		ExitCode: 0,
	}, "", 1)
	if got.Status != StatusFailed || !strings.Contains(got.Error, "terminal result") {
		t.Fatalf("got=%+v", got)
	}
}

func TestHeadlessSuccessAndUsesAgentAndTimeoutFlags(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args.txt")
	got := runOneWithSpec(t, fakeSpec{
		DumpArgsFile: argsFile,
		Stdout:       `{"event":"result","result":{"status":"SUCCESS","response":"ok"}}`,
		ExitCode:     0,
	}, "code-reviewer", 3)
	if got.Status != StatusSucceeded {
		t.Fatalf("got=%+v", got)
	}
	b, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	args := string(b)
	if !strings.Contains(args, "--agent code-reviewer") || !strings.Contains(args, "--print-timeout 3m") || !strings.Contains(args, "--output-format stream-json") {
		t.Fatalf("unexpected args: %s", args)
	}
}

func TestExecutionPromptDoesNotSendDesktopGoalSlashCommandToHeadlessCLI(t *testing.T) {
	got := executionPrompt(model.TaskRecord{Prompt: "build it", UseNativeGoal: true})
	if strings.HasPrefix(strings.TrimSpace(got), "/goal") {
		t.Fatalf("desktop /goal slash command must not be sent to AGY headless: %q", got)
	}
	if !strings.Contains(got, "completely and verifiably finished") {
		t.Fatalf("headless until-done contract missing: %q", got)
	}
}
