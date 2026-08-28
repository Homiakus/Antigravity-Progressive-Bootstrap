package loop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/homiakus/agctl/internal/engineering"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
)

func testPaths(t *testing.T) paths.Paths {
	t.Helper()
	root := t.TempDir()
	p := paths.Paths{Home: root, GeminiRoot: filepath.Join(root, ".gemini"), ConfigRoot: filepath.Join(root, ".gemini", "config")}
	p.GlobalSkillsRoot = filepath.Join(p.ConfigRoot, "skills")
	p.CLISkillsRoot = filepath.Join(p.GeminiRoot, "antigravity-cli", "skills")
	p.GlobalMCP = filepath.Join(p.ConfigRoot, "mcp_config.json")
	p.GlobalHooks = filepath.Join(p.ConfigRoot, "hooks.json")
	p.GlobalRule = filepath.Join(p.GeminiRoot, "GEMINI.md")
	p.CLISettings = filepath.Join(p.GeminiRoot, "antigravity-cli", "settings.json")
	p.AppRoot = filepath.Join(p.ConfigRoot, "agctl")
	p.BinRoot = filepath.Join(p.AppRoot, "bin")
	p.BackupsRoot = filepath.Join(p.AppRoot, "backups")
	p.StateRoot = filepath.Join(p.AppRoot, "state")
	p.RouterConfig = filepath.Join(p.AppRoot, "router.json")
	p.LoopConfig = filepath.Join(p.AppRoot, "loop.json")
	p.LogRoot = filepath.Join(p.AppRoot, "logs")
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	return p
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

func TestStateLifecycle(t *testing.T) {
	p := testPaths(t)
	chdir(t, p.Home)
	in := model.PreInvocationInput{CommonHookInput: model.CommonHookInput{ConversationID: "c1"}, InitialNumSteps: 10}
	st, err := EnsureTaskState(p, in)
	if err != nil {
		t.Fatal(err)
	}
	if st.TaskID == "" || st.Complete {
		t.Fatalf("bad initial state: %+v", st)
	}
	st2, err := EnsureTaskState(p, model.PreInvocationInput{CommonHookInput: model.CommonHookInput{ConversationID: "c1"}, InitialNumSteps: 20})
	if err != nil {
		t.Fatal(err)
	}
	if st2.TaskID != st.TaskID {
		t.Fatal("active incomplete task was overwritten")
	}
	if err := MarkComplete(p, "c1", st.TaskID, "done", []string{"go test ./... passed"}); err != nil {
		t.Fatal(err)
	}
	done, ok, err := LoadState(p, "c1")
	if err != nil || !ok {
		t.Fatal(err)
	}
	if !done.Complete || !done.Verified || len(done.Verification) != 1 {
		t.Fatalf("completion not recorded: %+v", done)
	}
	next, err := EnsureTaskState(p, model.PreInvocationInput{CommonHookInput: model.CommonHookInput{ConversationID: "c1"}, InitialNumSteps: 30})
	if err != nil {
		t.Fatal(err)
	}
	if next.TaskID == st.TaskID {
		t.Fatal("new user turn did not create a new task")
	}
}

func TestCompleteRequiresEvidence(t *testing.T) {
	p := testPaths(t)
	chdir(t, p.Home)
	st, err := EnsureTaskState(p, model.PreInvocationInput{CommonHookInput: model.CommonHookInput{ConversationID: "c"}, InitialNumSteps: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := MarkComplete(p, "c", st.TaskID, "done", nil); err == nil {
		t.Fatal("expected verification error")
	}
}

func TestPersistedWorkspaceOverridesAmbientCWD(t *testing.T) {
	p := testPaths(t)
	managedAmbient := t.TempDir()
	if err := os.WriteFile(filepath.Join(managedAmbient, engineering.PlanFileName), []byte("# ambient plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	chdir(t, managedAmbient)
	unmanagedWorkspace := t.TempDir()
	st, err := EnsureTaskState(p, model.PreInvocationInput{
		CommonHookInput: model.CommonHookInput{ConversationID: "workspace-bound", WorkspacePaths: []string{unmanagedWorkspace}},
		InitialNumSteps: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(st.WorkspacePaths) != 1 || st.WorkspacePaths[0] != unmanagedWorkspace {
		t.Fatalf("workspace identity was not persisted: %+v", st.WorkspacePaths)
	}
	if err := MarkComplete(p, "workspace-bound", st.TaskID, "done", []string{"synthetic verification"}); err != nil {
		t.Fatalf("ambient cwd leaked into completion policy: %v", err)
	}
}

func TestCompletionInjectionIncludesLivingPlanProtocol(t *testing.T) {
	msg := CompletionInjection("agctl", model.TaskState{ConversationID: "c", TaskID: "generated-task"})
	for _, want := range []string{
		"MASTER_PLAN.md is the only execution roadmap",
		"SELECT exactly one T-XXX",
		"Unexpected substantial problems",
		"mutation",
		"push-main",
		"checkpoint",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("completion injection missing %q", want)
		}
	}
}

func TestManagedCompletionStoresPlanDigest(t *testing.T) {
	p := testPaths(t)
	workspace := t.TempDir()
	chdir(t, workspace)
	plan := "# MASTER PLAN\n\n### F-027 — process\n**Status:** Resolved.\n\n### T-027 — process\n**Status:** DONE.\n"
	if err := os.WriteFile(filepath.Join(workspace, engineering.PlanFileName), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := EnsureTaskState(p, model.PreInvocationInput{CommonHookInput: model.CommonHookInput{ConversationID: "managed", WorkspacePaths: []string{workspace}}, InitialNumSteps: 1})
	if err != nil {
		t.Fatal(err)
	}
	evidence := []string{
		"task:T-027",
		"preflight:recorded",
		"characterization:recorded",
		"edge-space:recorded",
		"tests:passed",
		"mutation:passed semantic omission sentinel",
		"race:n/a: no concurrent behavior changed",
		"static:go vet passed",
		"security:reviewed",
		"compatibility:reviewed",
		"performance:n/a: no hot path changed",
		"findings:F-027",
		"self-review:passed",
		"plan-reconcile:updated",
		"process-review:updated",
		"push-main:verified",
		"checkpoint:recorded",
	}
	if err := MarkComplete(p, "managed", st.TaskID, "done", evidence); err != nil {
		t.Fatal(err)
	}
	done, ok, err := LoadState(p, "managed")
	if err != nil || !ok {
		t.Fatal(err)
	}
	if last := done.Verification[len(done.Verification)-1]; !strings.HasPrefix(last, "plan-digest:") {
		t.Fatalf("missing plan digest: %v", done.Verification)
	}
}
