package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/homiakus/agctl/internal/harness/migratelegacy"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/planner"
	"github.com/homiakus/agctl/internal/tasks"
)

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
	DurableEvents      int    `json:"durableEvents"`
}

func runHarness(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl harness migrate [--dry-run] | status")
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
	default:
		return fmt.Errorf("usage: agctl harness migrate [--dry-run] | status")
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
		{"SELECT COUNT(*) FROM events", &status.DurableEvents},
	}
	for _, q := range queries {
		if err := db.SQLDB().QueryRow(q.query).Scan(q.out); err != nil {
			return err
		}
	}
	printJSON(status)
	return nil
}
