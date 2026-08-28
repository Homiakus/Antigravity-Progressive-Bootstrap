package model

import (
	"fmt"
	"math"
	"time"
)

type ProviderAssignmentState string

const (
	ProviderAssignmentActive     ProviderAssignmentState = "ACTIVE"
	ProviderAssignmentCompleted  ProviderAssignmentState = "COMPLETED"
	ProviderAssignmentSuperseded ProviderAssignmentState = "SUPERSEDED"
	ProviderAssignmentReleased   ProviderAssignmentState = "RELEASED"
)

func (s ProviderAssignmentState) Valid() bool {
	switch s {
	case ProviderAssignmentActive, ProviderAssignmentCompleted, ProviderAssignmentSuperseded, ProviderAssignmentReleased:
		return true
	default:
		return false
	}
}

func (s ProviderAssignmentState) Terminal() bool {
	return s == ProviderAssignmentCompleted || s == ProviderAssignmentSuperseded || s == ProviderAssignmentReleased
}

// ProviderAssignment is a durable historical routing decision for one Attempt.
// Multiple records may exist over the lifetime of an Attempt (for example after
// a safe provider handoff), but persistence enforces at most one ACTIVE record.
// Account/model identity is immutable; SessionID may be bound once after a new
// provider session is actually created.
type ProviderAssignment struct {
	ID        ProviderAssignmentID    `json:"id"`
	AttemptID AttemptID               `json:"attemptId"`
	AccountID ProviderAccountID       `json:"accountId"`
	ModelID   ProviderModelID         `json:"modelId"`
	SessionID ProviderSessionID       `json:"sessionId,omitempty"`
	State     ProviderAssignmentState `json:"state"`
	Revision  uint64                   `json:"revision"`
	CreatedAt time.Time                `json:"createdAt"`
	UpdatedAt time.Time                `json:"updatedAt"`
}

func (a ProviderAssignment) Validate() error {
	if a.ID == "" || a.AttemptID == "" || a.AccountID == "" || a.ModelID == "" {
		return fmt.Errorf("provider assignment id, attempt id, account id and model id are required")
	}
	if !a.State.Valid() {
		return fmt.Errorf("invalid provider assignment state %q", a.State)
	}
	if a.Revision == 0 {
		return fmt.Errorf("provider assignment revision must be positive")
	}
	if a.CreatedAt.IsZero() || a.UpdatedAt.IsZero() || a.UpdatedAt.Before(a.CreatedAt) {
		return fmt.Errorf("invalid provider assignment timestamps")
	}
	return nil
}

func ValidProviderAssignmentTransition(from, to ProviderAssignmentState) bool {
	if !from.Valid() || !to.Valid() {
		return false
	}
	if from == ProviderAssignmentActive {
		return to == ProviderAssignmentActive || to.Terminal()
	}
	return false
}

type ProviderReservationState string

const (
	ProviderReservationActive   ProviderReservationState = "ACTIVE"
	ProviderReservationSettled  ProviderReservationState = "SETTLED"
	ProviderReservationReleased ProviderReservationState = "RELEASED"
	ProviderReservationExpired  ProviderReservationState = "EXPIRED"
)

func (s ProviderReservationState) Valid() bool {
	switch s {
	case ProviderReservationActive, ProviderReservationSettled, ProviderReservationReleased, ProviderReservationExpired:
		return true
	default:
		return false
	}
}

func (s ProviderReservationState) Terminal() bool {
	return s == ProviderReservationSettled || s == ProviderReservationReleased || s == ProviderReservationExpired
}

func ValidProviderReservationTransition(from, to ProviderReservationState) bool {
	if !from.Valid() || !to.Valid() {
		return false
	}
	return from == ProviderReservationActive && to.Terminal()
}

// ProviderReservation is an immutable capacity claim except for State/Revision/
// UpdatedAt. Amount remains in Metric's native unit. OPAQUE is intentionally not
// reservable; a selector may observe it but cannot perform safe arithmetic on it.
type ProviderReservation struct {
	ID           ProviderReservationID    `json:"id"`
	AssignmentID ProviderAssignmentID     `json:"assignmentId"`
	AccountID    ProviderAccountID        `json:"accountId"`
	WindowID     string                   `json:"windowId"`
	ModelID      ProviderModelID          `json:"modelId,omitempty"`
	Metric       QuotaMetricKind          `json:"metric"`
	Amount       float64                  `json:"amount"`
	State        ProviderReservationState `json:"state"`
	Revision     uint64                   `json:"revision"`
	CreatedAt    time.Time                `json:"createdAt"`
	ExpiresAt    time.Time                `json:"expiresAt"`
	UpdatedAt    time.Time                `json:"updatedAt"`
}

func (r ProviderReservation) Validate() error {
	if r.ID == "" || r.AssignmentID == "" || r.AccountID == "" || r.WindowID == "" {
		return fmt.Errorf("provider reservation id, assignment id, account id and window id are required")
	}
	if !r.Metric.Valid() || r.Metric == QuotaMetricOpaque {
		return fmt.Errorf("provider reservation metric %q is not reservable", r.Metric)
	}
	if math.IsNaN(r.Amount) || math.IsInf(r.Amount, 0) || r.Amount <= 0 {
		return fmt.Errorf("provider reservation amount must be finite and positive")
	}
	if r.Metric == QuotaMetricFraction && r.Amount > 1 {
		return fmt.Errorf("fractional provider reservation amount must be within (0,1]")
	}
	if !r.State.Valid() {
		return fmt.Errorf("invalid provider reservation state %q", r.State)
	}
	if r.Revision == 0 {
		return fmt.Errorf("provider reservation revision must be positive")
	}
	if r.CreatedAt.IsZero() || r.ExpiresAt.IsZero() || r.UpdatedAt.IsZero() {
		return fmt.Errorf("provider reservation timestamps are required")
	}
	if !r.ExpiresAt.After(r.CreatedAt) || r.UpdatedAt.Before(r.CreatedAt) {
		return fmt.Errorf("invalid provider reservation timestamp ordering")
	}
	return nil
}

// ProviderUsageSample is immutable and idempotent by Key. Amount is always in
// the declared native Metric unit. ReservationID is optional because providers
// can report usage that was not reserved or can report it after a reservation
// has already transitioned terminal.
type ProviderUsageSample struct {
	Key           string                `json:"key"`
	AssignmentID  ProviderAssignmentID  `json:"assignmentId"`
	ReservationID ProviderReservationID `json:"reservationId,omitempty"`
	AccountID     ProviderAccountID     `json:"accountId"`
	ModelID       ProviderModelID       `json:"modelId,omitempty"`
	Metric        QuotaMetricKind       `json:"metric"`
	Amount        float64               `json:"amount"`
	ObservedAt    time.Time             `json:"observedAt"`
	CreatedAt     time.Time             `json:"createdAt"`
}

func (s ProviderUsageSample) Validate() error {
	if s.Key == "" || len(s.Key) > 256 || s.AssignmentID == "" || s.AccountID == "" {
		return fmt.Errorf("provider usage key, assignment id and account id are required")
	}
	if !s.Metric.Valid() || s.Metric == QuotaMetricOpaque {
		return fmt.Errorf("provider usage metric %q is not measurable", s.Metric)
	}
	if math.IsNaN(s.Amount) || math.IsInf(s.Amount, 0) || s.Amount < 0 {
		return fmt.Errorf("provider usage amount must be finite and non-negative")
	}
	if s.Metric == QuotaMetricFraction && s.Amount > 1 {
		return fmt.Errorf("fractional provider usage amount must be within [0,1]")
	}
	if s.ObservedAt.IsZero() || s.CreatedAt.IsZero() {
		return fmt.Errorf("provider usage timestamps are required")
	}
	return nil
}

// ProviderCircuitState is provider/account/model-scoped protection state. An
// empty ModelID means account-wide. Revision is a monotonic compare-and-swap
// fence, mirroring the existing service circuit breaker semantics without
// conflating the two scopes.
type ProviderCircuitState struct {
	AccountID           ProviderAccountID `json:"accountId"`
	ModelID             ProviderModelID   `json:"modelId,omitempty"`
	Revision            uint64            `json:"revision"`
	State               CircuitState      `json:"state"`
	ConsecutiveFailures int               `json:"consecutiveFailures"`
	OpenedAt            time.Time         `json:"openedAt,omitempty"`
	NextProbeAt         time.Time         `json:"nextProbeAt,omitempty"`
	ProbeInFlight       bool              `json:"probeInFlight,omitempty"`
	UpdatedAt           time.Time         `json:"updatedAt"`
}

func (s ProviderCircuitState) Validate() error {
	if s.AccountID == "" || s.Revision == 0 || s.ConsecutiveFailures < 0 || s.UpdatedAt.IsZero() {
		return fmt.Errorf("invalid provider circuit state")
	}
	switch s.State {
	case CircuitClosed, CircuitOpen, CircuitHalfOpen:
		return nil
	default:
		return fmt.Errorf("invalid provider circuit state %q", s.State)
	}
}
