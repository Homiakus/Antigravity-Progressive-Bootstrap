package model

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const MaxProviderDemandClassLength = 128

// ProviderDemandDimensions binds one canonical settled provider usage sample to
// the categorical dimensions used by the demand estimator. Persistence admits
// at most one canonical sample per assignment+metric; raw/intermediate usage
// updates remain in the authoritative usage ledger but are not independent
// training observations. UsageKey is the durable idempotency key from
// ProviderUsageSample. The projection stores no amount/provider/model values, so
// those facts cannot diverge from the authoritative usage ledger.
type ProviderDemandDimensions struct {
	UsageKey        string `json:"usageKey"`
	TaskClass       string `json:"taskClass"`
	RepositoryClass string `json:"repositoryClass"`
	ContextClass    string `json:"contextClass"`
}

func (d ProviderDemandDimensions) Validate() error {
	if d.UsageKey == "" || len(d.UsageKey) > 256 {
		return fmt.Errorf("provider demand usage key is required and must be at most 256 bytes")
	}
	if err := validateProviderDemandClass("task", d.TaskClass); err != nil {
		return err
	}
	if err := validateProviderDemandClass("repository", d.RepositoryClass); err != nil {
		return err
	}
	if err := validateProviderDemandClass("context", d.ContextClass); err != nil {
		return err
	}
	return nil
}

// ProviderDemandSample is a read projection over the immutable usage ledger and
// its categorical dimensions. Provider/account/model/metric/amount/timestamp are
// reconstructed from provider_usage_samples + provider_accounts, not duplicated
// in provider_demand_dimensions.
type ProviderDemandSample struct {
	UsageKey        string            `json:"usageKey"`
	AccountID       ProviderAccountID `json:"accountId"`
	Provider        ProviderKind      `json:"provider"`
	ModelID         ProviderModelID   `json:"modelId"`
	Metric          QuotaMetricKind   `json:"metric"`
	Amount          float64           `json:"amount"`
	TaskClass       string            `json:"taskClass"`
	RepositoryClass string            `json:"repositoryClass"`
	ContextClass    string            `json:"contextClass"`
	ObservedAt      time.Time         `json:"observedAt"`
}

func (s ProviderDemandSample) Validate() error {
	if s.UsageKey == "" || len(s.UsageKey) > 256 || s.AccountID == "" || s.ModelID == "" {
		return fmt.Errorf("provider demand usage key, account id and model id are required")
	}
	if !s.Provider.Valid() {
		return fmt.Errorf("invalid provider demand provider %q", s.Provider)
	}
	if !s.Metric.Valid() || s.Metric == QuotaMetricOpaque {
		return fmt.Errorf("provider demand metric %q is not estimable", s.Metric)
	}
	if math.IsNaN(s.Amount) || math.IsInf(s.Amount, 0) || s.Amount < 0 {
		return fmt.Errorf("provider demand amount must be finite and non-negative")
	}
	if s.Metric == QuotaMetricFraction && s.Amount > 1 {
		return fmt.Errorf("fractional provider demand amount must be within [0,1]")
	}
	if err := validateProviderDemandClass("task", s.TaskClass); err != nil {
		return err
	}
	if err := validateProviderDemandClass("repository", s.RepositoryClass); err != nil {
		return err
	}
	if err := validateProviderDemandClass("context", s.ContextClass); err != nil {
		return err
	}
	if s.ObservedAt.IsZero() {
		return fmt.Errorf("provider demand observedAt is required")
	}
	return nil
}

// ProviderDemandHistoryQuery requests newest-first history in exactly one native
// quota metric. Empty categorical fields are wildcards, but filters must be
// removed from least-specific to most-specific: context requires repository and
// task; repository requires task. This makes the estimator fallback hierarchy
// explicit and auditable.
type ProviderDemandHistoryQuery struct {
	Provider        ProviderKind    `json:"provider"`
	ModelID         ProviderModelID `json:"modelId"`
	Metric          QuotaMetricKind `json:"metric"`
	TaskClass       string          `json:"taskClass,omitempty"`
	RepositoryClass string          `json:"repositoryClass,omitempty"`
	ContextClass    string          `json:"contextClass,omitempty"`
	Since           time.Time       `json:"since"`
	Limit           int             `json:"limit"`
}

func (q ProviderDemandHistoryQuery) Validate() error {
	if !q.Provider.Valid() {
		return fmt.Errorf("invalid provider demand history provider %q", q.Provider)
	}
	if q.ModelID == "" {
		return fmt.Errorf("provider demand history model id is required")
	}
	if !q.Metric.Valid() || q.Metric == QuotaMetricOpaque {
		return fmt.Errorf("provider demand history metric %q is not estimable", q.Metric)
	}
	if q.Since.IsZero() {
		return fmt.Errorf("provider demand history since is required")
	}
	if q.Limit <= 0 || q.Limit > 10000 {
		return fmt.Errorf("provider demand history limit must be within [1,10000]")
	}
	if q.TaskClass != "" {
		if err := validateProviderDemandClass("task", q.TaskClass); err != nil {
			return err
		}
	}
	if q.RepositoryClass != "" {
		if q.TaskClass == "" {
			return fmt.Errorf("repository class filter requires task class")
		}
		if err := validateProviderDemandClass("repository", q.RepositoryClass); err != nil {
			return err
		}
	}
	if q.ContextClass != "" {
		if q.TaskClass == "" || q.RepositoryClass == "" {
			return fmt.Errorf("context class filter requires task and repository class")
		}
		if err := validateProviderDemandClass("context", q.ContextClass); err != nil {
			return err
		}
	}
	return nil
}

func validateProviderDemandClass(name, value string) error {
	if value == "" || len(value) > MaxProviderDemandClassLength {
		return fmt.Errorf("provider demand %s class is required and must be at most %d bytes", name, MaxProviderDemandClassLength)
	}
	if strings.TrimSpace(value) != value {
		return fmt.Errorf("provider demand %s class must not have leading or trailing whitespace", name)
	}
	return nil
}
