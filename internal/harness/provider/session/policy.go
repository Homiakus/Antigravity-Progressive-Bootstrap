package session

import (
	"fmt"
	"math"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

type Action string

const (
	ActionReuse            Action = "REUSE"
	ActionNew              Action = "NEW"
	ActionCheckpointAndNew Action = "CHECKPOINT_AND_NEW"
	ActionUnavailable      Action = "UNAVAILABLE"
)

func (a Action) Valid() bool {
	switch a {
	case ActionReuse, ActionNew, ActionCheckpointAndNew, ActionUnavailable:
		return true
	default:
		return false
	}
}

type Reason string

const (
	ReasonReuseCompatible          Reason = "REUSE_COMPATIBLE"
	ReasonReuseIncumbentHysteresis Reason = "REUSE_INCUMBENT_HYSTERESIS"
	ReasonCheckpointContextPressure Reason = "CHECKPOINT_CONTEXT_PRESSURE"
	ReasonCheckpointUnknownContext  Reason = "CHECKPOINT_UNKNOWN_CONTEXT"
	ReasonCheckpointSessionState    Reason = "CHECKPOINT_SESSION_STATE"
	ReasonNewNoReusableSession      Reason = "NEW_NO_REUSABLE_SESSION"
	ReasonUnavailableAccount        Reason = "UNAVAILABLE_ACCOUNT"
	ReasonUnavailableModel          Reason = "UNAVAILABLE_MODEL"
	ReasonUnavailableHealthEvidence Reason = "UNAVAILABLE_HEALTH_EVIDENCE"
	ReasonUnavailableProviderHealth Reason = "UNAVAILABLE_PROVIDER_HEALTH"
)

// Policy is explicit operator policy. Thresholds deliberately have no package
// defaults: context pressure and acceptable observation age are operational
// choices, not provider facts that core logic may guess.
type Policy struct {
	MaxSessionObservationAge time.Duration
	MaxHealthObservationAge  time.Duration
	MaxFutureSkew             time.Duration
	MinReuseRemainingFraction float64
	IncumbentHysteresis       float64
}

func (p Policy) Validate() error {
	if p.MaxSessionObservationAge <= 0 {
		return fmt.Errorf("session max observation age must be positive")
	}
	if p.MaxHealthObservationAge <= 0 {
		return fmt.Errorf("provider health max observation age must be positive")
	}
	if p.MaxFutureSkew < 0 {
		return fmt.Errorf("session max future skew must be non-negative")
	}
	for name, value := range map[string]float64{
		"min reuse remaining fraction": p.MinReuseRemainingFraction,
		"incumbent hysteresis":         p.IncumbentHysteresis,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
			return fmt.Errorf("%s must be finite within [0,1]", name)
		}
	}
	if p.IncumbentHysteresis > p.MinReuseRemainingFraction {
		return fmt.Errorf("incumbent hysteresis cannot exceed min reuse remaining fraction")
	}
	return nil
}

type Request struct {
	AccountID            harnessmodel.ProviderAccountID
	ModelID              harnessmodel.ProviderModelID
	WorkspaceFingerprint string
	IncumbentSessionID   harnessmodel.ProviderSessionID
	DecisionAt           time.Time
}

func (r Request) Validate() error {
	if r.AccountID == "" || r.ModelID == "" || r.DecisionAt.IsZero() {
		return fmt.Errorf("session broker account, model and decision time are required")
	}
	return nil
}

// Evidence is a coherent decision input assembled by the Store-backed service.
// HealthObservedAt and every reusable session ObservedAt are semantically
// distinct from LastUsedAt.
type Evidence struct {
	Account          harnessmodel.ProviderAccount
	Model            harnessmodel.ProviderModelDescriptor
	Health           harnessmodel.ProviderHealth
	HealthObservedAt time.Time
	Sessions         []harnessmodel.ProviderSessionSnapshot
}

type Decision struct {
	Action               Action                         `json:"action"`
	Reason               Reason                         `json:"reason"`
	SessionID            harnessmodel.ProviderSessionID `json:"sessionId,omitempty"`
	CheckpointSessionID  harnessmodel.ProviderSessionID `json:"checkpointSessionId,omitempty"`
	RemainingFraction    *float64                       `json:"remainingFraction,omitempty"`
	SessionObservedAt    time.Time                      `json:"sessionObservedAt,omitempty"`
}

type candidate struct {
	session   harnessmodel.ProviderSessionSnapshot
	remaining float64
	known     bool
}

func Decide(policy Policy, req Request, evidence Evidence) (Decision, error) {
	if err := policy.Validate(); err != nil {
		return Decision{}, err
	}
	if err := req.Validate(); err != nil {
		return Decision{}, err
	}
	if evidence.Account.ID != req.AccountID {
		return Decision{}, fmt.Errorf("session broker account evidence %s does not match request %s", evidence.Account.ID, req.AccountID)
	}
	if evidence.Account.Provider == "" || !evidence.Account.Provider.Valid() {
		return Decision{}, fmt.Errorf("session broker account has invalid provider %q", evidence.Account.Provider)
	}
	if evidence.Account.State != harnessmodel.ProviderAccountActive {
		return Decision{Action: ActionUnavailable, Reason: ReasonUnavailableAccount}, nil
	}
	if evidence.Model.AccountID != req.AccountID || evidence.Model.Provider != evidence.Account.Provider || evidence.Model.ID != req.ModelID || !evidence.Model.Enabled {
		return Decision{Action: ActionUnavailable, Reason: ReasonUnavailableModel}, nil
	}
	if !evidence.Health.Valid() || !isFresh(evidence.HealthObservedAt, req.DecisionAt, policy.MaxHealthObservationAge, policy.MaxFutureSkew) {
		return Decision{Action: ActionUnavailable, Reason: ReasonUnavailableHealthEvidence}, nil
	}
	if evidence.Health == harnessmodel.ProviderHealthUnavailable || evidence.Health == harnessmodel.ProviderHealthExhausted {
		return Decision{Action: ActionUnavailable, Reason: ReasonUnavailableProviderHealth}, nil
	}

	var bestReuse *candidate
	var incumbent *candidate
	var bestCheckpoint *candidate
	for i := range evidence.Sessions {
		s := evidence.Sessions[i]
		if s.AccountID != req.AccountID || s.Provider != evidence.Account.Provider {
			return Decision{}, fmt.Errorf("session broker received contradictory session identity %s", s.ID)
		}
		if err := s.Validate(); err != nil {
			return Decision{}, fmt.Errorf("validate session broker evidence %s: %w", s.ID, err)
		}
		if s.ModelID != req.ModelID {
			continue
		}
		if req.WorkspaceFingerprint != "" && s.WorkspaceFingerprint != req.WorkspaceFingerprint {
			continue
		}
		if !isFresh(s.ObservedAt, req.DecisionAt, policy.MaxSessionObservationAge, policy.MaxFutureSkew) {
			continue
		}
		if s.LastUsedAt.After(req.DecisionAt.Add(policy.MaxFutureSkew)) {
			continue
		}

		c := candidate{session: s}
		if s.ContextLimit > 0 {
			c.known = true
			c.remaining = float64(s.ContextLimit-s.ContextUsed) / float64(s.ContextLimit)
			if c.remaining < 0 {
				c.remaining = 0
			}
			if c.remaining > 1 {
				c.remaining = 1
			}
		}

		if s.ID == req.IncumbentSessionID {
			copy := c
			incumbent = &copy
		}

		if s.State == harnessmodel.ProviderSessionActive && c.known && c.remaining >= policy.MinReuseRemainingFraction {
			if bestReuse == nil || betterReuse(c, *bestReuse) {
				copy := c
				bestReuse = &copy
			}
		}

		if s.State != harnessmodel.ProviderSessionClosed {
			if s.State != harnessmodel.ProviderSessionActive || !c.known || c.remaining < policy.MinReuseRemainingFraction {
				if bestCheckpoint == nil || betterCheckpoint(c, *bestCheckpoint) {
					copy := c
					bestCheckpoint = &copy
				}
			}
		}
	}

	if incumbent != nil && incumbent.session.State == harnessmodel.ProviderSessionActive && incumbent.known {
		floor := policy.MinReuseRemainingFraction - policy.IncumbentHysteresis
		if incumbent.remaining >= floor {
			reason := ReasonReuseCompatible
			if incumbent.remaining < policy.MinReuseRemainingFraction {
				reason = ReasonReuseIncumbentHysteresis
			}
			return reuseDecision(*incumbent, reason), nil
		}
	}
	if bestReuse != nil {
		return reuseDecision(*bestReuse, ReasonReuseCompatible), nil
	}
	if bestCheckpoint != nil {
		reason := ReasonCheckpointContextPressure
		switch {
		case bestCheckpoint.session.State != harnessmodel.ProviderSessionActive:
			reason = ReasonCheckpointSessionState
		case !bestCheckpoint.known:
			reason = ReasonCheckpointUnknownContext
		}
		remaining := optionalRemaining(*bestCheckpoint)
		return Decision{
			Action:              ActionCheckpointAndNew,
			Reason:              reason,
			CheckpointSessionID: bestCheckpoint.session.ID,
			RemainingFraction:   remaining,
			SessionObservedAt:   bestCheckpoint.session.ObservedAt,
		}, nil
	}
	return Decision{Action: ActionNew, Reason: ReasonNewNoReusableSession}, nil
}

func isFresh(observedAt, now time.Time, maxAge, maxFutureSkew time.Duration) bool {
	if observedAt.IsZero() {
		return false
	}
	if observedAt.After(now.Add(maxFutureSkew)) {
		return false
	}
	return !observedAt.Before(now.Add(-maxAge))
}

func betterReuse(a, b candidate) bool {
	if a.remaining != b.remaining {
		return a.remaining > b.remaining
	}
	if !a.session.LastUsedAt.Equal(b.session.LastUsedAt) {
		return a.session.LastUsedAt.After(b.session.LastUsedAt)
	}
	return a.session.ID < b.session.ID
}

func betterCheckpoint(a, b candidate) bool {
	if !a.session.LastUsedAt.Equal(b.session.LastUsedAt) {
		return a.session.LastUsedAt.After(b.session.LastUsedAt)
	}
	return a.session.ID < b.session.ID
}

func reuseDecision(c candidate, reason Reason) Decision {
	return Decision{
		Action:             ActionReuse,
		Reason:             reason,
		SessionID:          c.session.ID,
		RemainingFraction: optionalRemaining(c),
		SessionObservedAt: c.session.ObservedAt,
	}
}

func optionalRemaining(c candidate) *float64 {
	if !c.known {
		return nil
	}
	v := c.remaining
	return &v
}
