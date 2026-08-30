package fault

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestConcurrentFaultDecisionsAndCircuitCAS(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	cm := NewCircuitManager(db)
	now := time.Unix(2000, 0).UTC()
	policy := DefaultPolicy()

	const workers = 64
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			accountID := harnessmodel.ProviderAccountID(fmt.Sprintf("acc_conc_%d", idx%4))
			modelID := harnessmodel.ProviderModelID(fmt.Sprintf("model_conc_%d", idx%2))

			// Classify concurrent error
			fault := Classify(fmt.Errorf("error from worker %d: 503 service unavailable, retry-after: %dms", idx, (idx%10)*100))
			if fault.Kind != FaultServerOverloaded {
				t.Errorf("worker %d unexpected fault kind: %v", idx, fault.Kind)
				return
			}

			// Decide retry
			dec, err := Decide(DecisionInput{
				Fault:                fault,
				TotalAttempts:        1 + (idx % 3),
				SameProviderAttempts: 1 + (idx % 2),
				Policy:               policy,
				Now:                  now,
			})
			if err != nil {
				t.Errorf("worker %d decide error: %v", idx, err)
				return
			}
			if dec.Action == "" {
				t.Errorf("worker %d empty decision action", idx)
				return
			}

			// Concurrent circuit breaker record
			if idx%2 == 0 {
				_, err := cm.RecordFailure(ctx, accountID, modelID, 5, 2*time.Minute, now)
				if err != nil {
					t.Errorf("worker %d RecordFailure error: %v", idx, err)
					return
				}
			} else {
				err := cm.RecordSuccess(ctx, accountID, modelID, now)
				if err != nil {
					t.Errorf("worker %d RecordSuccess error: %v", idx, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
}
