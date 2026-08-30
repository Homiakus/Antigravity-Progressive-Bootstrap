package coordinator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/engineering"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/task"
)

var (
	// ErrForcePushForbidden indicates a force-push was attempted, which is strictly prohibited.
	ErrForcePushForbidden = errors.New("force push is strictly forbidden by single-writer publication policy (force=false required)")
	// ErrDirtyWorkingTree indicates uncommitted changes exist in the working directory before publication.
	ErrDirtyWorkingTree = errors.New("working tree has uncommitted modifications; cannot publish")
	// ErrBranchMismatch indicates an invalid branch was targeted for publication.
	ErrBranchMismatch = errors.New("publication target branch mismatch: only main is allowed")
	// ErrRemoteMismatch indicates an invalid remote was targeted for publication.
	ErrRemoteMismatch = errors.New("publication target remote mismatch: only origin is allowed")
)

// GitClient abstracts Git interactions for deterministic testing and validation.
type GitClient interface {
	Push(ctx context.Context, remote, branch string, force bool) error
	GetHeadSHA(ctx context.Context) (string, error)
	IsWorkingTreeClean(ctx context.Context) (bool, error)
}

// PublicationRequest encapsulates parameters for publishing changes to the primary repository.
type PublicationRequest struct {
	Role            engineering.Role `json:"role"`
	RemoteName      string           `json:"remoteName"`
	BranchName      string           `json:"branchName"`
	PlanDigest      string           `json:"planDigest"`
	ForcePush       bool             `json:"forcePush"`
	ExpectedHeadSHA string           `json:"expectedHeadSha"`
}

// Validate checks the structural validity of the publication request.
func (r PublicationRequest) Validate() error {
	if strings.TrimSpace(r.RemoteName) == "" {
		return fmt.Errorf("%w: remoteName is required", ErrInvalidCandidate)
	}
	if strings.TrimSpace(r.BranchName) == "" {
		return fmt.Errorf("%w: branchName is required", ErrInvalidCandidate)
	}
	if err := harnessmodel.ValidatePlanDigest(r.PlanDigest); err != nil {
		return fmt.Errorf("%w: invalid plan digest: %v", ErrInvalidCandidate, err)
	}
	return nil
}

// PublicationResult captures the verified publication outcome.
type PublicationResult struct {
	Success     bool      `json:"success"`
	RemoteName  string    `json:"remoteName"`
	BranchName  string    `json:"branchName"`
	HeadSHA     string    `json:"headSha"`
	PlanDigest  string    `json:"planDigest"`
	PublishedAt time.Time `json:"publishedAt"`
}

// Publisher enforces single-writer coordinator publication policy.
type Publisher struct {
	git GitClient
	now func() time.Time
}

// NewPublisher creates a new single-writer policy publisher.
func NewPublisher(git GitClient, now func() time.Time) *Publisher {
	if now == nil {
		now = time.Now
	}
	return &Publisher{
		git: git,
		now: now,
	}
}

// Publish executes publication strictly under coordinator authority, rejecting workers, force-pushes,
// living plan drift, and dirty working trees.
func (p *Publisher) Publish(ctx context.Context, req PublicationRequest, currentPlan []byte) (PublicationResult, error) {
	if err := req.Validate(); err != nil {
		return PublicationResult{}, err
	}

	// Invariant I-008, I-031, I-032: Enforce typed coordinator authority.
	if !req.Role.AllowsCoordinatorAuthority() {
		return PublicationResult{}, fmt.Errorf("%w: role=%s is not coordinator", ErrUnauthorizedRole, req.Role)
	}

	// Invariant I-008: Strict force=false requirement.
	if req.ForcePush {
		return PublicationResult{}, ErrForcePushForbidden
	}

	// Invariant I-008: Only origin/main is the publication target.
	if req.RemoteName != "origin" {
		return PublicationResult{}, fmt.Errorf("%w: got %q", ErrRemoteMismatch, req.RemoteName)
	}
	if req.BranchName != "main" {
		return PublicationResult{}, fmt.Errorf("%w: got %q", ErrBranchMismatch, req.BranchName)
	}

	// Invariant I-001 & I-030: Verify living plan consistency.
	env := harnessmodel.TaskEnvelope{PlanDigest: req.PlanDigest}
	if err := task.CheckPlanDrift(env, currentPlan); err != nil {
		return PublicationResult{}, fmt.Errorf("%w: %v", ErrPlanDrift, err)
	}

	// Verify working tree cleanliness
	clean, err := p.git.IsWorkingTreeClean(ctx)
	if err != nil {
		return PublicationResult{}, fmt.Errorf("check working tree cleanliness: %w", err)
	}
	if !clean {
		return PublicationResult{}, ErrDirtyWorkingTree
	}

	// Verify HEAD SHA
	headSHA, err := p.git.GetHeadSHA(ctx)
	if err != nil {
		return PublicationResult{}, fmt.Errorf("get head SHA: %w", err)
	}
	if req.ExpectedHeadSHA != "" && req.ExpectedHeadSHA != headSHA {
		return PublicationResult{}, fmt.Errorf("head SHA mismatch: expected %q, got %q", req.ExpectedHeadSHA, headSHA)
	}

	// Dispatch publication with force=false
	if err := p.git.Push(ctx, req.RemoteName, req.BranchName, false); err != nil {
		return PublicationResult{}, fmt.Errorf("git push: %w", err)
	}

	now := p.now().UTC()

	return PublicationResult{
		Success:     true,
		RemoteName:  req.RemoteName,
		BranchName:  req.BranchName,
		HeadSHA:     headSHA,
		PlanDigest:  req.PlanDigest,
		PublishedAt: now,
	}, nil
}
