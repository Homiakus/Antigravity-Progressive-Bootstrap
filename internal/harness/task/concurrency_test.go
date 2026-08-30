package task

import (
	"fmt"
	"sync"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestConcurrentEnvelopeDigests(t *testing.T) {
	const goroutines = 128
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			env := harnessmodel.TaskEnvelope{
				ID:           harnessmodel.TaskEnvelopeID(fmt.Sprintf("tenv_%04d", idx)),
				TaskID:       fmt.Sprintf("task_%d", idx%10),
				PlanDigest:   samplePlanDigest,
				TaskClass:    harnessmodel.TaskClassCodegen,
				Title:        fmt.Sprintf("Task %d", idx),
				Objective:    "Execute concurrent test",
				Instructions: fmt.Sprintf("Run step %d", idx),
				Workspace: harnessmodel.WorkspaceSpec{
					RootPath: fmt.Sprintf("c:/repo/%d", idx%4),
					RepoID:   "repo_concurrent",
					ReadOnly: true,
				},
				Role:                 "worker",
				RequiredCapabilities: []string{"tools", "file_edit"},
				CreatedAt:            time.Now().UTC(),
			}

			if err := env.Validate(); err != nil {
				t.Errorf("goroutine %d: Validate() failed: %v", idx, err)
				return
			}

			d1, err := env.Digest()
			if err != nil {
				t.Errorf("goroutine %d: Digest() failed: %v", idx, err)
				return
			}
			d2, err := env.Digest()
			if err != nil {
				t.Errorf("goroutine %d: second Digest() failed: %v", idx, err)
				return
			}
			if d1 != d2 {
				t.Errorf("goroutine %d: non-deterministic digest: %q vs %q", idx, d1, d2)
			}
		}(i)
	}

	wg.Wait()
}
