package coordinator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/engineering"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestCommitCoordinatorSuccessfulApplication(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	targetRoot := filepath.Join(tempDir, "target")
	isolationRoot := filepath.Join(tempDir, "isolation")

	if err := os.MkdirAll(targetRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(isolationRoot, "pkg"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create files in isolation root
	if err := os.WriteFile(filepath.Join(isolationRoot, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(isolationRoot, "pkg", "util.go"), []byte("package pkg\n\nfunc Util() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	now := time.Unix(6000, 0).UTC()
	coord := NewCommitCoordinator(func() time.Time { return now })

	req := CandidateCommit{
		TaskID:        "T-018",
		AttemptID:     "att_001",
		Role:          engineering.RoleCoordinator,
		PlanDigest:    planDigest,
		TargetRoot:    targetRoot,
		IsolationRoot: isolationRoot,
		ModifiedFiles: []string{"main.go", "pkg/util.go"},
		CommitMessage: "feat: add main and util",
		Author:        "coordinator",
	}

	result, err := coord.ApplyAndVerify(ctx, req, planText)
	if err != nil {
		t.Fatalf("ApplyAndVerify failed: %v", err)
	}

	if !result.Success {
		t.Fatal("result.Success is false")
	}
	if result.TaskID != "T-018" {
		t.Fatalf("result.TaskID = %q, want T-018", result.TaskID)
	}
	if len(result.AppliedFiles) != 2 {
		t.Fatalf("len(result.AppliedFiles) = %d, want 2", len(result.AppliedFiles))
	}
	if result.TreeDigest == "" {
		t.Fatal("result.TreeDigest is empty")
	}

	// Verify target root has applied files
	mainContent, err := os.ReadFile(filepath.Join(targetRoot, "main.go"))
	if err != nil || string(mainContent) != "package main\n\nfunc main() {}\n" {
		t.Fatalf("target main.go content invalid: %v, content: %s", err, string(mainContent))
	}

	utilContent, err := os.ReadFile(filepath.Join(targetRoot, "pkg", "util.go"))
	if err != nil || string(utilContent) != "package pkg\n\nfunc Util() {}\n" {
		t.Fatalf("target util.go content invalid: %v, content: %s", err, string(utilContent))
	}
}

func TestCommitCoordinatorWorkerAuthorityRejection(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	targetRoot := filepath.Join(tempDir, "target")
	isolationRoot := filepath.Join(tempDir, "isolation")

	_ = os.MkdirAll(targetRoot, 0755)
	_ = os.MkdirAll(isolationRoot, 0755)
	_ = os.WriteFile(filepath.Join(isolationRoot, "worker.go"), []byte("package main\n"), 0644)

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	coord := NewCommitCoordinator(nil)

	// 1. RoleWorker must be rejected
	workerReq := CandidateCommit{
		TaskID:        "T-018",
		Role:          engineering.RoleWorker,
		PlanDigest:    planDigest,
		TargetRoot:    targetRoot,
		IsolationRoot: isolationRoot,
		ModifiedFiles: []string{"worker.go"},
	}

	_, err := coord.ApplyAndVerify(ctx, workerReq, planText)
	if err == nil {
		t.Fatal("expected ErrUnauthorizedRole for RoleWorker, got nil")
	}
	if !errors.Is(err, ErrUnauthorizedRole) {
		t.Fatalf("expected ErrUnauthorizedRole, got %v", err)
	}

	// 2. Unknown role must be rejected
	unknownReq := workerReq
	unknownReq.Role = engineering.Role("random_intruder")
	_, err = coord.ApplyAndVerify(ctx, unknownReq, planText)
	if err == nil {
		t.Fatal("expected ErrUnauthorizedRole for unknown role, got nil")
	}
	if !errors.Is(err, ErrUnauthorizedRole) {
		t.Fatalf("expected ErrUnauthorizedRole, got %v", err)
	}
}

func TestCommitCoordinatorPlanDriftRejection(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	targetRoot := filepath.Join(tempDir, "target")
	isolationRoot := filepath.Join(tempDir, "isolation")

	_ = os.MkdirAll(targetRoot, 0755)
	_ = os.MkdirAll(isolationRoot, 0755)
	_ = os.WriteFile(filepath.Join(isolationRoot, "file.go"), []byte("package main\n"), 0644)

	planText := []byte("# MASTER PLAN\n\nOriginal plan...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	tamperedPlan := []byte("# MASTER PLAN\n\nDrifted plan content...")

	coord := NewCommitCoordinator(nil)

	req := CandidateCommit{
		TaskID:        "T-018",
		Role:          engineering.RoleCoordinator,
		PlanDigest:    planDigest,
		TargetRoot:    targetRoot,
		IsolationRoot: isolationRoot,
		ModifiedFiles: []string{"file.go"},
	}

	_, err := coord.ApplyAndVerify(ctx, req, tamperedPlan)
	if err == nil {
		t.Fatal("expected ErrPlanDrift for drifted plan, got nil")
	}
	if !errors.Is(err, ErrPlanDrift) {
		t.Fatalf("expected ErrPlanDrift, got %v", err)
	}
}

func TestCommitCoordinatorPathEscapeRejection(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	targetRoot := filepath.Join(tempDir, "target")
	isolationRoot := filepath.Join(tempDir, "isolation")

	_ = os.MkdirAll(targetRoot, 0755)
	_ = os.MkdirAll(isolationRoot, 0755)

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	coord := NewCommitCoordinator(nil)

	cases := []string{
		"../escape.go",
		"../../outside.go",
		"/abs/path.go",
		"foo/../../bar.go",
	}

	for _, badPath := range cases {
		t.Run(badPath, func(t *testing.T) {
			req := CandidateCommit{
				TaskID:        "T-018",
				Role:          engineering.RoleCoordinator,
				PlanDigest:    planDigest,
				TargetRoot:    targetRoot,
				IsolationRoot: isolationRoot,
				ModifiedFiles: []string{badPath},
			}

			_, err := coord.ApplyAndVerify(ctx, req, planText)
			if err == nil {
				t.Fatalf("expected ErrPathEscape for %q, got nil", badPath)
			}
			if !errors.Is(err, ErrPathEscape) {
				t.Fatalf("expected ErrPathEscape, got %v", err)
			}
		})
	}
}

func TestCommitCoordinatorMissingIsolationRootOrFile(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	targetRoot := filepath.Join(tempDir, "target")
	isolationRoot := filepath.Join(tempDir, "isolation")
	_ = os.MkdirAll(targetRoot, 0755)

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	coord := NewCommitCoordinator(nil)

	// Nonexistent isolation root
	req := CandidateCommit{
		TaskID:        "T-018",
		Role:          engineering.RoleCoordinator,
		PlanDigest:    planDigest,
		TargetRoot:    targetRoot,
		IsolationRoot: filepath.Join(tempDir, "nonexistent"),
		ModifiedFiles: []string{"file.go"},
	}

	_, err := coord.ApplyAndVerify(ctx, req, planText)
	if err == nil {
		t.Fatal("expected ErrMissingIsolationRoot, got nil")
	}
	if !errors.Is(err, ErrMissingIsolationRoot) {
		t.Fatalf("expected ErrMissingIsolationRoot, got %v", err)
	}

	// Missing modified file inside isolation root
	_ = os.MkdirAll(isolationRoot, 0755)
	req.IsolationRoot = isolationRoot
	req.ModifiedFiles = []string{"missing.go"}

	_, err = coord.ApplyAndVerify(ctx, req, planText)
	if err == nil {
		t.Fatal("expected error for missing modified file, got nil")
	}
}
