package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/homiakus/agctl/internal/dashboard"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/planner"
	"github.com/homiakus/agctl/internal/replan"
	"github.com/homiakus/agctl/internal/securityaudit"
	"github.com/homiakus/agctl/internal/tasks"
)

func runPlan(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl plan create|list|show|history|enqueue|run")
	}
	switch args[0] {
	case "create":
		fs := flag.NewFlagSet("plan create", flag.ContinueOnError)
		workspace := fs.String("workspace", "", "workspace; default current directory")
		prompt := fs.String("prompt", "", "delegated task")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *prompt == "" {
			*prompt = strings.Join(fs.Args(), " ")
		}
		pl, err := planner.Create(p, *prompt, *workspace)
		if err == nil {
			printJSON(pl)
		}
		return err
	case "list":
		xs, err := planner.List(p)
		if err == nil {
			printJSON(xs)
		}
		return err
	case "show":
		if len(args) < 2 {
			return fmt.Errorf("plan id required")
		}
		pl, err := planner.Load(p, args[1])
		if err == nil {
			printJSON(pl)
		}
		return err
	case "history":
		if len(args) < 2 {
			return fmt.Errorf("plan id required")
		}
		pl, err := planner.Load(p, args[1])
		if err == nil {
			printJSON(pl.RevisionHistory)
		}
		return err
	case "enqueue":
		fs := flag.NewFlagSet("plan enqueue", flag.ContinueOnError)
		priority := fs.Int("priority", 0, "task priority")
		nativeGoal := fs.Bool("native-goal", false, "apply until-done goal semantics for every DAG node in headless mode")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if len(fs.Args()) < 1 {
			return fmt.Errorf("plan id required")
		}
		pl, err := planner.Load(p, fs.Args()[0])
		if err != nil {
			return err
		}
		xs, err := planner.Enqueue(p, pl, *priority, *nativeGoal)
		if err == nil {
			printJSON(xs)
		}
		return err
	case "run":
		fs := flag.NewFlagSet("plan run", flag.ContinueOnError)
		workspace := fs.String("workspace", "", "workspace; default current directory")
		prompt := fs.String("prompt", "", "delegated task")
		priority := fs.Int("priority", 0, "task priority")
		nativeGoal := fs.Bool("native-goal", false, "apply until-done goal semantics for every DAG node in headless mode")
		static := fs.Bool("static", false, "disable adaptive DAG replanning for this run")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *prompt == "" {
			*prompt = strings.Join(fs.Args(), " ")
		}
		pl, err := planner.Create(p, *prompt, *workspace)
		if err != nil {
			return err
		}
		created, err := planner.Enqueue(p, pl, *priority, *nativeGoal)
		if err != nil {
			return err
		}
		fmt.Printf("plan %s enqueued: %d nodes\n", pl.ID, len(created))
		var results []model.TaskRecord
		if *static {
			results, err = tasks.RunPending(p)
		} else {
			results, err = replan.RunPending(p)
		}
		printJSON(results)
		return err
	default:
		return fmt.Errorf("unknown plan command %q", args[0])
	}
}

func runSecurity(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl security audit [--workspace PATH]")
	}
	if args[0] != "audit" {
		return fmt.Errorf("unknown security command %q", args[0])
	}
	fs := flag.NewFlagSet("security audit", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "workspace scope")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	rep, err := securityaudit.Audit(p, *workspace)
	if err == nil {
		printJSON(rep)
	}
	return err
}

func runDashboard(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl dashboard serve [--listen 127.0.0.1:8787] [--workspace PATH]")
	}
	if args[0] != "serve" {
		return fmt.Errorf("unknown dashboard command %q", args[0])
	}
	fs := flag.NewFlagSet("dashboard serve", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:8787", "listen address")
	workspace := fs.String("workspace", "", "workspace for security/capability context")
	allowRemote := fs.Bool("allow-remote", false, "allow binding to a non-loopback address")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *workspace == "" {
		*workspace, _ = os.Getwd()
	}
	return dashboard.Serve(p, *workspace, *listen, *allowRemote)
}
