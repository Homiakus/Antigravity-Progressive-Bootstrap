package state

import harnessmodel "github.com/homiakus/agctl/internal/harness/model"

type WorkflowState = harnessmodel.WorkflowState

const (
	WorkflowCreated    = harnessmodel.WorkflowCreated
	WorkflowValidating = harnessmodel.WorkflowValidating
	WorkflowQueued     = harnessmodel.WorkflowQueued
	WorkflowRunning    = harnessmodel.WorkflowRunning
	WorkflowPausing    = harnessmodel.WorkflowPausing
	WorkflowPaused     = harnessmodel.WorkflowPaused
	WorkflowCancelling = harnessmodel.WorkflowCancelling
	WorkflowSucceeded  = harnessmodel.WorkflowSucceeded
	WorkflowFailed     = harnessmodel.WorkflowFailed
	WorkflowCancelled  = harnessmodel.WorkflowCancelled
	WorkflowBlocked    = harnessmodel.WorkflowBlocked
)
