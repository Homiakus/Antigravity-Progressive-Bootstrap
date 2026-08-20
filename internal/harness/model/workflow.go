package model

import "time"

type WorkflowState string

const (
	WorkflowCreated    WorkflowState = "CREATED"
	WorkflowValidating WorkflowState = "VALIDATING"
	WorkflowQueued     WorkflowState = "QUEUED"
	WorkflowRunning    WorkflowState = "RUNNING"
	WorkflowPausing    WorkflowState = "PAUSING"
	WorkflowPaused     WorkflowState = "PAUSED"
	WorkflowCancelling WorkflowState = "CANCELLING"
	WorkflowSucceeded  WorkflowState = "SUCCEEDED"
	WorkflowFailed     WorkflowState = "FAILED"
	WorkflowCancelled  WorkflowState = "CANCELLED"
	WorkflowBlocked    WorkflowState = "BLOCKED"
)

func (s WorkflowState) Terminal() bool {
	switch s {
	case WorkflowSucceeded, WorkflowFailed, WorkflowCancelled:
		return true
	default:
		return false
	}
}

type WorkflowDefinition struct {
	ID              WorkflowDefinitionID `json:"id"`
	Version         int                  `json:"version"`
	Name            string               `json:"name"`
	CreatedAt       time.Time            `json:"createdAt"`
	CompilerVersion string               `json:"compilerVersion"`
	Nodes           []NodeSpec           `json:"nodes"`
	EntryNodes      []NodeID             `json:"entryNodes,omitempty"`
	Metadata        map[string]string    `json:"metadata,omitempty"`
}

type WorkflowRun struct {
	ID                   WorkflowRunID        `json:"id"`
	DefinitionID         WorkflowDefinitionID `json:"definitionId"`
	DefinitionVersion    int                  `json:"definitionVersion"`
	State                WorkflowState        `json:"state"`
	CurrentGraphRevision int                  `json:"currentGraphRevision"`
	CreatedAt            time.Time            `json:"createdAt"`
	UpdatedAt            time.Time            `json:"updatedAt"`
}

type GraphRevision struct {
	WorkflowRunID WorkflowRunID `json:"workflowRunId"`
	Number        int           `json:"number"`
	CreatedAt     time.Time     `json:"createdAt"`
	Reason        string        `json:"reason,omitempty"`
	ParentNumber  int           `json:"parentNumber,omitempty"`
}
