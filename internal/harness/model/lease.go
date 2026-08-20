package model

import "time"

type LeaseState string

const (
	LeaseActive   LeaseState = "ACTIVE"
	LeaseExpired  LeaseState = "EXPIRED"
	LeaseReleased LeaseState = "RELEASED"
)

type Lease struct {
	ID          LeaseID      `json:"id"`
	AttemptID   AttemptID    `json:"attemptId"`
	WorkerID    WorkerID     `json:"workerId"`
	Epoch       uint64       `json:"epoch"`
	State       LeaseState   `json:"state"`
	ClaimedAt   time.Time    `json:"claimedAt"`
	HeartbeatAt time.Time    `json:"heartbeatAt"`
	ExpiresAt   time.Time    `json:"expiresAt"`
	ClosedAt    time.Time    `json:"closedAt,omitempty"`
}

func (l Lease) Authoritative(workerID WorkerID, epoch uint64, now time.Time) bool {
	return l.State == LeaseActive && l.WorkerID == workerID && l.Epoch == epoch && !now.After(l.ExpiresAt)
}
