package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/homiakus/agctl/internal/harness/migratelegacy"
	"github.com/homiakus/agctl/internal/harness/resource"
	"github.com/homiakus/agctl/internal/harness/scheduler"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/planner"
	"github.com/homiakus/agctl/internal/tasks"
)

const harnessUsage = "usage: agctl harness migrate [--dry-run] | status | explain --node-run ID | pause --run ID | resume --run ID | cancel --run ID | signal --run ID --name NAME --message-id ID [--payload DATA] | approvals --run ID [--limit N] | approve --id ID --actor ACTOR | reject --id ID --actor ACTOR | sweep [--limit N]"

// init is a temporary strangler entrypoint for the Harness migration line.
// It intercepts only `agctl harness ...`; every legacy 3.2 command continues
// through main.go unchanged. Replace this shim with the normal command registry
// when the Harness API/CLI layer becomes authoritative.
func init() {
	if len(os.Args) < 2 || os.Args[1] != "harness" {
		return
	}
	p, err := paths.Detect()
	if err == nil {
		err = p.Ensure()
	}
	if err == nil {
		err = runHarness(p, os.Args[2:])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

type harnessMigrationPreview struct {
	Report  migratelegacy.Report `json:"report"`
	Bundles []harnessBundleInfo  `json:"bundles"`
}

type harnessBundleInfo struct {
	SourceID           string `json:"sourceId"`
	WorkflowDefinition string `json:"workflowDefinitionId"`
	WorkflowRun        string `json:"workflowRunId"`
	NodeRuns           int    `json:"nodeRuns"`
	RunState           string `json:"runState"`
}

type harnessStatus struct {
	DatabasePath       string `json:"databasePath"`
	SchemaVersion      int    `json:"schemaVersion"`
	LegacyPlans        int    `json:"legacyPlans"`
	LegacyTasks        int    `json:"legacyTasks"`
	DurableDefinitions int    `json:"durableDefinitions"`
	DurableRuns        int    `json:"durableRuns"`
	DurableNodeRuns    int    `json:"durableNodeRuns"`
	DurableReadyNodes  int    `json:"durableReadyNodes"`
	DurableEvents      int    `json:"durableEvents"`
	PendingTimers      int    `json:"pendingTimers"`
	PendingSignals     int    `json:"pendingSignals"`
	WaitingSignals     int    `json:"waitingSignals"`
	PendingApprovals   int    `json:"pendingApprovals"`
	ActiveRetries      int    `json:"activeRetries"`
	PausedRuns         int    `json:"pausedRuns"`
	PausingRuns        int    `json:"pausingRuns"`
}

func runHarness(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(harnessUsage)
	}
	switch args[0] {
	case "migrate":
		fs := flag.NewFlagSet("harness migrate", flag.ContinueOnError)
		dryRun := fs.Bool("dry-run", false, "plan legacy import without changing durable state")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return runHarnessMigrate(p, *dryRun)
	case "status":
		return runHarnessStatus(p)
	case "explain":
		fs := flag.NewFlagSet("harness explain", flag.ContinueOnError)
		nodeRunID := fs.String("node-run", "", "durable node run id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *nodeRunID == "" {
			return fmt.Errorf("--node-run is required")
		}
		return runHarnessExplain(p, harnessmodel.NodeRunID(*nodeRunID))
	case "pause", "resume", "cancel":
		fs := flag.NewFlagSet("harness "+args[0], flag.ContinueOnError)
		runID := fs.String("run", "", "workflow run id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *runID == "" {
			return fmt.Errorf("--run is required")
		}
		switch args[0] {
		case "pause":
			return runHarnessPause(p, harnessmodel.WorkflowRunID(*runID))
		case "resume":
			return runHarnessResume(p, harnessmodel.WorkflowRunID(*runID))
		default:
			return runHarnessCancel(p, harnessmodel.WorkflowRunID(*runID))
		}
	case "signal":
		fs := flag.NewFlagSet("harness signal", flag.ContinueOnError)
		runID := fs.String("run", "", "workflow run id")
		name := fs.String("name", "", "signal name")
		messageID := fs.String("message-id", "", "producer idempotency key")
		payload := fs.String("payload", "", "signal payload")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *runID == "" || *name == "" || *messageID == "" {
			return fmt.Errorf("--run, --name and --message-id are required")
		}
		return runHarnessSignal(p, harnessmodel.WorkflowRunID(*runID), *name, *messageID, *payload)
	case "approvals":
		fs := flag.NewFlagSet("harness approvals", flag.ContinueOnError)
		runID := fs.String("run", "", "workflow run id")
		limit := fs.Int("limit", 100, "maximum pending approvals")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *runID == "" {
			return fmt.Errorf("--run is required")
		}
		if *limit <= 0 {
			return fmt.Errorf("--limit must be positive")
		}
		return runHarnessApprovals(p, harnessmodel.WorkflowRunID(*runID), *limit)
	case "approve", "reject":
		fs := flag.NewFlagSet("harness "+args[0], flag.ContinueOnError)
		approvalID := fs.String("id", "", "approval id")
		actor := fs.String("actor", "", "human/operator identity")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *approvalID == "" || *actor == "" {
			return fmt.Errorf("--id and --actor are required")
		}
		return runHarnessApprovalDecision(p, harnessmodel.ApprovalID(*approvalID), *actor, args[0] == "approve")
	case "sweep":
		fs := flag.NewFlagSet("harness sweep", flag.ContinueOnError)
		limit := fs.Int("limit", 1000, "maximum due items per durable wait class")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return runHarnessSweep(p, *limit)
	default:
		return fmt.Errorf(harnessUsage)
	}
}

func runHarnessMigrate(p paths.Paths, dryRun bool) error {
	plans, err := planner.List(p)
	if err != nil {
		return fmt.Errorf("list legacy plans: %w", err)
	}
	records, err := tasks.List(p)
	if err != nil {
		return fmt.Errorf("list legacy tasks: %w", err)
	}
	bundles, report, err := migratelegacy.BuildBundles(plans, records)
	if err != nil {
		return err
	}
	for _, b := range bundles {
		report.Warnings = append(report.Warnings, b.Warnings...)
	}
	if dryRun {
		preview := harnessMigrationPreview{Report: report, Bundles: make([]harnessBundleInfo, 0, len(bundles))}
		for _, b := range bundles {
			preview.Bundles = append(preview.Bundles, harnessBundleInfo{SourceID: b.SourceID, WorkflowDefinition: string(b.Definition.ID), WorkflowRun: string(b.Run.ID), NodeRuns: len(b.NodeRuns), RunState: string(b.Run.State)})
		}
		printJSON(preview)
		return nil
	}

	db, err := sqlitestore.Open(context.Background(), p.HarnessDB, sqlitestore.Options{})
	if err != nil {
		return err
	}
	defer db.Close()
	for _, b := range bundles {
		result, err := migratelegacy.Apply(context.Background(), db, b)
		if err != nil {
			return fmt.Errorf("import %s: %w", b.SourceID, err)
		}
		if result.Imported {
			report.Imported++
		}
		if result.AlreadyImported {
			report.AlreadyImported++
		}
	}
	printJSON(report)
	return nil
}

func runHarnessStatus(p paths.Paths) error {
	plans, err := planner.List(p)
	if err != nil {
		return err
	}
	records, err := tasks.List(p)
	if err != nil {
		return err
	}
	db, err := sqlitestore.Open(context.Background(), p.HarnessDB, sqlitestore.Options{})
	if err != nil {
		return err
	}
	defer db.Close()
	status := harnessStatus{DatabasePath: db.Path(), LegacyPlans: len(plans), LegacyTasks: len(records)}
	if status.SchemaVersion, err = db.SchemaVersion(context.Background()); err != nil {
		return err
	}
	queries := []struct {
		query string
		out   *int
	}{
		{"SELECT COUNT(*) FROM workflow_definitions", &status.DurableDefinitions},
		{"SELECT COUNT(*) FROM workflow_runs", &status.DurableRuns},
		{"SELECT COUNT(*) FROM node_runs", &status.DurableNodeRuns},
		{"SELECT COUNT(*) FROM ready_queue", &status.DurableReadyNodes},
		{"SELECT COUNT(*) FROM events", &status.DurableEvents},
		{"SELECT COUNT(*) FROM timers WHERE state='PENDING'", &status.PendingTimers},
		{"SELECT COUNT(*) FROM signals WHERE state='PENDING'", &status.PendingSignals},
		{"SELECT COUNT(*) FROM signal_waits WHERE state='WAITING'", &status.WaitingSignals},
		{"SELECT COUNT(*) FROM approvals WHERE state='PENDING'", &status.PendingApprovals},
		{"SELECT COUNT(*) FROM retry_schedule", &status.ActiveRetries},
		{"SELECT COUNT(*) FROM workflow_runs WHERE state='PAUSED'", &status.PausedRuns},
		{"SELECT COUNT(*) FROM workflow_runs WHERE state='PAUSING'", &status.PausingRuns},
	}
	for _, q := range queries {
		if err := db.SQLDB().QueryRow(q.query).Scan(q.out); err != nil {
			return err
		}
	}
	printJSON(status)
	return nil
}

func runHarnessExplain(p paths.Paths, nodeRunID harnessmodel.NodeRunID) error {
	ctx := context.Background()
	db, err := sqlitestore.Open(ctx, p.HarnessDB, sqlitestore.Options{})
	if err != nil {
		return err
	}
	defer db.Close()
	cfg, err := tasks.LoadConfig(p)
	if err != nil {
		return err
	}
	sched, err := scheduler.New(db, scheduler.Options{Capacity: resource.Capacity{
		CPUWeight: cfg.CPUWeight, MemoryBytes: 1 << 62, GPUCount: 0, MaxVRAMBytes: 0,
		DiskBytes: 1 << 62, BuildSlots: cfg.BuildSlots, BrowserSlots: cfg.BrowserSlots,
	}})
	if err != nil {
		return err
	}
	explanation, err := sched.ExplainNode(ctx, nodeRunID)
	if err != nil {
		return err
	}
	printJSON(explanation)
	return nil
}
