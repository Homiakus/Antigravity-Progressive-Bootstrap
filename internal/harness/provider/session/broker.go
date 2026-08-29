package session

import (
	"context"
	"errors"
	"fmt"
	"sort"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

// Action is the session/context decision returned before provider selection
// commits an assignment. It deliberately does not execute or create sessions.
type Action string

const (
	ActionReuse            Action = "REUSE"
	ActionNew              Action = "NEW"
	ActionCheckpointAndNew Action = "CHECKPOINT_AND_NEW"
	ActionUnavailable      Action = "UNAVAILABLE"
)

// Reason is a stable machine-readable explanation for a broker decision.
type Reason string

const (
	ReasonReusableSession      Reason = "REUSABLE_SESSION"
	ReasonRetainedByHysteresis Reason = "RETAINED_BY_HYSTERESIS"
	ReasonNoReusableSession    Reason = "NO_REUSABLE_SESSION"
	ReasonContextRotation      Reason = "CONTEXT_ROTATION_REQUIRED"
	ReasonAccountDisabled      Reason = "ACCOUNT_DISABLED"
	ReasonAccountDraining      Reason = "ACCOUNT_DRAINING"
	ReasonProviderUnavailable  Reason = "PROVIDER_UNAVAILABLE"
	ReasonProviderExhausted    Reason = "PROVIDER_EXHAUSTED"
	ReasonModelNotFound        Reason = "MODEL_NOT_FOUND"
	ReasonModelDisabled        Reason = "MODEL_DISABLED"
	ReasonCapabilityMismatch   Reason = "CAPABILITY_MISMATCH"
	ReasonModelContextTooSmall Reason = "MODEL_CONTEXT_TOO_SMALL"
)

// Policy defines the reuse hysteresis. A session not already preferred must
// have at least AcquireReuseHeadroomFraction remaining to be acquired. The
// currently preferred session may continue down to the lower retain threshold.
// Callers choose the thresholds; the broker does not embed guessed provider
// constants.
type Policy struct {
	AcquireReuseHeadroomFraction float64
	RetainReuseHeadroomFraction  float64
}

func (p Policy) Validate() error {
	if p.AcquireReuseHeadroomFraction < 0 || p.AcquireReuseHeadroomFraction > 1 {
		return fmt.Errorf("acquire reuse headroom fraction must be within [0,1]")
	}
	if p.RetainReuseHeadroomFraction < 0 || p.RetainReuseHeadroomFraction > 1 {
		return fmt.Errorf("retain reuse headroom fraction must be within [0,1]")
	}
	if p.RetainReuseHeadroomFraction > p.AcquireReuseHeadroomFraction {
		return fmt.Errorf("retain reuse headroom fraction must not exceed acquire threshold")
	}
	return nil
}

// Request describes session requirements after a provider account/model has
// become a routing candidate. WorkspaceFingerprint is already privacy-safe
// provider-domain data; raw repository paths never enter this policy.
type Request struct {
	ModelID              harnessmodel.ProviderModelID
	RequiredCapabilities []string
	WorkspaceFingerprint string
	RequiredContext      int64
	PreferredSessionID   harnessmodel.ProviderSessionID
}

func (r Request) Validate() error {
	if r.ModelID == "" {
		return fmt.Errorf("session request model id is required")
	}
	if r.RequiredContext < 0 {
		return fmt.Errorf("session request required context must be non-negative")
	}
	for i, capability := range r.RequiredCapabilities {
		if capability == "" {
			return fmt.Errorf("session request capability %d is empty", i)
		}
	}
	return nil
}

// Decision is deterministic for one Snapshot+Request+Policy. Headroom fields
// are populated only for REUSE decisions where the session context limit is
// authoritative and non-zero.
type Decision struct {
	Action           Action                         `json:"action"`
	SessionID        harnessmodel.ProviderSessionID `json:"sessionId,omitempty"`
	Headroom         int64                          `json:"headroom,omitempty"`
	HeadroomFraction float64                        `json:"headroomFraction,omitempty"`
	Reasons          []Reason                       `json:"reasons,omitempty"`
}

// Snapshot is the coherent durable input used by the broker. Health UNKNOWN is
// valid and means no authoritative health block is known; EXHAUSTED and
// UNAVAILABLE are hard execution blocks.
type Snapshot struct {
	Account  harnessmodel.ProviderAccount
	Models   []harnessmodel.ProviderModelDescriptor
	Sessions []harnessmodel.ProviderSessionSnapshot
	Health   harnessmodel.ProviderHealth
}

// Source supplies one coherent account/model/session/health snapshot.
type Source interface {
	Snapshot(context.Context, harnessmodel.ProviderAccountID) (Snapshot, error)
}

// StoreSource adapts the durable harness Store to the narrow broker source.
// All reads happen inside one Store.View so a decision never mixes separate
// durable revisions. Absence of a capacity row is represented as UNKNOWN
// health rather than fabricated quota evidence.
type StoreSource struct {
	Store harnessstore.Store
}

var _ Source = StoreSource{}

func (s StoreSource) Snapshot(ctx context.Context, accountID harnessmodel.ProviderAccountID) (Snapshot, error) {
	if s.Store == nil {
		return Snapshot{}, fmt.Errorf("session broker store is required")
	}
	if accountID == "" {
		return Snapshot{}, fmt.Errorf("provider account id is required")
	}

	out := Snapshot{Health: harnessmodel.ProviderHealthUnknown}
	err := s.Store.View(ctx, func(reader harnessstore.Reader) error {
		account, err := reader.GetProviderAccount(ctx, accountID)
		if err != nil {
			return err
		}
		models, err := reader.ListProviderModels(ctx, accountID)
		if err != nil {
			return err
		}
		sessions, err := reader.ListProviderSessions(ctx, accountID)
		if err != nil {
			return err
		}
		capacity, capacityErr := reader.GetLatestProviderCapacity(ctx, accountID)
		if capacityErr != nil && !errors.Is(capacityErr, harnessstore.ErrNotFound) {
			return capacityErr
		}

		out.Account = account
		out.Models = models
		out.Sessions = sessions
		if capacityErr == nil {
			if capacity.AccountID != account.ID || capacity.Provider != account.Provider {
				return fmt.Errorf("provider capacity is inconsistent with account %s", account.ID)
			}
			if !capacity.Health.Valid() {
				return fmt.Errorf("invalid provider capacity health %q", capacity.Health)
			}
			out.Health = capacity.Health
		}
		return nil
	})
	if err != nil {
		return Snapshot{}, err
	}
	return out, nil
}

// Broker loads the snapshot and applies the pure deterministic policy.
type Broker struct {
	Source Source
	Policy Policy
}

func (b Broker) Decide(ctx context.Context, accountID harnessmodel.ProviderAccountID, request Request) (Decision, error) {
	if b.Source == nil {
		return Decision{}, fmt.Errorf("session broker source is required")
	}
	if err := b.Policy.Validate(); err != nil {
		return Decision{}, fmt.Errorf("invalid session broker policy: %w", err)
	}
	if err := request.Validate(); err != nil {
		return Decision{}, err
	}
	snapshot, err := b.Source.Snapshot(ctx, accountID)
	if err != nil {
		return Decision{}, err
	}
	return Evaluate(snapshot, request, b.Policy)
}

// Evaluate contains no provider-specific heuristics. In particular, it never
// infers a Codex thread->model association: only ProviderSessionSnapshot rows
// with an authoritative ModelID can become REUSE candidates. When a provider
// exposes no such sessions, an otherwise usable active account yields NEW.
func Evaluate(snapshot Snapshot, request Request, policy Policy) (Decision, error) {
	if err := policy.Validate(); err != nil {
		return Decision{}, fmt.Errorf("invalid session broker policy: %w", err)
	}
	if err := request.Validate(); err != nil {
		return Decision{}, err
	}
	if err := validateSnapshot(snapshot); err != nil {
		return Decision{}, err
	}

	if snapshot.Account.State == harnessmodel.ProviderAccountDisabled {
		return unavailable(ReasonAccountDisabled), nil
	}
	if snapshot.Health == harnessmodel.ProviderHealthUnavailable {
		return unavailable(ReasonProviderUnavailable), nil
	}
	if snapshot.Health == harnessmodel.ProviderHealthExhausted {
		return unavailable(ReasonProviderExhausted), nil
	}

	model, found := findModel(snapshot.Models, request.ModelID)
	if !found {
		return unavailable(ReasonModelNotFound), nil
	}
	if !model.Enabled {
		return unavailable(ReasonModelDisabled), nil
	}
	if !hasCapabilities(model.Capabilities, request.RequiredCapabilities) {
		return unavailable(ReasonCapabilityMismatch), nil
	}
	if model.ContextLimit > 0 && request.RequiredContext > model.ContextLimit {
		return unavailable(ReasonModelContextTooSmall), nil
	}

	candidates := make([]candidate, 0, len(snapshot.Sessions))
	rotationRequired := false
	for _, session := range snapshot.Sessions {
		if session.ModelID != request.ModelID || session.State == harnessmodel.ProviderSessionClosed {
			continue
		}
		if request.WorkspaceFingerprint != "" && session.WorkspaceFingerprint != request.WorkspaceFingerprint {
			continue
		}

		// A same-affinity non-closed session means prior context exists. If it
		// cannot be reused safely, rotate through CHECKPOINT_AND_NEW rather than
		// silently abandoning that context.
		rotationRequired = true
		if session.State != harnessmodel.ProviderSessionActive || session.ContextLimit <= 0 {
			continue
		}

		headroom := session.ContextLimit - session.ContextUsed
		if headroom < request.RequiredContext {
			continue
		}
		fraction := float64(headroom) / float64(session.ContextLimit)
		threshold := policy.AcquireReuseHeadroomFraction
		preferred := request.PreferredSessionID != "" && session.ID == request.PreferredSessionID
		if preferred {
			threshold = policy.RetainReuseHeadroomFraction
		}
		if fraction < threshold {
			continue
		}
		candidates = append(candidates, candidate{
			session:   session,
			headroom:  headroom,
			fraction:  fraction,
			preferred: preferred,
		})
	}

	if len(candidates) > 0 {
		sort.Slice(candidates, func(i, j int) bool {
			left, right := candidates[i], candidates[j]
			if left.preferred != right.preferred {
				return left.preferred
			}
			if left.fraction != right.fraction {
				return left.fraction > right.fraction
			}
			if left.headroom != right.headroom {
				return left.headroom > right.headroom
			}
			if !left.session.LastUsedAt.Equal(right.session.LastUsedAt) {
				return left.session.LastUsedAt.After(right.session.LastUsedAt)
			}
			return left.session.ID < right.session.ID
		})
		selected := candidates[0]
		reason := ReasonReusableSession
		if selected.preferred && selected.fraction < policy.AcquireReuseHeadroomFraction {
			reason = ReasonRetainedByHysteresis
		}
		return Decision{
			Action:           ActionReuse,
			SessionID:        selected.session.ID,
			Headroom:         selected.headroom,
			HeadroomFraction: selected.fraction,
			Reasons:          []Reason{reason},
		}, nil
	}

	if snapshot.Account.State == harnessmodel.ProviderAccountDraining {
		return unavailable(ReasonAccountDraining), nil
	}
	if rotationRequired {
		return Decision{Action: ActionCheckpointAndNew, Reasons: []Reason{ReasonContextRotation}}, nil
	}
	return Decision{Action: ActionNew, Reasons: []Reason{ReasonNoReusableSession}}, nil
}

type candidate struct {
	session   harnessmodel.ProviderSessionSnapshot
	headroom  int64
	fraction  float64
	preferred bool
}

func validateSnapshot(snapshot Snapshot) error {
	account := snapshot.Account
	if account.ID == "" || !account.Provider.Valid() || !account.State.Valid() {
		return fmt.Errorf("invalid provider account snapshot")
	}
	if !snapshot.Health.Valid() {
		return fmt.Errorf("invalid provider health %q", snapshot.Health)
	}

	modelIDs := make(map[harnessmodel.ProviderModelID]struct{}, len(snapshot.Models))
	for i, model := range snapshot.Models {
		if model.ID == "" || model.AccountID != account.ID || model.Provider != account.Provider {
			return fmt.Errorf("provider model %d is inconsistent with account %s", i, account.ID)
		}
		if model.ContextLimit < 0 {
			return fmt.Errorf("provider model %s context limit must be non-negative", model.ID)
		}
		if _, exists := modelIDs[model.ID]; exists {
			return fmt.Errorf("duplicate provider model %s", model.ID)
		}
		modelIDs[model.ID] = struct{}{}
	}

	sessionIDs := make(map[harnessmodel.ProviderSessionID]struct{}, len(snapshot.Sessions))
	for i, providerSession := range snapshot.Sessions {
		if err := providerSession.Validate(); err != nil {
			return fmt.Errorf("provider session %d: %w", i, err)
		}
		if providerSession.AccountID != account.ID || providerSession.Provider != account.Provider {
			return fmt.Errorf("provider session %s is inconsistent with account %s", providerSession.ID, account.ID)
		}
		if _, exists := sessionIDs[providerSession.ID]; exists {
			return fmt.Errorf("duplicate provider session %s", providerSession.ID)
		}
		sessionIDs[providerSession.ID] = struct{}{}
	}
	return nil
}

func findModel(models []harnessmodel.ProviderModelDescriptor, id harnessmodel.ProviderModelID) (harnessmodel.ProviderModelDescriptor, bool) {
	for _, model := range models {
		if model.ID == id {
			return model, true
		}
	}
	return harnessmodel.ProviderModelDescriptor{}, false
}

func hasCapabilities(actual, required []string) bool {
	if len(required) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(actual))
	for _, capability := range actual {
		set[capability] = struct{}{}
	}
	for _, capability := range required {
		if _, ok := set[capability]; !ok {
			return false
		}
	}
	return true
}

func unavailable(reason Reason) Decision {
	return Decision{Action: ActionUnavailable, Reasons: []Reason{reason}}
}
