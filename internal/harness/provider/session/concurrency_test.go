package session

import (
	"sync"
	"testing"
)

// The broker evaluator is intentionally pure and may be called by parallel
// selectors. This sentinel catches accidental shared mutable state or
// nondeterministic candidate selection under concurrency; the repository-wide
// race gate provides the corresponding data-race proof.
func TestEvaluateConcurrentDeterministic(t *testing.T) {
	const workers = 128

	snapshot := baseSnapshot()
	request := baseRequest()
	policy := Policy{AcquireReuseHeadroomFraction: 0.30, RetainReuseHeadroomFraction: 0.15}
	want, err := Evaluate(snapshot, request, policy)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := Evaluate(snapshot, request, policy)
			if err != nil {
				errs <- err
				return
			}
			if got.Action != want.Action || got.SessionID != want.SessionID || got.Headroom != want.Headroom || got.HeadroomFraction != want.HeadroomFraction {
				errs <- decisionMismatch{got: got, want: want}
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatal(err)
	}
}

type decisionMismatch struct {
	got  Decision
	want Decision
}

func (e decisionMismatch) Error() string {
	return "concurrent session decision diverged from deterministic baseline"
}
