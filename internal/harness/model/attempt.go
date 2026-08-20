package model

import "time"

type AttemptState string

const (
	AttemptCreated   AttemptState = "CREATED"
	AttemptClaimed   AttemptState = "CLAIMED"
	AttemptRunning   AttemptState = "RUNNING"
	AttemptSucceeded AttemptState = "SUCCEEDED"
	AttemptFailed    AttemptState = "FAILED"
	AttemptTimedOut  AttemptState = "TIMED_OUT"
	AttemptCancelled AttemptState = "CANCELLED"
	AttemptLost      AttemptState = "LOST"
	AttemptInDoubt   AttemptState = "IN_DOUBT"
)

func (s AttemptState) Terminal() bool {
	switch s {
	case AttemptSucceeded, AttemptFailed, AttemptTimedOut, AttemptCancelled, AttemptLost, AttemptInDoubt:
		return true
	default:
		return false
	}
}

type Attempt struct {
	ID           AttemptID    `json:"id"`
	NodeRunID    NodeRunID    `json:"nodeRunId"`
	Number       int          `json:"number"`
	State        AttemptState `json:"state"`
	WorkerID     WorkerID     `json:"workerId,omitempty"`
	LeaseEpoch   uint64       `json:"leaseEpoch,omitempty"`
	CreatedAt    time.Time    `json:"createdAt"`
	StartedAt    time.Time    `json:"startedAt,omitempty"`
	FinishedAt   time.Time    `json:"finishedAt,omitempty"`
	ErrorClass   string       `json:"errorClass,omitempty"`
	ErrorMessage string       `json:"errorMessage,omitempty"`
}
