package replan

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
	"github.com/homiakus/agctl/internal/planner"
	"github.com/homiakus/agctl/internal/tasks"
)

func testPaths(t *testing.T) paths.Paths {
	t.Helper()
	root := t.TempDir()
	p := paths.Paths{Home: root, AppRoot: filepath.Join(root, "app")}
	p.BackupsRoot = filepath.Join(p.AppRoot, "backups")
	p.TasksRoot = filepath.Join(p.AppRoot, "tasks")
	p.TaskConfig = filepath.Join(p.AppRoot, "task-supervisor.json")
	p.PlansRoot = filepath.Join(p.AppRoot, "plans")
	p.ReplanRoot = filepath.Join(p.AppRoot, "replan")
	p.ReplanConfig = filepath.Join(p.ReplanRoot, "config.json")
	p.ReplanInbox = filepath.Join(p.ReplanRoot, "inbox")
	p.ReplanArchive = filepath.Join(p.ReplanRoot, "archive")
	p.TelemetryRoot = filepath.Join(p.AppRoot, "telemetry")
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfig(p, DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProposalAddsDynamicNodeAndRewiresDownstream(t *testing.T) {
	p := testPaths(t)
	workspace := t.TempDir()
	pl := model.ExecutionPlan{ID: "plan-x", Prompt: "fix service", Workspace: workspace, Status: "active", Nodes: []model.PlanNode{{ID: "inspect"}, {ID: "review", DependsOn: []string{"inspect"}}}}
	if err := planner.Save(p, pl); err != nil {
		t.Fatal(err)
	}
	parent, err := tasks.AddAdvanced(p, tasks.Spec{Prompt: "inspect", Workspace: workspace, PlanID: pl.ID, NodeID: "inspect"})
	if err != nil {
		t.Fatal(err)
	}
	parent.Status = tasks.StatusSucceeded
	if err := tasks.SaveRecord(p, parent); err != nil {
		t.Fatal(err)
	}
	child, err := tasks.AddAdvanced(p, tasks.Spec{Prompt: "review", Workspace: workspace, PlanID: pl.ID, NodeID: "review", Dependencies: []string{parent.ID}})
	if err != nil {
		t.Fatal(err)
	}
	proposal := `{"version":1,"planId":"plan-x","parentNodeId":"inspect","parentTaskId":"` + parent.ID + `","reason":"integration contract missing","evidence":["API test shows missing validation"],"confidence":0.95,"actions":[{"id":"fix-validation","title":"Fix validation","objective":"Implement missing validation and add tests","agent":"implementer","verification":["targeted tests pass"],"risk":"write-medium"}]}`
	if err := os.WriteFile(parent.ReplanProposalPath, []byte(proposal), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ProcessRecord(p, parent)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied || len(res.AddedNodes) != 1 {
		t.Fatalf("unexpected result: %#v", res)
	}
	gotChild, err := tasks.Load(p, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotChild.Dependencies) != 1 || gotChild.Dependencies[0] == parent.ID {
		t.Fatalf("downstream was not rewired: %#v", gotChild.Dependencies)
	}
	gotPlan, err := planner.Load(p, pl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotPlan.Revision != 1 || gotPlan.DynamicNodeCount != 1 || len(gotPlan.RevisionHistory) != 1 {
		t.Fatalf("plan revision not recorded: %#v", gotPlan)
	}
}

func TestFailedTaskCreatesRecoveryChainAndSupersedesFailure(t *testing.T) {
	p := testPaths(t)
	workspace := t.TempDir()
	pl := model.ExecutionPlan{ID: "plan-f", Prompt: "fix", Workspace: workspace, Status: "active", Nodes: []model.PlanNode{{ID: "implement"}, {ID: "review", DependsOn: []string{"implement"}}}}
	if err := planner.Save(p, pl); err != nil {
		t.Fatal(err)
	}
	failed, err := tasks.AddAdvanced(p, tasks.Spec{Prompt: "implement", Workspace: workspace, PlanID: pl.ID, NodeID: "implement"})
	if err != nil {
		t.Fatal(err)
	}
	failed.Status = tasks.StatusFailed
	failed.Error = "tests failed: expected 2 got 3"
	failed.Attempts = 2
	if err := tasks.SaveRecord(p, failed); err != nil {
		t.Fatal(err)
	}
	child, err := tasks.AddAdvanced(p, tasks.Spec{Prompt: "review", Workspace: workspace, PlanID: pl.ID, NodeID: "review", Dependencies: []string{failed.ID}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := ProcessRecord(p, failed)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied || len(res.CreatedTasks) != 3 {
		t.Fatalf("expected recovery chain: %#v", res)
	}
	gotFailed, _ := tasks.Load(p, failed.ID)
	if gotFailed.Status != tasks.StatusSuperseded {
		t.Fatalf("failed task should be superseded, got %s", gotFailed.Status)
	}
	gotChild, _ := tasks.Load(p, child.ID)
	if len(gotChild.Dependencies) != 1 || gotChild.Dependencies[0] == failed.ID {
		t.Fatalf("child dependency not rewired: %#v", gotChild.Dependencies)
	}
	gotPlan, _ := planner.Load(p, pl.ID)
	if gotPlan.Revision != 1 || gotPlan.DynamicNodeCount != 3 {
		t.Fatalf("bad recovery plan revision: rev=%d dyn=%d", gotPlan.Revision, gotPlan.DynamicNodeCount)
	}
}

func TestNoProgressStopsRepeatedFailure(t *testing.T) {
	p := testPaths(t)
	cfg := DefaultConfig()
	cfg.MaxSameFailure = 1
	if err := SaveConfig(p, cfg); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	pl := model.ExecutionPlan{ID: "plan-np", Prompt: "fix", Workspace: workspace, Status: "active", Revision: 1, RevisionHistory: []model.PlanRevision{{Number: 1, FailureSignature: "placeholder"}}, Nodes: []model.PlanNode{{ID: "verify"}}}
	if err := planner.Save(p, pl); err != nil {
		t.Fatal(err)
	}
	failed, err := tasks.AddAdvanced(p, tasks.Spec{Prompt: "verify", Workspace: workspace, PlanID: pl.ID, NodeID: "verify", DynamicDepth: cfg.MaxRepairDepth})
	if err != nil {
		t.Fatal(err)
	}
	failed.Status = tasks.StatusFailed
	failed.Error = "same failure"
	_ = tasks.SaveRecord(p, failed)
	res, err := ProcessRecord(p, failed)
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied || !res.NoProgress {
		t.Fatalf("expected no-progress block: %#v", res)
	}
	gotPlan, _ := planner.Load(p, pl.ID)
	if gotPlan.Status != "blocked" {
		t.Fatalf("plan status=%s", gotPlan.Status)
	}
}

func TestRunPendingRecoversFailedNodeEndToEnd(t *testing.T) {
	p := testPaths(t)
	cfg := tasks.DefaultConfig()
	cfg.MaxRetries = 0
	if err := tasks.SaveConfig(p, cfg); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	pl := model.ExecutionPlan{ID: "plan-e2e", Prompt: "recover", Workspace: workspace, Status: "active", Nodes: []model.PlanNode{
		{ID: "implement", Objective: "ORIGINAL_FAIL", Agent: "implementer", Resources: model.ResourceRequest{CPUWeight: 20}},
		{ID: "review", Objective: "review recovered state", Agent: "code-reviewer", DependsOn: []string{"implement"}, Resources: model.ResourceRequest{CPUWeight: 10, ReadOnly: true}},
	}}
	if err := planner.Save(p, pl); err != nil {
		t.Fatal(err)
	}
	if _, err := planner.Enqueue(p, pl, 0, false); err != nil {
		t.Fatal(err)
	}
	fake := writeFakeAGYWithSpec(t, fakeSpec{
		MatchFail:  "ORIGINAL_FAIL",
		FailStderr: "original-failure",
		FailExit:   7,
		Stdout:     `{"event":"result","result":{"status":"SUCCESS","response":"ok"}}`,
	})
	t.Setenv("AGCTL_AGY_COMMAND", fake)
	results, err := RunPending(p)
	if err != nil {
		t.Fatalf("adaptive run should recover failure: %v results=%#v", err, results)
	}
	got, err := planner.Load(p, pl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "completed" {
		t.Fatalf("plan status=%s reason=%s", got.Status, got.BlockReason)
	}
	if got.Revision < 1 || got.DynamicNodeCount < 3 {
		t.Fatalf("expected recovery revision, got rev=%d dyn=%d", got.Revision, got.DynamicNodeCount)
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

func TestParallelProposalCreatesWorktreesAndIntegrationGate(t *testing.T) {
	p := testPaths(t)
	workspace := t.TempDir()
	cmds := [][]string{{"git", "init"}, {"git", "config", "user.email", "test@example.com"}, {"git", "config", "user.name", "Test"}}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = workspace
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git unavailable: %v %s", err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = workspace
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	cmd = exec.Command("git", "commit", "-m", "base")
	cmd.Dir = workspace
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}

	pl := model.ExecutionPlan{ID: "plan-wt", Prompt: "parallel fixes", Workspace: workspace, Status: "active", Nodes: []model.PlanNode{{ID: "inspect", Resources: model.ResourceRequest{ReadOnly: true}}, {ID: "review", DependsOn: []string{"inspect"}}}}
	if err := planner.Save(p, pl); err != nil {
		t.Fatal(err)
	}
	parent, err := tasks.AddAdvanced(p, tasks.Spec{Prompt: "inspect", Workspace: workspace, PlanID: pl.ID, NodeID: "inspect", Resources: model.ResourceRequest{ReadOnly: true}})
	if err != nil {
		t.Fatal(err)
	}
	parent.Status = tasks.StatusSucceeded
	if err := tasks.SaveRecord(p, parent); err != nil {
		t.Fatal(err)
	}
	child, err := tasks.AddAdvanced(p, tasks.Spec{Prompt: "review", Workspace: workspace, PlanID: pl.ID, NodeID: "review", Dependencies: []string{parent.ID}})
	if err != nil {
		t.Fatal(err)
	}
	proposal := `{"version":1,"planId":"plan-wt","parentNodeId":"inspect","parentTaskId":"` + parent.ID + `","reason":"two independent defects","evidence":["defect A reproduced","defect B reproduced"],"confidence":0.9,"actions":[{"id":"fix-a","title":"Fix A","objective":"fix A","agent":"implementer","risk":"write-medium","parallelizable":true},{"id":"fix-b","title":"Fix B","objective":"fix B","agent":"implementer","risk":"write-medium","parallelizable":true}]}`
	if err := os.WriteFile(parent.ReplanProposalPath, []byte(proposal), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ProcessRecord(p, parent)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied || len(res.AddedNodes) != 3 {
		t.Fatalf("expected 2 worktree actions + integration: %#v", res)
	}
	gotChild, _ := tasks.Load(p, child.ID)
	if len(gotChild.Dependencies) != 1 {
		t.Fatalf("review should wait for integration gate: %#v", gotChild.Dependencies)
	}
	all, _ := tasks.List(p)
	branches := 0
	integration := false
	for _, tr := range all {
		if tr.PlanID != pl.ID {
			continue
		}
		if tr.WorktreeBranch != "" {
			branches++
		}
		if strings.Contains(tr.NodeID, "integrate") {
			integration = true
		}
	}
	if branches != 2 || !integration {
		t.Fatalf("branches=%d integration=%v", branches, integration)
	}
}

func TestProposalRiskAboveThresholdIsRejected(t *testing.T) {
	p := testPaths(t)
	workspace := t.TempDir()
	pl := model.ExecutionPlan{ID: "plan-risk", Prompt: "x", Workspace: workspace, Status: "active", Nodes: []model.PlanNode{{ID: "inspect"}}}
	if err := planner.Save(p, pl); err != nil {
		t.Fatal(err)
	}
	parent, err := tasks.AddAdvanced(p, tasks.Spec{Prompt: "inspect", Workspace: workspace, PlanID: pl.ID, NodeID: "inspect"})
	if err != nil {
		t.Fatal(err)
	}
	parent.Status = tasks.StatusSucceeded
	_ = tasks.SaveRecord(p, parent)
	proposal := `{"version":1,"planId":"plan-risk","parentNodeId":"inspect","parentTaskId":"` + parent.ID + `","reason":"danger","evidence":["evidence"],"confidence":0.9,"actions":[{"id":"destroy","objective":"destroy production","risk":"destructive-critical"}]}`
	if err := os.WriteFile(parent.ReplanProposalPath, []byte(proposal), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ProcessRecord(p, parent)
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied {
		t.Fatalf("dangerous proposal must not auto-apply: %#v", res)
	}
}
