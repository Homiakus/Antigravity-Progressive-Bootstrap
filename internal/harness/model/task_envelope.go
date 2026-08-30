package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type TaskClass string

const (
	TaskClassCodegen TaskClass = "codegen"
	TaskClassReview  TaskClass = "review"
	TaskClassTest    TaskClass = "test"
	TaskClassAudit   TaskClass = "audit"
	TaskClassDocs    TaskClass = "docs"
)

func (c TaskClass) Valid() bool {
	switch c {
	case TaskClassCodegen, TaskClassReview, TaskClassTest, TaskClassAudit, TaskClassDocs:
		return true
	default:
		return len(c) > 0 && len(c) <= 64
	}
}

type ContextRefRole string

const (
	ContextRoleInputCode     ContextRefRole = "input_code"
	ContextRolePreflight     ContextRefRole = "preflight"
	ContextRoleSpecification ContextRefRole = "specification"
	ContextRoleDiff          ContextRefRole = "diff"
	ContextRoleEvidence      ContextRefRole = "evidence"
)

type ContextRef struct {
	ID          string         `json:"id"`
	URI         string         `json:"uri"`
	Digest      string         `json:"digest,omitempty"`
	Role        ContextRefRole `json:"role,omitempty"`
	Description string         `json:"description,omitempty"`
}

func (r ContextRef) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("context ref id is required")
	}
	if strings.TrimSpace(r.URI) == "" {
		return fmt.Errorf("context ref uri is required")
	}
	if r.Digest != "" {
		if err := ValidatePlanDigest(r.Digest); err != nil {
			return fmt.Errorf("context ref digest invalid: %w", err)
		}
	}
	return nil
}

type WorkspaceSpec struct {
	RootPath   string `json:"rootPath"`
	RepoID     string `json:"repoId,omitempty"`
	BaseCommit string `json:"baseCommit,omitempty"`
	WorktreeID string `json:"worktreeId,omitempty"`
	ReadOnly   bool   `json:"readOnly,omitempty"`
}

func (w WorkspaceSpec) Validate() error {
	if strings.TrimSpace(w.RootPath) == "" {
		return fmt.Errorf("workspace root path is required")
	}
	return nil
}

// Fingerprint returns a stable workspace affinity fingerprint for provider session reuse.
func (w WorkspaceSpec) Fingerprint() string {
	parts := []string{w.RootPath}
	if w.RepoID != "" {
		parts = append(parts, w.RepoID)
	}
	if w.WorktreeID != "" {
		parts = append(parts, w.WorktreeID)
	}
	return strings.Join(parts, ":")
}

// TaskEnvelope is a self-contained, conversation-independent task execution contract.
// It completely specifies the objective, instructions, input context, workspace, role,
// capabilities, and governance plan digest for execution across any provider.
type TaskEnvelope struct {
	ID                    TaskEnvelopeID    `json:"id"`
	TaskID                string            `json:"taskId"`
	WorkflowRunID         WorkflowRunID     `json:"workflowRunId,omitempty"`
	NodeID                NodeID            `json:"nodeId,omitempty"`
	NodeRunID             NodeRunID         `json:"nodeRunId,omitempty"`
	AttemptID             AttemptID         `json:"attemptId,omitempty"`
	AttemptNumber         int               `json:"attemptNumber,omitempty"`
	PlanDigest            string            `json:"planDigest"`
	TaskClass             TaskClass         `json:"taskClass"`
	Title                 string            `json:"title"`
	Objective             string            `json:"objective"`
	Instructions          string            `json:"instructions"`
	ContextRefs           []ContextRef      `json:"contextRefs,omitempty"`
	Workspace             WorkspaceSpec     `json:"workspace"`
	Role                  string            `json:"role,omitempty"`
	RequiredCapabilities  []string          `json:"requiredCapabilities,omitempty"`
	ForbiddenCapabilities []string          `json:"forbiddenCapabilities,omitempty"`
	Determinism           DeterminismClass  `json:"determinism,omitempty"`
	PreferredProvider     ProviderKind      `json:"preferredProvider,omitempty"`
	PreferredModel        ProviderModelID   `json:"preferredModel,omitempty"`
	MaxTokens             int               `json:"maxTokens,omitempty"`
	Timeout               time.Duration     `json:"timeout,omitempty"`
	CreatedAt             time.Time         `json:"createdAt"`
	Metadata              map[string]string `json:"metadata,omitempty"`
}

func (e TaskEnvelope) Validate() error {
	if strings.TrimSpace(string(e.ID)) == "" {
		return fmt.Errorf("task envelope id is required")
	}
	if strings.TrimSpace(e.TaskID) == "" {
		return fmt.Errorf("task envelope task id is required")
	}
	if err := ValidatePlanDigest(e.PlanDigest); err != nil {
		return fmt.Errorf("task envelope plan digest invalid: %w", err)
	}
	if !e.TaskClass.Valid() {
		return fmt.Errorf("task envelope task class invalid: %q", e.TaskClass)
	}
	if strings.TrimSpace(e.Title) == "" {
		return fmt.Errorf("task envelope title is required")
	}
	if strings.TrimSpace(e.Objective) == "" {
		return fmt.Errorf("task envelope objective is required")
	}
	if strings.TrimSpace(e.Instructions) == "" {
		return fmt.Errorf("task envelope instructions are required")
	}
	if err := e.Workspace.Validate(); err != nil {
		return fmt.Errorf("task envelope workspace invalid: %w", err)
	}
	if e.AttemptNumber < 0 {
		return fmt.Errorf("task envelope attempt number must be non-negative")
	}
	if e.Timeout < 0 {
		return fmt.Errorf("task envelope timeout must be non-negative")
	}
	if e.CreatedAt.IsZero() {
		return fmt.Errorf("task envelope created at is required")
	}
	for i, ref := range e.ContextRefs {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("context ref [%d] invalid: %w", i, err)
		}
	}
	return nil
}

// MatchesPlan returns true if the envelope was constructed under the specified plan digest.
func (e TaskEnvelope) MatchesPlan(planDigest string) bool {
	return strings.TrimSpace(e.PlanDigest) != "" && e.PlanDigest == strings.TrimSpace(planDigest)
}

// canonicalEnvelopePayload represents the deterministic subset of fields for hashing.
type canonicalEnvelopePayload struct {
	TaskID                string            `json:"taskId"`
	PlanDigest            string            `json:"planDigest"`
	TaskClass             string            `json:"taskClass"`
	Title                 string            `json:"title"`
	Objective             string            `json:"objective"`
	Instructions          string            `json:"instructions"`
	ContextRefs           []ContextRef      `json:"contextRefs"`
	Workspace             WorkspaceSpec     `json:"workspace"`
	Role                  string            `json:"role"`
	RequiredCapabilities  []string          `json:"requiredCapabilities"`
	ForbiddenCapabilities []string          `json:"forbiddenCapabilities"`
	Determinism           string            `json:"determinism"`
	PreferredProvider     string            `json:"preferredProvider"`
	PreferredModel        string            `json:"preferredModel"`
	MaxTokens             int               `json:"maxTokens"`
	TimeoutSeconds        int64             `json:"timeoutSeconds"`
	Metadata              map[string]string `json:"metadata"`
}

// Digest computes a canonical, deterministic SHA-256 hexadecimal digest of the envelope.
// The digest captures all semantic instructions, inputs, constraints, and plan bindings.
func (e TaskEnvelope) Digest() (string, error) {
	if err := e.Validate(); err != nil {
		return "", err
	}

	reqCaps := append([]string(nil), e.RequiredCapabilities...)
	sort.Strings(reqCaps)

	forbCaps := append([]string(nil), e.ForbiddenCapabilities...)
	sort.Strings(forbCaps)

	refs := append([]ContextRef(nil), e.ContextRefs...)
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].ID != refs[j].ID {
			return refs[i].ID < refs[j].ID
		}
		return refs[i].URI < refs[j].URI
	})

	meta := make(map[string]string, len(e.Metadata))
	for k, v := range e.Metadata {
		meta[k] = v
	}

	payload := canonicalEnvelopePayload{
		TaskID:                e.TaskID,
		PlanDigest:            e.PlanDigest,
		TaskClass:             string(e.TaskClass),
		Title:                 e.Title,
		Objective:             e.Objective,
		Instructions:          e.Instructions,
		ContextRefs:           refs,
		Workspace:             e.Workspace,
		Role:                  e.Role,
		RequiredCapabilities:  reqCaps,
		ForbiddenCapabilities: forbCaps,
		Determinism:           string(e.Determinism),
		PreferredProvider:     string(e.PreferredProvider),
		PreferredModel:        string(e.PreferredModel),
		MaxTokens:             e.MaxTokens,
		TimeoutSeconds:        int64(e.Timeout.Seconds()),
		Metadata:              meta,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal canonical envelope: %w", err)
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
