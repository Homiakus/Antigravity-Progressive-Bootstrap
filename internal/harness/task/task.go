package task

import (
	"fmt"
	"strings"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/provider/selector"
)

// Builder facilitates fluent construction of a verified TaskEnvelope.
type Builder struct {
	env harnessmodel.TaskEnvelope
	err error
}

// NewBuilder initializes a new TaskEnvelope builder.
func NewBuilder(id harnessmodel.TaskEnvelopeID, taskID string, planDigest string) *Builder {
	b := &Builder{
		env: harnessmodel.TaskEnvelope{
			ID:         id,
			TaskID:     taskID,
			PlanDigest: planDigest,
			CreatedAt:  time.Now().UTC(),
			Metadata:   make(map[string]string),
		},
	}
	if id == "" {
		b.err = fmt.Errorf("task envelope id is required")
	}
	if taskID == "" {
		b.err = fmt.Errorf("task id is required")
	}
	if err := harnessmodel.ValidatePlanDigest(planDigest); err != nil {
		b.err = err
	}
	return b
}

func (b *Builder) WithLineage(wfrID harnessmodel.WorkflowRunID, nodeID harnessmodel.NodeID, nrID harnessmodel.NodeRunID, attemptID harnessmodel.AttemptID, attemptNum int) *Builder {
	if b.err != nil {
		return b
	}
	b.env.WorkflowRunID = wfrID
	b.env.NodeID = nodeID
	b.env.NodeRunID = nrID
	b.env.AttemptID = attemptID
	b.env.AttemptNumber = attemptNum
	return b
}

func (b *Builder) WithSpec(taskClass harnessmodel.TaskClass, title, objective, instructions string) *Builder {
	if b.err != nil {
		return b
	}
	b.env.TaskClass = taskClass
	b.env.Title = title
	b.env.Objective = objective
	b.env.Instructions = instructions
	return b
}

func (b *Builder) WithWorkspace(ws harnessmodel.WorkspaceSpec) *Builder {
	if b.err != nil {
		return b
	}
	b.env.Workspace = ws
	return b
}

func (b *Builder) WithRole(role string) *Builder {
	if b.err != nil {
		return b
	}
	b.env.Role = role
	return b
}

func (b *Builder) WithCapabilities(required, forbidden []string) *Builder {
	if b.err != nil {
		return b
	}
	b.env.RequiredCapabilities = append([]string(nil), required...)
	b.env.ForbiddenCapabilities = append([]string(nil), forbidden...)
	return b
}

func (b *Builder) WithDeterminism(det harnessmodel.DeterminismClass) *Builder {
	if b.err != nil {
		return b
	}
	b.env.Determinism = det
	return b
}

func (b *Builder) WithGuidance(provider harnessmodel.ProviderKind, model harnessmodel.ProviderModelID, maxTokens int, timeout time.Duration) *Builder {
	if b.err != nil {
		return b
	}
	b.env.PreferredProvider = provider
	b.env.PreferredModel = model
	b.env.MaxTokens = maxTokens
	b.env.Timeout = timeout
	return b
}

func (b *Builder) WithContextRefs(refs ...harnessmodel.ContextRef) *Builder {
	if b.err != nil {
		return b
	}
	b.env.ContextRefs = append(b.env.ContextRefs, refs...)
	return b
}

func (b *Builder) WithMetadata(key, value string) *Builder {
	if b.err != nil {
		return b
	}
	if b.env.Metadata == nil {
		b.env.Metadata = make(map[string]string)
	}
	b.env.Metadata[key] = value
	return b
}

// Build validates and returns the constructed TaskEnvelope.
func (b *Builder) Build() (harnessmodel.TaskEnvelope, error) {
	if b.err != nil {
		return harnessmodel.TaskEnvelope{}, b.err
	}
	if err := b.env.Validate(); err != nil {
		return harnessmodel.TaskEnvelope{}, err
	}
	return b.env, nil
}

// FromNodeRun constructs a TaskEnvelope from DAG execution state and governing plan digest.
func FromNodeRun(
	id harnessmodel.TaskEnvelopeID,
	spec harnessmodel.NodeSpec,
	run harnessmodel.NodeRun,
	attempt harnessmodel.Attempt,
	planDigest string,
	workspace harnessmodel.WorkspaceSpec,
	instructions string,
) (harnessmodel.TaskEnvelope, error) {
	taskClass := harnessmodel.TaskClass(spec.Metadata["taskClass"])
	if taskClass == "" {
		taskClass = harnessmodel.TaskClassCodegen
	}

	title := spec.Metadata["title"]
	if title == "" {
		title = fmt.Sprintf("Node %s execution", spec.ID)
	}

	objective := spec.Metadata["objective"]
	if objective == "" {
		objective = title
	}

	if strings.TrimSpace(instructions) == "" {
		instructions = fmt.Sprintf("Execute node %s with executor %s", spec.ID, spec.ExecutorKind)
	}

	role := spec.Metadata["role"]
	if role == "" {
		role = "worker"
	}

	refs := make([]harnessmodel.ContextRef, 0, len(spec.InputRefs))
	for _, ref := range spec.InputRefs {
		refs = append(refs, harnessmodel.ContextRef{
			ID:          ref.Name,
			URI:         fmt.Sprintf("node://%s/%s", ref.FromNodeID, ref.ArtifactID),
			Role:        harnessmodel.ContextRoleSpecification,
			Description: fmt.Sprintf("Input from node %s", ref.FromNodeID),
		})
	}

	builder := NewBuilder(id, string(spec.ID), planDigest).
		WithLineage(run.WorkflowRunID, spec.ID, run.ID, attempt.ID, attempt.Number).
		WithSpec(taskClass, title, objective, instructions).
		WithWorkspace(workspace).
		WithRole(role).
		WithCapabilities(spec.Policy.RequiredCapabilities, nil).
		WithDeterminism(spec.Determinism).
		WithContextRefs(refs...)

	for k, v := range spec.Metadata {
		builder.WithMetadata(k, v)
	}

	return builder.Build()
}

// ToSelectorRequest converts a TaskEnvelope into a selector.Request for routing evaluation.
func ToSelectorRequest(env harnessmodel.TaskEnvelope) selector.Request {
	return selector.Request{
		TaskClass:            string(env.TaskClass),
		RepositoryID:         env.Workspace.RepoID,
		RequiredContext:      int64(env.MaxTokens),
		RequiredCapabilities: env.RequiredCapabilities,
		WorkspaceFingerprint: env.Workspace.Fingerprint(),
		PreferredProvider:    env.PreferredProvider,
		PreferredModelID:     env.PreferredModel,
	}
}

// CheckPlanDrift verifies that the envelope was issued under the same plan digest as
// the active plan content. If content has changed, ErrStalePlan is returned.
func CheckPlanDrift(env harnessmodel.TaskEnvelope, activePlanContent []byte) error {
	return harnessmodel.VerifyPlanConsistency(env.PlanDigest, activePlanContent)
}
