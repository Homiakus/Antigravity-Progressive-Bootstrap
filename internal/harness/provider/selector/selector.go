package selector

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/provider/capacity"
	"github.com/homiakus/agctl/internal/harness/provider/demand"
	"github.com/homiakus/agctl/internal/harness/provider/session"
)

const (
	fractionEpsilon = 1e-9
)

// FilterReason is a stable, machine-readable explanation for candidate elimination.
type FilterReason string

const (
	FilterNone                     FilterReason = ""
	FilterAccountDisabled          FilterReason = "FILTER_ACCOUNT_DISABLED"
	FilterAccountDrainingNoSession FilterReason = "FILTER_ACCOUNT_DRAINING_NO_REUSABLE_SESSION"
	FilterProviderNotAllowed       FilterReason = "FILTER_PROVIDER_NOT_ALLOWED"
	FilterHealthUnavailable        FilterReason = "FILTER_HEALTH_UNAVAILABLE"
	FilterHealthExhausted          FilterReason = "FILTER_HEALTH_EXHAUSTED"
	FilterModelDisabled            FilterReason = "FILTER_MODEL_DISABLED"
	FilterCapabilityMismatch       FilterReason = "FILTER_CAPABILITY_MISMATCH"
	FilterContextTooSmall          FilterReason = "FILTER_CONTEXT_TOO_SMALL"
	FilterCircuitOpen              FilterReason = "FILTER_CIRCUIT_OPEN"
	FilterSessionUnavailable       FilterReason = "FILTER_SESSION_UNAVAILABLE"
	FilterInsufficientHeadroom     FilterReason = "FILTER_INSUFFICIENT_HEADROOM"
)

// Request defines the task requirements and caller preferences for provider selection.
type Request struct {
	TaskClass            string                         `json:"taskClass"`
	RepositoryID         string                         `json:"repositoryId,omitempty"`
	ContextClass         string                         `json:"contextClass,omitempty"`
	RequiredContext      int64                          `json:"requiredContext,omitempty"`
	RequiredCapabilities []string                       `json:"requiredCapabilities,omitempty"`
	AllowedProviders     []harnessmodel.ProviderKind    `json:"allowedProviders,omitempty"`
	WorkspaceFingerprint string                         `json:"workspaceFingerprint,omitempty"`
	PreferredSessionID   harnessmodel.ProviderSessionID `json:"preferredSessionId,omitempty"`
	PreferredProvider    harnessmodel.ProviderKind      `json:"preferredProvider,omitempty"`
	PreferredModelID     harnessmodel.ProviderModelID   `json:"preferredModelId,omitempty"`
}

func (r Request) Validate() error {
	if strings.TrimSpace(r.TaskClass) == "" {
		return fmt.Errorf("selector request task class is required")
	}
	if r.RequiredContext < 0 {
		return fmt.Errorf("selector request required context must be non-negative")
	}
	for i, c := range r.RequiredCapabilities {
		if strings.TrimSpace(c) == "" {
			return fmt.Errorf("selector request capability %d is empty", i)
		}
	}
	for i, p := range r.AllowedProviders {
		if !p.Valid() {
			return fmt.Errorf("selector request allowed provider %d (%q) is invalid", i, p)
		}
	}
	if r.PreferredProvider != "" && !r.PreferredProvider.Valid() {
		return fmt.Errorf("selector request preferred provider %q is invalid", r.PreferredProvider)
	}
	return nil
}

// Policy specifies weights, thresholds, and sub-policies governing evaluation.
type Policy struct {
	CapacityPolicy     capacity.Policy `json:"capacityPolicy"`
	SessionPolicy      session.Policy  `json:"sessionPolicy"`
	HeadroomWeight     float64         `json:"headroomWeight"`
	ResetHorizonWeight float64         `json:"resetHorizonWeight"`
	SessionReuseBonus  float64         `json:"sessionReuseBonus"`
	ReliabilityWeight  float64         `json:"reliabilityWeight"`
	SwitchPenalty      float64         `json:"switchPenalty"`
	UncertaintyPenalty float64         `json:"uncertaintyPenalty"`
}

func DefaultPolicy() Policy {
	return Policy{
		CapacityPolicy: capacity.ConservativePolicy(),
		SessionPolicy: session.Policy{
			AcquireReuseHeadroomFraction: 0.20,
			RetainReuseHeadroomFraction:  0.10,
		},
		HeadroomWeight:     0.35,
		ResetHorizonWeight: 0.15,
		SessionReuseBonus:  0.20,
		ReliabilityWeight:  0.15,
		SwitchPenalty:      0.10,
		UncertaintyPenalty: 0.05,
	}
}

func (p Policy) Validate() error {
	if err := p.CapacityPolicy.Validate(); err != nil {
		return fmt.Errorf("selector capacity policy: %w", err)
	}
	if err := p.SessionPolicy.Validate(); err != nil {
		return fmt.Errorf("selector session policy: %w", err)
	}
	if p.HeadroomWeight < 0 || p.ResetHorizonWeight < 0 || p.SessionReuseBonus < 0 || p.ReliabilityWeight < 0 {
		return fmt.Errorf("selector score weights must be non-negative")
	}
	if p.SwitchPenalty < 0 || p.UncertaintyPenalty < 0 {
		return fmt.Errorf("selector penalties must be non-negative")
	}
	positiveSum := p.HeadroomWeight + p.ResetHorizonWeight + p.SessionReuseBonus + p.ReliabilityWeight
	if positiveSum <= 0 {
		return fmt.Errorf("selector requires at least one positive score weight")
	}
	return nil
}

// Candidate represents an account/model candidate with its telemetry and state.
type Candidate struct {
	Account         harnessmodel.ProviderAccount
	Model           harnessmodel.ProviderModelDescriptor
	Capacity        *harnessmodel.ProviderCapacitySnapshot
	Sessions        []harnessmodel.ProviderSessionSnapshot
	Circuit         *harnessmodel.ProviderCircuitState
	DemandEstimates []demand.Estimate
}

// ScoreBreakdown details the explainable components of a candidate's soft score.
type ScoreBreakdown struct {
	HeadroomScore      float64 `json:"headroomScore"`
	ResetHorizonScore  float64 `json:"resetHorizonScore"`
	SessionReuseScore  float64 `json:"sessionReuseScore"`
	ReliabilityScore   float64 `json:"reliabilityScore"`
	SwitchPenalty      float64 `json:"switchPenalty"`
	UncertaintyPenalty float64 `json:"uncertaintyPenalty"`
	CompositeScore     float64 `json:"compositeScore"`
}

// CandidateEvaluation records the outcome for one candidate.
type CandidateEvaluation struct {
	AccountID          harnessmodel.ProviderAccountID `json:"accountId"`
	Provider           harnessmodel.ProviderKind      `json:"provider"`
	ModelID            harnessmodel.ProviderModelID   `json:"modelId"`
	Eliminated         bool                           `json:"eliminated"`
	EliminationReason  FilterReason                   `json:"eliminationReason,omitempty"`
	EliminationDetail  string                         `json:"eliminationDetail,omitempty"`
	SessionDecision    session.Decision               `json:"sessionDecision"`
	NormalizedCapacity *capacity.Summary              `json:"normalizedCapacity,omitempty"`
	Score              ScoreBreakdown                 `json:"score"`
}

// Decision represents the complete, explainable selector output.
type Decision struct {
	Selected    *CandidateEvaluation  `json:"selected,omitempty"`
	Evaluations []CandidateEvaluation `json:"evaluations"`
	Rationale   string                `json:"rationale"`
	DecidedAt   time.Time             `json:"decidedAt"`
}

// Evaluate is a pure, deterministic multi-candidate evaluation function.
func Evaluate(ctx context.Context, req Request, candidates []Candidate, now time.Time, policy Policy) (Decision, error) {
	if err := req.Validate(); err != nil {
		return Decision{}, fmt.Errorf("selector request invalid: %w", err)
	}
	if err := policy.Validate(); err != nil {
		return Decision{}, fmt.Errorf("selector policy invalid: %w", err)
	}
	if now.IsZero() {
		return Decision{}, fmt.Errorf("selector decision time is required")
	}
	if err := ctx.Err(); err != nil {
		return Decision{}, err
	}

	evaluations := make([]CandidateEvaluation, 0, len(candidates))
	for _, c := range candidates {
		eval := evaluateCandidate(c, req, now, policy)
		evaluations = append(evaluations, eval)
	}

	// Filter down to eligible candidates for ranking
	eligible := make([]*CandidateEvaluation, 0, len(evaluations))
	for i := range evaluations {
		if !evaluations[i].Eliminated {
			eligible = append(eligible, &evaluations[i])
		}
	}

	// Deterministic sorting of eligible candidates
	sort.SliceStable(eligible, func(i, j int) bool {
		return isBetterCandidate(eligible[i], eligible[j], req)
	})

	var selected *CandidateEvaluation
	if len(eligible) > 0 {
		selected = eligible[0]
	}

	rationale := buildRationale(selected, evaluations, len(eligible))

	return Decision{
		Selected:    selected,
		Evaluations: evaluations,
		Rationale:   rationale,
		DecidedAt:   now.UTC(),
	}, nil
}

func evaluateCandidate(c Candidate, req Request, now time.Time, policy Policy) CandidateEvaluation {
	eval := CandidateEvaluation{
		AccountID: c.Account.ID,
		Provider:  c.Account.Provider,
		ModelID:   c.Model.ID,
	}

	// 1. Evaluate session/context broker for this candidate
	sessionSnap := session.Snapshot{
		Account:  c.Account,
		Models:   []harnessmodel.ProviderModelDescriptor{c.Model},
		Sessions: c.Sessions,
		Health:   harnessmodel.ProviderHealthUnknown,
	}
	if c.Capacity != nil {
		sessionSnap.Health = c.Capacity.Health
	}

	sessionReq := session.Request{
		ModelID:              c.Model.ID,
		RequiredCapabilities: req.RequiredCapabilities,
		WorkspaceFingerprint: req.WorkspaceFingerprint,
		RequiredContext:      req.RequiredContext,
		PreferredSessionID:   req.PreferredSessionID,
	}
	sessionDec, err := session.Evaluate(sessionSnap, sessionReq, policy.SessionPolicy)
	if err == nil {
		eval.SessionDecision = sessionDec
	} else {
		eval.SessionDecision = session.Decision{
			Action:  session.ActionUnavailable,
			Reasons: []session.Reason{session.ReasonModelNotFound},
		}
	}

	// 2. Hard filter: AllowedProviders
	if len(req.AllowedProviders) > 0 {
		allowed := false
		for _, p := range req.AllowedProviders {
			if c.Account.Provider == p {
				allowed = true
				break
			}
		}
		if !allowed {
			eval.Eliminated = true
			eval.EliminationReason = FilterProviderNotAllowed
			eval.EliminationDetail = fmt.Sprintf("provider %s not in allowed list", c.Account.Provider)
			return eval
		}
	}

	// 3. Hard filter: Account state
	if c.Account.State == harnessmodel.ProviderAccountDisabled {
		eval.Eliminated = true
		eval.EliminationReason = FilterAccountDisabled
		eval.EliminationDetail = fmt.Sprintf("account %s is disabled", c.Account.ID)
		return eval
	}
	if c.Account.State == harnessmodel.ProviderAccountDraining {
		if eval.SessionDecision.Action != session.ActionReuse {
			eval.Eliminated = true
			eval.EliminationReason = FilterAccountDrainingNoSession
			eval.EliminationDetail = fmt.Sprintf("account %s is draining and has no reusable session", c.Account.ID)
			return eval
		}
	}

	// 4. Hard filter: Capacity health
	if c.Capacity != nil {
		if c.Capacity.Health == harnessmodel.ProviderHealthUnavailable {
			eval.Eliminated = true
			eval.EliminationReason = FilterHealthUnavailable
			eval.EliminationDetail = "provider capacity health is unavailable"
			return eval
		}
		if c.Capacity.Health == harnessmodel.ProviderHealthExhausted {
			eval.Eliminated = true
			eval.EliminationReason = FilterHealthExhausted
			eval.EliminationDetail = "provider capacity health is exhausted"
			return eval
		}
	}

	// 5. Hard filter: Model enabled
	if !c.Model.Enabled {
		eval.Eliminated = true
		eval.EliminationReason = FilterModelDisabled
		eval.EliminationDetail = fmt.Sprintf("model %s is disabled", c.Model.ID)
		return eval
	}

	// 6. Hard filter: Required capabilities
	capMap := make(map[string]bool, len(c.Model.Capabilities))
	for _, cap := range c.Model.Capabilities {
		capMap[cap] = true
	}
	for _, reqCap := range req.RequiredCapabilities {
		if !capMap[reqCap] {
			eval.Eliminated = true
			eval.EliminationReason = FilterCapabilityMismatch
			eval.EliminationDetail = fmt.Sprintf("model %s lacks required capability %s", c.Model.ID, reqCap)
			return eval
		}
	}

	// 7. Hard filter: Context limit
	if req.RequiredContext > 0 && c.Model.ContextLimit > 0 && c.Model.ContextLimit < req.RequiredContext {
		eval.Eliminated = true
		eval.EliminationReason = FilterContextTooSmall
		eval.EliminationDetail = fmt.Sprintf("model context limit %d < required context %d", c.Model.ContextLimit, req.RequiredContext)
		return eval
	}

	// 8. Hard filter: Circuit breaker open
	if c.Circuit != nil && c.Circuit.State == harnessmodel.CircuitOpen {
		eval.Eliminated = true
		eval.EliminationReason = FilterCircuitOpen
		eval.EliminationDetail = fmt.Sprintf("circuit breaker is open (consecutive failures: %d)", c.Circuit.ConsecutiveFailures)
		return eval
	}

	// 9. Hard filter: Session broker unavailable
	if eval.SessionDecision.Action == session.ActionUnavailable {
		eval.Eliminated = true
		eval.EliminationReason = FilterSessionUnavailable
		eval.EliminationDetail = fmt.Sprintf("session broker returned unavailable: %v", eval.SessionDecision.Reasons)
		return eval
	}

	// 10. Normalize capacity if snapshot is present
	var norm *capacity.Summary
	if c.Capacity != nil {
		n, err := capacity.Normalize(*c.Capacity, now, policy.CapacityPolicy)
		if err == nil {
			norm = &n
			eval.NormalizedCapacity = norm
		}
	}

	// Check evidence state
	if norm != nil {
		if norm.State == capacity.EvidenceUnavailable {
			eval.Eliminated = true
			eval.EliminationReason = FilterHealthUnavailable
			eval.EliminationDetail = "normalized capacity state is unavailable"
			return eval
		}
		if norm.State == capacity.EvidenceExhausted {
			eval.Eliminated = true
			eval.EliminationReason = FilterHealthExhausted
			eval.EliminationDetail = "normalized capacity state is exhausted"
			return eval
		}

		// Check headroom feasibility against demand estimates
		for _, est := range c.DemandEstimates {
			if est.Key.Provider == c.Account.Provider && est.Key.ModelID == c.Model.ID {
				for _, w := range norm.Windows {
					if (w.ModelID == "" || w.ModelID == c.Model.ID) && w.Metric == est.Key.Metric {
						if w.EffectiveRemaining != nil && *w.EffectiveRemaining < est.Reservation {
							eval.Eliminated = true
							eval.EliminationReason = FilterInsufficientHeadroom
							eval.EliminationDetail = fmt.Sprintf("window %s effective remaining %v < demand %v", w.ID, *w.EffectiveRemaining, est.Reservation)
							return eval
						}
						if w.EffectiveFraction != nil && est.Reservation > 0 && *w.EffectiveFraction <= 0 {
							eval.Eliminated = true
							eval.EliminationReason = FilterInsufficientHeadroom
							eval.EliminationDetail = fmt.Sprintf("window %s effective fraction is zero for positive demand", w.ID)
							return eval
						}
					}
				}
			}
		}
	}

	// Soft scoring for eligible candidate
	eval.Score = computeScore(c, eval, req, norm, now, policy)
	return eval
}

func computeScore(c Candidate, eval CandidateEvaluation, req Request, norm *capacity.Summary, now time.Time, policy Policy) ScoreBreakdown {
	var breakdown ScoreBreakdown

	// 1. Headroom score in [0, 1]
	if norm != nil && norm.HeadroomFraction != nil {
		breakdown.HeadroomScore = math.Max(0, math.Min(1, *norm.HeadroomFraction))
	} else {
		// Unknown or non-fractional headroom is given neutral 0.5, penalized under uncertainty
		breakdown.HeadroomScore = 0.5
	}

	// 2. Reset horizon score in [0, 1]
	if norm != nil && norm.EarliestResetAt != nil {
		timeUntilReset := norm.EarliestResetAt.Sub(now)
		if timeUntilReset <= 0 {
			breakdown.ResetHorizonScore = 1.0
		} else {
			// Window resetting sooner with positive headroom gets higher score
			hours := timeUntilReset.Hours()
			breakdown.ResetHorizonScore = 1.0 / (1.0 + hours/12.0)
		}
	} else {
		breakdown.ResetHorizonScore = 0.5
	}

	// 3. Session reuse bonus
	if eval.SessionDecision.Action == session.ActionReuse {
		breakdown.SessionReuseScore = 1.0
	} else {
		breakdown.SessionReuseScore = 0.0
	}

	// 4. Reliability score
	health := harnessmodel.ProviderHealthUnknown
	if c.Capacity != nil {
		health = c.Capacity.Health
	}
	switch health {
	case harnessmodel.ProviderHealthHealthy:
		breakdown.ReliabilityScore = 1.0
	case harnessmodel.ProviderHealthDegraded:
		breakdown.ReliabilityScore = 0.6
	default:
		breakdown.ReliabilityScore = 0.4
	}
	if c.Circuit != nil && c.Circuit.State == harnessmodel.CircuitHalfOpen {
		breakdown.ReliabilityScore *= 0.5
	}

	// 5. Switch penalty
	switchPenalty := 0.0
	if req.PreferredProvider != "" && c.Account.Provider != req.PreferredProvider {
		switchPenalty += 0.5
	}
	if req.PreferredModelID != "" && c.Model.ID != req.PreferredModelID {
		switchPenalty += 0.5
	}
	breakdown.SwitchPenalty = math.Min(1.0, switchPenalty)

	// 6. Uncertainty penalty
	uncertainty := 0.0
	if norm == nil || norm.State == capacity.EvidenceUnknown || norm.State == capacity.EvidenceStale {
		uncertainty += 0.5
	}
	for _, est := range c.DemandEstimates {
		if est.Key.Provider == c.Account.Provider && est.Key.ModelID == c.Model.ID {
			if est.Source == demand.SourceColdStart || est.Source == demand.SourceProviderMetric {
				uncertainty += 0.25
			}
		}
	}
	breakdown.UncertaintyPenalty = math.Min(1.0, uncertainty)

	// Composite weighted score
	posWeights := policy.HeadroomWeight + policy.ResetHorizonWeight + policy.SessionReuseBonus + policy.ReliabilityWeight
	weightedPositive := (policy.HeadroomWeight * breakdown.HeadroomScore) +
		(policy.ResetHorizonWeight * breakdown.ResetHorizonScore) +
		(policy.SessionReuseBonus * breakdown.SessionReuseScore) +
		(policy.ReliabilityWeight * breakdown.ReliabilityScore)

	normalizedPositive := weightedPositive / posWeights
	composite := normalizedPositive - (policy.SwitchPenalty * breakdown.SwitchPenalty) - (policy.UncertaintyPenalty * breakdown.UncertaintyPenalty)
	breakdown.CompositeScore = math.Max(0.0, math.Min(1.0, composite))

	return breakdown
}

// isBetterCandidate implements strict deterministic ordering:
// 1. Composite score descending.
// 2. Preferred session match.
// 3. Session Action priority: REUSE > NEW > CHECKPOINT_AND_NEW.
// 4. Headroom fraction descending.
// 5. Lexical ProviderKind ("ANTIGRAVITY" < "CODEX").
// 6. Lexical AccountID.
// 7. Lexical ModelID.
// 8. Lexical SessionID.
func isBetterCandidate(a, b *CandidateEvaluation, req Request) bool {
	scoreDiff := a.Score.CompositeScore - b.Score.CompositeScore
	if math.Abs(scoreDiff) > fractionEpsilon {
		return scoreDiff > 0
	}

	// Preferred session match
	aPref := req.PreferredSessionID != "" && a.SessionDecision.SessionID == req.PreferredSessionID
	bPref := req.PreferredSessionID != "" && b.SessionDecision.SessionID == req.PreferredSessionID
	if aPref != bPref {
		return aPref
	}

	// Action priority
	aPrio := actionPriority(a.SessionDecision.Action)
	bPrio := actionPriority(b.SessionDecision.Action)
	if aPrio != bPrio {
		return aPrio > bPrio
	}

	// Headroom fraction
	aHead := getHeadroomFraction(a.NormalizedCapacity)
	bHead := getHeadroomFraction(b.NormalizedCapacity)
	if math.Abs(aHead-bHead) > fractionEpsilon {
		return aHead > bHead
	}

	// Lexical provider
	if a.Provider != b.Provider {
		return a.Provider < b.Provider
	}

	// Lexical account ID
	if a.AccountID != b.AccountID {
		return a.AccountID < b.AccountID
	}

	// Lexical model ID
	if a.ModelID != b.ModelID {
		return a.ModelID < b.ModelID
	}

	// Lexical session ID
	return a.SessionDecision.SessionID < b.SessionDecision.SessionID
}

func actionPriority(a session.Action) int {
	switch a {
	case session.ActionReuse:
		return 3
	case session.ActionNew:
		return 2
	case session.ActionCheckpointAndNew:
		return 1
	default:
		return 0
	}
}

func getHeadroomFraction(norm *capacity.Summary) float64 {
	if norm != nil && norm.HeadroomFraction != nil {
		return *norm.HeadroomFraction
	}
	return 0
}

func buildRationale(selected *CandidateEvaluation, evaluations []CandidateEvaluation, eligibleCount int) string {
	if selected == nil {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("No eligible provider candidate found out of %d evaluated.\n", len(evaluations)))
		for _, e := range evaluations {
			sb.WriteString(fmt.Sprintf("- %s/%s/%s: rejected with %s: %s\n", e.Provider, e.AccountID, e.ModelID, e.EliminationReason, e.EliminationDetail))
		}
		return sb.String()
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Selected %s on account %s (model %s) with score %.4f (eligible: %d/%d).\n",
		selected.Provider, selected.AccountID, selected.ModelID, selected.Score.CompositeScore, eligibleCount, len(evaluations)))
	sb.WriteString(fmt.Sprintf("Score breakdown: headroom=%.3f, reset=%.3f, sessionReuse=%.3f, reliability=%.3f, switchPenalty=%.3f, uncertaintyPenalty=%.3f.\n",
		selected.Score.HeadroomScore, selected.Score.ResetHorizonScore, selected.Score.SessionReuseScore,
		selected.Score.ReliabilityScore, selected.Score.SwitchPenalty, selected.Score.UncertaintyPenalty))
	sb.WriteString(fmt.Sprintf("Session action: %s (session %s, reasons: %v).\n",
		selected.SessionDecision.Action, selected.SessionDecision.SessionID, selected.SessionDecision.Reasons))

	if len(evaluations) > 1 {
		sb.WriteString("Other candidates:\n")
		for _, e := range evaluations {
			if e.AccountID == selected.AccountID && e.ModelID == selected.ModelID {
				continue
			}
			if e.Eliminated {
				sb.WriteString(fmt.Sprintf("- %s/%s/%s: eliminated (%s: %s)\n", e.Provider, e.AccountID, e.ModelID, e.EliminationReason, e.EliminationDetail))
			} else {
				sb.WriteString(fmt.Sprintf("- %s/%s/%s: score %.4f (action: %s)\n", e.Provider, e.AccountID, e.ModelID, e.Score.CompositeScore, e.SessionDecision.Action))
			}
		}
	}
	return sb.String()
}
