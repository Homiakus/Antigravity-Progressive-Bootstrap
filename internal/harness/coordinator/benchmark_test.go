package coordinator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/engineering"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func BenchmarkCommitCoordinatorApply100Operations(b *testing.B) {
	ctx := context.Background()
	tempDir := b.TempDir()

	targetRoot := filepath.Join(tempDir, "target")
	isolationRoot := filepath.Join(tempDir, "isolation")

	if err := os.MkdirAll(targetRoot, 0755); err != nil {
		b.Fatal(err)
	}
	if err := os.MkdirAll(isolationRoot, 0755); err != nil {
		b.Fatal(err)
	}

	for i := 0; i < 100; i++ {
		fileName := fmt.Sprintf("file_%d.go", i)
		content := []byte(fmt.Sprintf("package main\n\nconst Val = %d\n", i))
		if err := os.WriteFile(filepath.Join(isolationRoot, fileName), content, 0644); err != nil {
			b.Fatal(err)
		}
	}

	planText := []byte("# MASTER PLAN\n\nBenchmark commit coordination...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	now := time.Unix(8000, 0).UTC()
	coord := NewCommitCoordinator(func() time.Time { return now })

	files := make([]string, 100)
	for i := 0; i < 100; i++ {
		files[i] = fmt.Sprintf("file_%d.go", i)
	}

	req := CandidateCommit{
		TaskID:        "T-018",
		AttemptID:     "att_bench",
		Role:          engineering.RoleCoordinator,
		PlanDigest:    planDigest,
		TargetRoot:    targetRoot,
		IsolationRoot: isolationRoot,
		ModifiedFiles: files,
		CommitMessage: "bench commit",
		Author:        "coordinator",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		res, err := coord.ApplyAndVerify(ctx, req, planText)
		if err != nil || !res.Success {
			b.Fatalf("ApplyAndVerify failed: %v", err)
		}
	}
}

func BenchmarkPublisherEvaluate100Operations(b *testing.B) {
	ctx := context.Background()
	git := &mockGitClient{
		headSHA: "abc1234567890abcdef1234567890abcdef12345",
		isClean: true,
	}

	planText := []byte("# MASTER PLAN\n\nBenchmark publisher...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	now := time.Unix(11000, 0).UTC()
	pub := NewPublisher(git, func() time.Time { return now })

	req := PublicationRequest{
		Role:            engineering.RoleCoordinator,
		RemoteName:      "origin",
		BranchName:      "main",
		PlanDigest:      planDigest,
		ForcePush:       false,
		ExpectedHeadSHA: "abc1234567890abcdef1234567890abcdef12345",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			res, err := pub.Publish(ctx, req, planText)
			if err != nil || !res.Success {
				b.Fatalf("Publish failed: %v", err)
			}
		}
	}
}
