package doctor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/agents"
	"github.com/homiakus/agctl/internal/capability"
	"github.com/homiakus/agctl/internal/execx"
	"github.com/homiakus/agctl/internal/hooks"
	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/loop"
	"github.com/homiakus/agctl/internal/mcp"
	"github.com/homiakus/agctl/internal/mcpprobe"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/permissions"
	"github.com/homiakus/agctl/internal/planner"
	"github.com/homiakus/agctl/internal/plugin"
	"github.com/homiakus/agctl/internal/replan"
	"github.com/homiakus/agctl/internal/router"
	"github.com/homiakus/agctl/internal/securityaudit"
	"github.com/homiakus/agctl/internal/sidecar"
	"github.com/homiakus/agctl/internal/skills"
	"github.com/homiakus/agctl/internal/tasks"
)

type Finding struct {
	Level   string
	Area    string
	Message string
}

type Report struct{ Findings []Finding }

func (r *Report) add(level, area, msg string) {
	r.Findings = append(r.Findings, Finding{level, area, msg})
}
func (r Report) HasErrors() bool {
	for _, f := range r.Findings {
		if f.Level == "ERROR" {
			return true
		}
	}
	return false
}

func Run(p paths.Paths, workspace string) Report {
	return RunAdvanced(p, workspace, false)
}

func RunAdvanced(p paths.Paths, workspace string, probeMCP bool) Report {
	var r Report
	commands := []string{"agy", "git", "node", "npx", "uvx", "gh", "docker", "go", "gopls"}
	for _, c := range commands {
		if execx.Exists(c) {
			r.add("OK", "prereq", c+" installed")
		} else {
			r.add("WARN", "prereq", c+" missing")
		}
	}

	checkJSON := func(area, path string) {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			r.add("WARN", area, "missing: "+path)
			return
		}
		var v any
		b, err := os.ReadFile(path)
		if err != nil {
			r.add("ERROR", area, err.Error())
			return
		}
		if len(bytes.TrimSpace(b)) == 0 {
			r.add("WARN", area, "empty: "+path)
			return
		}
		if err := json.Unmarshal(b, &v); err != nil {
			r.add("ERROR", area, "invalid JSON: "+err.Error())
			return
		}
		r.add("OK", area, "valid: "+path)
	}
	checkJSON("settings", p.CLISettings)
	checkJSON("mcp", p.GlobalMCP)
	checkJSON("config", p.GlobalConfig)
	checkJSON("hooks", p.GlobalHooks)
	checkJSON("router", p.RouterConfig)
	checkJSON("loop", p.LoopConfig)

	if b, err := os.ReadFile(p.GlobalRule); err == nil {
		n := len([]rune(string(b)))
		if n > 12000 {
			r.add("WARN", "rules", fmt.Sprintf("GEMINI.md %d/12000 chars; exceeds documented per-file limit", n))
		} else {
			r.add("OK", "rules", fmt.Sprintf("GEMINI.md %d/12000 chars", n))
		}
	} else {
		r.add("WARN", "rules", "global GEMINI.md missing")
	}

	rcfg, err := router.Load(p)
	if err != nil {
		r.add("ERROR", "router", err.Error())
	} else {
		r.add("OK", "router", fmt.Sprintf("enabled=%v mode=%s", rcfg.Enabled, rcfg.Mode))
	}
	lcfg, err := loop.Load(p)
	if err != nil {
		r.add("ERROR", "loop", err.Error())
	} else {
		r.add("OK", "loop", fmt.Sprintf("enabled=%v mode=%s max=%d", lcfg.Enabled, lcfg.PermissionMode, lcfg.MaxExecutions))
	}

	rawHooks, err := jsonx.ReadMap(p.GlobalHooks)
	if err == nil {
		for _, name := range []string{hooks.RouterHookName, hooks.LoopHookName, hooks.ToolHookName} {
			if _, ok := rawHooks[name]; ok {
				r.add("OK", "hooks", name+" present")
			} else {
				r.add("WARN", "hooks", name+" missing")
			}
		}
	}
	cliManagedPlugin := filepath.Join(p.CLIPluginsRoot, "agctl-control-plane")
	if _, err := os.Stat(filepath.Join(cliManagedPlugin, "hooks.json")); err == nil {
		r.add("OK", "hooks-cli", "agctl-control-plane CLI hook plugin present")
		if execx.Exists("agy") {
			if out, e := execx.Run(30*time.Second, "", "agy", "plugin", "list"); e != nil {
				r.add("WARN", "hooks-cli", "could not verify AGY plugin registry: "+e.Error())
			} else if !strings.Contains(out, "agctl-control-plane") {
				r.add("WARN", "hooks-cli", "managed hook plugin exists on disk but is not visible in `agy plugin list`; run install full to re-register it")
			} else {
				r.add("OK", "hooks-cli", "agctl-control-plane registered in AGY plugin registry")
			}
		}
	} else if execx.Exists("agy") {
		r.add("WARN", "hooks-cli", "managed CLI hook plugin missing; headless AGY tasks may not receive agctl lifecycle hooks")
	}

	ss, err := skills.List(p)
	if err != nil {
		r.add("ERROR", "skills", err.Error())
	} else {
		r.add("OK", "skills", fmt.Sprintf("%d global skills detected", len(ss)))
	}
	requiredSkills := []string{"adaptive-tool-router", "autonomous-engineering", "autonomous-completion-loop", "editorial-quality-director"}
	installed := map[string]bool{}
	for _, s := range ss {
		installed[s.Name] = true
	}
	for _, s := range requiredSkills {
		if installed[s] {
			r.add("OK", "skills", s+" installed")
		} else {
			r.add("WARN", "skills", s+" missing")
		}
	}

	agentItems, _ := agents.List(p, workspace)
	if len(agentItems) == 0 {
		r.add("WARN", "agents", "no custom agents/subagents detected")
	} else {
		r.add("OK", "agents", fmt.Sprintf("%d custom agents detected", len(agentItems)))
	}
	for _, line := range agents.Doctor(p, workspace) {
		if strings.Contains(line, "INVALID") || strings.Contains(line, "UNSUPPORTED") {
			r.add("WARN", "agents", line)
		}
	}

	plugins := plugin.Doctor(p, workspace)
	validPlugins := 0
	for _, pl := range plugins {
		if pl.Valid {
			validPlugins++
		}
		if len(pl.Issues) > 0 {
			r.add("WARN", "plugins", pl.Name+": "+strings.Join(pl.Issues, "; "))
		}
	}
	r.add("OK", "plugins", fmt.Sprintf("%d plugins detected (%d valid)", len(plugins), validPlugins))

	sidecars, sidecarErr := sidecar.List(p)
	if sidecarErr != nil {
		r.add("WARN", "sidecars", sidecarErr.Error())
	} else {
		validSidecars := 0
		for _, sc := range sidecars {
			if sc.Valid {
				validSidecars++
			} else {
				r.add("WARN", "sidecars", sc.ID+": "+sc.Issue)
			}
		}
		r.add("OK", "sidecars", fmt.Sprintf("%d sidecars detected (%d valid)", len(sidecars), validSidecars))
	}

	reg, capErr := capability.Build(p, nonEmptyWorkspace(workspace))
	if capErr != nil {
		r.add("WARN", "capability", capErr.Error())
	} else {
		r.add("OK", "capability", capability.Summary(reg))
	}

	if cfg, err := tasks.LoadConfig(p); err != nil {
		r.add("WARN", "tasks", err.Error())
	} else {
		r.add("OK", "tasks", fmt.Sprintf("maxParallel=%d cpu=%d buildSlots=%d browserSlots=%d", cfg.MaxParallel, cfg.CPUWeight, cfg.BuildSlots, cfg.BrowserSlots))
	}

	if plans, err := planner.List(p); err == nil {
		revisions, dynamic, blocked := 0, 0, 0
		for _, pl := range plans {
			revisions += pl.Revision
			dynamic += pl.DynamicNodeCount
			if pl.Status == "blocked" {
				blocked++
				r.add("WARN", "plans", pl.ID+": blocked: "+pl.BlockReason)
			}
		}
		r.add("OK", "plans", fmt.Sprintf("plans=%d revisions=%d dynamicNodes=%d blocked=%d", len(plans), revisions, dynamic, blocked))
	}

	if cfg, err := replan.LoadConfig(p); err != nil {
		r.add("WARN", "replan", err.Error())
	} else {
		level := "OK"
		if !cfg.Enabled {
			level = "INFO"
		}
		r.add(level, "replan", fmt.Sprintf("enabled=%v maxRevisions=%d maxDynamicNodes=%d maxRepairDepth=%d maxSameFailure=%d minConfidence=%.2f riskMax=%s worktrees=%v", cfg.Enabled, cfg.MaxRevisions, cfg.MaxDynamicNodes, cfg.MaxRepairDepth, cfg.MaxSameFailure, cfg.MinConfidence, cfg.AutoApplyRiskMax, cfg.PreferWorktrees))
		if inbox, e := replan.Inbox(p); e == nil && len(inbox) > 0 {
			r.add("INFO", "replan", fmt.Sprintf("%d pending proposal files", len(inbox)))
		}
	}

	if sec, err := securityaudit.Audit(p, workspace); err != nil {
		r.add("WARN", "security", err.Error())
	} else {
		level := "OK"
		if sec.Score < 70 {
			level = "WARN"
		}
		r.add(level, "security", fmt.Sprintf("score=%d grade=%s findings=%d", sec.Score, sec.Grade, len(sec.Findings)))
		for _, f := range sec.Findings {
			if f.Severity == "critical" || f.Severity == "high" {
				r.add("WARN", "security", fmt.Sprintf("%s/%s: %s", f.Area, f.ID, f.Message))
			}
		}
	}

	if entries, err := os.ReadDir(p.LocksRoot); err == nil {
		locks := 0
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				locks++
			}
		}
		r.add("OK", "provenance", fmt.Sprintf("%d lock/provenance records", locks))
	}

	for _, line := range mcp.Doctor(p, workspace) {
		if strings.Contains(line, "MISSING") || strings.Contains(line, "INVALID") || strings.HasPrefix(line, "ERROR") {
			r.add("WARN", "mcp", line)
		} else {
			r.add("OK", "mcp", line)
		}
	}
	names, _ := mcp.Names(p, workspace)
	if contains(names, "chrome-devtools") && contains(names, "chrome-devtools-mcp") {
		r.add("WARN", "mcp", "duplicate Chrome DevTools capability: chrome-devtools and chrome-devtools-mcp")
	}
	if contains(names, "memory") {
		r.add("WARN", "mcp", "memory is the MCP reference server; keep it experimental rather than relying on it as production persistence")
	}
	if probeMCP {
		for _, pr := range mcpprobe.ProbeAll(p, workspace, 12*time.Second) {
			if pr.OK {
				r.add("OK", "mcp-probe", fmt.Sprintf("%s %s %s tools=%d resources=%d prompts=%d latency=%s", pr.Name, pr.Transport, pr.Protocol, len(pr.Tools), len(pr.Resources), len(pr.Prompts), time.Duration(pr.LatencyMS)*time.Millisecond))
			} else {
				r.add("WARN", "mcp-probe", pr.Name+": "+pr.Error)
			}
		}
	}

	pa, err := permissions.AuditSettings(p)
	if err != nil {
		r.add("ERROR", "permissions", err.Error())
	} else {
		r.add("OK", "permissions", fmt.Sprintf("toolPermission=%s artifactReview=%s", pa.ToolPermission, pa.ArtifactReview))
		for _, c := range pa.Conflicts {
			r.add("WARN", "permissions", c)
		}
	}
	if workspace != "" {
		if _, err := os.Stat(paths.WorkspaceMCP(workspace)); os.IsNotExist(err) {
			r.add("INFO", "workspace", "project MCP missing (valid if global-only): "+paths.WorkspaceMCP(workspace))
		}
	}
	sort.SliceStable(r.Findings, func(i, j int) bool { return r.Findings[i].Area < r.Findings[j].Area })
	return r
}

func SelfTest() error {
	root, err := os.MkdirTemp("", "agctl-selftest-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	p := paths.Paths{
		Home: root, GeminiRoot: filepath.Join(root, ".gemini"), ConfigRoot: filepath.Join(root, ".gemini", "config"),
		GlobalSkillsRoot: filepath.Join(root, ".gemini", "config", "skills"), CLISkillsRoot: filepath.Join(root, ".gemini", "antigravity-cli", "skills"),
		GlobalMCP: filepath.Join(root, ".gemini", "config", "mcp_config.json"), GlobalHooks: filepath.Join(root, ".gemini", "config", "hooks.json"), GlobalRule: filepath.Join(root, ".gemini", "GEMINI.md"),
		CLISettings: filepath.Join(root, ".gemini", "antigravity-cli", "settings.json"), AppRoot: filepath.Join(root, ".gemini", "config", "agctl"),
	}
	p.BinRoot = filepath.Join(p.AppRoot, "bin")
	p.BackupsRoot = filepath.Join(p.AppRoot, "backups")
	p.StateRoot = filepath.Join(p.AppRoot, "state")
	p.RouterConfig = filepath.Join(p.AppRoot, "router.json")
	p.LoopConfig = filepath.Join(p.AppRoot, "loop.json")
	p.LogRoot = filepath.Join(p.AppRoot, "logs")
	p.GlobalAgentsRoot = filepath.Join(p.ConfigRoot, "agents")
	p.GlobalPluginsRoot = filepath.Join(p.ConfigRoot, "plugins")
	p.CapabilityDB = filepath.Join(p.AppRoot, "capabilities.json")
	p.PlansRoot = filepath.Join(p.AppRoot, "plans")
	p.SecurityRoot = filepath.Join(p.AppRoot, "security")
	p.ReplanRoot = filepath.Join(p.AppRoot, "replan")
	p.ReplanConfig = filepath.Join(p.ReplanRoot, "config.json")
	p.ReplanInbox = filepath.Join(p.ReplanRoot, "inbox")
	p.ReplanArchive = filepath.Join(p.ReplanRoot, "archive")
	p.LocksRoot = filepath.Join(p.AppRoot, "locks")
	p.TasksRoot = filepath.Join(p.AppRoot, "tasks")
	p.TaskConfig = filepath.Join(p.AppRoot, "task-supervisor.json")
	p.TelemetryRoot = filepath.Join(p.AppRoot, "telemetry")
	if err := p.Ensure(); err != nil {
		return err
	}
	if err := router.Enable(p, router.ModeTransparent); err != nil {
		return err
	}
	if _, err := loop.EnableProfile(p, "deep"); err != nil {
		return err
	}
	if err := replan.SaveConfig(p, replan.DefaultConfig()); err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Join(p.GlobalSkillsRoot, "adaptive-tool-router"), 0o755)
	_ = os.WriteFile(filepath.Join(p.GlobalSkillsRoot, "adaptive-tool-router", "SKILL.md"), []byte("---\ndescription: test\n---\n"), 0o644)
	_ = jsonx.WriteAtomic(p.GlobalMCP, map[string]any{"mcpServers": map[string]any{"context7": map[string]any{"command": "npx"}}}, "")

	pre := model.PreInvocationInput{CommonHookInput: model.CommonHookInput{ConversationID: "conv-test", WorkspacePaths: []string{root}}, InvocationNum: 0, InitialNumSteps: 5}
	b, _ := json.Marshal(pre)
	var out bytes.Buffer
	if err := hooks.HandleRouterPreInvocation(p, bytes.NewReader(b), &out); err != nil {
		return err
	}
	var rout model.PreInvocationOutput
	if err := json.Unmarshal(out.Bytes(), &rout); err != nil {
		return err
	}
	if len(rout.InjectSteps) == 0 {
		return fmt.Errorf("router pre-invocation produced no injection")
	}

	out.Reset()
	if err := hooks.HandleLoopPreInvocation(p, "agctl", bytes.NewReader(b), &out); err != nil {
		return err
	}
	var lout model.PreInvocationOutput
	if err := json.Unmarshal(out.Bytes(), &lout); err != nil {
		return err
	}
	if len(lout.InjectSteps) == 0 {
		return fmt.Errorf("loop pre-invocation produced no injection")
	}

	stop := model.StopInput{CommonHookInput: model.CommonHookInput{ConversationID: "conv-test", WorkspacePaths: []string{root}}, ExecutionNum: 1, TerminationReason: "model_stop", FullyIdle: true}
	sb, _ := json.Marshal(stop)
	out.Reset()
	if err := hooks.HandleLoopStop(p, bytes.NewReader(sb), &out); err != nil {
		return err
	}
	var so model.StopOutput
	if err := json.Unmarshal(out.Bytes(), &so); err != nil {
		return err
	}
	if so.Decision != "continue" {
		return fmt.Errorf("premature stop was not rejected: %s", out.String())
	}

	st, ok, err := loop.LoadState(p, "conv-test")
	if err != nil || !ok {
		return fmt.Errorf("state missing: %v", err)
	}
	if err := loop.MarkComplete(p, "conv-test", st.TaskID, "selftest", []string{"synthetic verification"}); err != nil {
		return err
	}
	out.Reset()
	if err := hooks.HandleLoopStop(p, bytes.NewReader(sb), &out); err != nil {
		return err
	}
	if err := json.Unmarshal(out.Bytes(), &so); err != nil {
		return err
	}
	if so.Decision == "continue" {
		return fmt.Errorf("verified completion still rejected")
	}

	tool := model.PreToolUseInput{CommonHookInput: model.CommonHookInput{ConversationID: "conv-test", WorkspacePaths: []string{root}}, ToolCall: model.ToolCall{Name: "run_command", Args: map[string]any{"CommandLine": "go test ./...", "Cwd": root}}, StepIdx: 1}
	tb, _ := json.Marshal(tool)
	out.Reset()
	if err := hooks.HandleLoopPreTool(p, bytes.NewReader(tb), &out); err != nil {
		return err
	}
	var to model.PreToolUseOutput
	if err := json.Unmarshal(out.Bytes(), &to); err != nil {
		return err
	}
	if to.Decision != "allow" {
		return fmt.Errorf("normal tool not auto-allowed")
	}

	// Semantic MCP destructive action must be denied in guarded mode.
	danger := model.PreToolUseInput{CommonHookInput: model.CommonHookInput{ConversationID: "conv-test", WorkspacePaths: []string{root}}, ToolCall: model.ToolCall{Name: "mcp__database__drop_database", Args: map[string]any{"name": "production"}}, StepIdx: 2}
	db, _ := json.Marshal(danger)
	out.Reset()
	if err := hooks.HandleLoopPreTool(p, bytes.NewReader(db), &out); err != nil {
		return err
	}
	if err := json.Unmarshal(out.Bytes(), &to); err != nil {
		return err
	}
	if to.Decision != "deny" {
		return fmt.Errorf("destructive MCP tool was not denied: %s", out.String())
	}

	// Control-plane planner must build a valid persisted DAG.
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module selftest.local/x\n\ngo 1.23\n"), 0o644); err != nil {
		return err
	}
	pl, err := planner.Create(p, "Проведи аудит Go сервиса, исправь найденные ошибки и проверь тестами", root)
	if err != nil {
		return err
	}
	if len(pl.Nodes) < 4 {
		return fmt.Errorf("planner produced insufficient DAG: %d nodes", len(pl.Nodes))
	}

	// Adaptive replanning must supersede a failed node and rewire downstream work.
	synth := model.ExecutionPlan{ID: "selftest-replan", Prompt: "repair", Workspace: root, Status: "active", Nodes: []model.PlanNode{{ID: "implement"}, {ID: "review", DependsOn: []string{"implement"}}}}
	if err := planner.Save(p, synth); err != nil {
		return err
	}
	failed, err := tasks.AddAdvanced(p, tasks.Spec{Prompt: "synthetic failing implementation", Workspace: root, PlanID: synth.ID, NodeID: "implement"})
	if err != nil {
		return err
	}
	failed.Status = tasks.StatusFailed
	failed.Error = "synthetic verifier failure"
	if err := tasks.SaveRecord(p, failed); err != nil {
		return err
	}
	downstream, err := tasks.AddAdvanced(p, tasks.Spec{Prompt: "downstream review", Workspace: root, PlanID: synth.ID, NodeID: "review", Dependencies: []string{failed.ID}})
	if err != nil {
		return err
	}
	rp, err := replan.ProcessRecord(p, failed)
	if err != nil {
		return err
	}
	if !rp.Applied || len(rp.CreatedTasks) != 3 {
		return fmt.Errorf("adaptive recovery self-test failed: %#v", rp)
	}
	failedAfter, _ := tasks.Load(p, failed.ID)
	if failedAfter.Status != tasks.StatusSuperseded {
		return fmt.Errorf("adaptive recovery did not supersede failed task")
	}
	downstreamAfter, _ := tasks.Load(p, downstream.ID)
	if len(downstreamAfter.Dependencies) != 1 || downstreamAfter.Dependencies[0] == failed.ID {
		return fmt.Errorf("adaptive recovery did not rewire downstream dependency")
	}

	sec, err := securityaudit.Audit(p, "")
	if err != nil {
		return err
	}
	if sec.Score < 0 || sec.Score > 100 {
		return fmt.Errorf("security score out of range: %d", sec.Score)
	}
	return nil
}

func nonEmptyWorkspace(workspace string) []string {
	if strings.TrimSpace(workspace) == "" {
		return nil
	}
	return []string{workspace}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
