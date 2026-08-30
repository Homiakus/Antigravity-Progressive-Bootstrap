package task

import (
	"errors"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

const samplePlanDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func baseEnvelope() harnessmodel.TaskEnvelope {
	return harnessmodel.TaskEnvelope{
		ID:         "tenv_test_001",
		TaskID:     "T-013",
		PlanDigest: samplePlanDigest,
		TaskClass:  harnessmodel.TaskClassCodegen,
		Title:      "Implement TaskEnvelope",
		Objective:  "Introduce self-contained execution envelope",
		Instructions: "Define TaskEnvelope domain model with plan digest binding",
		Workspace: harnessmodel.WorkspaceSpec{
			RootPath: "c:/repo",
			RepoID:   "repo1",
		},
		Role:                 "worker",
		RequiredCapabilities: []string{"tools", "file_edit"},
		CreatedAt:            time.Now().UTC(),
	}
}

func TestValidation(t *testing.T) {
	t.Run("valid envelope passes", func(t *testing.T) {
		env := baseEnvelope()
		if err := env.Validate(); err != nil {
			t.Fatalf("expected valid envelope, got %v", err)
		}
	})

	t.Run("missing required fields fail", func(t *testing.T) {
		cases := []struct {
			name   string
			mutate func(*harnessmodel.TaskEnvelope)
		}{
			{"missing id", func(e *harnessmodel.TaskEnvelope) { e.ID = "" }},
			{"missing task id", func(e *harnessmodel.TaskEnvelope) { e.TaskID = "" }},
			{"missing plan digest", func(e *harnessmodel.TaskEnvelope) { e.PlanDigest = "" }},
			{"invalid plan digest length", func(e *harnessmodel.TaskEnvelope) { e.PlanDigest = "short" }},
			{"invalid plan digest chars", func(e *harnessmodel.TaskEnvelope) {
				e.PlanDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdeg"
			}},
			{"missing title", func(e *harnessmodel.TaskEnvelope) { e.Title = "" }},
			{"missing objective", func(e *harnessmodel.TaskEnvelope) { e.Objective = "" }},
			{"missing instructions", func(e *harnessmodel.TaskEnvelope) { e.Instructions = "" }},
			{"missing workspace root", func(e *harnessmodel.TaskEnvelope) { e.Workspace.RootPath = "" }},
			{"negative attempt number", func(e *harnessmodel.TaskEnvelope) { e.AttemptNumber = -1 }},
			{"negative timeout", func(e *harnessmodel.TaskEnvelope) { e.Timeout = -time.Second }},
			{"zero created at", func(e *harnessmodel.TaskEnvelope) { e.CreatedAt = time.Time{} }},
			{"invalid context ref uri", func(e *harnessmodel.TaskEnvelope) {
				e.ContextRefs = []harnessmodel.ContextRef{{ID: "ref1", URI: ""}}
			}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				env := baseEnvelope()
				tc.mutate(&env)
				if err := env.Validate(); err == nil {
					t.Fatalf("expected validation error for %s", tc.name)
				}
			})
		}
	})
}

func TestCanonicalDigestDeterminism(t *testing.T) {
	env1 := baseEnvelope()
	env1.RequiredCapabilities = []string{"beta", "alpha", "gamma"}
	env1.Metadata = map[string]string{"k2": "v2", "k1": "v1"}
	env1.ContextRefs = []harnessmodel.ContextRef{
		{ID: "b", URI: "uri://b"},
		{ID: "a", URI: "uri://a"},
	}

	env2 := baseEnvelope()
	env2.RequiredCapabilities = []string{"gamma", "alpha", "beta"} // reordered
	env2.Metadata = map[string]string{"k1": "v1", "k2": "v2"}      // reordered
	env2.ContextRefs = []harnessmodel.ContextRef{
		{ID: "a", URI: "uri://a"},
		{ID: "b", URI: "uri://b"}, // reordered
	}

	d1, err := env1.Digest()
	if err != nil {
		t.Fatal(err)
	}
	d2, err := env2.Digest()
	if err != nil {
		t.Fatal(err)
	}

	if d1 != d2 {
		t.Fatalf("digests differ for reordered fields: %q vs %q", d1, d2)
	}

	// Tampering test: altering instructions changes digest
	envTampered := env1
	envTampered.Instructions = "Altered instructions"
	dTampered, err := envTampered.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if d1 == dTampered {
		t.Fatal("digest did not change after altering instructions")
	}

	// Tampering test: altering plan digest changes digest
	envTamperedPlan := env1
	envTamperedPlan.PlanDigest = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	dTamperedPlan, err := envTamperedPlan.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if d1 == dTamperedPlan {
		t.Fatal("digest did not change after altering plan digest")
	}
}

func TestFromNodeRun(t *testing.T) {
	spec := harnessmodel.NodeSpec{
		ID:           "node_audit",
		Kind:         harnessmodel.NodeKindAction,
		ExecutorKind: harnessmodel.ExecutorAgent,
		Policy: harnessmodel.PolicySpec{
			RequiredCapabilities: []string{"tools"},
		},
		InputRefs: []harnessmodel.InputRef{
			{Name: "input1", FromNodeID: "node_prev", ArtifactID: "art1"},
		},
		Metadata: map[string]string{
			"taskClass": "audit",
			"title":     "System audit",
			"objective": "Verify invariants",
			"role":      "worker",
		},
	}
	run := harnessmodel.NodeRun{
		ID:            "nr_001",
		WorkflowRunID: "wfr_001",
		NodeID:        "node_audit",
	}
	attempt := harnessmodel.Attempt{
		ID:     "att_001",
		Number: 1,
	}
	workspace := harnessmodel.WorkspaceSpec{
		RootPath: "c:/workspace",
		RepoID:   "repo_main",
	}

	env, err := FromNodeRun("tenv_nr_001", spec, run, attempt, samplePlanDigest, workspace, "Run full audit")
	if err != nil {
		t.Fatalf("FromNodeRun failed: %v", err)
	}

	if env.TaskClass != harnessmodel.TaskClassAudit {
		t.Fatalf("task class = %q, want audit", env.TaskClass)
	}
	if env.Title != "System audit" {
		t.Fatalf("title = %q, want 'System audit'", env.Title)
	}
	if env.Role != "worker" {
		t.Fatalf("role = %q, want 'worker'", env.Role)
	}
	if len(env.ContextRefs) != 1 || env.ContextRefs[0].ID != "input1" {
		t.Fatalf("unexpected context refs: %+v", env.ContextRefs)
	}
	if env.PlanDigest != samplePlanDigest {
		t.Fatalf("plan digest = %q, want %q", env.PlanDigest, samplePlanDigest)
	}
}

func TestToSelectorRequest(t *testing.T) {
	env := baseEnvelope()
	env.PreferredProvider = harnessmodel.ProviderAntigravity
	env.PreferredModel = "gemini-2.5-pro"
	env.MaxTokens = 128000

	req := ToSelectorRequest(env)
	if req.TaskClass != "codegen" {
		t.Fatalf("req.TaskClass = %q, want codegen", req.TaskClass)
	}
	if req.RepositoryID != "repo1" {
		t.Fatalf("req.RepositoryID = %q, want repo1", req.RepositoryID)
	}
	if req.RequiredContext != 128000 {
		t.Fatalf("req.RequiredContext = %d, want 128000", req.RequiredContext)
	}
	if req.PreferredProvider != harnessmodel.ProviderAntigravity {
		t.Fatalf("req.PreferredProvider = %q, want antigravity", req.PreferredProvider)
	}
	if req.PreferredModelID != "gemini-2.5-pro" {
		t.Fatalf("req.PreferredModelID = %q, want gemini-2.5-pro", req.PreferredModelID)
	}
}

func TestCheckPlanDrift(t *testing.T) {
	planText := []byte("# MASTER PLAN\n\nTask list here...")
	digest := harnessmodel.ComputePlanDigest(planText)

	env := baseEnvelope()
	env.PlanDigest = digest

	// Consistent plan
	if err := CheckPlanDrift(env, planText); err != nil {
		t.Fatalf("expected plan consistency, got %v", err)
	}

	// Drifted plan
	mutatedPlan := []byte("# MASTER PLAN\n\nTask list here... (mutated!)")
	err := CheckPlanDrift(env, mutatedPlan)
	if err == nil {
		t.Fatal("expected plan drift error, got nil")
	}
	if !errors.Is(err, harnessmodel.ErrStalePlan) {
		t.Fatalf("expected ErrStalePlan, got %v", err)
	}
}
