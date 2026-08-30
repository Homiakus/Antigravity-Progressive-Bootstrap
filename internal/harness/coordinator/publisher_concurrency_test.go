package coordinator

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/engineering"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestPublisherConcurrency64Workers(t *testing.T) {
	ctx := context.Background()
	git := &mockGitClient{
		headSHA: "abc1234567890abcdef1234567890abcdef12345",
		isClean: true,
	}

	planText := []byte("# MASTER PLAN\n\nConcurrent publication...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	now := time.Unix(10000, 0).UTC()
	pub := NewPublisher(git, func() time.Time { return now })

	const workers = 64
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()

			req := PublicationRequest{
				Role:            engineering.RoleCoordinator,
				RemoteName:      "origin",
				BranchName:      "main",
				PlanDigest:      planDigest,
				ForcePush:       false,
				ExpectedHeadSHA: "abc1234567890abcdef1234567890abcdef12345",
			}

			res, err := pub.Publish(ctx, req, planText)
			if err != nil {
				t.Errorf("worker %d publish: %v", idx, err)
				return
			}
			if !res.Success {
				t.Errorf("worker %d res not success", idx)
			}
		}(i)
	}

	wg.Wait()
}
