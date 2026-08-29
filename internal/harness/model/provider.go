package model

import (
	"fmt"
	"time"
)

type ProviderKind string

const (
	ProviderAntigravity ProviderKind = "ANTIGRAVITY"
	ProviderCodex       ProviderKind = "CODEX"
)

func (k ProviderKind) Valid() bool {
	switch k {
	case ProviderAntigravity, ProviderCodex:
		return true
	default:
		return false
	}
}

type ProviderHealth string

const (
	ProviderHealthUnknown     ProviderHealth = "UNKNOWN"
	ProviderHealthHealthy     ProviderHealth = "HEALTHY"
	ProviderHealthDegraded    ProviderHealth = "DEGRADED"
	ProviderHealthExhausted   ProviderHealth = "EXHAUSTED"
	ProviderHealthUnavailable ProviderHealth = "UNAVAILABLE"
)

func (h ProviderHealth) Valid() bool {
	switch h {
	case ProviderHealthUnknown, ProviderHealthHealthy, ProviderHealthDegraded, ProviderHealthExhausted, ProviderHealthUnavailable:
		return true
	default:
		return false
	}
}

type ProviderAccountState string

const (
	ProviderAccountActive   ProviderAccountState = "ACTIVE"
	ProviderAccountDraining ProviderAccountState = "DRAINING"
	ProviderAccountDisabled ProviderAccountState = "DISABLED"
)

func (s ProviderAccountState) Valid() bool {
	switch s {
	case ProviderAccountActive, ProviderAccountDraining, ProviderAccountDisabled:
		return true
	default:
		return false
	}
}

type ProviderModelID string

type ProviderAccount struct {
	ID        ProviderAccountID    `json:"id"`
	Provider  ProviderKind         `json:"provider"`
	Name      string               `json:"name,omitempty"`
	State     ProviderAccountState `json:"state"`
	CreatedAt time.Time            `json:"createdAt"`
	UpdatedAt time.Time            `json:"updatedAt"`
}

type ProviderModelDescriptor struct {
	AccountID    ProviderAccountID `json:"accountId"`
	Provider     ProviderKind      `json:"provider"`
	ID           ProviderModelID   `json:"id"`
	DisplayName  string            `json:"displayName,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	ContextLimit int64             `json:"contextLimit,omitempty"`
	Enabled      bool              `json:"enabled"`
}

type QuotaMetricKind string

const (
	QuotaMetricTokens   QuotaMetricKind = "TOKENS"
	QuotaMetricRequests QuotaMetricKind = "REQUESTS"
	QuotaMetricCost     QuotaMetricKind = "COST"
	QuotaMetricFraction QuotaMetricKind = "FRACTION"
	QuotaMetricOpaque   QuotaMetricKind = "OPAQUE"
)

func (k QuotaMetricKind) Valid() bool {
	switch k {
	case QuotaMetricTokens, QuotaMetricRequests, QuotaMetricCost, QuotaMetricFraction, QuotaMetricOpaque:
		return true
	default:
		return false
	}
}

type QuotaWindow struct {
	ID                string          `json:"id"`
	ModelID           ProviderModelID `json:"modelId,omitempty"`
	Metric            QuotaMetricKind `json:"metric"`
	Limit             *float64        `json:"limit,omitempty"`
	Remaining         *float64        `json:"remaining,omitempty"`
	RemainingFraction *float64        `json:"remainingFraction,omitempty"`
	ResetAt           *time.Time      `json:"resetAt,omitempty"`
	ObservedAt        time.Time       `json:"observedAt"`
	Confidence        float64         `json:"confidence"`
}

func (w QuotaWindow) Validate() error {
	if w.ID == "" {
		return fmt.Errorf("quota window id is required")
	}
	if !w.Metric.Valid() {
		return fmt.Errorf("invalid quota metric %q", w.Metric)
	}
	if w.ObservedAt.IsZero() {
		return fmt.Errorf("quota window observedAt is required")
	}
	if w.Confidence < 0 || w.Confidence > 1 {
		return fmt.Errorf("quota window confidence must be within [0,1]")
	}
	if w.Limit != nil && *w.Limit < 0 {
		return fmt.Errorf("quota window limit must be non-negative")
	}
	if w.Remaining != nil && *w.Remaining < 0 {
		return fmt.Errorf("quota window remaining must be non-negative")
	}
	if w.RemainingFraction != nil && (*w.RemainingFraction < 0 || *w.RemainingFraction > 1) {
		return fmt.Errorf("quota window remainingFraction must be within [0,1]")
	}
	return nil
}

type ProviderCapacitySnapshot struct {
	AccountID  ProviderAccountID `json:"accountId"`
	Provider   ProviderKind      `json:"provider"`
	Health     ProviderHealth    `json:"health"`
	Windows    []QuotaWindow     `json:"windows,omitempty"`
	ActiveRuns int               `json:"activeRuns,omitempty"`
	ObservedAt time.Time         `json:"observedAt"`
}

func (s ProviderCapacitySnapshot) Validate() error {
	if s.AccountID == "" {
		return fmt.Errorf("provider capacity account id is required")
	}
	if !s.Provider.Valid() {
		return fmt.Errorf("invalid provider %q", s.Provider)
	}
	if !s.Health.Valid() {
		return fmt.Errorf("invalid provider health %q", s.Health)
	}
	if s.ActiveRuns < 0 {
		return fmt.Errorf("provider capacity activeRuns must be non-negative")
	}
	if s.ObservedAt.IsZero() {
		return fmt.Errorf("provider capacity observedAt is required")
	}
	for i, window := range s.Windows {
		if err := window.Validate(); err != nil {
			return fmt.Errorf("quota window %d: %w", i, err)
		}
	}
	return nil
}

type ProviderSessionState string

const (
	ProviderSessionActive    ProviderSessionState = "ACTIVE"
	ProviderSessionDraining  ProviderSessionState = "DRAINING"
	ProviderSessionExhausted ProviderSessionState = "EXHAUSTED"
	ProviderSessionClosed    ProviderSessionState = "CLOSED"
)

func (s ProviderSessionState) Valid() bool {
	switch s {
	case ProviderSessionActive, ProviderSessionDraining, ProviderSessionExhausted, ProviderSessionClosed:
		return true
	default:
		return false
	}
}

type ProviderSessionSnapshot struct {
	ID                   ProviderSessionID    `json:"id"`
	Provider             ProviderKind         `json:"provider"`
	AccountID            ProviderAccountID    `json:"accountId"`
	ModelID              ProviderModelID      `json:"modelId"`
	State                ProviderSessionState `json:"state"`
	ContextUsed          int64                `json:"contextUsed,omitempty"`
	ContextLimit         int64                `json:"contextLimit,omitempty"`
	LastUsedAt           time.Time            `json:"lastUsedAt"`
	ObservedAt           time.Time            `json:"observedAt,omitempty"`
	WorkspaceFingerprint string               `json:"workspaceFingerprint,omitempty"`
}

func (s ProviderSessionSnapshot) Validate() error {
	if s.ID == "" || s.AccountID == "" || s.ModelID == "" {
		return fmt.Errorf("provider session id, account id and model id are required")
	}
	if !s.Provider.Valid() {
		return fmt.Errorf("invalid provider %q", s.Provider)
	}
	if !s.State.Valid() {
		return fmt.Errorf("invalid provider session state %q", s.State)
	}
	if s.ContextUsed < 0 || s.ContextLimit < 0 {
		return fmt.Errorf("provider session context values must be non-negative")
	}
	if s.ContextLimit > 0 && s.ContextUsed > s.ContextLimit {
		return fmt.Errorf("provider session contextUsed exceeds contextLimit")
	}
	if s.LastUsedAt.IsZero() {
		return fmt.Errorf("provider session lastUsedAt is required")
	}
	return nil
}
