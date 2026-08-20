package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/agents"
	"github.com/homiakus/agctl/internal/capability"
	"github.com/homiakus/agctl/internal/goal"
	"github.com/homiakus/agctl/internal/mcp"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/plugin"
	"github.com/homiakus/agctl/internal/project"
	"github.com/homiakus/agctl/internal/provenance"
	"github.com/homiakus/agctl/internal/registry"
	"github.com/homiakus/agctl/internal/replan"
	"github.com/homiakus/agctl/internal/risk"
	"github.com/homiakus/agctl/internal/securityaudit"
	"github.com/homiakus/agctl/internal/sidecar"
	"github.com/homiakus/agctl/internal/tasks"
	"github.com/homiakus/agctl/internal/telemetry"
	"github.com/homiakus/agctl/internal/workflow"
	"github.com/homiakus/agctl/internal/worktree"
)

func runSidecars(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl sidecars list|doctor|install-dir|schedule|enable|disable|remove")
	}
	fs := flag.NewFlagSet("sidecars", flag.ContinueOnError)
	id := fs.String("id", "", "sidecar id")
	cron := fs.String("cron", "", "5-field cron expression")
	command := fs.String("command", "", "scheduled command")
	projectID := fs.String("project-id", "", "Antigravity project id for agentapi conversations")
	description := fs.String("description", "", "description")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	rest := fs.Args()
	switch args[0] {
	case "list":
		xs, err := sidecar.List(p)
		if err == nil {
			printJSON(xs)
		}
		return err
	case "doctor":
		printJSON(sidecar.Doctor(p))
		return nil
	case "install-dir":
		if len(rest) < 1 {
			return fmt.Errorf("source directory required")
		}
		x, err := sidecar.InstallDir(p, rest[0], *id)
		if err == nil {
			printJSON(x)
		}
		return err
	case "schedule":
		if *id == "" || *cron == "" || *command == "" {
			return fmt.Errorf("--id, --cron and --command are required")
		}
		x, err := sidecar.CreateSchedule(p, *id, *cron, *command, rest, *description)
		if err == nil {
			printJSON(x)
		}
		return err
	case "enable":
		if len(rest) < 1 {
			return fmt.Errorf("sidecar id required")
		}
		return sidecar.Enable(p, rest[0], *projectID)
	case "disable":
		if len(rest) < 1 {
			return fmt.Errorf("sidecar id required")
		}
		return sidecar.Disable(p, rest[0])
	case "remove":
		if len(rest) < 1 {
			return fmt.Errorf("sidecar id required")
		}
		return sidecar.Remove(p, rest[0])
	default:
		return fmt.Errorf("unknown sidecars command %q", args[0])
	}
}

func runPlugins(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl plugins list|inspect|install-dir|install-git|import-bundle|enable|disable|remove|doctor")
	}
	fs := flag.NewFlagSet("plugins", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "workspace scope; empty=global")
	ref := fs.String("ref", "", "git ref/tag/branch")
	name := fs.String("name", "", "plugin name for imported bundle")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	rest := fs.Args()
	switch args[0] {
	case "list":
		xs, err := plugin.List(p, *workspace)
		if err != nil {
			return err
		}
		printJSON(xs)
		return nil
	case "inspect":
		if len(rest) < 1 {
			return fmt.Errorf("path required")
		}
		printJSON(plugin.Inspect(rest[0]))
		return nil
	case "install-dir":
		if len(rest) < 1 {
			return fmt.Errorf("source directory required")
		}
		x, err := plugin.InstallDir(p, *workspace, rest[0])
		if err == nil {
			printJSON(x)
		}
		return err
	case "install-git":
		if len(rest) < 1 {
			return fmt.Errorf("repository URL required")
		}
		x, err := plugin.InstallGit(p, *workspace, rest[0], *ref)
		if err == nil {
			printJSON(x)
		}
		return err
	case "import-bundle":
		if len(rest) < 1 {
			return fmt.Errorf("repository URL required")
		}
		x, err := plugin.ImportBundle(p, *workspace, rest[0], *ref, *name)
		if err == nil {
			printJSON(x)
		}
		return err
	case "enable":
		if *workspace != "" {
			return fmt.Errorf("CLI plugin enable/disable is global; omit --workspace")
		}
		if len(rest) < 1 {
			return fmt.Errorf("plugin name required")
		}
		return plugin.SetCLIEnabled(rest[0], true)
	case "disable":
		if *workspace != "" {
			return fmt.Errorf("CLI plugin enable/disable is global; omit --workspace")
		}
		if len(rest) < 1 {
			return fmt.Errorf("plugin name required")
		}
		return plugin.SetCLIEnabled(rest[0], false)
	case "remove":
		if len(rest) < 1 {
			return fmt.Errorf("plugin name required")
		}
		return plugin.Remove(p, *workspace, rest[0])
	case "doctor":
		printJSON(plugin.Doctor(p, *workspace))
		return nil
	default:
		return fmt.Errorf("unknown plugins command %q", args[0])
	}
}

func runAgents(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl agents install|list|doctor|status|enable")
	}
	fs := flag.NewFlagSet("agents", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "workspace scope; empty=global")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	switch args[0] {
	case "install":
		return agents.InstallEmbedded(p, *workspace)
	case "list":
		xs, e := agents.List(p, *workspace)
		if e == nil {
			printJSON(xs)
		}
		return e
	case "doctor":
		printJSON(agents.Doctor(p, *workspace))
		return nil
	case "status":
		c, e := agents.LoadConfig(p)
		if e == nil {
			printJSON(c)
		}
		return e
	case "enable":
		rest := fs.Args()
		if len(rest) < 1 {
			return fmt.Errorf("mode required: off|balanced|parallel|maximum")
		}
		return agents.Enable(p, rest[0])
	default:
		return fmt.Errorf("unknown agents command %q", args[0])
	}
}

func runCapabilities(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl capabilities build|list|search|rank|summary")
	}
	fs := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	var workspaces multiFlag
	fs.Var(&workspaces, "workspace", "workspace path; repeatable")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	switch args[0] {
	case "build":
		reg, e := capability.Build(p, workspaces)
		if e == nil {
			fmt.Println(capability.Summary(reg))
		}
		return e
	case "list":
		reg, e := capability.Load(p)
		if e == nil {
			printJSON(reg.Capabilities)
		}
		return e
	case "search":
		rest := fs.Args()
		if len(rest) < 1 {
			return fmt.Errorf("query required")
		}
		reg, e := capability.Load(p)
		if e != nil {
			return e
		}
		printJSON(capability.Search(reg, strings.Join(rest, " ")))
		return nil
	case "rank":
		rest := fs.Args()
		if len(rest) < 1 {
			return fmt.Errorf("task text required")
		}
		reg, e := capability.Load(p)
		if e != nil {
			return e
		}
		printJSON(capability.Rank(reg, strings.Join(rest, " "), 16))
		return nil
	case "summary":
		reg, e := capability.Load(p)
		if e == nil {
			fmt.Println(capability.Summary(reg))
		}
		return e
	default:
		return fmt.Errorf("unknown capabilities command %q", args[0])
	}
}

func runRegistry(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl registry search|info")
	}
	fs := flag.NewFlagSet("registry", flag.ContinueOnError)
	limit := fs.Int("limit", 25, "result limit")
	version := fs.String("version", "latest", "server version")
	workspace := fs.String("workspace", "", "workspace scope; empty=global")
	alias := fs.String("as", "", "configured MCP name; default derived from registry name")
	allowLowScore := fs.Bool("allow-low-security-score", false, "allow Registry install with security score below 60")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	client := registry.New(p)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	switch args[0] {
	case "search":
		q := strings.Join(fs.Args(), " ")
		page, e := client.Search(ctx, q, *limit)
		if e == nil {
			printJSON(page)
		}
		return e
	case "info":
		if len(fs.Args()) < 1 {
			return fmt.Errorf("server name required")
		}
		s, e := client.Detail(ctx, fs.Args()[0], *version)
		if e == nil {
			printJSON(s)
			fmt.Println("SECURITY SCORE:")
			printJSON(securityaudit.AssessRegistryServer(s.Name, s.Status, s.Raw))
			if plan, pe := registry.Plan(s); pe == nil {
				fmt.Println("INSTALL PLAN:")
				printJSON(plan)
			}
			for _, h := range registry.InstallHints(s) {
				fmt.Println("HINT:", h)
			}
		}
		return e
	case "install":
		if len(fs.Args()) < 1 {
			return fmt.Errorf("server name required")
		}
		s, e := client.Detail(ctx, fs.Args()[0], *version)
		if e != nil {
			return e
		}
		securityScore := securityaudit.AssessRegistryServer(s.Name, s.Status, s.Raw)
		if securityScore.Score < 60 && !*allowLowScore {
			printJSON(securityScore)
			return fmt.Errorf("registry server security score %d/%s is below install threshold; inspect findings or pass --allow-low-security-score", securityScore.Score, securityScore.Grade)
		}
		plan, e := registry.Plan(s)
		if e != nil {
			return e
		}
		name := strings.TrimSpace(*alias)
		if name == "" {
			name = registryAlias(s.Name)
		}
		if name == "" {
			return fmt.Errorf("could not derive --as name")
		}
		if e := mcp.InstallServer(p, *workspace, name, plan.Server); e != nil {
			return e
		}
		fmt.Printf("installed %s from %s\n", name, plan.Source)
		for _, w := range plan.Warnings {
			fmt.Println("WARN:", w)
		}
		return nil
	default:
		return fmt.Errorf("unknown registry command %q", args[0])
	}
}

func runProject(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl project detect|init|profiles|fingerprint")
	}
	fs := flag.NewFlagSet("project", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "workspace; default current directory")
	profilesFlag := fs.String("profiles", "", "comma-separated profiles; empty=auto-detected")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *workspace == "" {
		*workspace, _ = os.Getwd()
	}
	switch args[0] {
	case "profiles":
		printJSON(project.Profiles())
		return nil
	case "detect":
		d, e := project.Detect(*workspace)
		if e == nil {
			printJSON(d)
		}
		return e
	case "fingerprint":
		x, e := project.Fingerprint(*workspace)
		if e == nil {
			fmt.Println(x)
		}
		return e
	case "init":
		var selected []string
		if strings.TrimSpace(*profilesFlag) != "" {
			for _, x := range strings.Split(*profilesFlag, ",") {
				if x = strings.TrimSpace(x); x != "" {
					selected = append(selected, x)
				}
			}
		} else {
			d, e := project.Detect(*workspace)
			if e != nil {
				return e
			}
			selected = d.Profiles
		}
		d, e := project.Init(p, *workspace, selected)
		if e == nil {
			printJSON(d)
			_, _ = capability.Build(p, []string{*workspace})
		}
		return e
	default:
		return fmt.Errorf("unknown project command %q", args[0])
	}
}

func runWorkflow(p paths.Paths, args []string) error {
	_ = p
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl workflow install|list|remove")
	}
	fs := flag.NewFlagSet("workflow", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "workspace; default current directory")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *workspace == "" {
		*workspace, _ = os.Getwd()
	}
	switch args[0] {
	case "install":
		return workflow.InstallEmbedded(*workspace, fs.Args()...)
	case "list":
		xs, e := workflow.List(*workspace)
		if e == nil {
			printJSON(xs)
		}
		return e
	case "remove":
		if len(fs.Args()) < 1 {
			return fmt.Errorf("workflow name required")
		}
		return workflow.Remove(*workspace, fs.Args()[0])
	default:
		return fmt.Errorf("unknown workflow command %q", args[0])
	}
}

func runGoal(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl goal native|headless TEXT")
	}
	text := strings.TrimSpace(strings.Join(args[1:], " "))
	var out string
	var err error
	switch args[0] {
	case "native":
		out, err = goal.NativeCommand(text)
	case "headless":
		out, err = goal.HeadlessPrompt(text)
	default:
		return fmt.Errorf("unknown goal command %q", args[0])
	}
	if err == nil {
		fmt.Println(out)
	}
	return err
}

func runTasks(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl tasks add|list|show|run|run-pending|cancel|retry|remove|config")
	}
	switch args[0] {
	case "add":
		fs := flag.NewFlagSet("tasks add", flag.ContinueOnError)
		workspace := fs.String("workspace", "", "workspace")
		prompt := fs.String("prompt", "", "task prompt")
		priority := fs.Int("priority", 0, "priority")
		nativeGoal := fs.Bool("native-goal", false, "apply verified until-done semantics in AGY headless mode (desktop /goal is not assumed)")
		agent := fs.String("agent", "", "preferred coordinator agent")
		var tags multiFlag
		fs.Var(&tags, "tag", "tag; repeatable")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *prompt == "" {
			*prompt = strings.Join(fs.Args(), " ")
		}
		rec, e := tasks.Add(p, *prompt, *workspace, *priority, *nativeGoal, *agent, tags)
		if e == nil {
			printJSON(rec)
		}
		return e
	case "list":
		xs, e := tasks.List(p)
		if e == nil {
			printJSON(xs)
		}
		return e
	case "show":
		if len(args) < 2 {
			return fmt.Errorf("task id required")
		}
		x, e := tasks.Load(p, args[1])
		if e == nil {
			printJSON(x)
		}
		return e
	case "run":
		if len(args) < 2 {
			return fmt.Errorf("task id required")
		}
		x, e := tasks.Run(p, args[1])
		printJSON(x)
		return e
	case "run-pending":
		fs := flag.NewFlagSet("tasks run-pending", flag.ContinueOnError)
		static := fs.Bool("static", false, "disable adaptive DAG replanning for this run")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		var xs []model.TaskRecord
		var e error
		if *static {
			xs, e = tasks.RunPending(p)
		} else {
			xs, e = replan.RunPending(p)
		}
		printJSON(xs)
		return e
	case "cancel":
		if len(args) < 2 {
			return fmt.Errorf("task id required")
		}
		return tasks.Cancel(p, args[1])
	case "retry":
		if len(args) < 2 {
			return fmt.Errorf("task id required")
		}
		x, e := tasks.Retry(p, args[1])
		if e == nil {
			printJSON(x)
		}
		return e
	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("task id required")
		}
		return tasks.Remove(p, args[1])
	case "config":
		fs := flag.NewFlagSet("tasks config", flag.ContinueOnError)
		max := fs.Int("max-parallel", 0, "parallel task limit")
		cpu := fs.Int("cpu-weight", 0, "scheduler CPU capacity; default 100")
		build := fs.Int("build-slots", -1, "parallel build slots; -1 leaves current value")
		browser := fs.Int("browser-slots", -1, "parallel browser slots; -1 leaves current value")
		maxRetries := fs.Int("max-retries", -1, "automatic retries after failed task; -1 leaves current value")
		maxMinutes := fs.Int("max-task-minutes", 0, "per-task watchdog in minutes; 0 leaves current value")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		cfg, e := tasks.LoadConfig(p)
		if e != nil {
			return e
		}
		changed := false
		if *max > 0 {
			cfg.MaxParallel = *max
			changed = true
		}
		if *cpu > 0 {
			cfg.CPUWeight = *cpu
			changed = true
		}
		if *build >= 0 {
			cfg.BuildSlots = *build
			changed = true
		}
		if *browser >= 0 {
			cfg.BrowserSlots = *browser
			changed = true
		}
		if *maxRetries >= 0 {
			cfg.MaxRetries = *maxRetries
			changed = true
		}
		if *maxMinutes > 0 {
			cfg.MaxTaskMinutes = *maxMinutes
			changed = true
		}
		if changed {
			if e = tasks.SaveConfig(p, cfg); e != nil {
				return e
			}
		}
		printJSON(cfg)
		return nil
	default:
		return fmt.Errorf("unknown tasks command %q", args[0])
	}
}

func runReplan(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl replan status|config|enable|disable|apply|run|inbox")
	}
	switch args[0] {
	case "status":
		planID := ""
		if len(args) > 1 {
			planID = args[1]
		}
		x, e := replan.Status(p, planID)
		if e == nil {
			printJSON(x)
		}
		return e
	case "inbox":
		xs, e := replan.Inbox(p)
		if e == nil {
			printJSON(xs)
		}
		return e
	case "enable":
		cfg, e := replan.LoadConfig(p)
		if e != nil {
			return e
		}
		cfg.Enabled = true
		if e = replan.SaveConfig(p, cfg); e == nil {
			printJSON(cfg)
		}
		return e
	case "disable":
		cfg, e := replan.LoadConfig(p)
		if e != nil {
			return e
		}
		cfg.Enabled = false
		if e = replan.SaveConfig(p, cfg); e == nil {
			printJSON(cfg)
		}
		return e
	case "config":
		fs := flag.NewFlagSet("replan config", flag.ContinueOnError)
		maxRev := fs.Int("max-revisions", 0, "maximum DAG revisions")
		maxNodes := fs.Int("max-dynamic-nodes", 0, "maximum dynamic nodes per plan")
		maxDepth := fs.Int("max-repair-depth", 0, "maximum nested automatic repair depth")
		maxSame := fs.Int("max-same-failure", 0, "stop after same failure signature repeats this many times")
		minConfidence := fs.Float64("min-confidence", 0, "minimum proposal confidence 0..1")
		riskMax := fs.String("auto-risk-max", "", "maximum auto-applied risk: read-low|write-medium|execution-high|external-write-high|destructive-critical")
		preferWT := fs.String("prefer-worktrees", "", "true|false")
		requireEvidence := fs.String("require-evidence", "", "true|false")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		cfg, e := replan.LoadConfig(p)
		if e != nil {
			return e
		}
		changed := false
		if *maxRev > 0 {
			cfg.MaxRevisions = *maxRev
			changed = true
		}
		if *maxNodes > 0 {
			cfg.MaxDynamicNodes = *maxNodes
			changed = true
		}
		if *maxDepth > 0 {
			cfg.MaxRepairDepth = *maxDepth
			changed = true
		}
		if *maxSame > 0 {
			cfg.MaxSameFailure = *maxSame
			changed = true
		}
		if *minConfidence > 0 {
			cfg.MinConfidence = *minConfidence
			changed = true
		}
		if strings.TrimSpace(*riskMax) != "" {
			cfg.AutoApplyRiskMax = strings.TrimSpace(*riskMax)
			changed = true
		}
		if strings.TrimSpace(*preferWT) != "" {
			v, err := strconv.ParseBool(*preferWT)
			if err != nil {
				return err
			}
			cfg.PreferWorktrees = v
			changed = true
		}
		if strings.TrimSpace(*requireEvidence) != "" {
			v, err := strconv.ParseBool(*requireEvidence)
			if err != nil {
				return err
			}
			cfg.RequireEvidence = v
			changed = true
		}
		if changed {
			if e = replan.SaveConfig(p, cfg); e != nil {
				return e
			}
		}
		printJSON(cfg)
		return nil
	case "apply":
		if len(args) < 2 {
			return fmt.Errorf("task id required")
		}
		x, e := replan.ProcessTask(p, args[1])
		if e == nil {
			printJSON(x)
		}
		return e
	case "run":
		xs, e := replan.RunPending(p)
		printJSON(xs)
		return e
	default:
		return fmt.Errorf("unknown replan command %q", args[0])
	}
}

func runRisk(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl risk classify|policy|reset-policy")
	}
	if args[0] == "policy" {
		pol, e := risk.LoadPolicy(p)
		if e == nil {
			printJSON(pol)
		}
		return e
	}
	if args[0] == "reset-policy" {
		return risk.ResetPolicy(p)
	}
	if args[0] != "classify" {
		return fmt.Errorf("usage: agctl risk classify --tool NAME [--args JSON] [--mode guarded|unrestricted] [--workspace PATH]")
	}
	fs := flag.NewFlagSet("risk classify", flag.ContinueOnError)
	tool := fs.String("tool", "", "tool name")
	raw := fs.String("args", "{}", "tool args JSON")
	mode := fs.String("mode", "guarded", "permission mode")
	var workspaces multiFlag
	fs.Var(&workspaces, "workspace", "workspace; repeatable")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *tool == "" {
		return fmt.Errorf("--tool required")
	}
	m := map[string]any{}
	if err := json.Unmarshal([]byte(*raw), &m); err != nil {
		return err
	}
	d := risk.ClassifyConfigured(p, model.ToolCall{Name: *tool, Args: m}, risk.Context{PermissionMode: *mode, Workspaces: workspaces, Home: p.Home})
	printJSON(d)
	return nil
}

func runWorktree(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl worktree list|create|remove|prune")
	}
	fs := flag.NewFlagSet("worktree", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "repository; default current directory")
	base := fs.String("base", "HEAD", "base revision")
	force := fs.Bool("force", false, "force removal")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *workspace == "" {
		*workspace, _ = os.Getwd()
	}
	switch args[0] {
	case "list":
		xs, e := worktree.List(*workspace)
		if e == nil {
			printJSON(xs)
		}
		return e
	case "create":
		if len(fs.Args()) < 1 {
			return fmt.Errorf("name required")
		}
		x, e := worktree.Create(*workspace, fs.Args()[0], *base)
		if e == nil {
			printJSON(x)
		}
		return e
	case "remove":
		if len(fs.Args()) < 1 {
			return fmt.Errorf("path required")
		}
		return worktree.Remove(*workspace, fs.Args()[0], *force)
	case "prune":
		return worktree.Prune(*workspace)
	default:
		return fmt.Errorf("unknown worktree command %q", args[0])
	}
}

func runTelemetry(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl telemetry recent|summary")
	}
	fs := flag.NewFlagSet("telemetry", flag.ContinueOnError)
	limit := fs.Int("limit", 100, "event count")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	events, e := telemetry.Recent(p, *limit)
	if e != nil {
		return e
	}
	switch args[0] {
	case "recent":
		printJSON(events)
		return nil
	case "summary":
		printJSON(telemetry.Summarize(events))
		return nil
	default:
		return fmt.Errorf("unknown telemetry command %q", args[0])
	}
}

func runProvenance(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl provenance list|verify")
	}
	switch args[0] {
	case "list":
		xs, e := provenance.List(p)
		if e == nil {
			printJSON(xs)
		}
		return e
	case "verify":
		xs, e := provenance.VerifyAll(p)
		if e == nil {
			printJSON(xs)
		}
		return e
	default:
		return fmt.Errorf("unknown provenance command %q", args[0])
	}
}

func registryAlias(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimPrefix(name, "mcp-server-")
	name = strings.TrimPrefix(name, "server-")
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func parseInt(s string, def int) int {
	n, e := strconv.Atoi(s)
	if e != nil {
		return def
	}
	return n
}
func cleanWorkspace(s string) string {
	if s == "" {
		s, _ = os.Getwd()
	}
	a, e := filepath.Abs(s)
	if e == nil {
		return a
	}
	return s
}
