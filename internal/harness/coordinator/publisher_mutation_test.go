package coordinator

import (
	"context"
	"errors"
	"testing"

	"github.com/homiakus/agctl/internal/engineering"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestPublisherMutationSentinels(t *testing.T) {
	ctx := context.Background()
	git := &mockGitClient{
		headSHA: "abc1234567890abcdef1234567890abcdef12345",
		isClean: true,
	}

	planText := []byte("# MASTER PLAN\n\nPublication test...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	pub := NewPublisher(git, nil)

	t.Run("mutant: worker role authorized to publish", func(t *testing.T) {
		req := PublicationRequest{
			Role:       engineering.RoleWorker,
			RemoteName: "origin",
			BranchName: "main",
			PlanDigest: planDigest,
			ForcePush:  false,
		}

		_, err := pub.Publish(ctx, req, planText)
		if err == nil {
			t.Fatal("mutant survived: worker role was permitted to publish")
		}
		if !errors.Is(err, ErrUnauthorizedRole) {
			t.Fatalf("expected ErrUnauthorizedRole, got %v", err)
		}
	})

	t.Run("mutant: force push permitted", func(t *testing.T) {
		req := PublicationRequest{
			Role:       engineering.RoleCoordinator,
			RemoteName: "origin",
			BranchName: "main",
			PlanDigest: planDigest,
			ForcePush:  true,
		}

		_, err := pub.Publish(ctx, req, planText)
		if err == nil {
			t.Fatal("mutant survived: force push was permitted")
		}
		if !errors.Is(err, ErrForcePushForbidden) {
			t.Fatalf("expected ErrForcePushForbidden, got %v", err)
		}
	})

	t.Run("mutant: living plan drift admitted by a single character", func(t *testing.T) {
		tamperedPlan := append([]byte(nil), planText...)
		tamperedPlan[len(tamperedPlan)-1] = 'Z'

		req := PublicationRequest{
			Role:       engineering.RoleCoordinator,
			RemoteName: "origin",
			BranchName: "main",
			PlanDigest: planDigest,
			ForcePush:  false,
		}

		_, err := pub.Publish(ctx, req, tamperedPlan)
		if err == nil {
			t.Fatal("mutant survived: plan drift was admitted in publisher")
		}
		if !errors.Is(err, ErrPlanDrift) {
			t.Fatalf("expected ErrPlanDrift, got %v", err)
		}
	})

	t.Run("mutant: dirty working tree ignored", func(t *testing.T) {
		dirtyGit := &mockGitClient{headSHA: "abc", isClean: false}
		dirtyPub := NewPublisher(dirtyGit, nil)

		req := PublicationRequest{
			Role:       engineering.RoleCoordinator,
			RemoteName: "origin",
			BranchName: "main",
			PlanDigest: planDigest,
			ForcePush:  false,
		}

		_, err := dirtyPub.Publish(ctx, req, planText)
		if err == nil {
			t.Fatal("mutant survived: dirty working tree was ignored")
		}
		if !errors.Is(err, ErrDirtyWorkingTree) {
			t.Fatalf("expected ErrDirtyWorkingTree, got %v", err)
		}
	})
}
