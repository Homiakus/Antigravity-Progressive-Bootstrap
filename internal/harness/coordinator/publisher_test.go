package coordinator

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/engineering"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

type mockGitClient struct {
	mu           sync.Mutex
	headSHA      string
	isClean      bool
	pushError    error
	pushedRemote string
	pushedBranch string
	pushedForce  bool
}

func (m *mockGitClient) Push(ctx context.Context, remote, branch string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pushedRemote = remote
	m.pushedBranch = branch
	m.pushedForce = force
	return m.pushError
}

func (m *mockGitClient) GetHeadSHA(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.headSHA, nil
}

func (m *mockGitClient) IsWorkingTreeClean(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isClean, nil
}

func TestPublisherSuccessfulPublication(t *testing.T) {
	ctx := context.Background()
	git := &mockGitClient{
		headSHA: "abc1234567890abcdef1234567890abcdef12345",
		isClean: true,
	}

	planText := []byte("# MASTER PLAN\n\nPublication test...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	now := time.Unix(9000, 0).UTC()
	pub := NewPublisher(git, func() time.Time { return now })

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
		t.Fatalf("Publish failed: %v", err)
	}

	if !res.Success {
		t.Fatal("res.Success is false")
	}
	if res.RemoteName != "origin" || res.BranchName != "main" {
		t.Fatalf("res target mismatch: %s/%s", res.RemoteName, res.BranchName)
	}
	if res.HeadSHA != "abc1234567890abcdef1234567890abcdef12345" {
		t.Fatalf("res.HeadSHA mismatch: %s", res.HeadSHA)
	}
	if git.pushedForce {
		t.Fatal("git.pushedForce is true; must be false")
	}
}

func TestPublisherWorkerAuthorityRejection(t *testing.T) {
	ctx := context.Background()
	git := &mockGitClient{headSHA: "abc", isClean: true}

	planText := []byte("# MASTER PLAN\n\nPublication test...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	pub := NewPublisher(git, nil)

	// 1. RoleWorker must fail
	req := PublicationRequest{
		Role:       engineering.RoleWorker,
		RemoteName: "origin",
		BranchName: "main",
		PlanDigest: planDigest,
		ForcePush:  false,
	}

	_, err := pub.Publish(ctx, req, planText)
	if err == nil {
		t.Fatal("expected ErrUnauthorizedRole for worker, got nil")
	}
	if !errors.Is(err, ErrUnauthorizedRole) {
		t.Fatalf("expected ErrUnauthorizedRole, got %v", err)
	}

	// 2. Untyped role must fail
	req.Role = engineering.Role("random_actor")
	_, err = pub.Publish(ctx, req, planText)
	if err == nil {
		t.Fatal("expected ErrUnauthorizedRole for untyped role, got nil")
	}
	if !errors.Is(err, ErrUnauthorizedRole) {
		t.Fatalf("expected ErrUnauthorizedRole, got %v", err)
	}
}

func TestPublisherForcePushRejection(t *testing.T) {
	ctx := context.Background()
	git := &mockGitClient{headSHA: "abc", isClean: true}

	planText := []byte("# MASTER PLAN\n\nPublication test...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	pub := NewPublisher(git, nil)

	req := PublicationRequest{
		Role:       engineering.RoleCoordinator,
		RemoteName: "origin",
		BranchName: "main",
		PlanDigest: planDigest,
		ForcePush:  true, // illegal!
	}

	_, err := pub.Publish(ctx, req, planText)
	if err == nil {
		t.Fatal("expected ErrForcePushForbidden, got nil")
	}
	if !errors.Is(err, ErrForcePushForbidden) {
		t.Fatalf("expected ErrForcePushForbidden, got %v", err)
	}
}

func TestPublisherPlanDriftRejection(t *testing.T) {
	ctx := context.Background()
	git := &mockGitClient{headSHA: "abc", isClean: true}

	planText := []byte("# MASTER PLAN\n\nPublication test...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	tamperedPlan := []byte("# MASTER PLAN\n\nTampered publication...")

	pub := NewPublisher(git, nil)

	req := PublicationRequest{
		Role:       engineering.RoleCoordinator,
		RemoteName: "origin",
		BranchName: "main",
		PlanDigest: planDigest,
		ForcePush:  false,
	}

	_, err := pub.Publish(ctx, req, tamperedPlan)
	if err == nil {
		t.Fatal("expected ErrPlanDrift, got nil")
	}
	if !errors.Is(err, ErrPlanDrift) {
		t.Fatalf("expected ErrPlanDrift, got %v", err)
	}
}

func TestPublisherDirtyWorkingTreeRejection(t *testing.T) {
	ctx := context.Background()
	git := &mockGitClient{headSHA: "abc", isClean: false} // dirty!

	planText := []byte("# MASTER PLAN\n\nPublication test...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	pub := NewPublisher(git, nil)

	req := PublicationRequest{
		Role:       engineering.RoleCoordinator,
		RemoteName: "origin",
		BranchName: "main",
		PlanDigest: planDigest,
		ForcePush:  false,
	}

	_, err := pub.Publish(ctx, req, planText)
	if err == nil {
		t.Fatal("expected ErrDirtyWorkingTree, got nil")
	}
	if !errors.Is(err, ErrDirtyWorkingTree) {
		t.Fatalf("expected ErrDirtyWorkingTree, got %v", err)
	}
}

func TestPublisherRemoteAndBranchValidation(t *testing.T) {
	ctx := context.Background()
	git := &mockGitClient{headSHA: "abc", isClean: true}

	planText := []byte("# MASTER PLAN\n\nPublication test...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	pub := NewPublisher(git, nil)

	// Non-origin remote
	req := PublicationRequest{
		Role:       engineering.RoleCoordinator,
		RemoteName: "upstream",
		BranchName: "main",
		PlanDigest: planDigest,
	}
	_, err := pub.Publish(ctx, req, planText)
	if err == nil || !errors.Is(err, ErrRemoteMismatch) {
		t.Fatalf("expected ErrRemoteMismatch, got %v", err)
	}

	// Non-main branch
	req.RemoteName = "origin"
	req.BranchName = "feature"
	_, err = pub.Publish(ctx, req, planText)
	if err == nil || !errors.Is(err, ErrBranchMismatch) {
		t.Fatalf("expected ErrBranchMismatch, got %v", err)
	}
}
