package migratelegacy

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
	legacymodel "github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/tasks"
)

func legacyFixture() (legacymodel.ExecutionPlan, []legacymodel.TaskRecord) {
	now := time.Unix(1700000000, 0).UTC()
	plan := legacymodel.ExecutionPlan{
		ID: "plan-test", Revision: 2, Status: "active", CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Add(2 * time.Second).Format(time.RFC3339Nano), GeneratedBy: "test", Prompt: "test", Workspace: "/tmp/test",
		RevisionHistory: []legacymodel.PlanRevision{{Number: 1, CreatedAt: now.Add(time.Second).Format(time.RFC3339Nano), Reason: "repair"}, {Number: 2, CreatedAt: now.Add(2 * time.Second).Format(time.RFC3339Nano), Reason: "verify"}},
		Nodes: []legacymodel.PlanNode{{ID: "a", Title: "A", Objective: "A", Agent: "implementer"}, {ID: "b", Title: "B", Objective: "B", Agent: "test-engineer", DependsOn: []string{"a"}}},
	}
	records := []legacymodel.TaskRecord{
		{ID: "task-a", PlanID: plan.ID, NodeID: "a", Status: tasks.StatusSucceeded, Attempts: 3, CreatedAt: now.Format(time.RFC3339Nano), StartedAt: now.Add(time.Second).Format(time.RFC3339Nano), FinishedAt: now.Add(2 * time.Second).Format(time.RFC3339Nano), UpdatedAt: now.Add(2 * time.Second).Format(time.RFC3339Nano)},
		{ID: "task-b", PlanID: plan.ID, NodeID: "b", Status: tasks.StatusRunning, Attempts: 1, Dependencies: []string{"task-a"}, CreatedAt: now.Add(3 * time.Second).Format(time.RFC3339Nano), StartedAt: now.Add(4 * time.Second).Format(time.RFC3339Nano), UpdatedAt: now.Add(4 * time.Second).Format(time.RFC3339Nano)},
	}
	return plan, records
}

func TestBuildPlanBundlePreservesKnownFactsWithoutInventingAttempts(t *testing.T) {
	plan, records := legacyFixture()
	bundle, err := BuildPlanBundle(plan, records)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Run.State != harnessmodel.WorkflowBlocked {
		t.Fatalf("legacy running task must block imported workflow, got %s", bundle.Run.State)
	}
	if len(bundle.NodeRuns) != 2 || bundle.NodeRuns[0].State != harnessmodel.NodeSucceeded || bundle.NodeRuns[1].State != harnessmodel.NodeInDoubt {
		t.Fatalf("unexpected imported node states: %+v", bundle.NodeRuns)
	}
	if bundle.NodeRuns[1].RemainingDependencies != 0 {
		t.Fatalf("succeeded legacy dependency should be satisfied: %+v", bundle.NodeRuns[1])
	}
	if len(bundle.Events) != 3 || bundle.Events[len(bundle.Events)-1].Type != "LegacyImportCompleted" {
		t.Fatalf("missing immutable import marker: %+v", bundle.Events)
	}
	var summary map[string]any
	if err := json.Unmarshal(bundle.Events[0].Payload, &summary); err != nil {
		t.Fatal(err)
	}
	if summary["legacyAttemptCount"].(float64) != 3 || summary["historyIncomplete"] != true {
		t.Fatalf("legacy attempt summary lost: %+v", summary)
	}
}

func TestBuildPlanBundleIsDeterministic(t *testing.T) {
	plan, records := legacyFixture()
	a, err := BuildPlanBundle(plan, records)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildPlanBundle(plan, records)
	if err != nil {
		t.Fatal(err)
	}
	if a.Definition.ID != b.Definition.ID || a.Run.ID != b.Run.ID || a.SourceFingerprint != b.SourceFingerprint || a.NodeRuns[0].ID != b.NodeRuns[0].ID || a.Events[0].ID != b.Events[0].ID {
		t.Fatalf("legacy import identity is not deterministic\na=%+v\nb=%+v", a, b)
	}
}

func TestBuildPlanBundleRejectsTaskMissingFromDefinition(t *testing.T) {
	plan, records := legacyFixture()
	records[1].NodeID = "missing"
	if _, err := BuildPlanBundle(plan, records); err == nil {
		t.Fatal("expected missing-node migration error")
	}
}

func TestStandaloneTaskGetsSyntheticWorkflow(t *testing.T) {
	_, records := legacyFixture()
	r := records[0]
	r.PlanID = ""
	r.NodeID = ""
	bundle, err := BuildStandaloneBundle(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Definition.Nodes) != 1 || len(bundle.NodeRuns) != 1 || bundle.NodeRuns[0].State != harnessmodel.NodeSucceeded {
		t.Fatalf("unexpected standalone import: %+v", bundle)
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	plan, records := legacyFixture()
	bundle, err := BuildPlanBundle(plan, records)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlitestore.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"), sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	first, err := Apply(context.Background(), db, bundle)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Imported || first.AlreadyImported {
		t.Fatalf("unexpected first apply: %+v", first)
	}
	second, err := Apply(context.Background(), db, bundle)
	if err != nil {
		t.Fatal(err)
	}
	if second.Imported || !second.AlreadyImported {
		t.Fatalf("unexpected second apply: %+v", second)
	}
	var defs, runs, nodeRuns int
	if err := db.SQLDB().QueryRow(`SELECT COUNT(*) FROM workflow_definitions`).Scan(&defs); err != nil {
		t.Fatal(err)
	}
	if err := db.SQLDB().QueryRow(`SELECT COUNT(*) FROM workflow_runs`).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err := db.SQLDB().QueryRow(`SELECT COUNT(*) FROM node_runs`).Scan(&nodeRuns); err != nil {
		t.Fatal(err)
	}
	if defs != 1 || runs != 1 || nodeRuns != 2 {
		t.Fatalf("duplicate durable state after idempotent import: defs=%d runs=%d nodeRuns=%d", defs, runs, nodeRuns)
	}
}
