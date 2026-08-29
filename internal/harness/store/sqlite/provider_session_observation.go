package sqlite

import (
	"context"
	"fmt"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

var _ harnessstore.ObservedProviderSessionReader = (*transaction)(nil)

func (t *transaction) ListObservedProviderSessions(ctx context.Context, accountID harnessmodel.ProviderAccountID) ([]harnessmodel.ProviderSessionSnapshot, error) {
	if accountID == "" {
		return nil, fmt.Errorf("provider account id is required")
	}
	rows, err := t.tx.QueryContext(ctx, `
SELECT s.id, a.provider, s.account_id, s.model_id, s.state, s.context_used, s.context_limit,
       s.last_used_at, s.observed_at, s.workspace_fingerprint
FROM provider_sessions s
JOIN provider_accounts a ON a.id=s.account_id
WHERE s.account_id=?
ORDER BY s.last_used_at DESC, s.id`, string(accountID))
	if err != nil {
		return nil, fmt.Errorf("list observed provider sessions: %w", err)
	}
	defer rows.Close()

	var out []harnessmodel.ProviderSessionSnapshot
	for rows.Next() {
		var session harnessmodel.ProviderSessionSnapshot
		var id, provider, account, modelID, state, lastUsedAt, observedAt string
		if err := rows.Scan(
			&id,
			&provider,
			&account,
			&modelID,
			&state,
			&session.ContextUsed,
			&session.ContextLimit,
			&lastUsedAt,
			&observedAt,
			&session.WorkspaceFingerprint,
		); err != nil {
			return nil, fmt.Errorf("scan observed provider session: %w", err)
		}
		session.ID = harnessmodel.ProviderSessionID(id)
		session.Provider = harnessmodel.ProviderKind(provider)
		session.AccountID = harnessmodel.ProviderAccountID(account)
		session.ModelID = harnessmodel.ProviderModelID(modelID)
		session.State = harnessmodel.ProviderSessionState(state)
		if session.LastUsedAt, err = parseTime(lastUsedAt); err != nil {
			return nil, fmt.Errorf("parse provider session last_used_at: %w", err)
		}
		if session.ObservedAt, err = parseTime(observedAt); err != nil {
			return nil, fmt.Errorf("parse provider session observed_at: %w", err)
		}
		out = append(out, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate observed provider sessions: %w", err)
	}
	return out, nil
}
