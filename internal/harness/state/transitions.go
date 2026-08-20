package state

import "fmt"

type TransitionError struct {
	Entity string
	From   string
	To     string
}

func (e TransitionError) Error() string {
	return fmt.Sprintf("invalid %s state transition %s -> %s", e.Entity, e.From, e.To)
}

var workflowTransitions = map[WorkflowState]map[WorkflowState]struct{}{
	WorkflowCreated:    set(WorkflowValidating, WorkflowCancelled),
	WorkflowValidating: set(WorkflowQueued, WorkflowFailed, WorkflowCancelled, WorkflowBlocked),
	WorkflowQueued:     set(WorkflowRunning, WorkflowCancelling, WorkflowCancelled, WorkflowBlocked, WorkflowFailed),
	WorkflowRunning:    set(WorkflowPausing, WorkflowCancelling, WorkflowSucceeded, WorkflowFailed, WorkflowBlocked),
	WorkflowPausing:    set(WorkflowPaused, WorkflowCancelling, WorkflowFailed, WorkflowBlocked),
	WorkflowPaused:     set(WorkflowRunning, WorkflowCancelling, WorkflowCancelled, WorkflowBlocked),
	WorkflowCancelling: set(WorkflowCancelled, WorkflowFailed),
	WorkflowBlocked:    set(WorkflowRunning, WorkflowCancelling, WorkflowCancelled, WorkflowFailed),
}

var nodeTransitions = map[NodeState]map[NodeState]struct{}{
	NodePendingDependencies: set(NodeReady, NodeSkipped, NodeCancelled, NodeUnschedulable),
	NodeReady:               set(NodeQueued, NodeSkipped, NodeCancelled, NodeUnschedulable),
	NodeQueued:              set(NodeRunning, NodeCancelled, NodeUnschedulable),
	NodeRunning:             set(NodeWaiting, NodeRetryWait, NodeInDoubt, NodeSucceeded, NodeFailed, NodeTimedOut, NodeCancelled),
	NodeWaiting:             set(NodeReady, NodeRunning, NodeCancelled, NodeFailed, NodeTimedOut),
	NodeRetryWait:           set(NodeReady, NodeCancelled, NodeFailed),
	NodeInDoubt:             set(NodeSucceeded, NodeFailed, NodeRetryWait, NodeCancelled),
	NodeUnschedulable:       set(NodeReady, NodeCancelled, NodeSkipped),
}

var attemptTransitions = map[AttemptState]map[AttemptState]struct{}{
	AttemptCreated: set(AttemptClaimed, AttemptCancelled),
	AttemptClaimed: set(AttemptRunning, AttemptCancelled, AttemptLost),
	AttemptRunning: set(AttemptSucceeded, AttemptFailed, AttemptTimedOut, AttemptCancelled, AttemptLost, AttemptInDoubt),
}

func set[T comparable](values ...T) map[T]struct{} {
	m := make(map[T]struct{}, len(values))
	for _, v := range values {
		m[v] = struct{}{}
	}
	return m
}

func TransitionWorkflow(from, to WorkflowState) error {
	if _, ok := workflowTransitions[from][to]; !ok {
		return TransitionError{Entity: "workflow", From: string(from), To: string(to)}
	}
	return nil
}

func TransitionNode(from, to NodeState) error {
	if _, ok := nodeTransitions[from][to]; !ok {
		return TransitionError{Entity: "node", From: string(from), To: string(to)}
	}
	return nil
}

func TransitionAttempt(from, to AttemptState) error {
	if _, ok := attemptTransitions[from][to]; !ok {
		return TransitionError{Entity: "attempt", From: string(from), To: string(to)}
	}
	return nil
}
