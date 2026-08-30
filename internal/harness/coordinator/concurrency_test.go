package coordinator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/engineering"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestCommitCoordinatorConcurrency64Workers(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	planText := []byte("# MASTER PLAN\n\nConcurrent coordination...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	now := time.Unix(7000, 0).UTC()
	coord := NewCommitCoordinator(func() time.Time { return now })

	const workers = 64
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()

			targetRoot := filepath.Join(tempDir, fmt.Sprintf("target_%d", idx))
			isolationRoot := filepath.Join(tempDir, fmt.Sprintf("iso_%d", idx))

			if err := os.MkdirAll(targetRoot, 0755); err != nil {
				t.Errorf("worker %d mkdir target: %v", idx, err)
				return
			}
			if err := os.MkdirAll(isolationRoot, 0755); err != nil {
				t.Errorf("worker %d mkdir iso: %v", idx, err)
				return
			}

			fileName := fmt.Sprintf("worker_%d.go", idx)
			content := []byte(fmt.Sprintf("package main\n\nconst ID = %d\n", idx))
			if err := os.WriteFile(filepath.Join(isolationRoot, fileName), content, 0644); err != nil {
				t.Errorf("worker %d write file: %v", idx, err)
				return
			}

			req := CandidateCommit{
				TaskID:        fmt.Sprintf("T-%03d", idx),
				AttemptID:     fmt.Sprintf("att_%d", idx),
				Role:          engineering.RoleCoordinator,
				PlanDigest:    planDigest,
				TargetRoot:    targetRoot,
				IsolationRoot: isolationRoot,
				ModifiedFiles: []string{fileName},
				CommitMessage: fmt.Sprintf("commit %d", idx),
				Author:        "coordinator",
			}

			res, err := coord.ApplyAndVerify(ctx, req, planText)
			if err != nil {
				t.Errorf("worker %d ApplyAndVerify: %v", idx, err)
				return
			}
			if !res.Success {
				t.Errorf("worker %d res not success", idx)
			}
		}(i)
	}

	wg.Wait()
}
