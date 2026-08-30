package fault

import (
	"context"
	"errors"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

// CircuitManager manages provider/account/model-scoped circuit breakers in the durable store.
type CircuitManager struct {
	Store harnessstore.Store
}

// NewCircuitManager creates a new provider circuit manager.
func NewCircuitManager(store harnessstore.Store) *CircuitManager {
	return &CircuitManager{Store: store}
}

// RecordSuccess resets failure counts and closes the circuit on a successful execution.
func (m *CircuitManager) RecordSuccess(ctx context.Context, accountID harnessmodel.ProviderAccountID, modelID harnessmodel.ProviderModelID, now time.Time) error {
	if m.Store == nil {
		return fmt.Errorf("circuit manager store is nil")
	}
	if accountID == "" {
		return fmt.Errorf("accountID is required")
	}
	now = now.UTC()

	return m.Store.Update(ctx, func(tx harnessstore.Tx) error {
		existing, err := tx.GetProviderCircuitState(ctx, accountID, modelID)
		if errors.Is(err, harnessstore.ErrNotFound) {
			// No existing circuit record needed for fresh healthy state
			return nil
		}
		if err != nil {
			return fmt.Errorf("get provider circuit state: %w", err)
		}

		if existing.ConsecutiveFailures == 0 && existing.State == harnessmodel.CircuitClosed {
			return nil // already clean
		}

		updated := existing
		updated.State = harnessmodel.CircuitClosed
		updated.ConsecutiveFailures = 0
		updated.OpenedAt = time.Time{}
		updated.NextProbeAt = time.Time{}
		updated.ProbeInFlight = false
		updated.UpdatedAt = now
		updated.Revision = existing.Revision + 1

		if err := tx.CompareAndSwapProviderCircuitState(ctx, existing.Revision, updated); err != nil {
			return fmt.Errorf("CAS provider circuit state on success: %w", err)
		}
		return nil
	})
}

// RecordFailure records a consecutive failure, tripping the circuit to OPEN if threshold is exceeded.
func (m *CircuitManager) RecordFailure(ctx context.Context, accountID harnessmodel.ProviderAccountID, modelID harnessmodel.ProviderModelID, threshold int, cooldown time.Duration, now time.Time) (harnessmodel.ProviderCircuitState, error) {
	if m.Store == nil {
		return harnessmodel.ProviderCircuitState{}, fmt.Errorf("circuit manager store is nil")
	}
	if accountID == "" {
		return harnessmodel.ProviderCircuitState{}, fmt.Errorf("accountID is required")
	}
	if threshold < 1 {
		threshold = 3
	}
	if cooldown <= 0 {
		cooldown = 5 * time.Minute
	}
	now = now.UTC()

	var result harnessmodel.ProviderCircuitState

	err := m.Store.Update(ctx, func(tx harnessstore.Tx) error {
		existing, err := tx.GetProviderCircuitState(ctx, accountID, modelID)
		if errors.Is(err, harnessstore.ErrNotFound) {
			state := harnessmodel.CircuitClosed
			var openedAt, nextProbeAt time.Time
			if threshold <= 1 {
				state = harnessmodel.CircuitOpen
				openedAt = now
				nextProbeAt = now.Add(cooldown)
			}
			initial := harnessmodel.ProviderCircuitState{
				AccountID:           accountID,
				ModelID:             modelID,
				Revision:            1,
				State:               state,
				ConsecutiveFailures: 1,
				OpenedAt:            openedAt,
				NextProbeAt:         nextProbeAt,
				ProbeInFlight:       false,
				UpdatedAt:           now,
			}
			if err := tx.CreateProviderCircuitState(ctx, initial); err != nil {
				return fmt.Errorf("create initial provider circuit state: %w", err)
			}
			result = initial
			return nil
		}
		if err != nil {
			return fmt.Errorf("get provider circuit state: %w", err)
		}

		updated := existing
		updated.Revision = existing.Revision + 1
		updated.ConsecutiveFailures = existing.ConsecutiveFailures + 1
		updated.UpdatedAt = now
		updated.ProbeInFlight = false

		if updated.ConsecutiveFailures >= threshold || existing.State == harnessmodel.CircuitHalfOpen || existing.State == harnessmodel.CircuitOpen {
			updated.State = harnessmodel.CircuitOpen
			updated.OpenedAt = now
			updated.NextProbeAt = now.Add(cooldown)
		}

		if err := tx.CompareAndSwapProviderCircuitState(ctx, existing.Revision, updated); err != nil {
			return fmt.Errorf("CAS provider circuit state on failure: %w", err)
		}
		result = updated
		return nil
	})

	if err != nil {
		return harnessmodel.ProviderCircuitState{}, err
	}
	return result, nil
}
