package model

import "encoding/json"

type MCPConfig struct {
	MCPServers map[string]MCPServer `json:"mcpServers"`
}

type OAuthClientCredentials struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type MCPServer struct {
	Command          string                  `json:"command,omitempty"`
	Args             []string                `json:"args,omitempty"`
	Env              map[string]string       `json:"env,omitempty"`
	CWD              string                  `json:"cwd,omitempty"`
	ServerURL        string                  `json:"serverUrl,omitempty"`
	Headers          map[string]string       `json:"headers,omitempty"`
	AuthProviderType string                  `json:"authProviderType,omitempty"`
	OAuth            *OAuthClientCredentials `json:"oauth,omitempty"`
	Disabled         bool                    `json:"disabled,omitempty"`
	DisabledTools    []string                `json:"disabledTools,omitempty"`
}

type HookHandler struct {
	Type    string `json:"type,omitempty"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

type MatchedHook struct {
	Matcher string        `json:"matcher"`
	Hooks   []HookHandler `json:"hooks"`
}

type HookDefinition struct {
	Enabled        *bool         `json:"enabled,omitempty"`
	PreToolUse     []MatchedHook `json:"PreToolUse,omitempty"`
	PostToolUse    []MatchedHook `json:"PostToolUse,omitempty"`
	PreInvocation  []HookHandler `json:"PreInvocation,omitempty"`
	PostInvocation []HookHandler `json:"PostInvocation,omitempty"`
	Stop           []HookHandler `json:"Stop,omitempty"`
}

type HooksConfig map[string]HookDefinition

type RouterConfig struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode"`
}

type LoopConfig struct {
	Enabled                   bool   `json:"enabled"`
	MaxExecutions             int    `json:"maxExecutions"`
	PermissionMode            string `json:"permissionMode"`
	RequireVerification       bool   `json:"requireVerification"`
	ContinueOnRecoverableStop bool   `json:"continueOnRecoverableStop"`
}

type TaskState struct {
	ConversationID  string   `json:"conversationId"`
	TaskID          string   `json:"taskId"`
	WorkspacePaths  []string `json:"workspacePaths,omitempty"`
	Complete        bool     `json:"complete"`
	Verified        bool     `json:"verified"`
	HardBlocker     bool     `json:"hardBlocker"`
	Summary         string   `json:"summary"`
	Verification    []string `json:"verification"`
	InitialNumSteps int      `json:"initialNumSteps"`
	StartedAt       string   `json:"startedAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

type CommonHookInput struct {
	ConversationID        string   `json:"conversationId"`
	WorkspacePaths        []string `json:"workspacePaths"`
	TranscriptPath        string   `json:"transcriptPath"`
	ArtifactDirectoryPath string   `json:"artifactDirectoryPath"`
	ModelName             string   `json:"modelName"`
}

type PreInvocationInput struct {
	CommonHookInput
	InvocationNum   int `json:"invocationNum"`
	InitialNumSteps int `json:"initialNumSteps"`
}

type InjectStep struct {
	EphemeralMessage string          `json:"ephemeralMessage,omitempty"`
	UserMessage      string          `json:"userMessage,omitempty"`
	ToolCall         json.RawMessage `json:"toolCall,omitempty"`
}

type PreInvocationOutput struct {
	InjectSteps []InjectStep `json:"injectSteps,omitempty"`
}

type ToolCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type PreToolUseInput struct {
	CommonHookInput
	ToolCall ToolCall `json:"toolCall"`
	StepIdx  int      `json:"stepIdx"`
}

type PreToolUseOutput struct {
	Decision            string   `json:"decision"`
	Reason              string   `json:"reason,omitempty"`
	PermissionOverrides []string `json:"permissionOverrides,omitempty"`
}

type StopInput struct {
	CommonHookInput
	ExecutionNum      int    `json:"executionNum"`
	TerminationReason string `json:"terminationReason"`
	Error             string `json:"error,omitempty"`
	FullyIdle         bool   `json:"fullyIdle"`
}

type StopOutput struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

// Capability is a normalized unit that the control-plane can route to.
type Capability struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"` // skill|mcp|agent|plugin|workflow|sidecar|native
	Source      string   `json:"source,omitempty"`
	Description string   `json:"description,omitempty"`
	Domains     []string `json:"domains,omitempty"`
	Operations  []string `json:"operations,omitempty"`
	Risk        string   `json:"risk,omitempty"`
	Auth        string   `json:"auth,omitempty"`
	Scope       string   `json:"scope,omitempty"`
	Path        string   `json:"path,omitempty"`
	Enabled     bool     `json:"enabled"`
}

type CapabilityRegistry struct {
	GeneratedAt  string       `json:"generatedAt"`
	Capabilities []Capability `json:"capabilities"`
}

type PluginManifest struct {
	Schema      string `json:"$schema,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type ProvenanceLock struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Source      string            `json:"source"`
	Ref         string            `json:"ref,omitempty"`
	Commit      string            `json:"commit,omitempty"`
	SHA256      string            `json:"sha256,omitempty"`
	InstalledAt string            `json:"installedAt"`
	Path        string            `json:"path,omitempty"`
	Files       map[string]string `json:"files,omitempty"`
}

type OrchestratorConfig struct {
	Enabled             bool     `json:"enabled"`
	Mode                string   `json:"mode"`
	MaxParallel         int      `json:"maxParallel"`
	DefaultWorkspace    string   `json:"defaultWorkspace,omitempty"`
	PreferWorktrees     bool     `json:"preferWorktrees"`
	AutoRoles           []string `json:"autoRoles,omitempty"`
	UseNativeGoal       bool     `json:"useNativeGoal"`
	VerificationAgent   bool     `json:"verificationAgent"`
	SecurityReviewAgent bool     `json:"securityReviewAgent"`
}

type RiskDecision struct {
	Decision string `json:"decision"`
	Risk     string `json:"risk"`
	Reason   string `json:"reason"`
}

// ResourceRequest describes scheduler resources consumed by a headless task.
// Zero values are interpreted conservatively by the scheduler.
type ResourceRequest struct {
	CPUWeight          int  `json:"cpuWeight,omitempty"`
	BuildSlots         int  `json:"buildSlots,omitempty"`
	BrowserSlots       int  `json:"browserSlots,omitempty"`
	ExclusiveWorkspace bool `json:"exclusiveWorkspace,omitempty"`
	ReadOnly           bool `json:"readOnly,omitempty"`
}

// PlanNode is a node in an autonomous multi-agent execution DAG.
type PlanNode struct {
	ID             string          `json:"id"`
	Workspace      string          `json:"workspace,omitempty"`
	WorktreeBranch string          `json:"worktreeBranch,omitempty"`
	Dynamic        bool            `json:"dynamic,omitempty"`
	ParentNodeID   string          `json:"parentNodeId,omitempty"`
	Depth          int             `json:"depth,omitempty"`
	Title          string          `json:"title"`
	Objective      string          `json:"objective"`
	Agent          string          `json:"agent,omitempty"`
	DependsOn      []string        `json:"dependsOn,omitempty"`
	Verification   []string        `json:"verification,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
	Resources      ResourceRequest `json:"resources,omitempty"`
	Risk           string          `json:"risk,omitempty"`
}

// ExecutionPlan is a persisted, deterministic orchestration plan.
type ExecutionPlan struct {
	ID               string         `json:"id"`
	Revision         int            `json:"revision,omitempty"`
	Status           string         `json:"status,omitempty"`
	UpdatedAt        string         `json:"updatedAt,omitempty"`
	DynamicNodeCount int            `json:"dynamicNodeCount,omitempty"`
	BlockReason      string         `json:"blockReason,omitempty"`
	RevisionHistory  []PlanRevision `json:"revisionHistory,omitempty"`
	Prompt           string         `json:"prompt"`
	Workspace        string         `json:"workspace"`
	Profiles         []string       `json:"profiles,omitempty"`
	CreatedAt        string         `json:"createdAt"`
	GeneratedBy      string         `json:"generatedBy"`
	CapabilityHints  []string       `json:"capabilityHints,omitempty"`
	Nodes            []PlanNode     `json:"nodes"`
}

type TaskRecord struct {
	ID                 string          `json:"id"`
	Revision           int             `json:"revision,omitempty"`
	DynamicDepth       int             `json:"dynamicDepth,omitempty"`
	ParentTaskID       string          `json:"parentTaskId,omitempty"`
	ReplanProposalPath string          `json:"replanProposalPath,omitempty"`
	FailureSignature   string          `json:"failureSignature,omitempty"`
	BaseWorkspace      string          `json:"baseWorkspace,omitempty"`
	WorktreeBranch     string          `json:"worktreeBranch,omitempty"`
	Prompt             string          `json:"prompt"`
	Workspace          string          `json:"workspace"`
	PlanID             string          `json:"planId,omitempty"`
	NodeID             string          `json:"nodeId,omitempty"`
	Dependencies       []string        `json:"dependencies,omitempty"`
	Resources          ResourceRequest `json:"resources,omitempty"`
	Status             string          `json:"status"`
	Priority           int             `json:"priority"`
	CreatedAt          string          `json:"createdAt"`
	StartedAt          string          `json:"startedAt,omitempty"`
	FinishedAt         string          `json:"finishedAt,omitempty"`
	UpdatedAt          string          `json:"updatedAt,omitempty"`
	ExitCode           int             `json:"exitCode,omitempty"`
	ProcessID          int             `json:"processId,omitempty"`
	Attempts           int             `json:"attempts,omitempty"`
	Error              string          `json:"error,omitempty"`
	OutputLog          string          `json:"outputLog,omitempty"`
	UseNativeGoal      bool            `json:"useNativeGoal"`
	Agent              string          `json:"agent,omitempty"`
	Tags               []string        `json:"tags,omitempty"`
}

type TaskSupervisorConfig struct {
	MaxParallel    int `json:"maxParallel"`
	CPUWeight      int `json:"cpuWeight"`
	BuildSlots     int `json:"buildSlots"`
	BrowserSlots   int `json:"browserSlots"`
	MaxTaskMinutes int `json:"maxTaskMinutes"`
	MaxRetries     int `json:"maxRetries"`
}

// SecurityFinding is a normalized supply-chain/runtime security observation.
type SecurityFinding struct {
	Severity string `json:"severity"`
	Area     string `json:"area"`
	ID       string `json:"id"`
	Message  string `json:"message"`
	Penalty  int    `json:"penalty,omitempty"`
}

type SecurityReport struct {
	GeneratedAt string            `json:"generatedAt"`
	Workspace   string            `json:"workspace,omitempty"`
	Score       int               `json:"score"`
	Grade       string            `json:"grade"`
	Findings    []SecurityFinding `json:"findings,omitempty"`
}

// PlanRevision records an adaptive mutation of an execution DAG.
type PlanRevision struct {
	Number           int      `json:"number"`
	CreatedAt        string   `json:"createdAt"`
	TriggerTaskID    string   `json:"triggerTaskId,omitempty"`
	TriggerNodeID    string   `json:"triggerNodeId,omitempty"`
	Reason           string   `json:"reason"`
	Kind             string   `json:"kind"` // proposal|failure-recovery|manual
	AddedNodes       []string `json:"addedNodes,omitempty"`
	RewiredNodes     []string `json:"rewiredNodes,omitempty"`
	FailureSignature string   `json:"failureSignature,omitempty"`
	Fingerprints     []string `json:"fingerprints,omitempty"`
}

// ReplanConfig bounds autonomous graph growth and no-progress behavior.
type ReplanConfig struct {
	Enabled          bool    `json:"enabled"`
	MaxRevisions     int     `json:"maxRevisions"`
	MaxDynamicNodes  int     `json:"maxDynamicNodes"`
	MaxRepairDepth   int     `json:"maxRepairDepth"`
	MaxSameFailure   int     `json:"maxSameFailure"`
	MinConfidence    float64 `json:"minConfidence"`
	AutoApplyRiskMax string  `json:"autoApplyRiskMax"`
	PreferWorktrees  bool    `json:"preferWorktrees"`
	RequireEvidence  bool    `json:"requireEvidence"`
}

// ReplanProposal is the machine-readable contract an executing agent can emit
// when it discovers material work not represented in the current DAG.
type ReplanProposal struct {
	Version      int            `json:"version"`
	PlanID       string         `json:"planId"`
	ParentNodeID string         `json:"parentNodeId"`
	ParentTaskID string         `json:"parentTaskId"`
	Reason       string         `json:"reason"`
	Evidence     []string       `json:"evidence,omitempty"`
	Confidence   float64        `json:"confidence"`
	Actions      []ReplanAction `json:"actions"`
	CreatedAt    string         `json:"createdAt,omitempty"`
}

type ReplanAction struct {
	ID             string          `json:"id"`
	Title          string          `json:"title"`
	Objective      string          `json:"objective"`
	Agent          string          `json:"agent,omitempty"`
	DependsOn      []string        `json:"dependsOn,omitempty"`
	Verification   []string        `json:"verification,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
	Resources      ResourceRequest `json:"resources,omitempty"`
	Risk           string          `json:"risk,omitempty"`
	Parallelizable bool            `json:"parallelizable,omitempty"`
}

type ReplanResult struct {
	Applied      bool     `json:"applied"`
	PlanID       string   `json:"planId,omitempty"`
	Revision     int      `json:"revision,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	AddedNodes   []string `json:"addedNodes,omitempty"`
	CreatedTasks []string `json:"createdTasks,omitempty"`
	RewiredTasks []string `json:"rewiredTasks,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
	NoProgress   bool     `json:"noProgress,omitempty"`
}
