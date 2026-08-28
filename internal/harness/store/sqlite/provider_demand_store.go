package sqlite

import (
	"context"
	"errors"
	"fmt"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

var _ harnessstore.ProviderDemandReader = (*transaction)(nil)
var _ harnessstore.ProviderDemandTx = (*transaction)(nil)

func (t *transaction) GetProviderDemandDimensions(ctx context.Context, usageKey string) (harnessmodel.ProviderDemandDimensions, error) {
	if usageKey == "" {
		return harnessmodel.ProviderDemandDimensions{}, fmt.Errorf("provider demand usage key is required")
	}
	var d harnessmodel.ProviderDemandDimensions
	if err := t.tx.QueryRowContext(ctx, `
SELECT usage_key, task_class, repository_class, context_class
FROM provider_demand_dimensions WHERE usage_key=?`, usageKey).Scan(&d.UsageKey, &d.TaskClass, &d.RepositoryClass, &d.ContextClass); err != nil {
		return harnessmodel.ProviderDemandDimensions{}, mapNotFound(err)
	}
	return d, nil
}

func (t *transaction) PutProviderDemandDimensions(ctx context.Context, d harnessmodel.ProviderDemandDimensions) (harnessmodel.ProviderDemandDimensions, bool, error) {
	if err := d.Validate(); err != nil {
		return harnessmodel.ProviderDemandDimensions{}, false, err
	}
	usage, err := t.GetProviderUsageSample(ctx, d.UsageKey)
	if err != nil {
		return harnessmodel.ProviderDemandDimensions{}, false, err
	}
	if usage.ModelID == "" {
		return harnessmodel.ProviderDemandDimensions{}, false, fmt.Errorf("provider demand dimensions require authoritative usage model id: %w", harnessstore.ErrConflict)
	}
	if usage.ReservationID == "" {
		return harnessmodel.ProviderDemandDimensions{}, false, fmt.Errorf("provider demand dimensions require settled reservation-backed usage: %w", harnessstore.ErrConflict)
	}
	reservation, err := t.GetProviderReservation(ctx, usage.ReservationID)
	if err != nil {
		return harnessmodel.ProviderDemandDimensions{}, false, err
	}
	if reservation.State != harnessmodel.ProviderReservationSettled {
		return harnessmodel.ProviderDemandDimensions{}, false, fmt.Errorf("provider demand reservation %s is %s, want SETTLED: %w", reservation.ID, reservation.State, harnessstore.ErrConflict)
	}
	observedNS, err := checkedUnixNano(usage.ObservedAt)
	if err != nil {
		return harnessmodel.ProviderDemandDimensions{}, false, fmt.Errorf("provider demand observation time is outside durable range: %w", err)
	}
	res, err := t.tx.ExecContext(ctx, `
INSERT INTO provider_demand_dimensions(
    usage_key, assignment_id, metric, task_class, repository_class, context_class, usage_observed_at_ns
) VALUES(?,?,?,?,?,?,?) ON CONFLICT DO NOTHING`, d.UsageKey, string(usage.AssignmentID), string(usage.Metric), d.TaskClass, d.RepositoryClass, d.ContextClass, observedNS)
	if err != nil {
		return harnessmodel.ProviderDemandDimensions{}, false, fmt.Errorf("insert provider demand dimensions %q: %w", d.UsageKey, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return harnessmodel.ProviderDemandDimensions{}, false, fmt.Errorf("read provider demand dimension insert count: %w", err)
	}
	if n == 1 {
		return d, true, nil
	}
	existing, err := t.GetProviderDemandDimensions(ctx, d.UsageKey)
	if err == nil {
		if existing != d {
			return existing, false, fmt.Errorf("provider demand usage key %q was replayed with different dimensions: %w", d.UsageKey, harnessstore.ErrConflict)
		}
		return existing, false, nil
	}
	if !errors.Is(err, harnessstore.ErrNotFound) {
		return harnessmodel.ProviderDemandDimensions{}, false, err
	}
	var existingUsageKey string
	if err := t.tx.QueryRowContext(ctx, `
SELECT usage_key FROM provider_demand_dimensions WHERE assignment_id=? AND metric=?`, string(usage.AssignmentID), string(usage.Metric)).Scan(&existingUsageKey); err != nil {
		return harnessmodel.ProviderDemandDimensions{}, false, fmt.Errorf("resolve canonical provider demand conflict: %w", err)
	}
	return harnessmodel.ProviderDemandDimensions{}, false, fmt.Errorf("provider assignment %s metric %s already has canonical demand sample %q: %w", usage.AssignmentID, usage.Metric, existingUsageKey, harnessstore.ErrConflict)
}

func (t *transaction) ListProviderDemandHistory(ctx context.Context, q harnessmodel.ProviderDemandHistoryQuery) ([]harnessmodel.ProviderDemandSample, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}
	sinceNS, err := checkedUnixNano(q.Since)
	if err != nil {
		return nil, fmt.Errorf("provider demand history since is outside durable range: %w", err)
	}
	rows, err := t.tx.QueryContext(ctx, `
SELECT u.sample_key, u.account_id, a.provider, u.model_id, u.metric, u.amount,
       d.task_class, d.repository_class, d.context_class, u.observed_at
FROM provider_demand_dimensions d
JOIN provider_usage_samples u ON u.sample_key=d.usage_key
JOIN provider_accounts a ON a.id=u.account_id
WHERE a.provider=? AND u.model_id=? AND u.metric=? AND d.usage_observed_at_ns>=?
  AND (?='' OR d.task_class=?)
  AND (?='' OR d.repository_class=?)
  AND (?='' OR d.context_class=?)
ORDER BY d.usage_observed_at_ns DESC, u.sample_key DESC
LIMIT ?`, string(q.Provider), string(q.ModelID), string(q.Metric), sinceNS,
		q.TaskClass, q.TaskClass, q.RepositoryClass, q.RepositoryClass, q.ContextClass, q.ContextClass, q.Limit)
	if err != nil {
		return nil, fmt.Errorf("list provider demand history: %w", err)
	}
	defer rows.Close()
	out := make([]harnessmodel.ProviderDemandSample, 0)
	for rows.Next() {
		var s harnessmodel.ProviderDemandSample
		var accountID, provider, modelID, metric, observedAt string
		if err := rows.Scan(&s.UsageKey, &accountID, &provider, &modelID, &metric, &s.Amount,
			&s.TaskClass, &s.RepositoryClass, &s.ContextClass, &observedAt); err != nil {
			return nil, fmt.Errorf("scan provider demand history: %w", err)
		}
		s.AccountID = harnessmodel.ProviderAccountID(accountID)
		s.Provider = harnessmodel.ProviderKind(provider)
		s.ModelID = harnessmodel.ProviderModelID(modelID)
		s.Metric = harnessmodel.QuotaMetricKind(metric)
		if s.ObservedAt, err = parseTime(observedAt); err != nil {
			return nil, fmt.Errorf("parse provider demand observed_at: %w", err)
		}
		if err := s.Validate(); err != nil {
			return nil, fmt.Errorf("invalid provider demand history row %q: %w", s.UsageKey, err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider demand history: %w", err)
	}
	return out, nil
}
