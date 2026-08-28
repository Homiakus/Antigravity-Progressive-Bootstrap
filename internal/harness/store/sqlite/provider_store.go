package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

const providerAccountSelect = `
SELECT id, provider, name, state, created_at, updated_at
FROM provider_accounts`

func (t *transaction) UpsertProviderAccount(ctx context.Context, account harnessmodel.ProviderAccount) error {
	if account.ID == "" || !account.Provider.Valid() || !account.State.Valid() || account.CreatedAt.IsZero() || account.UpdatedAt.IsZero() {
		return fmt.Errorf("invalid provider account")
	}
	if account.UpdatedAt.Before(account.CreatedAt) {
		return fmt.Errorf("provider account updatedAt precedes createdAt")
	}

	existing, err := t.GetProviderAccount(ctx, account.ID)
	switch {
	case err == nil && existing.Provider != account.Provider:
		return fmt.Errorf("provider account %s provider is immutable: %s != %s: %w", account.ID, existing.Provider, account.Provider, harnessstore.ErrConflict)
	case err != nil && !errors.Is(err, harnessstore.ErrNotFound):
		return err
	}

	res, err := t.tx.ExecContext(ctx, `
INSERT INTO provider_accounts(id, provider, name, state, created_at, updated_at)
VALUES(?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET
    name=excluded.name,
    state=excluded.state,
    updated_at=excluded.updated_at
WHERE provider_accounts.provider=excluded.provider
  AND excluded.updated_at >= provider_accounts.updated_at`,
		string(account.ID), string(account.Provider), account.Name, string(account.State), formatTime(account.CreatedAt), formatTime(account.UpdatedAt))
	if err != nil {
		return fmt.Errorf("upsert provider account %s: %w", account.ID, err)
	}
	if err := requireOneAffected(res); err != nil {
		if errors.Is(err, harnessstore.ErrNotFound) {
			return fmt.Errorf("provider account %s observation is stale: %w", account.ID, harnessstore.ErrConflict)
		}
		return err
	}
	return nil
}

func (t *transaction) GetProviderAccount(ctx context.Context, id harnessmodel.ProviderAccountID) (harnessmodel.ProviderAccount, error) {
	if id == "" {
		return harnessmodel.ProviderAccount{}, fmt.Errorf("provider account id is required")
	}
	return scanProviderAccount(t.tx.QueryRowContext(ctx, providerAccountSelect+` WHERE id=?`, string(id)))
}

func (t *transaction) ListProviderAccounts(ctx context.Context, provider harnessmodel.ProviderKind, state harnessmodel.ProviderAccountState) ([]harnessmodel.ProviderAccount, error) {
	if provider != "" && !provider.Valid() {
		return nil, fmt.Errorf("invalid provider filter %q", provider)
	}
	if state != "" && !state.Valid() {
		return nil, fmt.Errorf("invalid provider account state filter %q", state)
	}
	rows, err := t.tx.QueryContext(ctx, providerAccountSelect+`
WHERE (?='' OR provider=?) AND (?='' OR state=?)
ORDER BY provider, id`, string(provider), string(provider), string(state), string(state))
	if err != nil {
		return nil, fmt.Errorf("list provider accounts: %w", err)
	}
	defer rows.Close()

	var out []harnessmodel.ProviderAccount
	for rows.Next() {
		account, err := scanProviderAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider accounts: %w", err)
	}
	return out, nil
}

func scanProviderAccount(row interface{ Scan(...any) error }) (harnessmodel.ProviderAccount, error) {
	var account harnessmodel.ProviderAccount
	var id, provider, state, createdAt, updatedAt string
	if err := row.Scan(&id, &provider, &account.Name, &state, &createdAt, &updatedAt); err != nil {
		return harnessmodel.ProviderAccount{}, mapNotFound(err)
	}
	account.ID = harnessmodel.ProviderAccountID(id)
	account.Provider = harnessmodel.ProviderKind(provider)
	account.State = harnessmodel.ProviderAccountState(state)
	var err error
	if account.CreatedAt, err = parseTime(createdAt); err != nil {
		return harnessmodel.ProviderAccount{}, fmt.Errorf("parse provider account created_at: %w", err)
	}
	if account.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return harnessmodel.ProviderAccount{}, fmt.Errorf("parse provider account updated_at: %w", err)
	}
	return account, nil
}

func (t *transaction) UpsertProviderModel(ctx context.Context, model harnessmodel.ProviderModelDescriptor, observedAt time.Time) error {
	if model.AccountID == "" || model.ID == "" || !model.Provider.Valid() || model.ContextLimit < 0 || observedAt.IsZero() {
		return fmt.Errorf("invalid provider model observation")
	}
	account, err := t.GetProviderAccount(ctx, model.AccountID)
	if err != nil {
		return err
	}
	if account.Provider != model.Provider {
		return fmt.Errorf("provider model %s provider mismatch: account=%s model=%s", model.ID, account.Provider, model.Provider)
	}
	caps, err := json.Marshal(model.Capabilities)
	if err != nil {
		return fmt.Errorf("marshal provider model capabilities: %w", err)
	}
	enabled := 0
	if model.Enabled {
		enabled = 1
	}
	res, err := t.tx.ExecContext(ctx, `
INSERT INTO provider_models(account_id, model_id, display_name, capabilities_json, context_limit, enabled, observed_at)
VALUES(?,?,?,?,?,?,?)
ON CONFLICT(account_id, model_id) DO UPDATE SET
    display_name=excluded.display_name,
    capabilities_json=excluded.capabilities_json,
    context_limit=excluded.context_limit,
    enabled=excluded.enabled,
    observed_at=excluded.observed_at
WHERE excluded.observed_at >= provider_models.observed_at`,
		string(model.AccountID), string(model.ID), model.DisplayName, caps, model.ContextLimit, enabled, formatTime(observedAt))
	if err != nil {
		return fmt.Errorf("upsert provider model %s/%s: %w", model.AccountID, model.ID, err)
	}
	if err := requireOneAffected(res); err != nil {
		if errors.Is(err, harnessstore.ErrNotFound) {
			return fmt.Errorf("provider model %s/%s observation is stale: %w", model.AccountID, model.ID, harnessstore.ErrConflict)
		}
		return err
	}
	return nil
}

func (t *transaction) ListProviderModels(ctx context.Context, accountID harnessmodel.ProviderAccountID) ([]harnessmodel.ProviderModelDescriptor, error) {
	if accountID == "" {
		return nil, fmt.Errorf("provider account id is required")
	}
	rows, err := t.tx.QueryContext(ctx, `
SELECT m.account_id, a.provider, m.model_id, m.display_name, m.capabilities_json, m.context_limit, m.enabled
FROM provider_models m
JOIN provider_accounts a ON a.id=m.account_id
WHERE m.account_id=?
ORDER BY m.model_id`, string(accountID))
	if err != nil {
		return nil, fmt.Errorf("list provider models: %w", err)
	}
	defer rows.Close()

	var out []harnessmodel.ProviderModelDescriptor
	for rows.Next() {
		var model harnessmodel.ProviderModelDescriptor
		var account, provider, modelID string
		var caps []byte
		var enabled int
		if err := rows.Scan(&account, &provider, &modelID, &model.DisplayName, &caps, &model.ContextLimit, &enabled); err != nil {
			return nil, fmt.Errorf("scan provider model: %w", err)
		}
		model.AccountID = harnessmodel.ProviderAccountID(account)
		model.Provider = harnessmodel.ProviderKind(provider)
		model.ID = harnessmodel.ProviderModelID(modelID)
		model.Enabled = enabled != 0
		if err := json.Unmarshal(caps, &model.Capabilities); err != nil {
			return nil, fmt.Errorf("decode provider model capabilities: %w", err)
		}
		out = append(out, model)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider models: %w", err)
	}
	return out, nil
}

func (t *transaction) AppendProviderCapacity(ctx context.Context, snapshot harnessmodel.ProviderCapacitySnapshot) error {
	if err := snapshot.Validate(); err != nil {
		return err
	}
	account, err := t.GetProviderAccount(ctx, snapshot.AccountID)
	if err != nil {
		return err
	}
	if account.Provider != snapshot.Provider {
		return fmt.Errorf("provider capacity provider mismatch: account=%s snapshot=%s", account.Provider, snapshot.Provider)
	}

	res, err := t.tx.ExecContext(ctx, `
INSERT INTO provider_capacity_snapshots(account_id, health, active_runs, observed_at)
VALUES(?,?,?,?)`, string(snapshot.AccountID), string(snapshot.Health), snapshot.ActiveRuns, formatTime(snapshot.ObservedAt))
	if err != nil {
		return fmt.Errorf("insert provider capacity snapshot: %w", err)
	}
	snapshotID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("read provider capacity snapshot id: %w", err)
	}
	for _, window := range snapshot.Windows {
		var resetAt any
		if window.ResetAt != nil {
			resetAt = formatTime(window.ResetAt.UTC())
		}
		if _, err := t.tx.ExecContext(ctx, `
INSERT INTO provider_quota_windows(
    snapshot_id, window_id, model_id, metric, limit_value, remaining_value,
    remaining_fraction, reset_at, observed_at, confidence
) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			snapshotID, window.ID, string(window.ModelID), string(window.Metric), nullableFloat(window.Limit), nullableFloat(window.Remaining),
			nullableFloat(window.RemainingFraction), resetAt, formatTime(window.ObservedAt), window.Confidence); err != nil {
			return fmt.Errorf("insert provider quota window %s/%s: %w", window.ID, window.ModelID, err)
		}
	}
	return nil
}

func (t *transaction) GetLatestProviderCapacity(ctx context.Context, accountID harnessmodel.ProviderAccountID) (harnessmodel.ProviderCapacitySnapshot, error) {
	if accountID == "" {
		return harnessmodel.ProviderCapacitySnapshot{}, fmt.Errorf("provider account id is required")
	}
	var snapshot harnessmodel.ProviderCapacitySnapshot
	var snapshotID int64
	var account, provider, health, observedAt string
	if err := t.tx.QueryRowContext(ctx, `
SELECT s.snapshot_id, s.account_id, a.provider, s.health, s.active_runs, s.observed_at
FROM provider_capacity_snapshots s
JOIN provider_accounts a ON a.id=s.account_id
WHERE s.account_id=?
ORDER BY s.observed_at DESC, s.snapshot_id DESC
LIMIT 1`, string(accountID)).Scan(&snapshotID, &account, &provider, &health, &snapshot.ActiveRuns, &observedAt); err != nil {
		return harnessmodel.ProviderCapacitySnapshot{}, mapNotFound(err)
	}
	snapshot.AccountID = harnessmodel.ProviderAccountID(account)
	snapshot.Provider = harnessmodel.ProviderKind(provider)
	snapshot.Health = harnessmodel.ProviderHealth(health)
	var err error
	if snapshot.ObservedAt, err = parseTime(observedAt); err != nil {
		return harnessmodel.ProviderCapacitySnapshot{}, fmt.Errorf("parse provider capacity observed_at: %w", err)
	}

	rows, err := t.tx.QueryContext(ctx, `
SELECT window_id, model_id, metric, limit_value, remaining_value, remaining_fraction, reset_at, observed_at, confidence
FROM provider_quota_windows
WHERE snapshot_id=?
ORDER BY window_id, model_id`, snapshotID)
	if err != nil {
		return harnessmodel.ProviderCapacitySnapshot{}, fmt.Errorf("list provider quota windows: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var window harnessmodel.QuotaWindow
		var modelID, metric, windowObservedAt string
		var limit, remaining, fraction sql.NullFloat64
		var reset sql.NullString
		if err := rows.Scan(&window.ID, &modelID, &metric, &limit, &remaining, &fraction, &reset, &windowObservedAt, &window.Confidence); err != nil {
			return harnessmodel.ProviderCapacitySnapshot{}, fmt.Errorf("scan provider quota window: %w", err)
		}
		window.ModelID = harnessmodel.ProviderModelID(modelID)
		window.Metric = harnessmodel.QuotaMetricKind(metric)
		window.Limit = floatPtr(limit)
		window.Remaining = floatPtr(remaining)
		window.RemainingFraction = floatPtr(fraction)
		if window.ObservedAt, err = parseTime(windowObservedAt); err != nil {
			return harnessmodel.ProviderCapacitySnapshot{}, fmt.Errorf("parse provider quota observed_at: %w", err)
		}
		if reset.Valid && reset.String != "" {
			resetAt, parseErr := parseTime(reset.String)
			if parseErr != nil {
				return harnessmodel.ProviderCapacitySnapshot{}, fmt.Errorf("parse provider quota reset_at: %w", parseErr)
			}
			window.ResetAt = &resetAt
		}
		snapshot.Windows = append(snapshot.Windows, window)
	}
	if err := rows.Err(); err != nil {
		return harnessmodel.ProviderCapacitySnapshot{}, fmt.Errorf("iterate provider quota windows: %w", err)
	}
	return snapshot, nil
}

func (t *transaction) UpsertProviderSession(ctx context.Context, session harnessmodel.ProviderSessionSnapshot, observedAt time.Time) error {
	if err := session.Validate(); err != nil {
		return err
	}
	if observedAt.IsZero() {
		return fmt.Errorf("provider session observedAt is required")
	}
	account, err := t.GetProviderAccount(ctx, session.AccountID)
	if err != nil {
		return err
	}
	if account.Provider != session.Provider {
		return fmt.Errorf("provider session %s provider mismatch: account=%s session=%s", session.ID, account.Provider, session.Provider)
	}
	res, err := t.tx.ExecContext(ctx, `
INSERT INTO provider_sessions(
    id, account_id, model_id, state, context_used, context_limit,
    last_used_at, workspace_fingerprint, observed_at
) VALUES(?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET
    model_id=excluded.model_id,
    state=excluded.state,
    context_used=excluded.context_used,
    context_limit=excluded.context_limit,
    last_used_at=excluded.last_used_at,
    workspace_fingerprint=excluded.workspace_fingerprint,
    observed_at=excluded.observed_at
WHERE provider_sessions.account_id=excluded.account_id
  AND excluded.observed_at >= provider_sessions.observed_at`,
		string(session.ID), string(session.AccountID), string(session.ModelID), string(session.State), session.ContextUsed, session.ContextLimit,
		formatTime(session.LastUsedAt), session.WorkspaceFingerprint, formatTime(observedAt))
	if err != nil {
		return fmt.Errorf("upsert provider session %s: %w", session.ID, err)
	}
	if err := requireOneAffected(res); err != nil {
		if errors.Is(err, harnessstore.ErrNotFound) {
			return fmt.Errorf("provider session %s is stale or bound to another account: %w", session.ID, harnessstore.ErrConflict)
		}
		return err
	}
	return nil
}

func (t *transaction) ListProviderSessions(ctx context.Context, accountID harnessmodel.ProviderAccountID) ([]harnessmodel.ProviderSessionSnapshot, error) {
	if accountID == "" {
		return nil, fmt.Errorf("provider account id is required")
	}
	rows, err := t.tx.QueryContext(ctx, `
SELECT s.id, a.provider, s.account_id, s.model_id, s.state, s.context_used, s.context_limit,
       s.last_used_at, s.workspace_fingerprint
FROM provider_sessions s
JOIN provider_accounts a ON a.id=s.account_id
WHERE s.account_id=?
ORDER BY s.last_used_at DESC, s.id`, string(accountID))
	if err != nil {
		return nil, fmt.Errorf("list provider sessions: %w", err)
	}
	defer rows.Close()

	var out []harnessmodel.ProviderSessionSnapshot
	for rows.Next() {
		var session harnessmodel.ProviderSessionSnapshot
		var id, provider, account, modelID, state, lastUsedAt string
		if err := rows.Scan(&id, &provider, &account, &modelID, &state, &session.ContextUsed, &session.ContextLimit, &lastUsedAt, &session.WorkspaceFingerprint); err != nil {
			return nil, fmt.Errorf("scan provider session: %w", err)
		}
		session.ID = harnessmodel.ProviderSessionID(id)
		session.Provider = harnessmodel.ProviderKind(provider)
		session.AccountID = harnessmodel.ProviderAccountID(account)
		session.ModelID = harnessmodel.ProviderModelID(modelID)
		session.State = harnessmodel.ProviderSessionState(state)
		var err error
		if session.LastUsedAt, err = parseTime(lastUsedAt); err != nil {
			return nil, fmt.Errorf("parse provider session last_used_at: %w", err)
		}
		out = append(out, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider sessions: %w", err)
	}
	return out, nil
}

func nullableFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func floatPtr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	v := value.Float64
	return &v
}
