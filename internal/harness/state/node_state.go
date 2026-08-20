package state

import harnessmodel "github.com/homiakus/agctl/internal/harness/model"

type NodeState = harnessmodel.NodeState

const (
	NodePendingDependencies = harnessmodel.NodePendingDependencies
	NodeReady               = harnessmodel.NodeReady
	NodeQueued              = harnessmodel.NodeQueued
	NodeRunning             = harnessmodel.NodeRunning
	NodeWaiting             = harnessmodel.NodeWaiting
	NodeRetryWait           = harnessmodel.NodeRetryWait
	NodeInDoubt             = harnessmodel.NodeInDoubt
	NodeSucceeded           = harnessmodel.NodeSucceeded
	NodeFailed              = harnessmodel.NodeFailed
	NodeTimedOut            = harnessmodel.NodeTimedOut
	NodeCancelled           = harnessmodel.NodeCancelled
	NodeSkipped             = harnessmodel.NodeSkipped
	NodeUnschedulable       = harnessmodel.NodeUnschedulable
)
