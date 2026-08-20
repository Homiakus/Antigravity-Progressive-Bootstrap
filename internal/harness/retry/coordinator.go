package retry

import (
	"context"
	"errors"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

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
	now := c.now().UTC()
	var result BreakerDecision
	err := c.store.Update(ctx, func(tx harnessstore.Tx) error {
		breaker, err := tx.GetCircuitBreaker(ctx, serviceKey)
		if errors.Is(err, harnessstore.ErrNotFound) {
			breaker = harnessmodel.CircuitBreaker{
				ServiceKey: serviceKey, State: harnessmodel.CircuitClosed,
				FailureThreshold: c.policy.FailureThreshold, UpdatedAt: now,
			}
			if err := tx.UpsertCircuitBreaker(ctx, breaker); err != nil {
				return err
			}
			result = BreakerDecision{Breaker: breaker, Allow: true, Reason: "circuit initialized closed"}
			return nil
		}
		if err != nil {
			return err
		}
		// Threshold is operational policy; updating it does not rewrite history.
		breaker.FailureThreshold = c.policy.FailureThreshold
		decision, err := Allow(breaker, now)
		if err != nil {
			return err
		}
		if decision.Breaker.State != breaker.State || decision.Breaker.ProbeInFlight != breaker.ProbeInFlight || decision.Breaker.FailureThreshold != breaker.FailureThreshold {
			if err := tx.CompareAndSwapCircuitBreaker(ctx, breaker.State, breaker.ProbeInFlight, decision.Breaker); err != nil {
				return err
			}
		}
		result = decision
		return nil
	})
	return result, err
}

func (c *CircuitCoordinator) RecordFailure(ctx context.Context, serviceKey string) (harnessmodel.CircuitBreaker, error) {
	if serviceKey == "" {
		return harnessmodel.CircuitBreaker{}, fmt.Errorf("service key is required")
	}
	now := c.now().UTC()
	var result harnessmodel.CircuitBreaker
	err := c.store.Update(ctx, func(tx harnessstore.Tx) error {
		breaker, err := tx.GetCircuitBreaker(ctx, serviceKey)
		if errors.Is(err, harnessstore.ErrNotFound) {
			breaker = harnessmodel.CircuitBreaker{ServiceKey: serviceKey, State: harnessmodel.CircuitClosed, FailureThreshold: c.policy.FailureThreshold, UpdatedAt: now}
			updated, err := RecordFailure(breaker, c.policy, now)
			if err != nil {
				return err
			}
			if err := tx.UpsertCircuitBreaker(ctx, updated); err != nil {
				return err
			}
			result = updated
			return nil
		}
		if err != nil {
			return err
		}
		expectedState, expectedProbe := breaker.State, breaker.ProbeInFlight
		updated, err := RecordFailure(breaker, c.policy, now)
		if err != nil {
			return err
		}
		if err := tx.CompareAndSwapCircuitBreaker(ctx, expectedState, expectedProbe, updated); err != nil {
			return err
		}
		result = updated
		return nil
	})
	return result, err
}

func (c *CircuitCoordinator) RecordSuccess(ctx context.Context, serviceKey string) (harnessmodel.CircuitBreaker, error) {
	if serviceKey == "" {
		return harnessmodel.CircuitBreaker{}, fmt.Errorf("service key is required")
	}
	now := c.now().UTC()
	var result harnessmodel.CircuitBreaker
	err := c.store.Update(ctx, func(tx harnessstore.Tx) error {
		breaker, err := tx.GetCircuitBreaker(ctx, serviceKey)
		if errors.Is(err, harnessstore.ErrNotFound) {
			breaker = harnessmodel.CircuitBreaker{ServiceKey: serviceKey, State: harnessmodel.CircuitClosed, FailureThreshold: c.policy.FailureThreshold, UpdatedAt: now}
			if err := tx.UpsertCircuitBreaker(ctx, breaker); err != nil {
				return err
			}
			result = breaker
			return nil
		}
		if err != nil {
			return err
		}
		expectedState, expectedProbe := breaker.State, breaker.ProbeInFlight
		updated, err := RecordSuccess(breaker, now)
		if err != nil {
			return err
		}
		updated.FailureThreshold = c.policy.FailureThreshold
		if err := tx.CompareAndSwapCircuitBreaker(ctx, expectedState, expectedProbe, updated); err != nil {
			return err
		}
		result = updated
		return nil
	})
	return result, err
}
