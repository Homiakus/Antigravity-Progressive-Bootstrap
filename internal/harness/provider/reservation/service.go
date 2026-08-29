package reservation

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/provider/capacity"
	"github.com/homiakus/agctl/internal/harness/provider/demand"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

var (
	ErrCapacityUnavailable = errors.New("provider reservation: capacity unavailable")
	ErrInsufficientCapacity = errors.New("provider reservation: insufficient capacity")
	ErrReservationConflict = errors.New("provider reservation: conflicting active assignment or claim")
)

// Policy controls the evidence freshness inherited from T-009 and the maximum
// lifetime of a newly-created claim. ClaimTTL is intentionally operational: it
// never invents provider quota semantics and is additionally capped by the
// selected window's reset/evidence horizon.
type Policy struct {
	Capacity capacity.Policy
	ClaimTTL time.Duration
}

func (p Policy) Validate() error {
	if err := p.Capacity.Validate(); err != nil {
		return err
	}
	if p.ClaimTTL <= 0 {
		return fmt.Errorf("provider reservation claim TTL must be positive")
	}
	return nil
}

// Request is an immutable reservation decision input. AssignmentID is supplied
// by the caller and acts as the durable idempotency identity for this routing
// generation. Reservation IDs are derived deterministically per quota window.
type Request struct {
	AssignmentID harnessmodel.ProviderAssignmentID
	AttemptID    harnessmodel.AttemptID
	AccountID    harnessmodel.ProviderAccountID
	ModelID      harnessmodel.ProviderModelID
	SessionID    harnessmodel.ProviderSessionID
	Demand       demand.Estimate
	DecisionAt   time.Time
}

// WindowClaim is the explainable result for one simultaneously-applicable quota
// window. A positive demand is reserved in every applicable window because one
// provider execution can consume both short and long rolling windows at once.
type WindowClaim struct {
	WindowID          string
	ModelID           harnessmodel.ProviderModelID
	Metric            harnessmodel.QuotaMetricKind
	EffectiveCapacity float64
	AlreadyClaimed    float64
	Reserved          float64
	RemainingAfter    float64
	ExpiresAt         time.Time
	ReservationID     harnessmodel.ProviderReservationID
}

// Result is returned only after the Store.Update transaction commits.
type Result struct {
	Assignment   harnessmodel.ProviderAssignment
	Reservations []harnessmodel.ProviderReservation
	Claims       []WindowClaim
	Replayed     bool
}

type Service struct {
	Store  harnessstore.Store
	Policy Policy
}

// Reserve atomically validates provider evidence, accounts for the complete
// compatible ACTIVE claim set, and persists assignment+claims. All correctness
// reads and writes happen in the same Store.Update callback.
func (s Service) Reserve(ctx context.Context, req Request) (Result, error) {
	if s.Store == nil {
		return Result{}, fmt.Errorf("provider reservation store is required")
	}
	if err := s.Policy.Validate(); err != nil {
		return Result{}, err
	}
	if err := validateRequest(req); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	var result Result
	err := s.Store.Update(ctx, func(tx harnessstore.Tx) error {
		account, err := tx.GetProviderAccount(ctx, req.AccountID)
		if err != nil {
			return fmt.Errorf("read provider account: %w", err)
		}
		if account.State != harnessmodel.ProviderAccountActive {
			return fmt.Errorf("provider account %s is %s: %w", account.ID, account.State, ErrCapacityUnavailable)
		}
		if account.Provider != req.Demand.Key.Provider {
			return fmt.Errorf("demand provider %s does not match account provider %s: %w", req.Demand.Key.Provider, account.Provider, ErrReservationConflict)
		}
		if req.Demand.Key.ModelID != req.ModelID {
			return fmt.Errorf("demand model %s does not match assignment model %s: %w", req.Demand.Key.ModelID, req.ModelID, ErrReservationConflict)
		}
		if err := requireEnabledModel(ctx, tx, req.AccountID, req.ModelID); err != nil {
			return err
		}

		snapshot, err := tx.GetLatestProviderCapacity(ctx, req.AccountID)
		if err != nil {
			if errors.Is(err, harnessstore.ErrNotFound) {
				return fmt.Errorf("provider capacity not observed: %w", ErrCapacityUnavailable)
			}
			return fmt.Errorf("read provider capacity: %w", err)
		}
		if snapshot.Provider != account.Provider {
			return fmt.Errorf("capacity provider %s differs from account provider %s: %w", snapshot.Provider, account.Provider, ErrReservationConflict)
		}
		normalized, err := capacity.Normalize(snapshot, req.DecisionAt, s.Policy.Capacity)
		if err != nil {
			return fmt.Errorf("normalize provider capacity: %w", err)
		}
		if normalized.State == capacity.EvidenceUnavailable || normalized.State == capacity.EvidenceExhausted || normalized.State == capacity.EvidenceStale {
			return fmt.Errorf("provider capacity evidence state %s: %w", normalized.State, ErrCapacityUnavailable)
		}

		windows, err := applicableWindows(normalized, req.ModelID, req.Demand.Key.Metric)
		if err != nil {
			return err
		}

		activeAssignment, activeErr := tx.GetActiveProviderAssignment(ctx, req.AttemptID)
		if activeErr != nil && !errors.Is(activeErr, harnessstore.ErrNotFound) {
			return fmt.Errorf("read active provider assignment: %w", activeErr)
		}
		if activeErr == nil {
			return replayExisting(ctx, tx, req, windows, activeAssignment, &result)
		}
		// A terminal historical record with the same id makes the caller's
		// idempotency identity stale; never resurrect it as a new generation.
		if prior, err := tx.GetProviderAssignment(ctx, req.AssignmentID); err == nil {
			return fmt.Errorf("assignment id %s already exists in state %s: %w", prior.ID, prior.State, ErrReservationConflict)
		} else if !errors.Is(err, harnessstore.ErrNotFound) {
			return fmt.Errorf("read provider assignment identity: %w", err)
		}

		allClaims, err := tx.ListAllActiveProviderReservations(ctx, req.AccountID, req.DecisionAt)
		if err != nil {
			return fmt.Errorf("read complete active provider claims: %w", err)
		}
		claims, err := evaluateClaims(windows, allClaims, req.Demand.Reservation, req.DecisionAt, s.Policy)
		if err != nil {
			return err
		}

		assignment := harnessmodel.ProviderAssignment{
			ID: req.AssignmentID, AttemptID: req.AttemptID, AccountID: req.AccountID,
			ModelID: req.ModelID, SessionID: req.SessionID,
			State: harnessmodel.ProviderAssignmentActive, Revision: 1,
			CreatedAt: req.DecisionAt, UpdatedAt: req.DecisionAt,
		}
		if err := tx.CreateProviderAssignment(ctx, assignment); err != nil {
			if errors.Is(err, harnessstore.ErrConflict) {
				return fmt.Errorf("create provider assignment: %w", ErrReservationConflict)
			}
			return fmt.Errorf("create provider assignment: %w", err)
		}

		reservations := make([]harnessmodel.ProviderReservation, 0, len(claims))
		if req.Demand.Reservation > 0 {
			for i := range claims {
				claim := &claims[i]
				reservation := harnessmodel.ProviderReservation{
					ID: claim.ReservationID, AssignmentID: assignment.ID, AccountID: assignment.AccountID,
					WindowID: claim.WindowID, ModelID: claim.ModelID, Metric: claim.Metric,
					Amount: req.Demand.Reservation, State: harnessmodel.ProviderReservationActive, Revision: 1,
					CreatedAt: req.DecisionAt, ExpiresAt: claim.ExpiresAt, UpdatedAt: req.DecisionAt,
				}
				if err := tx.CreateProviderReservation(ctx, reservation); err != nil {
					return fmt.Errorf("create provider reservation for window %s: %w", claim.WindowID, err)
				}
				reservations = append(reservations, reservation)
			}
		}
		result = Result{Assignment: assignment, Reservations: reservations, Claims: claims}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func validateRequest(req Request) error {
	if req.AssignmentID == "" || req.AttemptID == "" || req.AccountID == "" || req.ModelID == "" {
		return fmt.Errorf("provider reservation assignment, attempt, account and model ids are required")
	}
	if req.DecisionAt.IsZero() {
		return fmt.Errorf("provider reservation decision time is required")
	}
	if err := req.Demand.Key.Validate(); err != nil {
		return fmt.Errorf("validate demand key: %w", err)
	}
	if !req.Demand.Available {
		return fmt.Errorf("demand estimate is unavailable: %w", ErrCapacityUnavailable)
	}
	values := []float64{req.Demand.P50, req.Demand.P80, req.Demand.Reservation, req.Demand.Confidence}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return fmt.Errorf("demand estimate contains invalid numeric value")
		}
	}
	if req.Demand.P80 < req.Demand.P50 || req.Demand.Reservation != req.Demand.P80 {
		return fmt.Errorf("demand estimate violates p50/p80 reservation contract: %w", ErrReservationConflict)
	}
	if req.Demand.Confidence > 1 {
		return fmt.Errorf("demand confidence must be within [0,1]")
	}
	if req.Demand.Key.Metric == harnessmodel.QuotaMetricOpaque {
		return fmt.Errorf("OPAQUE demand cannot be reserved: %w", ErrCapacityUnavailable)
	}
	if req.Demand.Key.Metric == harnessmodel.QuotaMetricFraction && req.Demand.Reservation > 1 {
		return fmt.Errorf("fraction demand exceeds one: %w", ErrReservationConflict)
	}
	return nil
}

func requireEnabledModel(ctx context.Context, tx harnessstore.Reader, accountID harnessmodel.ProviderAccountID, modelID harnessmodel.ProviderModelID) error {
	models, err := tx.ListProviderModels(ctx, accountID)
	if err != nil {
		return fmt.Errorf("list provider models: %w", err)
	}
	for _, model := range models {
		if model.ID == modelID {
			if !model.Enabled {
				return fmt.Errorf("provider model %s is disabled: %w", modelID, ErrCapacityUnavailable)
			}
			return nil
		}
	}
	return fmt.Errorf("provider model %s is not in authoritative catalog: %w", modelID, ErrCapacityUnavailable)
}

func applicableWindows(summary capacity.Summary, modelID harnessmodel.ProviderModelID, metric harnessmodel.QuotaMetricKind) ([]capacity.Window, error) {
	var out []capacity.Window
	for _, window := range summary.Windows {
		if window.Metric != metric {
			continue
		}
		if window.ModelID != "" && window.ModelID != modelID {
			continue
		}
		if window.Expired {
			return nil, fmt.Errorf("quota window %s is expired: %w", window.ID, ErrCapacityUnavailable)
		}
		if _, ok := effectiveAvailable(window); !ok {
			return nil, fmt.Errorf("quota window %s has no reservable native headroom: %w", window.ID, ErrCapacityUnavailable)
		}
		out = append(out, window)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no applicable %s quota window for model %s: %w", metric, modelID, ErrCapacityUnavailable)
	}
	return out, nil
}

func effectiveAvailable(window capacity.Window) (float64, bool) {
	switch window.Metric {
	case harnessmodel.QuotaMetricTokens, harnessmodel.QuotaMetricRequests, harnessmodel.QuotaMetricCost:
		if window.EffectiveRemaining == nil {
			return 0, false
		}
		return *window.EffectiveRemaining, true
	case harnessmodel.QuotaMetricFraction:
		if window.EffectiveFraction == nil {
			return 0, false
		}
		return *window.EffectiveFraction, true
	default:
		return 0, false
	}
}

func evaluateClaims(windows []capacity.Window, reservations []harnessmodel.ProviderReservation, amount float64, now time.Time, policy Policy) ([]WindowClaim, error) {
	claims := make([]WindowClaim, 0, len(windows))
	for _, window := range windows {
		available, ok := effectiveAvailable(window)
		if !ok || math.IsNaN(available) || math.IsInf(available, 0) || available < 0 {
			return nil, fmt.Errorf("window %s has invalid effective headroom: %w", window.ID, ErrCapacityUnavailable)
		}
		claimed := 0.0
		for _, existing := range reservations {
			if existing.WindowID != window.ID {
				continue
			}
			// Window ID is provider-native identity. A different metric/model scope
			// for the same ID is contradictory evidence, not an unrelated claim.
			if existing.Metric != window.Metric || existing.ModelID != window.ModelID {
				return nil, fmt.Errorf("active claim %s contradicts window %s dimension: %w", existing.ID, window.ID, ErrReservationConflict)
			}
			if math.IsNaN(existing.Amount) || math.IsInf(existing.Amount, 0) || existing.Amount <= 0 {
				return nil, fmt.Errorf("active claim %s has invalid amount: %w", existing.ID, ErrReservationConflict)
			}
			claimed += existing.Amount
			if math.IsInf(claimed, 0) || math.IsNaN(claimed) {
				return nil, fmt.Errorf("claims for window %s overflow: %w", window.ID, ErrReservationConflict)
			}
		}
		remaining := available - claimed
		if remaining < 0 || amount > remaining {
			return nil, fmt.Errorf("window %s effective=%.12g claimed=%.12g demand=%.12g: %w", window.ID, available, claimed, amount, ErrInsufficientCapacity)
		}
		expiresAt, err := claimExpiry(window, now, policy)
		if err != nil {
			return nil, err
		}
		claims = append(claims, WindowClaim{
			WindowID: window.ID, ModelID: window.ModelID, Metric: window.Metric,
			EffectiveCapacity: available, AlreadyClaimed: claimed, Reserved: amount,
			RemainingAfter: remaining - amount, ExpiresAt: expiresAt,
			ReservationID: deterministicReservationID(reqIDContext{WindowID: window.ID, ModelID: window.ModelID, Metric: window.Metric}),
		})
	}
	return claims, nil
}

// reqIDContext is populated with the assignment id immediately before claims
// are persisted. Keeping ID derivation separate makes the dimension hash easy
// to mutation-test without leaking provider window text into identifiers.
type reqIDContext struct {
	AssignmentID harnessmodel.ProviderAssignmentID
	WindowID     string
	ModelID      harnessmodel.ProviderModelID
	Metric       harnessmodel.QuotaMetricKind
}

func deterministicReservationID(v reqIDContext) harnessmodel.ProviderReservationID {
	sum := sha256.Sum256([]byte(string(v.AssignmentID) + "\x00" + v.WindowID + "\x00" + string(v.ModelID) + "\x00" + string(v.Metric)))
	return harnessmodel.ProviderReservationID(fmt.Sprintf("pres_%x", sum[:16]))
}

func claimExpiry(window capacity.Window, now time.Time, policy Policy) (time.Time, error) {
	expiry := now.Add(policy.ClaimTTL)
	evidenceExpiry := window.ObservedAt.Add(policy.Capacity.ExpireAfter)
	if evidenceExpiry.Before(expiry) {
		expiry = evidenceExpiry
	}
	if window.ResetAt != nil && window.ResetAt.Before(expiry) {
		expiry = *window.ResetAt
	}
	if !expiry.After(now) {
		return time.Time{}, fmt.Errorf("quota window %s has no positive claim horizon: %w", window.ID, ErrCapacityUnavailable)
	}
	return expiry.UTC(), nil
}

func replayExisting(ctx context.Context, tx harnessstore.Reader, req Request, windows []capacity.Window, existing harnessmodel.ProviderAssignment, result *Result) error {
	if existing.ID != req.AssignmentID || existing.AccountID != req.AccountID || existing.ModelID != req.ModelID {
		return fmt.Errorf("attempt already has a different active provider assignment %s: %w", existing.ID, ErrReservationConflict)
	}
	if req.SessionID != "" && existing.SessionID != req.SessionID {
		return fmt.Errorf("active assignment session differs from replay request: %w", ErrReservationConflict)
	}
	reservations, err := tx.ListProviderReservationsByAssignment(ctx, existing.ID)
	if err != nil {
		return fmt.Errorf("read replay reservations: %w", err)
	}
	active := make(map[string]harnessmodel.ProviderReservation)
	for _, reservation := range reservations {
		if reservation.State == harnessmodel.ProviderReservationActive && reservation.ExpiresAt.After(req.DecisionAt) {
			key := reservationDimension(reservation.WindowID, reservation.ModelID, reservation.Metric)
			active[key] = reservation
		}
	}
	if req.Demand.Reservation == 0 {
		if len(active) != 0 {
			return fmt.Errorf("zero-demand replay has %d active claims: %w", len(active), ErrReservationConflict)
		}
		*result = Result{Assignment: existing, Replayed: true}
		return nil
	}
	if len(active) != len(windows) {
		return fmt.Errorf("active assignment has %d live claims, expected %d: %w", len(active), len(windows), ErrReservationConflict)
	}
	claims := make([]WindowClaim, 0, len(windows))
	matched := make([]harnessmodel.ProviderReservation, 0, len(windows))
	for _, window := range windows {
		key := reservationDimension(window.ID, window.ModelID, window.Metric)
		reservation, ok := active[key]
		if !ok || reservation.Amount != req.Demand.Reservation {
			return fmt.Errorf("active assignment claim for window %s differs from replay demand: %w", window.ID, ErrReservationConflict)
		}
		expectedID := deterministicReservationID(reqIDContext{AssignmentID: req.AssignmentID, WindowID: window.ID, ModelID: window.ModelID, Metric: window.Metric})
		if reservation.ID != expectedID {
			return fmt.Errorf("active assignment claim id for window %s is non-canonical: %w", window.ID, ErrReservationConflict)
		}
		available, _ := effectiveAvailable(window)
		claims = append(claims, WindowClaim{
			WindowID: window.ID, ModelID: window.ModelID, Metric: window.Metric,
			EffectiveCapacity: available, Reserved: reservation.Amount,
			ExpiresAt: reservation.ExpiresAt, ReservationID: reservation.ID,
		})
		matched = append(matched, reservation)
	}
	*result = Result{Assignment: existing, Reservations: matched, Claims: claims, Replayed: true}
	return nil
}

func reservationDimension(windowID string, modelID harnessmodel.ProviderModelID, metric harnessmodel.QuotaMetricKind) string {
	return windowID + "\x00" + string(modelID) + "\x00" + string(metric)
}
