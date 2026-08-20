package sqlite

import (
	"context"
	"sync"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestRetryBudgetConcurrentReservationsNeverExceedLimit(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(18_000, 123_456_789).UTC()
	const callers = 32
	const limit = 7

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	errs := make([]error, 0)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := db.Update(ctx, func(tx harnessstore.Tx) error {
				_, ok, err := tx.ReserveRetryBudget(ctx, harnessmodel.RetryBudgetService, "provider-contention", time.Minute, limit, now)
				if err != nil {
					return err
				}
				if ok {
					mu.Lock()
					allowed++
					mu.Unlock()
				}
				return nil
			})
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(errs) != 0 {
		t.Fatalf("concurrent budget reservations returned errors: %v", errs)
	}
	if allowed != limit {
		t.Fatalf("allowed reservations=%d want=%d", allowed, limit)
	}

	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		budget, err := reader.GetRetryBudget(ctx, harnessmodel.RetryBudgetService, "provider-contention")
		if err != nil {
			return err
		}
		if budget.Used != limit || budget.Limit != limit || budget.Window != time.Minute {
			t.Fatalf("durable budget mismatch after contention: %+v", budget)
		}
		if !budget.WindowStart.Equal(now) {
			t.Fatalf("budget window start=%s want=%s", budget.WindowStart, now)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRetryBudgetConcurrentExpiredWindowResetsOnlyOnce(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	start := time.Unix(19_000, 0).UTC()
	const limit = 5
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		_, ok, err := tx.ReserveRetryBudget(ctx, harnessmodel.RetryBudgetWorkflow, "window-reset", time.Second, limit, start)
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("initial reservation denied")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	now := start.Add(2 * time.Second)
	const callers = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	errs := make([]error, 0)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := db.Update(ctx, func(tx harnessstore.Tx) error {
				_, ok, err := tx.ReserveRetryBudget(ctx, harnessmodel.RetryBudgetWorkflow, "window-reset", time.Second, limit, now)
				if err != nil {
					return err
				}
				if ok {
					mu.Lock()
					allowed++
					mu.Unlock()
				}
				return nil
			})
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(errs) != 0 {
		t.Fatalf("concurrent reset reservations returned errors: %v", errs)
	}
	if allowed != limit {
		t.Fatalf("post-reset allowed=%d want=%d", allowed, limit)
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		budget, err := reader.GetRetryBudget(ctx, harnessmodel.RetryBudgetWorkflow, "window-reset")
		if err != nil {
			return err
		}
		if budget.Used != limit || !budget.WindowStart.Equal(now) {
			t.Fatalf("expired window reset incorrectly: %+v", budget)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
