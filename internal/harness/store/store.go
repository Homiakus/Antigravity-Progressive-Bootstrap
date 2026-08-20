package store

import (
	"context"
	"errors"
	"time"

	"github.com/homiakus/agctl/internal/harness/events"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

var (
	ErrNotFound = errors.New("harness store: not found")
	ErrConflict = errors.New("harness store: concurrent state conflict")
)

type Store interface {
	View(context.Context, func(Reader) error) error
	Update(context.Context, func(Tx) error) error
	Close() error
}

type Reader interface {
	GetWorkflowDefinition(context.Context, harnessmodel.WorkflowDefinitionID, int) (harnessmodel.WorkflowDefinition, error)
	GetWorkflowRun(context.Context, harnessmodel.WorkflowRunID) (harnessmodel.WorkflowRun, error)
	GetWorkflowProgress(context.Context, harnessmodel.WorkflowRunID) (harnessmodel.WorkflowProgress, error)
	GetNodeRun(context.Context, harnessmodel.NodeRunID) (harnessmodel.NodeRun, error)
	GetAttempt(context.Context, harnessmodel.AttemptID) (harnessmodel.Attempt, error)
	ListDependentNodeRuns(context.Context, harnessmodel.WorkflowRunID, harnessmodel.NodeID) ([]harnessmodel.NodeRun, error)
	GetReadyNode(context.Context, harnessmodel.NodeRunID) (harnessmodel.ReadyNode, error)
	ListReadyWorkflowLanes(context.Context, time.Time, int) ([]harnessmodel.WorkflowScheduleState, error)
	ListReadyNodes(context.Context, harnessmodel.WorkflowRunID, time.Time, int) ([]harnessmodel.ReadyNode, error)
	ListEvents(context.Context, harnessmodel.WorkflowRunID, int64, int) ([]events.Event, error)
}

type Tx interface {
	Reader
	CreateWorkflowDefinition(context.Context, harnessmodel.WorkflowDefinition) error
	CreateWorkflowRun(context.Context, harnessmodel.WorkflowRun) error
	UpdateWorkflowRunState(context.Context, harnessmodel.WorkflowRunID, harnessmodel.WorkflowState, time.Time) error // compatibility helper
	CompareAndSwapWorkflowRun(context.Context, harnessmodel.WorkflowState, harnessmodel.WorkflowRun) error
	CreateGraphRevision(context.Context, harnessmodel.GraphRevision) error
	CreateWorkflowProgress(context.Context, harnessmodel.WorkflowProgress) error
	IncrementWorkflowProgress(context.Context, harnessmodel.WorkflowRunID, bool, time.Time) (harnessmodel.WorkflowProgress, error)
	CreateNodeRun(context.Context, harnessmodel.NodeRun) error
	CompareAndSwapNodeRun(context.Context, harnessmodel.NodeState, harnessmodel.NodeRun) error
	DecrementNodeRemainingDependencies(context.Context, harnessmodel.NodeRunID, time.Time) (int, error)
	CreateNextAttempt(context.Context, harnessmodel.Attempt) (harnessmodel.Attempt, error)
	CompareAndSwapAttempt(context.Context, harnessmodel.AttemptState, harnessmodel.Attempt) error
	CreateWorkflowScheduleState(context.Context, harnessmodel.WorkflowScheduleState) error
	SetWorkflowScheduleWeight(context.Context, harnessmodel.WorkflowRunID, int, time.Time) error
	EnqueueReadyNode(context.Context, harnessmodel.NodeRunID, time.Time, time.Time, string) error
	RemoveReadyNode(context.Context, harnessmodel.NodeRunID) error
	SetReadyWait(context.Context, harnessmodel.NodeRunID, harnessmodel.WaitReason, string, time.Time) error
	RecordWorkflowService(context.Context, harnessmodel.WorkflowRunID, time.Time) error
	AppendEvent(context.Context, events.Event, *events.OutboxMessage) (events.Event, error)
}
