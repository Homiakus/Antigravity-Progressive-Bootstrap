package hooks

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/homiakus/agctl/internal/loop"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
)

func hookPaths(t *testing.T) paths.Paths {
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
func encode(v any) *bytes.Reader { b, _ := json.Marshal(v); return bytes.NewReader(b) }

func TestStopRejectsPrematureAndAcceptsVerified(t *testing.T) {
	p := hookPaths(t)
	if _, err := loop.EnableProfile(p, "deep"); err != nil {
		t.Fatal(err)
	}
	pre := model.PreInvocationInput{CommonHookInput: model.CommonHookInput{ConversationID: "c"}, InitialNumSteps: 1}
	var out bytes.Buffer
	if err := HandleLoopPreInvocation(p, "agctl", encode(pre), &out); err != nil {
		t.Fatal(err)
	}
	st, ok, err := loop.LoadState(p, "c")
	if err != nil || !ok {
		t.Fatal("missing state")
	}
	stop := model.StopInput{CommonHookInput: model.CommonHookInput{ConversationID: "c"}, ExecutionNum: 1, TerminationReason: "model_stop", FullyIdle: true}
	out.Reset()
	if err := HandleLoopStop(p, encode(stop), &out); err != nil {
		t.Fatal(err)
	}
	var so model.StopOutput
	if err := json.Unmarshal(out.Bytes(), &so); err != nil {
		t.Fatal(err)
	}
	if so.Decision != "continue" {
		t.Fatalf("got %s", out.String())
	}
	if err := loop.MarkComplete(p, "c", st.TaskID, "done", []string{"test passed"}); err != nil {
		t.Fatal(err)
	}
	// Completion evidence alone is not enough while background work/subagents
	// are still active; Stop exposes fullyIdle specifically for this gate.
	stop.FullyIdle = false
	out.Reset()
	_ = HandleLoopStop(p, encode(stop), &out)
	_ = json.Unmarshal(out.Bytes(), &so)
	if so.Decision != "continue" {
		t.Fatalf("non-idle verified task must continue: %s", out.String())
	}

	stop.FullyIdle = true
	out.Reset()
	_ = HandleLoopStop(p, encode(stop), &out)
	_ = json.Unmarshal(out.Bytes(), &so)
	if so.Decision == "continue" {
		t.Fatalf("verified idle completion rejected: %s", out.String())
	}
}

func TestPreToolGuarded(t *testing.T) {
	p := hookPaths(t)
	if _, err := loop.EnableProfile(p, "deep"); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, tool, cmd, want string }{
		{"normal", "run_command", "go test ./...", "allow"},
		{"destructive", "run_command", "git reset --hard HEAD", "deny"},
		{"question", "ask_question", "", "deny"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := model.PreToolUseInput{CommonHookInput: model.CommonHookInput{ConversationID: "c", WorkspacePaths: []string{t.TempDir()}}, ToolCall: model.ToolCall{Name: tc.tool, Args: map[string]any{"CommandLine": tc.cmd}}}
			var out bytes.Buffer
			if err := HandleLoopPreTool(p, encode(in), &out); err != nil {
				t.Fatal(err)
			}
			var got model.PreToolUseOutput
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Decision != tc.want {
				t.Fatalf("want %s got %s: %s", tc.want, got.Decision, out.String())
			}
		})
	}
}

func TestPreToolMalformedInputDefersToPermissionPolicy(t *testing.T) {
	p := hookPaths(t)
	if _, err := loop.EnableProfile(p, "deep"); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := HandleLoopPreTool(p, bytes.NewBufferString("{broken"), &out); err != nil {
		t.Fatal(err)
	}
	var got model.PreToolUseOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Decision != "ask" {
		t.Fatalf("malformed hook input must defer to permission policy, got %s: %s", got.Decision, out.String())
	}
}
