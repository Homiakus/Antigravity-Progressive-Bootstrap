package model

import "time"

type TimerKind string

type TimerState string

const (
	TimerNodeWait       TimerKind = "NODE_WAIT"
	TimerSignalTimeout  TimerKind = "SIGNAL_TIMEOUT"
	TimerApprovalExpiry TimerKind = "APPROVAL_EXPIRY"

	TimerPending   TimerState = "PENDING"
	TimerFired     TimerState = "FIRED"
	TimerCancelled TimerState = "CANCELLED"
)

func (s TimerState) Valid() bool {
	switch s {
	case TimerPending, TimerFired, TimerCancelled:
		return true
	default:
		return false
	}
}

func (s TimerState) Terminal() bool { return s == TimerFired || s == TimerCancelled }

type Timer struct {
	ID            TimerID       `json:"id"`
	WorkflowRunID WorkflowRunID `json:"workflowRunId"`
	NodeRunID     NodeRunID     `json:"nodeRunId,omitempty"`
	Kind          TimerKind     `json:"kind"`
	Payload       []byte        `json:"payload,omitempty"`
	State         TimerState    `json:"state"`
	DueAt         time.Time     `json:"dueAt"`
	CreatedAt     time.Time     `json:"createdAt"`
	ResolvedAt    time.Time     `json:"resolvedAt,omitempty"`
}

type SignalState string

const (
	SignalPending  SignalState = "PENDING"
	SignalConsumed SignalState = "CONSUMED"
)

func (s SignalState) Valid() bool { return s == SignalPending || s == SignalConsumed }

type Signal struct {
	ID                  SignalID      `json:"id"`
	WorkflowRunID       WorkflowRunID `json:"workflowRunId"`
	Name                string        `json:"name"`
	MessageID           string        `json:"messageId"`
	Payload             []byte        `json:"payload,omitempty"`
	State               SignalState   `json:"state"`
	ReceivedAt          time.Time     `json:"receivedAt"`
	ConsumedByNodeRunID NodeRunID     `json:"consumedByNodeRunId,omitempty"`
	ConsumedAt          time.Time     `json:"consumedAt,omitempty"`
}

type SignalWaitState string

const (
	SignalWaitWaiting   SignalWaitState = "WAITING"
	SignalWaitDelivered SignalWaitState = "DELIVERED"
	SignalWaitCancelled SignalWaitState = "CANCELLED"
	SignalWaitTimedOut  SignalWaitState = "TIMED_OUT"
)

func (s SignalWaitState) Valid() bool {
	switch s {
	case SignalWaitWaiting, SignalWaitDelivered, SignalWaitCancelled, SignalWaitTimedOut:
		return true
	default:
		return false
	}
}

func (s SignalWaitState) Terminal() bool { return s != SignalWaitWaiting }

type SignalWait struct {
	NodeRunID        NodeRunID       `json:"nodeRunId"`
	WorkflowRunID    WorkflowRunID   `json:"workflowRunId"`
	SignalName       string          `json:"signalName"`
	State            SignalWaitState `json:"state"`
	CreatedAt        time.Time       `json:"createdAt"`
	DeliveredSignalID SignalID       `json:"deliveredSignalId,omitempty"`
	ResolvedAt       time.Time       `json:"resolvedAt,omitempty"`
}

type ApprovalState string

const (
	ApprovalPending   ApprovalState = "PENDING"
	ApprovalApproved  ApprovalState = "APPROVED"
	ApprovalRejected  ApprovalState = "REJECTED"
	ApprovalExpired   ApprovalState = "EXPIRED"
	ApprovalCancelled ApprovalState = "CANCELLED"
)

func (s ApprovalState) Valid() bool {
	switch s {
	case ApprovalPending, ApprovalApproved, ApprovalRejected, ApprovalExpired, ApprovalCancelled:
		return true
	default:
		return false
	}
}

func (s ApprovalState) Terminal() bool { return s != ApprovalPending }

type Approval struct {
	ID                  ApprovalID    `json:"id"`
	WorkflowRunID       WorkflowRunID `json:"workflowRunId"`
	NodeRunID           NodeRunID     `json:"nodeRunId"`
	RequestedCapability string        `json:"requestedCapability"`
	Risk                string        `json:"risk"`
	Reason              string        `json:"reason"`
	RequestedAt         time.Time     `json:"requestedAt"`
	ExpiresAt           time.Time     `json:"expiresAt,omitempty"`
	State               ApprovalState `json:"state"`
	Actor               string        `json:"actor,omitempty"`
	ResolvedAt          time.Time     `json:"resolvedAt,omitempty"`
}
