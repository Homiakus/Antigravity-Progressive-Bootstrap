package selector

import (
	"context"
	"sync"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestConcurrentEvaluations(t *testing.T) {
	now := time.Now().UTC()
	policy := DefaultPolicy()
	req := Request{
		TaskClass:            "codegen",
		RequiredCapabilities: []string{"tools"},
	}

	candidates := []Candidate{
		baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model1"),
		baseCandidate(harnessmodel.ProviderCodex, "acc2", "model2"),
	}

	const workers = 128
	var wg sync.WaitGroup
	wg.Add(workers)

	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			dec, err := Evaluate(context.Background(), req, candidates, now, policy)
			if err != nil {
				errs <- err
				return
			}
			if dec.Selected == nil {
				errs <- err
				return
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent evaluation error: %v", err)
		}
	}
}
