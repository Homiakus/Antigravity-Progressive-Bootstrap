package planner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
)

func TestCreateBuildsAcyclicGoPlan(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := paths.Paths{PlansRoot: filepath.Join(root, "plans"), CapabilityDB: filepath.Join(root, "caps.json"), BackupsRoot: filepath.Join(root, "backups"), GlobalSkillsRoot: filepath.Join(root, "skills"), GlobalAgentsRoot: filepath.Join(root, "agents"), GlobalPluginsRoot: filepath.Join(root, "plugins"), GlobalMCP: filepath.Join(root, "mcp.json")}
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	pl, err := Create(p, "Проведи архитектурный аудит Go сервиса, исправь ошибки и проверь тестами", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Nodes) < 4 {
		t.Fatalf("expected multi-stage plan, got %d nodes", len(pl.Nodes))
	}
	foundGo := false
	for _, profile := range pl.Profiles {
		if profile == "go" {
			foundGo = true
		}
	}
	if !foundGo {
		t.Fatalf("expected go profile, got %v", pl.Profiles)
	}
	if err := validate(pl); err != nil {
		t.Fatal(err)
	}
}

func TestReadOnlyAuditSkipsImplement(t *testing.T) {
	root := t.TempDir()
	p := paths.Paths{PlansRoot: filepath.Join(root, "plans"), CapabilityDB: filepath.Join(root, "caps.json"), GlobalSkillsRoot: filepath.Join(root, "skills"), GlobalAgentsRoot: filepath.Join(root, "agents"), GlobalPluginsRoot: filepath.Join(root, "plugins"), GlobalMCP: filepath.Join(root, "mcp.json")}
	_ = p.Ensure()
	pl, err := Create(p, "Только анализ, ничего не меняй: проведи аудит архитектуры", root)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range pl.Nodes {
		if n.ID == "implement" {
			t.Fatal("read-only plan must not contain implement node")
		}
	}
}

func TestEnqueueBindsEveryNodeToWorkerAuthority(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	p := paths.Paths{Home: root, AppRoot: filepath.Join(root, "app")}
	p.BackupsRoot = filepath.Join(p.AppRoot, "backups")
	p.TasksRoot = filepath.Join(p.AppRoot, "tasks")
	p.TelemetryRoot = filepath.Join(p.AppRoot, "telemetry")
	p.ReplanInbox = filepath.Join(p.AppRoot, "replan")
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}

	plan := model.ExecutionPlan{
		ID:        "plan-role-boundary",
		Revision:  3,
		Prompt:    "implement and verify",
		Workspace: workspace,
		Nodes: []model.PlanNode{
			{ID: "implement", Objective: "change code", Agent: "implementer"},
			{ID: "verify", Objective: "verify code", Agent: "test-engineer", DependsOn: []string{"implement"}, Verification: []string{"go test ./..."}},
		},
	}

	records, err := Enqueue(p, plan, 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(records))
	}
	for _, rec := range records {
		for _, want := range []string{
			"ENGINEERING ROLE: WORKER",
			"Task-selection authority: DENIED",
			"Living-plan mutation authority: DENIED",
			"Repository commit authority: DENIED",
			"Main publication authority: DENIED",
			"Plan ID: plan-role-boundary",
			"Node ID: " + rec.NodeID,
		} {
			if !strings.Contains(rec.Prompt, want) {
				t.Fatalf("node %s missing worker boundary %q", rec.NodeID, want)
			}
		}
		if strings.Contains(rec.Prompt, "select exactly one T-XXX") || strings.Contains(rec.Prompt, "PUSH MAIN without force") {
			t.Fatalf("node %s leaked coordinator instructions", rec.NodeID)
		}
		foundRoleTag := false
		for _, tag := range rec.Tags {
			if tag == "engineering-role:worker" {
				foundRoleTag = true
				break
			}
		}
		if !foundRoleTag {
			t.Fatalf("node %s missing machine-readable worker role tag: %v", rec.NodeID, rec.Tags)
		}
	}
	if len(records[1].Dependencies) != 1 || records[1].Dependencies[0] != records[0].ID {
		t.Fatalf("worker contract propagation changed DAG dependencies: %#v", records[1].Dependencies)
	}
	if !strings.Contains(records[1].Prompt, "Required verification:\n- go test ./...") {
		t.Fatal("worker wrapping lost node verification requirements")
	}
}
