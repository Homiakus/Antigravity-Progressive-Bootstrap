package router

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestRouterIsolatedWriteConcurrency64Workers(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	now := time.Unix(4000, 0).UTC()
	seedTestAccounts(t, db, now)

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	router := NewRouter(db, Options{
		Now: func() time.Time { return now },
	})

	const workers = 64
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()

			env := harnessmodel.TaskEnvelope{
				ID:           harnessmodel.TaskEnvelopeID(fmt.Sprintf("tenv_conc_write_%d", idx)),
				TaskID:       fmt.Sprintf("T-%03d", 100+idx),
				AttemptID:    harnessmodel.AttemptID(fmt.Sprintf("att_conc_write_%d", idx)),
				PlanDigest:   planDigest,
				TaskClass:    harnessmodel.TaskClassCodegen,
				Title:        fmt.Sprintf("Concurrent task %d", idx),
				Objective:    "Perform isolated concurrent edit",
				Instructions: "Safely execute isolated write in parallel",
				Workspace: harnessmodel.WorkspaceSpec{
					RootPath:      "c:/repo",
					RepoID:        "repo1",
					ReadOnly:      false,
					Isolated:      true,
					IsolationRoot: fmt.Sprintf("c:/repo/.scratch/worker_%d", idx),
				},
				Role:                 "worker",
				RequiredCapabilities: []string{"tools", "file_edit"},
				MaxTokens:            1000,
				CreatedAt:            now,
			}

			route, err := router.RouteIsolatedWrite(ctx, env, planText)
			if err != nil {
				t.Errorf("worker %d: RouteIsolatedWrite failed: %v", idx, err)
				return
			}

			result, err := router.ExecuteIsolatedWrite(ctx, route, func(ctx context.Context, e harnessmodel.TaskEnvelope, a harnessmodel.ProviderAssignment) (IsolatedWriteOutput, error) {
				return IsolatedWriteOutput{
					ModifiedFiles: []string{fmt.Sprintf("worker_%d.go", idx)},
					DiffSummary:   "+10 -1",
					Output:        "completed",
					TokensUsed:    500,
				}, nil
			})
			if err != nil {
				t.Errorf("worker %d: ExecuteIsolatedWrite failed: %v", idx, err)
				return
			}
			if !result.Success {
				t.Errorf("worker %d: result not success: %s", idx, result.Error)
			}
		}(i)
	}

	wg.Wait()
}
