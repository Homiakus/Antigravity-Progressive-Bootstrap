package state

import (
	"math/rand"
	"reflect"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestTerminalAttemptCannotRestart(t *testing.T) {
	terminals := []AttemptState{AttemptSucceeded, AttemptFailed, AttemptTimedOut, AttemptCancelled, AttemptLost, AttemptInDoubt}
	for _, from := range terminals {
		if err := TransitionAttempt(from, AttemptRunning); err == nil {
			t.Errorf("terminal attempt %s restarted", from)
		}
	}
}

func TestTerminalWorkflowCannotReopen(t *testing.T) {
	terminals := []WorkflowState{WorkflowSucceeded, WorkflowFailed, WorkflowCancelled}
	for _, from := range terminals {
		for _, to := range []WorkflowState{WorkflowQueued, WorkflowRunning, WorkflowPaused} {
			if err := TransitionWorkflow(from, to); err == nil {
				t.Errorf("terminal workflow %s reopened to %s", from, to)
			}
		}
}

func TestRepresentativeAllowedTransitions(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{"workflow created-validating", func() error { return TransitionWorkflow(WorkflowCreated, WorkflowValidating) }},
		{"workflow running-pausing", func() error { return TransitionWorkflow(WorkflowRunning, WorkflowPausing) }},
		{"node pending-ready", func() error { return TransitionNode(NodePendingDependencies, NodeReady) }},
		{"node running-retry", func() error { return TransitionNode(NodeRunning, NodeRetryWait) }},
		{"attempt created-claimed", func() error { return TransitionAttempt(AttemptCreated, AttemptClaimed) }},
		{"attempt running-success", func() error { return TransitionAttempt(AttemptRunning, AttemptSucceeded) }},
	}
	for _, tt := range tests {
		if err := tt.fn(); err != nil {
			t.Errorf("%s: %v", tt.name, err)
		}
	}
}

func TestNewRetryAttemptDoesNotMutatePrevious(t *testing.T) {
	prev := harnessmodel.Attempt{ID: "att_0000000000000_00000000000000000000", NodeRunID: "nr_0000000000000_00000000000000000000", Number: 2, WorkerID: "wrk_0000000000000_00000000000000000000"}
	before := prev
	next, err := NewRetryAttempt(prev, AttemptFailed, "att_0000000000001_00000000000000000000", time.Unix(10, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(prev, before) {
		t.Fatalf("previous attempt mutated: before=%+v after=%+v", before, prev)
	}
	if next.ID == prev.ID || next.Number != prev.Number+1 || next.NodeRunID != prev.NodeRunID {
		t.Fatalf("invalid retry attempt: %+v", next)
	}
}

func TestRandomTransitionSequencesNeverEscapeTerminalAttempt(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	states := []AttemptState{AttemptCreated, AttemptClaimed, AttemptRunning, AttemptSucceeded, AttemptFailed, AttemptTimedOut, AttemptCancelled, AttemptLost, AttemptInDoubt}
	for run := 0; run < 2000; run++ {
		current := AttemptCreated
		terminalReached := false
		for step := 0; step < 32; step++ {
			target := states[rng.Intn(len(states))]
			err := TransitionAttempt(current, target)
			if terminalReached && err == nil {
				t.Fatalf("accepted transition out of terminal state %s -> %s", current, target)
			}
			if err == nil {
				current = target
				terminalReached = current.Terminal()
			}
		}
	}
}

func TestWorkflowTransitionTableExhaustive(t *testing.T) {
	states := []WorkflowState{WorkflowCreated, WorkflowValidating, WorkflowQueued, WorkflowRunning, WorkflowPausing, WorkflowPaused, WorkflowCancelling, WorkflowSucceeded, WorkflowFailed, WorkflowCancelled, WorkflowBlocked}
	for _, from := range states {
		for _, to := range states {
			_, allowed := workflowTransitions[from][to]
			err := TransitionWorkflow(from, to)
			if allowed && err != nil {
				t.Errorf("allowed workflow transition %s -> %s rejected: %v", from, to, err)
			}
			if !allowed && err == nil {
				t.Errorf("forbidden workflow transition %s -> %s accepted", from, to)
			}
		}
	}
}

func TestNodeTransitionTableExhaustive(t *testing.T) {
	states := []NodeState{NodePendingDependencies, NodeReady, NodeQueued, NodeRunning, NodeWaiting, NodeRetryWait, NodeInDoubt, NodeSucceeded, NodeFailed, NodeTimedOut, NodeCancelled, NodeSkipped, NodeUnschedulable}
	for _, from := range states {
		for _, to := range states {
			_, allowed := nodeTransitions[from][to]
			err := TransitionNode(from, to)
			if allowed && err != nil {
				t.Errorf("allowed node transition %s -> %s rejected: %v", from, to, err)
			}
			if !allowed && err == nil {
				t.Errorf("forbidden node transition %s -> %s accepted", from, to)
			}
		}
	}
}

func TestAttemptTransitionTableExhaustive(t *testing.T) {
	states := []AttemptState{AttemptCreated, AttemptClaimed, AttemptRunning, AttemptSucceeded, AttemptFailed, AttemptTimedOut, AttemptCancelled, AttemptLost, AttemptInDoubt}
	for _, from := range states {
		for _, to := range states {
			_, allowed := attemptTransitions[from][to]
			err := TransitionAttempt(from, to)
			if allowed && err != nil {
				t.Errorf("allowed attempt transition %s -> %s rejected: %v", from, to, err)
			}
			if !allowed && err == nil {
				t.Errorf("forbidden attempt transition %s -> %s accepted", from, to)
			}
		}
	}
}
