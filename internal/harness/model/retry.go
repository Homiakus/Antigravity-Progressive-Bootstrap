package model

import "time"

type ErrorClass string

const (
	ErrorApplicationPermanent ErrorClass = "APPLICATION_PERMANENT"
	ErrorApplicationTransient ErrorClass = "APPLICATION_TRANSIENT"
	ErrorInfraTransient       ErrorClass = "INFRA_TRANSIENT"
	ErrorRateLimited          ErrorClass = "RATE_LIMITED"
	ErrorTimeout              ErrorClass = "TIMEOUT"
	ErrorCancelled            ErrorClass = "CANCELLED"
	ErrorPolicyDenied         ErrorClass = "POLICY_DENIED"
	ErrorUnschedulable        ErrorClass = "UNSCHEDULABLE"
	ErrorProtocol             ErrorClass = "PROTOCOL_ERROR"
	ErrorUnknownEffect        ErrorClass = "UNKNOWN_EFFECT"
)

func (c ErrorClass) Valid() bool {
	switch c {
	case ErrorApplicationPermanent, ErrorApplicationTransient, ErrorInfraTransient,
		ErrorRateLimited, ErrorTimeout, ErrorCancelled, ErrorPolicyDenied,
		ErrorUnschedulable, ErrorProtocol, ErrorUnknownEffect:
		return true
	default:
		return false
	}
}

// RetryPolicySpec is immutable workflow-definition data. Runtime behavior lives
// in internal/harness/retry; the model contains only serializable policy facts.
type RetryPolicySpec struct {
	MaxAttempts         int           `json:"maxAttempts"`
	MaxElapsedTime      time.Duration `json:"maxElapsedTime,omitempty"`
	InitialDelay        time.Duration `json:"initialDelay,omitempty"`
	BackoffFactor       float64       `json:"backoffFactor,omitempty"`
	MaxDelay            time.Duration `json:"maxDelay,omitempty"`
	Jitter              float64       `json:"jitter,omitempty"`
	RetryableClasses    []ErrorClass  `json:"retryableClasses,omitempty"`
	NonRetryableClasses []ErrorClass  `json:"nonRetryableClasses,omitempty"`

	// Budgets are protection envelopes, not replacements for MaxAttempts.
	// A zero limit/window disables that scope. Both fields of a scope must be
	// configured together so the snapshot is unambiguous after restart.
	WorkflowBudgetLimit  int           `json:"workflowBudgetLimit,omitempty"`
	WorkflowBudgetWindow time.Duration `json:"workflowBudgetWindow,omitempty"`
	ServiceBudgetLimit   int           `json:"serviceBudgetLimit,omitempty"`
	ServiceBudgetWindow  time.Duration `json:"serviceBudgetWindow,omitempty"`
}

// Failure is the typed outcome passed from an executor/tool adapter to retry
// policy. RetryAfter and ServiceKey are advisory metadata, not state-machine
// authority.
type Failure struct {
	Class      ErrorClass        `json:"class"`
	Message    string            `json:"message,omitempty"`
	RetryAfter time.Duration     `json:"retryAfter,omitempty"`
	ServiceKey string            `json:"serviceKey,omitempty"`
	Details    map[string]string `json:"details,omitempty"`
}

// RetrySchedule is the durable bridge between a terminal Attempt and a future
// immutable Attempt. It intentionally belongs to NodeRun, not Attempt.
type RetrySchedule struct {
	NodeRunID       NodeRunID     `json:"nodeRunId"`
	WorkflowRunID   WorkflowRunID `json:"workflowRunId"`
	FailedAttemptID AttemptID     `json:"failedAttemptId"`
	AttemptNumber   int           `json:"attemptNumber"`
	FailureClass    ErrorClass    `json:"failureClass"`
	NotBefore       time.Time     `json:"notBefore"`
	ScheduledAt     time.Time     `json:"scheduledAt"`
	PolicyRef       string        `json:"policyRef,omitempty"`
	ServiceKey      string        `json:"serviceKey,omitempty"`
}

type RetryBudgetScope string

const (
	RetryBudgetWorkflow RetryBudgetScope = "WORKFLOW"
	RetryBudgetService  RetryBudgetScope = "SERVICE"
)

type RetryBudget struct {
	Scope       RetryBudgetScope `json:"scope"`
	ScopeKey    string           `json:"scopeKey"`
	WindowStart time.Time        `json:"windowStart"`
	Window      time.Duration    `json:"window"`
	Limit       int              `json:"limit"`
	Used        int              `json:"used"`
	UpdatedAt   time.Time        `json:"updatedAt"`
}

type CircuitState string

const (
	CircuitClosed   CircuitState = "CLOSED"
	CircuitOpen     CircuitState = "OPEN"
	CircuitHalfOpen CircuitState = "HALF_OPEN"
)

type CircuitBreaker struct {
	ServiceKey          string       `json:"serviceKey"`
	State               CircuitState `json:"state"`
	ConsecutiveFailures int          `json:"consecutiveFailures"`
	FailureThreshold    int          `json:"failureThreshold"`
	OpenedAt            time.Time    `json:"openedAt,omitempty"`
	NextProbeAt         time.Time    `json:"nextProbeAt,omitempty"`
	ProbeInFlight       bool         `json:"probeInFlight,omitempty"`
	UpdatedAt           time.Time    `json:"updatedAt"`
}
