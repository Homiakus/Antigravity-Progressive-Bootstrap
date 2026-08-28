package antigravityide

const ProtocolVersion = 1

type Capabilities struct {
	ProtocolVersion         int    `json:"protocolVersion"`
	WorkspaceOpen           bool   `json:"workspaceOpen"`
	ConversationList        bool   `json:"conversationList"`
	ConversationCreate      bool   `json:"conversationCreate"`
	ConversationFocus       bool   `json:"conversationFocus"`
	ConversationSend        bool   `json:"conversationSend"`
	ConversationDirectSend  bool   `json:"conversationDirectSend"`
	MessageHistory          bool   `json:"messageHistory"`
	AgentEvents             bool   `json:"agentEvents"`
	Cancel                  bool   `json:"cancel"`
	ApprovalEvents          bool   `json:"approvalEvents"`
	ApprovalDecision        bool   `json:"approvalDecision"`
	NativeFork              bool   `json:"nativeFork"`
	ConversationCreateMode  string `json:"conversationCreateMode,omitempty"`
	ConversationDispatchMode string `json:"conversationDispatchMode,omitempty"`
}

type Context struct {
	InstanceID       string   `json:"instanceId"`
	BootNonce        string   `json:"bootNonce"`
	PID              int      `json:"pid"`
	WorkspaceFolders []string `json:"workspaceFolders"`
}

type Health struct {
	Status     string `json:"status"`
	InstanceID string `json:"instanceId"`
	BootNonce  string `json:"bootNonce"`
	PID        int    `json:"pid"`
}

type Conversation struct {
	ID             string `json:"id"`
	TrajectoryID   string `json:"trajectoryId,omitempty"`
	Title          string `json:"title,omitempty"`
	LastStepIndex  int    `json:"lastStepIndex,omitempty"`
	LastModifiedAt string `json:"lastModifiedAt,omitempty"`
}

type OpenWorkspaceResult struct {
	Scheduled bool `json:"scheduled"`
}
