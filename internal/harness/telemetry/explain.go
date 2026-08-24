package telemetry

import (
	"context"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type NodeExplanation struct {
	NodeRunID             harnessmodel.NodeRunID `json:"nodeRunId"`
	NodeID                harnessmodel.NodeID    `json:"nodeId"`
	State                 harnessmodel.NodeState `json:"state"`
	GraphRevision         int                    `json:"graphRevision"`
	RemainingDependencies int                    `json:"remainingDependencies"`
	HumanReason           string                 `json:"humanReason"`
	WaitReason            string                 `json:"waitReason,omitempty"`
	LeaseOwner            string                 `json:"leaseOwner,omitempty"`
	LeaseExpiresAt        *time.Time             `json:"leaseExpiresAt,omitempty"`
	RetryDueAt            *time.Time             `json:"retryDueAt,omitempty"`
}

type WorkflowExplanation struct {
	WorkflowRunID harnessmodel.WorkflowRunID `json:"workflowRunId"`
	State         harnessmodel.WorkflowState `json:"state"`
	TotalNodes    int                        `json:"totalNodes"`
	TerminalNodes int                        `json:"terminalNodes"`
	FailedNodes   int                        `json:"failedNodes"`
	ActiveNodes   []NodeExplanation          `json:"activeNodes,omitempty"`
	Summary       string                     `json:"summary"`
}

type Explainer struct {
	store harnessstore.Store
	now   func() time.Time
}

func NewExplainer(store harnessstore.Store, now func() time.Time) *Explainer {
	if now == nil {
		now = time.Now
	}
	return &Explainer{store: store, now: now}
}

func (e *Explainer) ExplainNode(ctx context.Context, id harnessmodel.NodeRunID) (NodeExplanation, error) {
	var exp NodeExplanation
	err := e.store.View(ctx, func(r harnessstore.Reader) error {
		nr, err := r.GetNodeRun(ctx, id)
		if err != nil {
			return err
		}
		exp.NodeRunID = nr.ID
		exp.NodeID = nr.NodeID
		exp.State = nr.State
		exp.GraphRevision = nr.GraphRevision
		exp.RemainingDependencies = nr.RemainingDependencies

		switch nr.State {
		case harnessmodel.NodePendingDependencies:
			exp.HumanReason = fmt.Sprintf("Waiting for %d upstream dependencies to complete", nr.RemainingDependencies)
		case harnessmodel.NodeReady:
			exp.HumanReason = "Ready to be claimed by an active worker from the scheduling queue"
		case harnessmodel.NodeQueued:
			exp.HumanReason = "Queued for worker execution"
		case harnessmodel.NodeRunning:
			exp.HumanReason = "Currently being executed by worker under active lease"
		case harnessmodel.NodeWaiting:
			exp.HumanReason = "Paused waiting on external signal, timer, approval, or retry backoff"
		case harnessmodel.NodeSucceeded:
			exp.HumanReason = "Execution completed successfully"
		case harnessmodel.NodeFailed:
			exp.HumanReason = "Execution failed terminal error or exhausted retry budget"
		case harnessmodel.NodeCancelled:
			exp.HumanReason = "Cancelled by user, timeout, or superseded by graph revision"
		default:
			exp.HumanReason = fmt.Sprintf("Node is in state %s", nr.State)
		}
		return nil
	})
	if err != nil {
		return NodeExplanation{}, err
	}
	return exp, nil
}

func (e *Explainer) ExplainWorkflow(ctx context.Context, id harnessmodel.WorkflowRunID) (WorkflowExplanation, error) {
	var exp WorkflowExplanation
	err := e.store.View(ctx, func(r harnessstore.Reader) error {
		run, err := r.GetWorkflowRun(ctx, id)
		if err != nil {
			return err
		}
		exp.WorkflowRunID = run.ID
		exp.State = run.State

		progress, pErr := r.GetWorkflowProgress(ctx, id)
		if pErr == nil {
			exp.TotalNodes = progress.TotalNodes
			exp.TerminalNodes = progress.TerminalNodes
			exp.FailedNodes = progress.FailedNodes
		}

		exp.Summary = fmt.Sprintf("Workflow %s (revision %d) is in state %s with %d/%d terminal nodes (%d failed)",
			run.ID, run.CurrentGraphRevision, run.State, exp.TerminalNodes, exp.TotalNodes, exp.FailedNodes)
		return nil
	})
	if err != nil {
		return WorkflowExplanation{}, err
	}
	return exp, nil
}
