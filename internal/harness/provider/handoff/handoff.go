package handoff

import (
	"context"
	"errors"
	"fmt"
	"time"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/provider/fault"
	"github.com/homiakus/agctl/internal/harness/provider/selector"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	"github.com/homiakus/agctl/internal/harness/task"
)

var (
	// ErrHandoffUnsafeInDoubt indicates the attempt has in-doubt side effects that could not be proved safe.
	ErrHandoffUnsafeInDoubt = errors.New("provider handoff rejected: attempt has uncertain side effects requiring reconciliation")
	// ErrNoActiveAssignment indicates the attempt has no active provider assignment to hand off.
	ErrNoActiveAssignment = errors.New("provider handoff failed: no active provider assignment found")
	// ErrNoViableReplacement indicates no eligible provider candidate passed hard and soft filters for handoff.
	ErrNoViableReplacement = errors.New("provider handoff failed: no viable replacement provider candidate available")
	// ErrInvalidHandoffRequest indicates required fields are missing or invalid in handoff request.
	ErrInvalidHandoffRequest = errors.New("invalid handoff request")
)

// HandoffRequest specifies the parameters for safe provider handoff.
type HandoffRequest struct {
	Envelope          harnessmodel.TaskEnvelope
	PlanText          []byte
	PriorAssignmentID harnessmodel.ProviderAssignmentID
	Fault             *fault.Classification
	Reconciler        harnessexecutor.EffectReconciler
	ExcludeAccountIDs []harnessmodel.ProviderAccountID
	ExcludeModelIDs   []harnessmodel.ProviderModelID
	Reason            string
}

// Validate checks the handoff request.
func (r HandoffRequest) Validate() error {
	if err := r.Envelope.Validate(); err != nil {
		return fmt.Errorf("%w: envelope invalid: %v", ErrInvalidHandoffRequest, err)
	}
	if r.PriorAssignmentID == "" {
		return fmt.Errorf("%w: prior assignment ID is required", ErrInvalidHandoffRequest)
	}
	if len(r.PlanText) == 0 {
		return fmt.Errorf("%w: plan text is required for plan consistency verification", ErrInvalidHandoffRequest)
	}
	return nil
}

// HandoffResult records the complete outcome of an atomic safe provider handoff.
type HandoffResult struct {
	PriorAssignment        harnessmodel.ProviderAssignment
	PriorReservation       *harnessmodel.ProviderReservation
	ReplacementAssignment  harnessmodel.ProviderAssignment
	ReplacementReservation *harnessmodel.ProviderReservation
	Decision               selector.Decision
	SelectedCandidate      selector.CandidateEvaluation
	ReconciledEffects      []harnessmodel.EffectIntent
	HandoffAt              time.Time
}

// Options configures the handoff manager.
type Options struct {
	Policy         selector.Policy
	ReservationTTL time.Duration
	Now            func() time.Time
	IDGen          harnessmodel.IDGenerator
}

func (o *Options) normalize() {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.ReservationTTL <= 0 {
		o.ReservationTTL = 15 * time.Minute
	}
	if o.IDGen == nil {
		o.IDGen = harnessmodel.NewIDGenerator()
	}
	if o.Policy.Validate() != nil {
		o.Policy = selector.DefaultPolicy()
	}
}

// HandoffManager coordinates safe atomic provider handoff across failures and retries.
type HandoffManager struct {
	store harnessstore.Store
	opts  Options
}

// NewManager creates a new HandoffManager.
func NewManager(store harnessstore.Store, opts Options) *HandoffManager {
	opts.normalize()
	return &HandoffManager{
		store: store,
		opts:  opts,
	}
}

// CheckEffectSafety inspects in-flight effect intents attached to an attempt and verifies or reconciles them.
func (m *HandoffManager) CheckEffectSafety(
	ctx context.Context,
	tx harnessstore.Tx,
	attemptID harnessmodel.AttemptID,
	reconciler harnessexecutor.EffectReconciler,
	now time.Time,
) ([]harnessmodel.EffectIntent, error) {
	effects, err := tx.ListEffectIntentsByAttempt(ctx, attemptID, 100)
	if err != nil {
		return nil, fmt.Errorf("list effect intents by attempt: %w", err)
	}

	var reconciled []harnessmodel.EffectIntent

	for _, eff := range effects {
		switch eff.State {
		case harnessmodel.EffectConfirmed, harnessmodel.EffectPrepared:
			// Confirmed or prepared (not dispatched) are safe
			continue
		case harnessmodel.EffectFailed:
			// Already terminalized failed
			continue
		case harnessmodel.EffectDispatched, harnessmodel.EffectInDoubt:
			if eff.Class.BlindRetrySafe() {
				// Pure read/idempotent get effects are blind-retry safe
				continue
			}

			// Non-idempotent or key-dependent effect in doubt -> requires reconciliation
			if reconciler == nil {
				return nil, fmt.Errorf("%w: effect %s (%s/%s) is %s and not blind-retry safe; no reconciler provided",
					ErrHandoffUnsafeInDoubt, eff.ID, eff.OperationNamespace, eff.Operation, eff.State)
			}

			req := harnessexecutor.EffectReconcileRequest{
				EffectIntentID:      eff.ID,
				WorkflowRunID:       eff.WorkflowRunID,
				NodeRunID:           eff.NodeRunID,
				OperationNamespace:  eff.OperationNamespace,
				Operation:           eff.Operation,
				Class:               eff.Class,
				IdempotencyKey:      eff.IdempotencyKey,
				SemanticInputDigest: eff.SemanticInputDigest,
				ProviderRef:         eff.ProviderRef,
			}
			if vErr := req.Validate(); vErr != nil {
				return nil, fmt.Errorf("effect reconcile request invalid: %w", vErr)
			}

			pRes, rErr := reconciler.ReconcileEffect(ctx, req)
			if rErr != nil {
				return nil, fmt.Errorf("%w: effect %s reconciliation error: %v", ErrHandoffUnsafeInDoubt, eff.ID, rErr)
			}
			if vErr := pRes.Validate(); vErr != nil {
				return nil, fmt.Errorf("effect reconcile provider result invalid: %w", vErr)
			}

			updated := eff
			updated.ResolvedAt = now

			switch pRes.Status {
			case harnessexecutor.EffectReconcileConfirmed:
				updated.State = harnessmodel.EffectConfirmed
				updated.ProviderRef = pRes.ProviderRef
				updated.ResultDigest = pRes.ResultDigest
				if err := tx.CompareAndSwapEffectIntent(ctx, eff.State, updated); err != nil {
					return nil, fmt.Errorf("CAS reconcile confirmed effect: %w", err)
				}
				reconciled = append(reconciled, updated)
			case harnessexecutor.EffectReconcileAbsent:
				updated.State = harnessmodel.EffectFailed
				updated.ErrorClass = "effect_absent"
				updated.ErrorMessage = "provider reconciliation proved effect absent"
				if err := tx.CompareAndSwapEffectIntent(ctx, eff.State, updated); err != nil {
					return nil, fmt.Errorf("CAS reconcile absent effect: %w", err)
				}
				reconciled = append(reconciled, updated)
			case harnessexecutor.EffectReconcileFailed:
				updated.State = harnessmodel.EffectFailed
				updated.ErrorClass = pRes.ErrorClass
				updated.ErrorMessage = pRes.ErrorMessage
				if err := tx.CompareAndSwapEffectIntent(ctx, eff.State, updated); err != nil {
					return nil, fmt.Errorf("CAS reconcile failed effect: %w", err)
				}
				reconciled = append(reconciled, updated)
			case harnessexecutor.EffectReconcileUnknown:
				return nil, fmt.Errorf("%w: effect %s reconciliation status UNKNOWN", ErrHandoffUnsafeInDoubt, eff.ID)
			default:
				return nil, fmt.Errorf("%w: unsupported effect reconciliation status %s", ErrHandoffUnsafeInDoubt, pRes.Status)
			}
		}
	}

	return reconciled, nil
}

// Handoff executes an atomic safe provider handoff for an attempt.
func (m *HandoffManager) Handoff(ctx context.Context, req HandoffRequest) (HandoffResult, error) {
	if err := req.Validate(); err != nil {
		return HandoffResult{}, err
	}

	// 1. Verify plan consistency (invariant I-030)
	if err := task.CheckPlanDrift(req.Envelope, req.PlanText); err != nil {
		return HandoffResult{}, fmt.Errorf("handoff plan verification failed: %w", err)
	}

	now := m.opts.Now().UTC()
	selReq := task.ToSelectorRequest(req.Envelope)

	// Build exclusion filters
	excludes := make(map[harnessmodel.ProviderAccountID]bool)
	for _, accID := range req.ExcludeAccountIDs {
		excludes[accID] = true
	}
	modelExcludes := make(map[harnessmodel.ProviderModelID]bool)
	for _, modID := range req.ExcludeModelIDs {
		modelExcludes[modID] = true
	}

	var result HandoffResult

	err := m.store.Update(ctx, func(tx harnessstore.Tx) error {
		// 2. Load and verify prior assignment
		priorAsn, err := tx.GetProviderAssignment(ctx, req.PriorAssignmentID)
		if err != nil {
			return fmt.Errorf("get prior provider assignment %s: %w", req.PriorAssignmentID, err)
		}
		if priorAsn.State != harnessmodel.ProviderAssignmentActive {
			return fmt.Errorf("%w: prior assignment %s is in state %s, not ACTIVE",
				ErrNoActiveAssignment, priorAsn.ID, priorAsn.State)
		}

		// Exclude prior account by default during handoff
		excludes[priorAsn.AccountID] = true

		// 3. Check and reconcile uncertain effect intents for this attempt (invariant I-007)
		reconciledEffects, err := m.CheckEffectSafety(ctx, tx, priorAsn.AttemptID, req.Reconciler, now)
		if err != nil {
			return err
		}

		// 4. Release prior reservation if active (invariant I-024)
		// Must be released BEFORE assignment transitions to SUPERSEDED
		var priorRes *harnessmodel.ProviderReservation
		resList, err := tx.ListProviderReservationsByAssignment(ctx, priorAsn.ID)
		if err != nil {
			return fmt.Errorf("list prior reservations: %w", err)
		}
		for _, r := range resList {
			if r.State == harnessmodel.ProviderReservationActive {
				releasedRes := r
				releasedRes.State = harnessmodel.ProviderReservationReleased
				releasedRes.Revision = r.Revision + 1
				releasedRes.UpdatedAt = now
				if err := tx.CompareAndSwapProviderReservation(ctx, r.Revision, releasedRes); err != nil {
					return fmt.Errorf("CAS release prior reservation %s: %w", r.ID, err)
				}
				priorRes = &releasedRes
				break
			}
		}

		// 5. Supersede prior assignment (invariant I-023)
		supersededAsn := priorAsn
		supersededAsn.State = harnessmodel.ProviderAssignmentSuperseded
		supersededAsn.Revision = priorAsn.Revision + 1
		supersededAsn.UpdatedAt = now

		if err := tx.CompareAndSwapProviderAssignment(ctx, priorAsn.Revision, supersededAsn); err != nil {
			return fmt.Errorf("CAS supersede prior assignment: %w", err)
		}

		// 6. Select replacement candidate via selector
		storeSource := selector.StoreSource{Store: m.store}
		decision, err := storeSource.Select(ctx, selReq, now, m.opts.Policy)
		if err != nil {
			return fmt.Errorf("selector evaluation failed: %w", err)
		}

		var winner *selector.CandidateEvaluation
		for i := range decision.Evaluations {
			cand := &decision.Evaluations[i]
			if cand.Eliminated {
				continue
			}
			if excludes[cand.AccountID] || modelExcludes[cand.ModelID] {
				continue
			}
			winner = cand
			break
		}

		if winner == nil {
			return fmt.Errorf("%w: selector rationale: %s", ErrNoViableReplacement, decision.Rationale)
		}

		// 7. Create replacement assignment in ACTIVE state
		newAsnIDRaw, err := m.opts.IDGen.New(harnessmodel.IDProviderAssignment)
		if err != nil {
			return fmt.Errorf("generate replacement assignment ID: %w", err)
		}
		newAsnID := harnessmodel.ProviderAssignmentID(newAsnIDRaw)

		newAsn := harnessmodel.ProviderAssignment{
			ID:         newAsnID,
			AttemptID:  priorAsn.AttemptID,
			AccountID:  winner.AccountID,
			ModelID:    winner.ModelID,
			SessionID:  winner.SessionDecision.SessionID,
			PlanDigest: req.Envelope.PlanDigest,
			State:      harnessmodel.ProviderAssignmentActive,
			Revision:   1,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := tx.CreateProviderAssignment(ctx, newAsn); err != nil {
			return fmt.Errorf("create replacement provider assignment: %w", err)
		}

		// 8. Create replacement reservation in ACTIVE state
		var newRes *harnessmodel.ProviderReservation
		if req.Envelope.MaxTokens > 0 && winner.NormalizedCapacity != nil {
			for _, w := range winner.NormalizedCapacity.Windows {
				if (w.Metric == harnessmodel.QuotaMetricTokens || w.Metric == harnessmodel.QuotaMetricFraction) && w.ID != "" {
					resIDRaw, err := m.opts.IDGen.New(harnessmodel.IDProviderReservation)
					if err != nil {
						return fmt.Errorf("generate replacement reservation ID: %w", err)
					}
					amount := float64(req.Envelope.MaxTokens)
					if w.Metric == harnessmodel.QuotaMetricFraction {
						amount = 0.05
					}
					reservation := harnessmodel.ProviderReservation{
						ID:           harnessmodel.ProviderReservationID(resIDRaw),
						AssignmentID: newAsnID,
						AccountID:    winner.AccountID,
						WindowID:     w.ID,
						ModelID:      winner.ModelID,
						Metric:       w.Metric,
						Amount:       amount,
						State:        harnessmodel.ProviderReservationActive,
						Revision:     1,
						CreatedAt:    now,
						ExpiresAt:    now.Add(m.opts.ReservationTTL),
						UpdatedAt:    now,
					}
					if err := tx.CreateProviderReservation(ctx, reservation); err != nil {
						return fmt.Errorf("create replacement provider reservation: %w", err)
					}
					newRes = &reservation
					break
				}
			}
		}

		result = HandoffResult{
			PriorAssignment:        supersededAsn,
			PriorReservation:       priorRes,
			ReplacementAssignment:  newAsn,
			ReplacementReservation: newRes,
			Decision:               decision,
			SelectedCandidate:      *winner,
			ReconciledEffects:      reconciledEffects,
			HandoffAt:              now,
		}
		return nil
	})

	if err != nil {
		return HandoffResult{}, err
	}
	return result, nil
}
