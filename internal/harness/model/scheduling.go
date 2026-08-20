package model

import "time"

type WaitReason string

const (
	WaitNone             WaitReason = ""
	WaitDependency       WaitReason = "DEPENDENCY"
	WaitNotBefore        WaitReason = "NOT_BEFORE"
	WaitResource         WaitReason = "RESOURCE"
	WaitCapability       WaitReason = "CAPABILITY"
	WaitWorkspaceLock    WaitReason = "WORKSPACE_LOCK"
	WaitConcurrencyLimit WaitReason = "CONCURRENCY_LIMIT"
	WaitRateLimit        WaitReason = "RATE_LIMIT"
	WaitPolicyApproval   WaitReason = "POLICY_APPROVAL"
	WaitTenantLimit      WaitReason = "TENANT_LIMIT"
	WaitNoEligibleWorker WaitReason = "NO_ELIGIBLE_WORKER"
	WaitFairness         WaitReason = "FAIRNESS"
)

type ReadyNode struct {
	NodeRunID         NodeRunID     `json:"nodeRunId"`
	WorkflowRunID     WorkflowRunID `json:"workflowRunId"`
	NodeID            NodeID        `json:"nodeId"`
	Priority          int           `json:"priority"`
	EffectivePriority int           `json:"effectivePriority"`
	ReadyAt           time.Time     `json:"readyAt"`
	NotBefore         time.Time     `json:"notBefore,omitempty"`
	ResourceClass     string        `json:"resourceClass,omitempty"`
	WaitReason        WaitReason    `json:"waitReason,omitempty"`
	WaitDetail        string        `json:"waitDetail,omitempty"`
	Resources         ResourceSpec  `json:"resources"`
}

type WorkflowScheduleState struct {
	WorkflowRunID WorkflowRunID `json:"workflowRunId"`
	Weight        int           `json:"weight"`
	ServiceCount  int64         `json:"serviceCount"`
	LastSelectedAt time.Time    `json:"lastSelectedAt,omitempty"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}

type NodeExplanation struct {
	NodeRunID     NodeRunID  `json:"nodeRunId"`
	State         NodeState  `json:"state"`
	Reason        WaitReason `json:"reason,omitempty"`
	Detail        string     `json:"detail,omitempty"`
	RemainingDependencies int `json:"remainingDependencies,omitempty"`
}
