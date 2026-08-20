package hooks

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/agents"
	"github.com/homiakus/agctl/internal/execx"
	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/loop"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/risk"
	"github.com/homiakus/agctl/internal/router"
	"github.com/homiakus/agctl/internal/telemetry"
)

const (
	RouterHookName = "agctl-adaptive-router"
	LoopHookName   = "agctl-autonomous-completion"
	ToolHookName   = "agctl-no-prompt-tools"
)

func Install(p paths.Paths, executable string) error {
	cfg, err := jsonx.ReadMap(p.GlobalHooks)
	if err != nil {
		return err
	}
	loopCfg, _ := loop.Load(p)
	loopEnabled := loopCfg.Enabled
	cmd := quoteCommand(executable)
	cfg[RouterHookName] = map[string]any{
		"enabled":       true,
		"PreInvocation": []any{map[string]any{"type": "command", "command": cmd + " hook router-pre-invocation", "timeout": 10}},
	}
	cfg[LoopHookName] = map[string]any{
		"enabled":       loopEnabled,
		"PreInvocation": []any{map[string]any{"type": "command", "command": cmd + " hook loop-pre-invocation", "timeout": 10}},
		"Stop":          []any{map[string]any{"type": "command", "command": cmd + " hook loop-stop", "timeout": 10}},
	}
	cfg[ToolHookName] = map[string]any{
		"enabled": loopEnabled,
		"PreToolUse": []any{map[string]any{
			"matcher": "*",
			"hooks":   []any{map[string]any{"type": "command", "command": cmd + " hook loop-pre-tool", "timeout": 10}},
		}},
	}
	if err := jsonx.WriteAtomic(p.GlobalHooks, cfg, p.BackupsRoot); err != nil {
		return err
	}
	return installCLIManagedPlugin(p, executable, cfg)
}

func DisableLoopHooks(p paths.Paths) error { return setManagedLoopEnabled(p, false) }
func EnableLoopHooks(p paths.Paths) error  { return setManagedLoopEnabled(p, true) }

func setManagedLoopEnabled(p paths.Paths, enabled bool) error {
	cfg, err := jsonx.ReadMap(p.GlobalHooks)
	if err != nil {
		return err
	}
	for _, name := range []string{LoopHookName, ToolHookName} {
		if raw, ok := cfg[name]; ok {
			if m, ok := raw.(map[string]any); ok {
				m["enabled"] = enabled
				cfg[name] = m
			}
		}
	}
	if err := jsonx.WriteAtomic(p.GlobalHooks, cfg, p.BackupsRoot); err != nil {
		return err
	}
	return syncCLIManagedHookState(p, cfg)
}

const managedCLIPluginName = "agctl-control-plane"

func installCLIManagedPlugin(p paths.Paths, executable string, hookCfg map[string]any) error {
	if strings.TrimSpace(p.AppRoot) == "" {
		return nil
	}
	stage := filepath.Join(p.AppRoot, "cli-plugin", managedCLIPluginName)
	if err := os.RemoveAll(stage); err != nil {
		return err
	}
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return err
	}
	manifest := map[string]any{
		"$schema":     "https://antigravity.google/schemas/v1/plugin.json",
		"name":        managedCLIPluginName,
		"description": "agctl managed routing, semantic-risk and verified-completion hooks",
	}
	if err := jsonx.WriteAtomic(filepath.Join(stage, "plugin.json"), manifest, ""); err != nil {
		return err
	}
	if err := jsonx.WriteAtomic(filepath.Join(stage, "hooks.json"), managedHookSubset(hookCfg), ""); err != nil {
		return err
	}

	// The CLI documentation defines `agy plugin install <path>` as the supported
	// registration path. Prefer it whenever AGY is available so internal plugin
	// registries are updated as well as the on-disk package directory.
	if execx.Exists("agy") {
		_, _ = execx.Run(30*time.Second, "", "agy", "plugin", "uninstall", managedCLIPluginName)
		if out, err := execx.Run(60*time.Second, "", "agy", "plugin", "install", stage); err != nil {
			return fmt.Errorf("register managed CLI plugin: %w: %s", err, strings.TrimSpace(out))
		}
		if out, err := execx.Run(30*time.Second, "", "agy", "plugin", "enable", managedCLIPluginName); err != nil {
			return fmt.Errorf("enable managed CLI plugin: %w: %s", err, strings.TrimSpace(out))
		}
		return nil
	}

	// Tests/minimal environments may not have AGY installed. Keep a filesystem
	// mirror so the package can be inspected, but doctor will report that CLI
	// registration could not be verified until `agy plugin install` is available.
	if strings.TrimSpace(p.CLIPluginsRoot) == "" {
		return nil
	}
	dst := filepath.Join(p.CLIPluginsRoot, managedCLIPluginName)
	return copyManagedPlugin(stage, dst)
}

func managedHookSubset(cfg map[string]any) map[string]any {
	out := map[string]any{}
	for _, name := range []string{RouterHookName, LoopHookName, ToolHookName} {
		if v, ok := cfg[name]; ok {
			out[name] = v
		}
	}
	return out
}

func syncCLIManagedHookState(p paths.Paths, cfg map[string]any) error {
	if strings.TrimSpace(p.CLIPluginsRoot) == "" {
		return nil
	}
	path := filepath.Join(p.CLIPluginsRoot, managedCLIPluginName, "hooks.json")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return jsonx.WriteAtomic(path, managedHookSubset(cfg), p.BackupsRoot)
}

func copyManagedPlugin(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}

func quoteCommand(path string) string {
	if strings.ContainsAny(path, " \t\"") {
		return `"` + strings.ReplaceAll(path, `"`, `\"`) + `"`
	}
	return path
}

func HandleRouterPreInvocation(p paths.Paths, r io.Reader, w io.Writer) error {
	var in model.PreInvocationInput
	if err := decode(r, &in); err != nil {
		return writeJSON(w, model.PreInvocationOutput{})
	}
	cfg, err := router.Load(p)
	if err != nil || !cfg.Enabled {
		return writeJSON(w, model.PreInvocationOutput{})
	}
	// The first model call in each execution attempt gets the route contract.
	// The message itself tells the model not to overuse capabilities.
	if in.InvocationNum != 0 {
		return writeJSON(w, model.PreInvocationOutput{})
	}
	inv := router.Discover(p, in.WorkspacePaths)
	msg := router.Injection(cfg, inv)
	if orchestration := agents.OrchestrationInjection(p, ""); orchestration != "" {
		msg += "\n\n" + orchestration
	}
	if msg == "" {
		return writeJSON(w, model.PreInvocationOutput{})
	}
	_ = telemetry.Record(p, telemetry.Event{Type: "router.pre_invocation", ConversationID: in.ConversationID, Workspace: in.WorkspacePaths, Data: map[string]any{"skills": len(inv.Skills), "mcp": len(inv.MCP), "agents": len(inv.Agents), "plugins": len(inv.Plugins), "workflows": len(inv.Workflows), "sidecars": len(inv.Sidecars)}})
	return writeJSON(w, model.PreInvocationOutput{InjectSteps: []model.InjectStep{{EphemeralMessage: msg}}})
}

func HandleLoopPreInvocation(p paths.Paths, executable string, r io.Reader, w io.Writer) error {
	var in model.PreInvocationInput
	if err := decode(r, &in); err != nil {
		return writeJSON(w, model.PreInvocationOutput{})
	}
	cfg, err := loop.Load(p)
	if err != nil || !cfg.Enabled {
		return writeJSON(w, model.PreInvocationOutput{})
	}
	if in.InvocationNum != 0 {
		return writeJSON(w, model.PreInvocationOutput{})
	}
	st, err := loop.EnsureTaskState(p, in)
	if err != nil {
		return writeJSON(w, model.PreInvocationOutput{})
	}
	msg := loop.CompletionInjection(executable, st)
	_ = telemetry.Record(p, telemetry.Event{Type: "loop.pre_invocation", ConversationID: in.ConversationID, Workspace: in.WorkspacePaths, Data: map[string]any{"taskId": st.TaskID}})
	return writeJSON(w, model.PreInvocationOutput{InjectSteps: []model.InjectStep{{EphemeralMessage: msg}}})
}

func HandleLoopPreTool(p paths.Paths, r io.Reader, w io.Writer) error {
	var in model.PreToolUseInput
	if err := decode(r, &in); err != nil {
		return writeJSON(w, model.PreToolUseOutput{Decision: "ask", Reason: "Invalid PreToolUse hook input; defer to Antigravity permission policy."})
	}
	cfg, err := loop.Load(p)
	if err != nil || !cfg.Enabled {
		// Do not add an extra prompt when the loop is disabled; defer to ordinary permissions.
		return writeJSON(w, model.PreToolUseOutput{Decision: "ask", Reason: "Autonomous Completion Loop is disabled."})
	}
	if cfg.PermissionMode == loop.PermissionUnrestricted {
		return writeJSON(w, model.PreToolUseOutput{Decision: "allow", Reason: "Unrestricted autonomous loop auto-approved tool call."})
	}

	if in.ToolCall.Name == "ask_question" {
		return writeJSON(w, model.PreToolUseOutput{
			Decision: "deny",
			Reason:   "Guarded autonomous mode suppresses routine user questions. Infer a reasonable reversible choice and continue; record a hard blocker only if the decision is genuinely non-inferable and irreversible.",
		})
	}
	if in.ToolCall.Name == "ask_permission" {
		return writeJSON(w, model.PreToolUseOutput{
			Decision: "deny",
			Reason:   "Guarded autonomous mode does not pause for routine permission requests. Use already granted capabilities or choose a safe alternative.",
		})
	}

	decision := risk.ClassifyConfigured(p, in.ToolCall, risk.Context{
		PermissionMode: cfg.PermissionMode,
		Workspaces:     in.WorkspacePaths,
		Home:           p.Home,
	})
	out := model.PreToolUseOutput{Decision: decision.Decision, Reason: fmt.Sprintf("agctl semantic risk=%s: %s", decision.Risk, decision.Reason)}
	_ = telemetry.Record(p, telemetry.Event{Type: "tool.permission", ConversationID: in.ConversationID, Tool: in.ToolCall.Name, Decision: out.Decision, Risk: decision.Risk, Reason: decision.Reason, Workspace: in.WorkspacePaths})
	return writeJSON(w, out)
}

func HandleLoopStop(p paths.Paths, r io.Reader, w io.Writer) error {
	var in model.StopInput
	if err := decode(r, &in); err != nil {
		return emitStop(p, w, in, model.StopOutput{Decision: "stop", Reason: "Invalid Stop hook input; fail-open."})
	}
	cfg, err := loop.Load(p)
	if err != nil || !cfg.Enabled {
		return emitStop(p, w, in, model.StopOutput{Decision: "stop"})
	}

	st, exists, err := loop.LoadState(p, in.ConversationID)
	if err != nil {
		return emitStop(p, w, in, model.StopOutput{Decision: "stop", Reason: "Unable to read autonomous state; fail-open."})
	}

	if cfg.MaxExecutions > 0 && in.ExecutionNum >= cfg.MaxExecutions {
		return emitStop(p, w, in, model.StopOutput{Decision: "stop", Reason: fmt.Sprintf("Autonomous execution watchdog reached %d attempts.", cfg.MaxExecutions)})
	}
	if exists && st.HardBlocker {
		return emitStop(p, w, in, model.StopOutput{Decision: "stop", Reason: "Recorded genuine external blocker: " + st.Summary})
	}
	if exists && st.Complete && st.Verified && len(st.Verification) > 0 && in.FullyIdle {
		return emitStop(p, w, in, model.StopOutput{Decision: "stop", Reason: "Verified completion gate satisfied and all background work is idle."})
	}
	if externalFatal(in.Error) {
		return emitStop(p, w, in, model.StopOutput{Decision: "stop", Reason: "External authentication/quota/platform blocker detected: " + in.Error})
	}

	stateHint := "No active completion state exists; reconstruct the Definition of Done and create/maintain the task state through agctl."
	if exists {
		stateHint = fmt.Sprintf("Current gate: complete=%v verified=%v hardBlocker=%v verificationItems=%d.", st.Complete, st.Verified, st.HardBlocker, len(st.Verification))
	}
	reason := fmt.Sprintf(`AUTONOMOUS COMPLETION LOOP: premature stop rejected.

Execution attempt: %d
Termination reason: %s
Fully idle: %v
%s

Continue without asking the user for routine guidance:
1. re-read the original task and identify missing requirements;
2. if the previous approach failed, use the actual error evidence and change the hypothesis/implementation;
3. implement the next missing requirement;
4. run targeted verification, then appropriate regression checks;
5. fix failures and re-run them;
6. only after verified completion, run agctl state complete with real verification evidence and then provide the final answer.

Do not stop merely to report progress. Do not label recoverable engineering failures as external blockers.`, in.ExecutionNum, in.TerminationReason, in.FullyIdle, stateHint)
	return emitStop(p, w, in, model.StopOutput{Decision: "continue", Reason: reason})
}

func emitStop(p paths.Paths, w io.Writer, in model.StopInput, out model.StopOutput) error {
	_ = telemetry.Record(p, telemetry.Event{Type: "loop.stop", ConversationID: in.ConversationID, Decision: out.Decision, Reason: out.Reason, Workspace: in.WorkspacePaths, Data: map[string]any{"executionNum": in.ExecutionNum, "terminationReason": in.TerminationReason}})
	return writeJSON(w, out)
}

func decode(r io.Reader, v any) error {
	dec := json.NewDecoder(bufio.NewReader(r))
	return dec.Decode(v)
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func stringArg(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprint(v)
	}
	// tolerate lower-case variants from wrappers
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return fmt.Sprint(v)
		}
	}
	return ""
}

var catastrophicPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(^|\s)diskpart(\.exe)?(\s|$)`),
	regexp.MustCompile(`(?i)(^|\s)format(\.com)?(\s|$)`),
	regexp.MustCompile(`(?i)(^|\s)bcdedit(\.exe)?(\s|$)`),
	regexp.MustCompile(`(?i)(^|\s)shutdown(\.exe)?(\s|$)`),
	regexp.MustCompile(`(?i)git\s+push\b[^\r\n]*(--force|-f)(\s|$)`),
	regexp.MustCompile(`(?i)git\s+reset\s+--hard\b`),
	regexp.MustCompile(`(?i)git\s+clean\s+-[^\r\n]*[xX]`),
	regexp.MustCompile(`(?i)rm\s+-rf\s+/(\s|$|\*)`),
	regexp.MustCompile(`(?i)Remove-Item[^\r\n]+-[Rr]ecurse[^\r\n]+-[Ff]orce[^\r\n]+[A-Za-z]:\\(?:\*|$)`),
}

func catastrophicCommand(s string) bool {
	for _, re := range catastrophicPatterns {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

func fileMutationTarget(call model.ToolCall) (string, bool) {
	switch call.Name {
	case "write_to_file", "replace_file_content", "multi_replace_file_content":
		t := stringArg(call.Args, "TargetFile")
		return t, t != ""
	default:
		return "", false
	}
}

func sensitiveOutsideWorkspace(target string, workspaces []string, home string) bool {
	if target == "" {
		return false
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	abs = filepath.Clean(abs)
	for _, w := range workspaces {
		wa, err := filepath.Abs(w)
		if err == nil && isWithin(abs, filepath.Clean(wa)) {
			return false
		}
	}
	sensitive := []string{".ssh", ".aws", ".gnupg", ".kube", filepath.Join(".config", "gcloud")}
	for _, rel := range sensitive {
		s := filepath.Join(home, rel)
		if isWithin(abs, s) {
			return true
		}
	}
	return false
}

func isWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func externalFatal(s string) bool {
	s = strings.ToLower(s)
	if strings.TrimSpace(s) == "" {
		return false
	}
	needles := []string{
		"quota exceeded", "rate limit", "billing", "account suspended", "authentication required",
		"not authenticated", "unauthorized", "invalid api key", "missing api key", "login required",
		"requires user authorization", "permission denied by organization policy",
	}
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func InstalledBinaryName() string {
	if runtime.GOOS == "windows" {
		return "agctl.exe"
	}
	return "agctl"
}
