package coordinator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/homiakus/agctl/internal/engineering"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestCommitCoordinatorMutationSentinels(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	targetRoot := filepath.Join(tempDir, "target")
	isolationRoot := filepath.Join(tempDir, "isolation")

	_ = os.MkdirAll(targetRoot, 0755)
	_ = os.MkdirAll(isolationRoot, 0755)
	_ = os.WriteFile(filepath.Join(isolationRoot, "file.go"), []byte("package main\n"), 0644)

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	coord := NewCommitCoordinator(nil)

	t.Run("mutant: worker role authorized to commit", func(t *testing.T) {
		req := CandidateCommit{
			TaskID:        "T-018",
			Role:          engineering.RoleWorker,
			PlanDigest:    planDigest,
			TargetRoot:    targetRoot,
			IsolationRoot: isolationRoot,
			ModifiedFiles: []string{"file.go"},
		}

		_, err := coord.ApplyAndVerify(ctx, req, planText)
		if err == nil {
			t.Fatal("mutant survived: worker role was permitted to commit")
		}
		if !errors.Is(err, ErrUnauthorizedRole) {
			t.Fatalf("expected ErrUnauthorizedRole, got %v", err)
		}
	})

	t.Run("mutant: arbitrary untyped role authorized to commit", func(t *testing.T) {
		req := CandidateCommit{
			TaskID:        "T-018",
			Role:          engineering.Role("worker_elevated"),
			PlanDigest:    planDigest,
			TargetRoot:    targetRoot,
			IsolationRoot: isolationRoot,
			ModifiedFiles: []string{"file.go"},
		}

		_, err := coord.ApplyAndVerify(ctx, req, planText)
		if err == nil {
			t.Fatal("mutant survived: untyped role was permitted to commit")
		}
		if !errors.Is(err, ErrUnauthorizedRole) {
			t.Fatalf("expected ErrUnauthorizedRole, got %v", err)
		}
	})

	t.Run("mutant: living plan drift admitted by a single character", func(t *testing.T) {
		tamperedPlan := append([]byte(nil), planText...)
		tamperedPlan[len(tamperedPlan)-1] = 'X'

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
			t.Fatal("mutant survived: plan drift was admitted in commit coordinator")
		}
		if !errors.Is(err, ErrPlanDrift) {
			t.Fatalf("expected ErrPlanDrift, got %v", err)
		}
	})

	t.Run("mutant: relative path escape admitted", func(t *testing.T) {
		req := CandidateCommit{
			TaskID:        "T-018",
			Role:          engineering.RoleCoordinator,
			PlanDigest:    planDigest,
			TargetRoot:    targetRoot,
			IsolationRoot: isolationRoot,
			ModifiedFiles: []string{"../escape.go"},
		}

		_, err := coord.ApplyAndVerify(ctx, req, planText)
		if err == nil {
			t.Fatal("mutant survived: path escape was admitted")
		}
		if !errors.Is(err, ErrPathEscape) {
			t.Fatalf("expected ErrPathEscape, got %v", err)
		}
	})
}
