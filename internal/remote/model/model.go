package model

import (
	"encoding/json"
	"time"
)

type SessionDesiredState string

const (
	SessionDesiredReady  SessionDesiredState = "READY"
	SessionDesiredPaused SessionDesiredState = "PAUSED"
	SessionDesiredClosed SessionDesiredState = "CLOSED"
)

func (s SessionDesiredState) Valid() bool {
	switch s {
	case SessionDesiredReady, SessionDesiredPaused, SessionDesiredClosed:
		return true
	default:
		return false
	}
}

type SessionObservedState string

const (
	SessionCreating              SessionObservedState = "CREATING"
	SessionResolvingInstance     SessionObservedState = "RESOLVING_INSTANCE"
	SessionWaitingInstance       SessionObservedState = "WAITING_INSTANCE"
	SessionWaitingBridge         SessionObservedState = "WAITING_BRIDGE"
	SessionOpeningWorkspace      SessionObservedState = "OPENING_WORKSPACE"
	SessionAttachingConversation SessionObservedState = "ATTACHING_CONVERSATION"
	SessionReady                 SessionObservedState = "READY"
	SessionRunning               SessionObservedState = "RUNNING"
	SessionWaitingApproval       SessionObservedState = "WAITING_APPROVAL"
	SessionPaused                SessionObservedState = "PAUSED"
	SessionDegraded              SessionObservedState = "DEGRADED"
	SessionNeedsAttention        SessionObservedState = "NEEDS_ATTENTION"
	SessionClosing               SessionObservedState = "CLOSING"
	SessionClosed                SessionObservedState = "CLOSED"
)

func (s SessionObservedState) Valid() bool {
	switch s {
	case SessionCreating, SessionResolvingInstance, SessionWaitingInstance, SessionWaitingBridge,
		SessionOpeningWorkspace, SessionAttachingConversation, SessionReady, SessionRunning,
		SessionWaitingApproval, SessionPaused, SessionDegraded, SessionNeedsAttention,
		SessionClosing, SessionClosed:
		return true
	default:
		return false
	}
}

type IsolationMode string

const (
	IsolationSharedRead      IsolationMode = "SHARED_READ"
	IsolationExclusiveWrite IsolationMode = "EXCLUSIVE_WRITE"
	IsolationWorktree        IsolationMode = "WORKTREE"
)

func (m IsolationMode) Valid() bool {
	switch m {
	case IsolationSharedRead, IsolationExclusiveWrite, IsolationWorktree:
		return true
	default:
		return false
	}
}

type MirrorMode string

const (
	MirrorFull       MirrorMode = "FULL"
	MirrorRemoteOnly MirrorMode = "REMOTE_ONLY"
	MirrorStatus     MirrorMode = "STATUS"
)

func (m MirrorMode) Valid() bool {
	switch m {
	case MirrorFull, MirrorRemoteOnly, MirrorStatus:
		return true
	default:
		return false
	}
}

type InstanceDesiredState string

const (
	InstanceDesiredStopped InstanceDesiredState = "STOPPED"
	InstanceDesiredRunning InstanceDesiredState = "RUNNING"
)

func (s InstanceDesiredState) Valid() bool {
	return s == InstanceDesiredStopped || s == InstanceDesiredRunning
}

type InstanceObservedState string

const (
	InstanceStopped        InstanceObservedState = "STOPPED"
	InstancePreparing      InstanceObservedState = "PREPARING"
	InstanceStarting       InstanceObservedState = "STARTING"
	InstanceProcessRunning InstanceObservedState = "PROCESS_RUNNING"
	InstanceWaitingBridge  InstanceObservedState = "WAITING_BRIDGE"
	InstanceReady          InstanceObservedState = "READY"
	InstanceStopping       InstanceObservedState = "STOPPING"
	InstanceDegraded       InstanceObservedState = "DEGRADED"
)

func (s InstanceObservedState) Valid() bool {
	switch s {
	case InstanceStopped, InstancePreparing, InstanceStarting, InstanceProcessRunning,
		InstanceWaitingBridge, InstanceReady, InstanceStopping, InstanceDegraded:
		return true
	default:
		return false
	}
}

type ConversationState string

const (
	ConversationActive    ConversationState = "ACTIVE"
	ConversationSuspended ConversationState = "SUSPENDED"
	ConversationClosed    ConversationState = "CLOSED"
)

func (s ConversationState) Valid() bool {
	return s == ConversationActive || s == ConversationSuspended || s == ConversationClosed
}

type CommandState string

const (
	CommandPending   CommandState = "PENDING"
	CommandRunning   CommandState = "RUNNING"
	CommandSucceeded CommandState = "SUCCEEDED"
	CommandFailed    CommandState = "FAILED"
	CommandCancelled CommandState = "CANCELLED"
)

func (s CommandState) Valid() bool {
	switch s {
	case CommandPending, CommandRunning, CommandSucceeded, CommandFailed, CommandCancelled:
		return true
	default:
		return false
	}
}

type Repository struct {
	ID            RepositoryID
	Name          string
	CanonicalPath string
	GitRoot       string
	GitRemote     string
	DefaultBranch string
	Enabled       bool
	CreatedAt     time.Time
	LastSeenAt    time.Time
}

type InstanceMirror struct {
	ID               InstanceID
	Name             string
	UserDataDir      string
	WorkingDir       string
	AccountID        string
	PID              int
	DesiredState     InstanceDesiredState
	ObservedState    InstanceObservedState
	BridgeID         string
	LastReconciledAt time.Time
	LastError        string
}

type Conversation struct {
	ID                     ConversationID
	ProviderConversationID string
	InstanceID             InstanceID
	WorkspaceID            WorkspaceID
	Title                  string
	State                  ConversationState
	MirrorMode             MirrorMode
	LastActivityAt         time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type RemoteSession struct {
	ID                RemoteSessionID
	HostID            HostID
	CockpitInstanceID InstanceID
	CockpitAccountID  string
	RepositoryID      RepositoryID
	WorkspaceID       WorkspaceID
	WorkspacePath     string
	ConversationID    ConversationID
	TelegramBindingID TelegramBindingID
	DesiredState      SessionDesiredState
	ObservedState     SessionObservedState
	IsolationMode     IsolationMode
	WorkflowRunID     string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type TelegramBinding struct {
	ID          TelegramBindingID
	SessionID   RemoteSessionID
	ChatID      int64
	ThreadID    int64
	OwnerUserID int64
	Enabled     bool
	CreatedAt   time.Time
}

type RemoteCommand struct {
	ID              RemoteCommandID
	Source          string
	SourceMessageID string
	SessionID       RemoteSessionID
	Kind            string
	Payload         json.RawMessage
	State           CommandState
	RequestedAt     time.Time
	StartedAt       *time.Time
	CompletedAt     *time.Time
	Error           string
}

type EventSource string

const (
	EventSourceIDE      EventSource = "IDE"
	EventSourceCockpit  EventSource = "COCKPIT"
	EventSourceHarness  EventSource = "HARNESS"
	EventSourceTelegram EventSource = "TELEGRAM"
	EventSourceRemote   EventSource = "REMOTE"
)

type EventType string

const (
	EventConversationStarted EventType = "conversation.started"
	EventUserMessage         EventType = "conversation.user_message"
	EventAgentDelta          EventType = "conversation.agent_delta"
	EventAgentMessage        EventType = "conversation.agent_message"
	EventAgentIdle           EventType = "conversation.agent_idle"
	EventToolStarted         EventType = "tool.started"
	EventToolFinished        EventType = "tool.finished"
	EventApprovalRequested   EventType = "approval.requested"
	EventApprovalResolved    EventType = "approval.resolved"
	EventWorkspaceChanged    EventType = "workspace.changed"
	EventBridgeConnected     EventType = "bridge.connected"
	EventBridgeDisconnected  EventType = "bridge.disconnected"
)

type RemoteEvent struct {
	ID            RemoteEventID
	SessionID     RemoteSessionID
	Seq           uint64
	Source        EventSource
	Type          EventType
	SourceEventID string
	Payload       json.RawMessage
	Timestamp     time.Time
}
