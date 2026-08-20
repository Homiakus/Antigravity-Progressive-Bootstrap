package model

import "time"

type NodeState string

const (
	NodePendingDependencies NodeState = "PENDING_DEPENDENCIES"
	NodeReady               NodeState = "READY"
	NodeQueued              NodeState = "QUEUED"
	NodeRunning             NodeState = "RUNNING"
	NodeWaiting             NodeState = "WAITING"
	NodeRetryWait           NodeState = "RETRY_WAIT"
	NodeInDoubt             NodeState = "IN_DOUBT"
	NodeSucceeded           NodeState = "SUCCEEDED"
	NodeFailed              NodeState = "FAILED"
	NodeTimedOut            NodeState = "TIMED_OUT"
	NodeCancelled           NodeState = "CANCELLED"
	NodeSkipped             NodeState = "SKIPPED"
	NodeUnschedulable       NodeState = "UNSCHEDULABLE"
)

func (s NodeState) Terminal() bool {
	switch s {
	case NodeSucceeded, NodeFailed, NodeTimedOut, NodeCancelled, NodeSkipped:
		return true
	default:
		return false
	}
}

type NodeKind string

const (
	NodeKindAction      NodeKind = "ACTION"
	NodeKindDecision    NodeKind = "DECISION"
	NodeKindWait        NodeKind = "WAIT"
	NodeKindSubworkflow NodeKind = "SUBWORKFLOW"
	NodeKindMap         NodeKind = "MAP"
)

func (k NodeKind) Valid() bool {
	switch k {
	case NodeKindAction, NodeKindDecision, NodeKindWait, NodeKindSubworkflow, NodeKindMap:
		return true
	default:
		return false
	}
}

type ExecutorKind string

const (
	ExecutorNone      ExecutorKind = ""
	ExecutorProcess   ExecutorKind = "PROCESS"
	ExecutorAgent     ExecutorKind = "AGENT"
	ExecutorMCPTool   ExecutorKind = "MCP_TOOL"
	ExecutorHTTP      ExecutorKind = "HTTP"
	ExecutorValidator ExecutorKind = "VALIDATOR"
	ExecutorContainer ExecutorKind = "CONTAINER"
	ExecutorPlugin    ExecutorKind = "PLUGIN"
)

func (k ExecutorKind) Valid() bool {
	switch k {
	case ExecutorProcess, ExecutorAgent, ExecutorMCPTool, ExecutorHTTP, ExecutorValidator, ExecutorContainer, ExecutorPlugin:
		return true
	default:
		return false
	}
}

type CachePolicy string

const (
	CacheUnspecified   CachePolicy = "UNSPECIFIED"
	CacheDisabled      CachePolicy = "DISABLED"
	CacheContent       CachePolicy = "CONTENT"
	CacheRunCheckpoint CachePolicy = "RUN_CHECKPOINT"
)

func (p CachePolicy) Valid() bool {
	switch p {
	case CacheUnspecified, CacheDisabled, CacheContent, CacheRunCheckpoint:
		return true
	default:
		return false
	}
}

type ResourceSpec struct {
	CPUWeight          int      `json:"cpuWeight,omitempty"`
	MemoryBytes        int64    `json:"memoryBytes,omitempty"`
	GPUCount           int      `json:"gpuCount,omitempty"`
	MinVRAMBytes       int64    `json:"minVramBytes,omitempty"`
	DiskBytes          int64    `json:"diskBytes,omitempty"`
	BuildSlots         int      `json:"buildSlots,omitempty"`
	BrowserSlots       int      `json:"browserSlots,omitempty"`
	ExclusiveWorkspace bool     `json:"exclusiveWorkspace,omitempty"`
	ReadOnly           bool     `json:"readOnly,omitempty"`
	Capabilities       []string `json:"capabilities,omitempty"`
}

type PolicySpec struct {
	Risk                 string   `json:"risk,omitempty"`
	RequiredCapabilities []string `json:"requiredCapabilities,omitempty"`
	ApprovalPolicyRef    string   `json:"approvalPolicyRef,omitempty"`
}

type InputRef struct {
	Name       string     `json:"name"`
	FromNodeID NodeID     `json:"fromNodeId,omitempty"`
	ArtifactID ArtifactID `json:"artifactId,omitempty"`
}

type OutputDeclaration struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type NodeSpec struct {
	ID                 NodeID              `json:"id"`
	Kind               NodeKind            `json:"kind"`
	ExecutorKind       ExecutorKind        `json:"executorKind,omitempty"`
	Dependencies       []NodeID            `json:"dependencies,omitempty"`
	RetryPolicyRef     string              `json:"retryPolicyRef,omitempty"`
	TimeoutPolicyRef   string              `json:"timeoutPolicyRef,omitempty"`
	Resources          ResourceSpec        `json:"resources,omitempty"`
	Policy             PolicySpec          `json:"policy,omitempty"`
	CachePolicy        CachePolicy         `json:"cachePolicy,omitempty"`
	InputRefs          []InputRef          `json:"inputRefs,omitempty"`
	OutputDeclarations []OutputDeclaration `json:"outputDeclarations,omitempty"`
	Metadata           map[string]string   `json:"metadata,omitempty"`
}

type NodeRun struct {
	ID                    NodeRunID     `json:"id"`
	WorkflowRunID         WorkflowRunID `json:"workflowRunId"`
	NodeID                NodeID        `json:"nodeId"`
	GraphRevision         int           `json:"graphRevision"`
	Generation            int           `json:"generation"`
	CreatedAt             time.Time     `json:"createdAt"`
	UpdatedAt             time.Time     `json:"updatedAt"`
	State                 NodeState     `json:"state"`
	RemainingDependencies int           `json:"remainingDependencies"`
}
