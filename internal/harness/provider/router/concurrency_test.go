package router

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestConcurrentReadOnlyRouting(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	now := time.Unix(2000, 0).UTC()
	seedTestAccounts(t, db, now)

	planText := []byte("# MASTER PLAN\n\nConcurrent tasks...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	router := NewRouter(db, Options{
		Now: func() time.Time { return now },
	})

	const concurrentWorkers = 64
	var wg sync.WaitGroup
	wg.Add(concurrentWorkers)

	for i := 0; i < concurrentWorkers; i++ {
		go func(idx int) {
			defer wg.Done()
			env := makeReadOnlyEnvelope(planDigest)
			env.ID = harnessmodel.TaskEnvelopeID(fmt.Sprintf("tenv_conc_%04d", idx))
			env.AttemptID = harnessmodel.AttemptID(fmt.Sprintf("att_conc_%04d", idx))
			env.Title = fmt.Sprintf("Audit task %d", idx)

			route, err := router.Route(ctx, env, planText)
			if err != nil {
				t.Errorf("worker %d Route failed: %v", idx, err)
				return
			}

			if route.Assignment.ID == "" || route.Assignment.State != harnessmodel.ProviderAssignmentActive {
				t.Errorf("worker %d invalid assignment: %+v", idx, route.Assignment)
				return
			}

			// Execute read-only task
			res, err := router.Execute(ctx, route, func(ctx context.Context, e harnessmodel.TaskEnvelope, a harnessmodel.ProviderAssignment) (string, int64, error) {
				return fmt.Sprintf("Result %d", idx), 100, nil
			})
			if err != nil {
				t.Errorf("worker %d Execute failed: %v", idx, err)
				return
			}
			if !res.Success {
				t.Errorf("worker %d Execute not successful", idx)
			}
		}(i)
	}

	wg.Wait()
}
