package retry

import (
	"context"
	"errors"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

var ErrStaleCircuitTicket = errors.New("stale circuit breaker ticket")

const maxCircuitCASRetries = 32

type CircuitCoordinator struct {
	store  harnessstore.Store
	policy BreakerPolicy
	now    func() time.Time
}

func NewCircuitCoordinator(store harnessstore.Store, policy BreakerPolicy, now func() time.Time) (*CircuitCoordinator, error) {
	if store == nil {
		return nil, fmt.Errorf("harness store is required")
	}
	if policy.FailureThreshold < 1 || policy.Cooldown <= 0 {
		return nil, fmt.Errorf("valid circuit breaker threshold and cooldown are required")
	}
	if now == nil {
		now = time.Now
	}
	return &CircuitCoordinator{store: store, policy: policy, now: now}, nil
}

func (c *CircuitCoordinator) Acquire(ctx context.Context, serviceKey string) (BreakerDecision, error) {
	if serviceKey == "" {
		return BreakerDecision{}, fmt.Errorf("service key is required")
	}
	for attempt := 0; attempt < maxCircuitCASRetries; attempt++ {
		now := c.now().UTC()
		var result BreakerDecision
		err := c.store.Update(ctx, func(tx harnessstore.Tx) error {
			breaker, err := tx.GetCircuitBreaker(ctx, serviceKey)
			if errors.Is(err, harnessstore.ErrNotFound) {
				breaker = harnessmodel.CircuitBreaker{
					ServiceKey: serviceKey, Revision: 1, State: harnessmodel.CircuitClosed,
					FailureThreshold: c.policy.FailureThreshold, UpdatedAt: now,
				}
				if err := tx.CreateCircuitBreaker(ctx, breaker); err != nil {
					return err
				}
				result = BreakerDecision{
					Breaker: breaker,
					Ticket:  CircuitTicket{ServiceKey: serviceKey, Revision: breaker.Revision},
					Allow:   true,
					Reason:  "circuit initialized closed",
				}
				return nil
			}
			if err != nil {
				return err
			}

			expectedRevision := breaker.Revision
			decision, err := Allow(breaker, now)
			if err != nil {
				return err
			}
			thresholdChanged := decision.Breaker.FailureThreshold != c.policy.FailureThreshold
			decision.Breaker.FailureThreshold = c.policy.FailureThreshold
			mutated := thresholdChanged || decision.Breaker.State != breaker.State || decision.Breaker.ProbeInFlight != breaker.ProbeInFlight
			if mutated {
				decision.Breaker.Revision = expectedRevision + 1
				decision.Breaker.UpdatedAt = now
				if err := tx.CompareAndSwapCircuitBreaker(ctx, expectedRevision, decision.Breaker); err != nil {
					return err
				}
			} else {
				decision.Breaker.Revision = expectedRevision
			}
			if decision.Allow {
				decision.Ticket = CircuitTicket{ServiceKey: serviceKey, Revision: decision.Breaker.Revision, Probe: decision.Probe}
			}
			result = decision
			return nil
		})
		if errors.Is(err, harnessstore.ErrConflict) {
			continue
		}
		return result, err
	}
	return BreakerDecision{}, fmt.Errorf("circuit acquire exceeded CAS retry limit: %w", harnessstore.ErrConflict)
}

// RecordFailure records a result for a call previously admitted by Acquire.
// Normal failures are serialized while CLOSED so concurrent failures are never
// lost. Once OPEN protection is active, late failures from pre-open calls are
// ignored rather than extending the cooldown. HALF_OPEN accepts only its probe
// ticket.
func (c *CircuitCoordinator) RecordFailure(ctx context.Context, ticket CircuitTicket) (harnessmodel.CircuitBreaker, error) {
	if err := validateCircuitTicket(ticket); err != nil {
		return harnessmodel.CircuitBreaker{}, err
	}
	for attempt := 0; attempt < maxCircuitCASRetries; attempt++ {
		now := c.now().UTC()
		var result harnessmodel.CircuitBreaker
		err := c.store.Update(ctx, func(tx harnessstore.Tx) error {
			breaker, err := tx.GetCircuitBreaker(ctx, ticket.ServiceKey)
			if err != nil {
				if errors.Is(err, harnessstore.ErrNotFound) {
					return ErrStaleCircuitTicket
				}
				return err
			}
			if ticket.Revision > breaker.Revision {
				return ErrStaleCircuitTicket
			}

			if ticket.Probe {
				if ticket.Revision != breaker.Revision || breaker.State != harnessmodel.CircuitHalfOpen || !breaker.ProbeInFlight {
					return ErrStaleCircuitTicket
				}
			} else if breaker.State != harnessmodel.CircuitClosed {
				// The breaker already advanced to a protective state. This is a
				// legitimate late result from a call admitted while CLOSED; do not
				// restart cooldown or interfere with the half-open owner.
				result = breaker
				return nil
			}

			expectedRevision := breaker.Revision
			updated, err := RecordFailure(breaker, c.policy, now)
			if err != nil {
				return err
			}
			updated.Revision = expectedRevision + 1
			if err := tx.CompareAndSwapCircuitBreaker(ctx, expectedRevision, updated); err != nil {
				return err
			}
			result = updated
			return nil
		})
		if errors.Is(err, harnessstore.ErrConflict) {
			continue
		}
		return result, err
	}
	return harnessmodel.CircuitBreaker{}, fmt.Errorf("circuit failure record exceeded CAS retry limit: %w", harnessstore.ErrConflict)
}

// RecordSuccess records a result for a call previously admitted by Acquire.
// Successes may reset consecutive failures only while the breaker is still
// CLOSED. A stale success from a pre-open call cannot close OPEN. HALF_OPEN can
// close only when the exact durable probe ticket reports success.
func (c *CircuitCoordinator) RecordSuccess(ctx context.Context, ticket CircuitTicket) (harnessmodel.CircuitBreaker, error) {
	if err := validateCircuitTicket(ticket); err != nil {
		return harnessmodel.CircuitBreaker{}, err
	}
	for attempt := 0; attempt < maxCircuitCASRetries; attempt++ {
		now := c.now().UTC()
		var result harnessmodel.CircuitBreaker
		err := c.store.Update(ctx, func(tx harnessstore.Tx) error {
			breaker, err := tx.GetCircuitBreaker(ctx, ticket.ServiceKey)
			if err != nil {
				if errors.Is(err, harnessstore.ErrNotFound) {
					return ErrStaleCircuitTicket
				}
				return err
			}
			if ticket.Revision > breaker.Revision {
				return ErrStaleCircuitTicket
			}

			if ticket.Probe {
				if ticket.Revision != breaker.Revision || breaker.State != harnessmodel.CircuitHalfOpen || !breaker.ProbeInFlight {
					return ErrStaleCircuitTicket
				}
			} else if breaker.State != harnessmodel.CircuitClosed {
				// A call admitted before OPEN may finish successfully later. It is
				// stale evidence and must not close protection established by newer
				// failures or steal a HALF_OPEN probe.
				result = breaker
				return nil
			}

			expectedRevision := breaker.Revision
			updated, err := RecordSuccess(breaker, now)
			if err != nil {
				return err
			}
			updated.FailureThreshold = c.policy.FailureThreshold
			updated.Revision = expectedRevision + 1
			if err := tx.CompareAndSwapCircuitBreaker(ctx, expectedRevision, updated); err != nil {
				return err
			}
			result = updated
			return nil
		})
		if errors.Is(err, harnessstore.ErrConflict) {
			continue
		}
		return result, err
	}
	return harnessmodel.CircuitBreaker{}, fmt.Errorf("circuit success record exceeded CAS retry limit: %w", harnessstore.ErrConflict)
}

func validateCircuitTicket(ticket CircuitTicket) error {
	if ticket.ServiceKey == "" || ticket.Revision == 0 {
		return fmt.Errorf("valid circuit ticket is required")
	}
	return nil
}
