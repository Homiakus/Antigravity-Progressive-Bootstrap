package sqlite

import (
	"context"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

// ListAllActiveProviderReservations is the correctness-oriented reservation
// read used by future feasibility/settlement logic. Unlike the paged diagnostic
// listing it is deliberately unbounded: truncating the active set would
// undercount already-claimed provider capacity and could permit oversubscription.
func (t *transaction) ListAllActiveProviderReservations(ctx context.Context, accountID harnessmodel.ProviderAccountID, now time.Time) ([]harnessmodel.ProviderReservation, error) {
	if accountID == "" || now.IsZero() {
		return nil, fmt.Errorf("provider account id and current time are required")
	}
	nowNS, err := checkedUnixNano(now)
	if err != nil {
		return nil, fmt.Errorf("invalid provider reservation current time: %w", err)
	}
	rows, err := t.tx.QueryContext(ctx, providerReservationSelect+`
WHERE account_id=? AND state='ACTIVE' AND expires_at_ns>?
ORDER BY expires_at_ns, id`, string(accountID), nowNS)
	if err != nil {
		return nil, fmt.Errorf("list complete active provider reservations: %w", err)
	}
	defer rows.Close()
	return scanProviderReservations(rows)
}
