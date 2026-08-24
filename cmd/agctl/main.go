package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/agents"
	"github.com/homiakus/agctl/internal/backup"
	"github.com/homiakus/agctl/internal/capability"
	"github.com/homiakus/agctl/internal/doctor"
	"github.com/homiakus/agctl/internal/hooks"
	"github.com/homiakus/agctl/internal/installer"
	"github.com/homiakus/agctl/internal/loop"
	"github.com/homiakus/agctl/internal/mcp"
	"github.com/homiakus/agctl/internal/mcpprobe"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/permissions"
	"github.com/homiakus/agctl/internal/project"
	"github.com/homiakus/agctl/internal/replan"
	"github.com/homiakus/agctl/internal/risk"
	"github.com/homiakus/agctl/internal/router"
	"github.com/homiakus/agctl/internal/skills"
	"github.com/homiakus/agctl/internal/tasks"
	"github.com/homiakus/agctl/internal/web"
)

const Version = "3.2.1"

func autoBootstrap(p paths.Paths, workspace string) {
	// 1. Ensure latest self binary is installed to user bin for background hooks and agent processes
	bin, err := installer.InstallSelf(p)
	if err == nil {
		_ = hooks.Install(p, bin)
		_ = installer.TouchManifest(p, Version)
	}

	// 2. Ensure global adaptive control-plane rule exists in rules directory and GEMINI.md
	_ = installer.InstallGlobalRule(p, router.ModeBalanced)

	// 3. Ensure risk policy, task config, and replan supervisor configs exist
	_ = risk.EnsurePolicy(p)
	if _, err := os.Stat(p.TaskConfig); os.IsNotExist(err) {
		_ = tasks.SaveConfig(p, tasks.DefaultConfig())
	}
	if _, err := os.Stat(p.ReplanConfig); os.IsNotExist(err) {
		_ = replan.SaveConfig(p, replan.DefaultConfig())
	}

	// 4. Ensure embedded core skills and agent profiles are installed
	_ = skills.InstallEmbedded(p)
	_ = agents.InstallEmbedded(p, "")

	// 5. If current directory is a workspace, ensure capabilities and project registration
	var workspaces []string
	if workspace != "" {
		workspaces = []string{workspace}
		if d, err := project.Detect(workspace); err == nil && len(d.Profiles) > 0 {
			_ = agents.InstallEmbedded(p, workspace)
		}
	}
	_, _ = capability.Build(p, workspaces)
}

func main() {
	p, err := paths.Detect()
	if err != nil {
		fatal(err)
	}
	if err := p.Ensure(); err != nil {
		fatal(err)
	}

	args := os.Args[1:]
	if len(args) == 0 {
		ws, _ := os.Getwd()
		autoBootstrap(p, ws)
		if err := web.Serve(p, ws, "127.0.0.1:8787", true, false); err != nil {
			fatal(err)
		}
		return
	}

	// Hook subprocesses must keep stdout strictly JSON.
	if args[0] == "hook" {
		if err := runHook(p, args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "agctl hook:", err)
			os.Exit(1)
		}
		return
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Println(Version)
	case "install":
		must(runInstall(p, args[1:]))
	case "doctor":
		must(runDoctor(p, args[1:]))
	case "skills":
		must(runSkills(p, args[1:]))
	case "sidecar", "sidecars":
		must(runSidecars(p, args[1:]))
	case "mcp":
		must(runMCP(p, args[1:]))
	case "router":
		must(runRouter(p, args[1:]))
	case "loop":
		must(runLoop(p, args[1:]))
	case "permissions":
		must(runPermissions(p, args[1:]))
	case "state":
		must(runState(p, args[1:]))
	case "backup":
		must(runBackup(p, args[1:]))
	case "prereq":
		must(runPrereq(args[1:]))
	case "migrate":
		must(runMigrate(p, args[1:]))
	case "plugins":
		must(runPlugins(p, args[1:]))
	case "agents", "orchestrator":
		must(runAgents(p, args[1:]))
	case "capabilities":
		must(runCapabilities(p, args[1:]))
	case "registry":
		must(runRegistry(p, args[1:]))
	case "project":
		must(runProject(p, args[1:]))
	case "workflow", "workflows":
		must(runWorkflow(p, args[1:]))
	case "goal":
		must(runGoal(args[1:]))
	case "tasks":
		must(runTasks(p, args[1:]))
	case "risk":
		must(runRisk(p, args[1:]))
	case "provenance", "locks":
		must(runProvenance(p, args[1:]))
	case "telemetry":
		must(runTelemetry(p, args[1:]))
	case "worktree", "worktrees":
		must(runWorktree(args[1:]))
	case "plan", "orchestrate":
		must(runPlan(p, args[1:]))
	case "replan":
		must(runReplan(p, args[1:]))
	case "security":
		must(runSecurity(p, args[1:]))
	case "web", "dashboard", "ui":
		must(runDashboard(p, args[1:]))
	case "platforms":
		must(runPlatforms(p, args[1:]))
	case "paths":
		printPaths(p)
	case "help", "--help", "-h":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func runHook(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("hook name required")
	}
	exe, _ := os.Executable()
	switch args[0] {
	case "router-pre-invocation":
		return hooks.HandleRouterPreInvocation(p, os.Stdin, os.Stdout)
	case "loop-pre-invocation":
		return hooks.HandleLoopPreInvocation(p, exe, os.Stdin, os.Stdout)
	case "loop-pre-tool":
		return hooks.HandleLoopPreTool(p, os.Stdin, os.Stdout)
	case "loop-stop":
		return hooks.HandleLoopStop(p, os.Stdin, os.Stdout)
	default:
		return fmt.Errorf("unknown hook %q", args[0])
	}
}

func runInstall(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl install recommended|full|self")
	}
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	pr := fs.Bool("prereqs", false, "install missing prerequisites where supported")
	_ = fs.Parse(args[1:])
	switch args[0] {
	case "self":
		b, e := installer.InstallSelf(p)
		if e == nil {
			fmt.Println(b)
			e = hooks.Install(p, b)
			_ = installer.TouchManifest(p, Version)
		}
		return e
	case "recommended":
		r, e := installer.Recommended(p, *pr)
		printInstallReport(r)
		if e == nil {
			_ = installer.TouchManifest(p, Version)
		}
		return e
	case "full":
		r, e := installer.Full(p, *pr)
		printInstallReport(r)
		if e == nil {
			_ = installer.TouchManifest(p, Version)
		}
		return e
	default:
		return fmt.Errorf("unknown install profile %q", args[0])
	}
}
func printInstallReport(r installer.Report) {
	fmt.Println("binary:", r.InstalledBinary)
	if len(r.SkillPackCounts) > 0 {
		fmt.Println("skill packs:", r.SkillPackCounts)
	}
	for _, w := range r.MCPWarnings {
		fmt.Println("WARN MCP:", w)
	}
	for _, n := range r.Notes {
		fmt.Println("NOTE:", n)
	}
}

func runDoctor(p paths.Paths, args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "workspace path")
	self := fs.Bool("self-test", false, "run hook/state self-test")
	probeMCP := fs.Bool("probe-mcp", false, "perform live MCP initialize/tools/resources/prompts probes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *self {
		if err := doctor.SelfTest(); err != nil {
			return err
		}
		fmt.Println("self-test: PASS")
	}
	r := doctor.RunAdvanced(p, *workspace, *probeMCP)
	for _, f := range r.Findings {
		fmt.Printf("%-5s %-12s %s\n", f.Level, f.Area, f.Message)
	}
	if r.HasErrors() {
		return fmt.Errorf("diagnostics found errors")
	}
	return nil
}

func runSkills(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl skills list|install|sync-pack|remove")
	}
	switch args[0] {
	case "list":
		xs, e := skills.List(p)
		for _, x := range xs {
			fmt.Println(x.Name)
		}
		return e
	case "install":
		if len(args) < 2 || args[1] != "embedded" {
			return fmt.Errorf("usage: agctl skills install embedded")
		}
		return skills.InstallEmbedded(p)
	case "sync-pack":
		if len(args) < 2 {
			return fmt.Errorf("pack id required")
		}
		xs, e := skills.SyncPack(p, args[1])
		if e == nil {
			fmt.Printf("synced %d skills\n", len(xs))
		}
		return e
	case "sync-recommended":
		r, e := skills.SyncRecommended(p)
		fmt.Println(r)
		return e
	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("skill name required")
		}
		return skills.Remove(p, args[1])
	default:
		return fmt.Errorf("unknown skills command %q", args[0])
	}
}

func runMCP(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl mcp list|add|remove|doctor|probe")
	}
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "workspace scope; empty=global")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	rest := fs.Args()
	switch args[0] {
	case "list":
		xs, e := mcp.Names(p, *workspace)
		for _, x := range xs {
			fmt.Println(x)
		}
		return e
	case "add":
		if len(rest) == 0 {
			return fmt.Errorf("MCP id required: context7|playwright|chrome-devtools|gopls|memory|github")
		}
		return mcp.Install(p, *workspace, rest...)
	case "remove":
		if len(rest) == 0 {
			return fmt.Errorf("MCP id required")
		}
		return mcp.Remove(p, *workspace, rest...)
	case "doctor":
		for _, x := range mcp.Doctor(p, *workspace) {
			fmt.Println(x)
		}
		return nil
	case "probe":
		timeout := 12 * time.Second
		if len(rest) == 0 {
			printJSON(mcpprobe.ProbeAll(p, *workspace, timeout))
			return nil
		}
		r, e := mcpprobe.ProbeConfigured(p, *workspace, rest[0], timeout)
		printJSON(r)
		return e
	default:
		return fmt.Errorf("unknown mcp command %q", args[0])
	}
}

func runRouter(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl router status|enable|disable|inventory")
	}
	switch args[0] {
	case "status":
		c, e := router.Load(p)
		if e == nil {
			printJSON(c)
		}
		return e
	case "enable":
		if len(args) < 2 {
			return fmt.Errorf("mode required: silent|balanced|transparent|maximum")
		}
		return installer.SetRouterMode(p, args[1])
	case "disable":
		return router.Disable(p)
	case "inventory":
		cwd, _ := os.Getwd()
		inv := router.Discover(p, []string{cwd})
		printJSON(inv)
		return nil
	default:
		return fmt.Errorf("unknown router command %q", args[0])
	}
}

func runLoop(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl loop status|enable|disable|test")
	}
	switch args[0] {
	case "status":
		c, e := loop.Load(p)
		if e == nil {
			printJSON(c)
		}
		return e
	case "enable":
		if len(args) < 2 {
			return fmt.Errorf("profile required: standard|deep|until-done|unrestricted")
		}
		if args[1] == "unrestricted" && !hasArg(args, "--yes-i-accept-risk") {
			return fmt.Errorf("unrestricted requires --yes-i-accept-risk")
		}
		return installer.EnableLoop(p, args[1])
	case "disable":
		return installer.DisableLoop(p)
	case "test":
		if err := doctor.SelfTest(); err != nil {
			return err
		}
		fmt.Println("completion-loop self-test: PASS")
		return nil
	default:
		return fmt.Errorf("unknown loop command %q", args[0])
	}
}

func runPermissions(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl permissions apply|audit|checklist")
	}
	switch args[0] {
	case "apply":
		if len(args) < 2 {
			return fmt.Errorf("profile required: safe|balanced|autonomous|yolo")
		}
		if args[1] == "yolo" && !hasArg(args, "--yes-i-accept-risk") {
			return fmt.Errorf("yolo requires --yes-i-accept-risk")
		}
		return permissions.Apply(p, args[1])
	case "audit":
		a, e := permissions.AuditSettings(p)
		if e == nil {
			printJSON(a)
		}
		return e
	case "checklist":
		fmt.Println(permissions.NoPromptsChecklist())
		return nil
	default:
		return fmt.Errorf("unknown permissions command %q", args[0])
	}
}

func runState(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl state status|complete|block|reset")
	}
	fs := flag.NewFlagSet("state", flag.ContinueOnError)
	conv := fs.String("conversation", "", "conversation id")
	task := fs.String("task-id", "", "task id")
	summary := fs.String("summary", "", "summary")
	var verifies multiFlag
	fs.Var(&verifies, "verify", "verification evidence; repeatable")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	switch args[0] {
	case "status":
		if *conv == "" {
			return fmt.Errorf("--conversation required")
		}
		s, ok, e := loop.LoadState(p, *conv)
		if e == nil {
			if !ok {
				return fmt.Errorf("state not found")
			}
			printJSON(s)
		}
		return e
	case "complete":
		if *conv == "" || *task == "" {
			return fmt.Errorf("--conversation and --task-id required")
		}
		return loop.MarkComplete(p, *conv, *task, *summary, verifies)
	case "block":
		if *conv == "" || *task == "" {
			return fmt.Errorf("--conversation and --task-id required")
		}
		return loop.MarkBlocker(p, *conv, *task, *summary)
	case "reset":
		if *conv == "" {
			return fmt.Errorf("--conversation required")
		}
		return loop.ResetState(p, *conv)
	default:
		return fmt.Errorf("unknown state command %q", args[0])
	}
}

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func runBackup(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl backup create|list|restore")
	}
	switch args[0] {
	case "create":
		xs, e := backup.CreateAll(p, "")
		for _, x := range xs {
			fmt.Println(x)
		}
		return e
	case "list":
		xs, e := backup.List(p)
		for _, x := range xs {
			fmt.Println(x)
		}
		return e
	case "restore":
		if len(args) < 2 {
			return fmt.Errorf("backup file required")
		}
		dst := ""
		if len(args) > 2 {
			dst = args[2]
		}
		return backup.Restore(p, args[1], dst)
	default:
		return fmt.Errorf("unknown backup command %q", args[0])
	}
}
func runPrereq(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl prereq list|install")
	}
	switch args[0] {
	case "list":
		missing := map[string]bool{}
		for _, x := range installer.MissingPrerequisites() {
			missing[x.ID] = true
		}
		for _, x := range installer.Prerequisites {
			status := "installed"
			if missing[x.ID] {
				status = "MISSING"
			}
			fmt.Printf("%-10s %-10s %s\n", x.ID, status, x.Label)
		}
		return nil
	case "install":
		if len(args) < 2 {
			return fmt.Errorf("id required")
		}
		return installer.InstallPrerequisite(args[1])
	default:
		return fmt.Errorf("unknown prereq command")
	}
}

func runMigrate(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl migrate legacy|v2|v3.1|v3.2")
	}
	var notes []string
	var err error
	switch args[0] {
	case "legacy":
		notes, err = installer.MigrateLegacy(p)
	case "v2":
		notes, err = installer.MigrateV2(p)
		if err == nil {
			_ = installer.TouchManifest(p, Version)
		}
	case "v3.1", "v31":
		notes, err = installer.MigrateV31(p)
		if err == nil {
			_ = installer.TouchManifest(p, Version)
		}
	case "v3.2", "v32", "v3.2.0":
		notes, err = installer.MigrateV32(p)
		if err == nil {
			_ = installer.TouchManifest(p, Version)
		}
	default:
		return fmt.Errorf("usage: agctl migrate legacy|v2|v3.1|v3.2")
	}
	for _, n := range notes {
		fmt.Println(n)
	}
	if err == nil && len(notes) == 0 {
		fmt.Println("no legacy managed hooks/rules found")
	}
	return err
}

func runPlatforms(p paths.Paths, args []string) error {
	fs := flag.NewFlagSet("platforms", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "output JSON format")
	if err := fs.Parse(args); err != nil {
		return err
	}
	infos := p.GetPlatformInfos()
	if *jsonOutput {
		printJSON(map[string]any{
			"activePlatform": p.ActivePlatform,
			"detected":       p.DetectedPlatforms,
			"platforms":      infos,
		})
		return nil
	}
	fmt.Printf("Active Platform: %s\n\nSupported and Detected Platforms:\n", strings.ToUpper(string(p.ActivePlatform)))
	for _, info := range infos {
		status := "[ ]"
		if info.Active {
			status = "[*] ACTIVE"
		} else if _, err := os.Stat(info.ConfigDir); err == nil {
			status = "[+] INSTALLED"
		}
		fmt.Printf("%-14s %-28s %s\n", status, info.Label, info.Description)
		fmt.Printf("               Config: %s\n", info.ConfigDir)
		fmt.Printf("               Rule:   %s\n\n", info.RulePath)
	}
	return nil
}

func printPaths(p paths.Paths) { printJSON(p) }
func printJSON(v any)          { b, _ := json.MarshalIndent(v, "", "  "); fmt.Println(string(b)) }
func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
func must(err error) {
	if err != nil {
		fatal(err)
	}
}
func fatal(err error) { fmt.Fprintln(os.Stderr, "ERROR:", err); os.Exit(1) }

func usage() {
	fmt.Printf(`agctl %s — Universal Antigravity & Multi-Platform Web Control Plane

Interactive:
  agctl                                         # Launches Web Control Plane in default browser
  agctl web [--listen 127.0.0.1:8787] [--no-browser]

Install:
  agctl install recommended [--prereqs]
  agctl install full [--prereqs]
  agctl install self

Core:
  agctl doctor [--workspace PATH] [--self-test] [--probe-mcp]
  agctl platforms [--json]
  agctl skills list|install embedded|sync-pack ID|sync-recommended|remove NAME
  agctl mcp list|add|remove|doctor|probe [--workspace PATH]
  agctl router status|enable MODE|disable|inventory
  agctl loop status|enable PROFILE|disable|test
  agctl permissions apply PROFILE|audit|checklist
  agctl backup create|list|restore FILE [DEST]
  agctl prereq list|install ID
  agctl migrate legacy|v2|v3.1|v3.2

Control Plane 3.2.1:
  agctl plugins list|inspect|install-dir|install-git|import-bundle|enable|disable|remove|doctor
  agctl agents install|list|doctor|status|enable MODE
  agctl sidecars list|doctor|install-dir|schedule|enable|disable|remove
  agctl capabilities build|list|search|rank|summary [--workspace PATH]
  agctl registry search QUERY | info NAME | install NAME [--workspace PATH] [--as ID] [--allow-low-security-score]
  agctl project detect|init|profiles|fingerprint [--workspace PATH] [--profiles go,web]
  agctl workflow install|list|remove [--workspace PATH]
  agctl goal native|headless TEXT
  agctl tasks add|list|show|run|run-pending|cancel|retry|remove|config
  agctl risk classify --tool NAME --args JSON --mode guarded|unrestricted
  agctl provenance list|verify
  agctl telemetry recent|summary [--limit N]
  agctl worktree list|create|remove|prune [--workspace PATH]
  agctl plan create|list|show|history|enqueue|run
  agctl security audit [--workspace PATH]
  agctl replan status|config|enable|disable|apply|run|inbox
  agctl dashboard serve [--listen 127.0.0.1:8787] [--workspace PATH]

Completion state (normally called by the agent itself):
  agctl state complete --conversation ID --task-id ID --summary TEXT --verify CHECK [--verify CHECK]
  agctl state block --conversation ID --task-id ID --summary TEXT

Hook entrypoints (called by Antigravity):
  agctl hook router-pre-invocation
  agctl hook loop-pre-invocation
  agctl hook loop-pre-tool
  agctl hook loop-stop

Risk acknowledgements:
  loop enable unrestricted --yes-i-accept-risk
  permissions apply yolo --yes-i-accept-risk
`, Version)
}

var _ = filepath.Separator
