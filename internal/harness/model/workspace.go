package model

import (
	"fmt"
	"strings"
	"time"
)

type WorkspaceID string

type WorkspaceKind string

const (
	WorkspacePersistent  WorkspaceKind = "PERSISTENT"
	WorkspaceSharedRead  WorkspaceKind = "SHARED_READ"
	WorkspaceExclusive   WorkspaceKind = "EXCLUSIVE"
	WorkspaceEphemeral   WorkspaceKind = "EPHEMERAL"
	WorkspaceGitWorktree WorkspaceKind = "GIT_WORKTREE"
)

func (k WorkspaceKind) Valid() bool {
	switch k {
	case WorkspacePersistent, WorkspaceSharedRead, WorkspaceExclusive, WorkspaceEphemeral, WorkspaceGitWorktree:
		return true
	default:
		return false
	}
}

type WorkspaceState string

const (
	WorkspaceAllocated WorkspaceState = "ALLOCATED"
	WorkspaceActive    WorkspaceState = "ACTIVE"
	WorkspaceReleased  WorkspaceState = "RELEASED"
	WorkspaceCorrupted WorkspaceState = "CORRUPTED"
)

func (s WorkspaceState) Valid() bool {
	switch s {
	case WorkspaceAllocated, WorkspaceActive, WorkspaceReleased, WorkspaceCorrupted:
		return true
	default:
		return false
	}
}

type WorkspaceRecord struct {
	ID                 WorkspaceID       `json:"id"`
	Kind               WorkspaceKind     `json:"kind"`
	State              WorkspaceState    `json:"state"`
	BasePath           string            `json:"basePath"`
	RepositoryID       string            `json:"repositoryId,omitempty"`
	Branch             string            `json:"branch,omitempty"`
	OwnerWorkflowRunID WorkflowRunID     `json:"ownerWorkflowRunId,omitempty"`
	OwnerNodeRunID     NodeRunID         `json:"ownerNodeRunId,omitempty"`
	OwnerAttemptID     AttemptID         `json:"ownerAttemptId,omitempty"`
	CreatedAt          time.Time         `json:"createdAt"`
	ExpiresAt          time.Time         `json:"expiresAt"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

func (w WorkspaceRecord) Validate() error {
	if strings.TrimSpace(string(w.ID)) == "" {
		return fmt.Errorf("workspace id is required")
	}
	if !w.Kind.Valid() {
		return fmt.Errorf("invalid workspace kind %q", w.Kind)
	}
	if !w.State.Valid() {
		return fmt.Errorf("invalid workspace state %q", w.State)
	}
	if strings.TrimSpace(w.BasePath) == "" {
		return fmt.Errorf("workspace base_path is required")
	}
	if w.CreatedAt.IsZero() {
		return fmt.Errorf("workspace created_at is required")
	}
	return nil
}
