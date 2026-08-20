package loop

import (
	"path/filepath"
	"testing"

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

func TestStateLifecycle(t *testing.T) {
	p := testPaths(t)
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
	st, err := EnsureTaskState(p, model.PreInvocationInput{CommonHookInput: model.CommonHookInput{ConversationID: "c"}, InitialNumSteps: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := MarkComplete(p, "c", st.TaskID, "done", nil); err == nil {
		t.Fatal("expected verification error")
	}
}
