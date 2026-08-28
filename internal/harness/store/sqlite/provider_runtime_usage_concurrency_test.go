package sqlite

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestProviderUsageSampleConcurrentReplayCreatesExactlyOnce(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(2300, 0).UTC()
	seedProviderRuntimeParents(t, db, now)
	assignment := harnessmodel.ProviderAssignment{
		ID: "pasn_usage_race", AttemptID: testProviderAttemptID, AccountID: testProviderAccountID, ModelID: "model-a",
		State: harnessmodel.ProviderAssignmentActive, Revision: 1, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateProviderAssignment(ctx, assignment) }); err != nil {
		t.Fatal(err)
	}
	sample := harnessmodel.ProviderUsageSample{
		Key: "provider:event:race-usage", AssignmentID: assignment.ID, AccountID: assignment.AccountID, ModelID: assignment.ModelID,
		Metric: harnessmodel.QuotaMetricTokens, Amount: 42, ObservedAt: now.Add(2 * time.Second), CreatedAt: now.Add(3 * time.Second),
	}

	var created atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			replay := sample
			replay.CreatedAt = sample.CreatedAt.Add(time.Duration(i) * time.Nanosecond)
			err := db.Update(ctx, func(tx harnessstore.Tx) error {
				stored, wasCreated, err := tx.PutProviderUsageSample(ctx, replay)
				if err != nil {
					return err
				}
				if stored.Key != sample.Key || stored.Amount != sample.Amount || !stored.ObservedAt.Equal(sample.ObservedAt) {
					return fmt.Errorf("unexpected replay result: %+v", stored)
				}
				if wasCreated {
					created.Add(1)
				}
				return nil
			})
			if err != nil {
				t.Errorf("concurrent usage replay: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := created.Load(); got != 1 {
		t.Fatalf("usage rows created=%d want=1", got)
	}

	if err := db.View(ctx, func(r harnessstore.Reader) error {
		samples, err := r.ListProviderUsageSamplesByAssignment(ctx, assignment.ID, 10)
		if err != nil {
			return err
		}
		if len(samples) != 1 || samples[0].Key != sample.Key {
			t.Fatalf("usage samples=%+v want one %s", samples, sample.Key)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
