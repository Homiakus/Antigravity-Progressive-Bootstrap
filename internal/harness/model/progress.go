package model

import "time"

// WorkflowProgress is durable materialized execution progress. It is kept
// separate from WorkflowRun so scheduler/progress counters can evolve without
// changing the immutable workflow identity/state record.
type WorkflowProgress struct {
	WorkflowRunID WorkflowRunID `json:"workflowRunId"`
	TotalNodes    int           `json:"totalNodes"`
	TerminalNodes int           `json:"terminalNodes"`
	FailedNodes   int           `json:"failedNodes"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}
