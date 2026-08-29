package sqlite

import (
	"context"
	"fmt"
	"math"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

var _ harnessstore.ActiveProviderReservationAggregator = (*transaction)(nil)

func (t *transaction) ListActiveProviderReservationTotalsByWindow(
	ctx context.Context,
	accountID harnessmodel.ProviderAccountID,
	windowID string,
	activeAt time.Time,
) ([]harnessstore.ActiveProviderReservationTotal, error) {
	if accountID == "" || windowID == "" || activeAt.IsZero() {
		return nil, fmt.Errorf("provider reservation aggregate account, window and activeAt are required")
	}
	activeAtNS, err := checkedUnixNano(activeAt)
	if err != nil {
		return nil, fmt.Errorf("invalid provider reservation aggregate active time: %w", err)
	}
	rows, err := t.tx.QueryContext(ctx, `
SELECT model_id, metric, SUM(amount), COUNT(*)
FROM provider_reservations
WHERE account_id=?
  AND state='ACTIVE'
  AND expires_at_ns>?
  AND window_id=?
GROUP BY model_id, metric
ORDER BY model_id, metric`, string(accountID), activeAtNS, windowID)
	if err != nil {
		return nil, fmt.Errorf("query active provider reservation totals: %w", err)
	}
	defer rows.Close()

	var out []harnessstore.ActiveProviderReservationTotal
	for rows.Next() {
		var modelID string
		var metric string
		var amount float64
		var count int64
		if err := rows.Scan(&modelID, &metric, &amount, &count); err != nil {
			return nil, fmt.Errorf("scan active provider reservation total: %w", err)
		}
		kind := harnessmodel.QuotaMetricKind(metric)
		if !kind.Valid() || kind == harnessmodel.QuotaMetricOpaque || math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 || count <= 0 {
			return nil, fmt.Errorf("invalid active provider reservation aggregate window=%s model=%s metric=%s amount=%v count=%d", windowID, modelID, metric, amount, count)
		}
		out = append(out, harnessstore.ActiveProviderReservationTotal{
			WindowID: windowID,
			ModelID: harnessmodel.ProviderModelID(modelID),
			Metric: kind,
			Amount: amount,
			Count: count,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active provider reservation totals: %w", err)
	}
	return out, nil
}
