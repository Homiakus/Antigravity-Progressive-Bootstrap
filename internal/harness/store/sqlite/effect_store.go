package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

const effectSelect = `
SELECT effect_intent_id, workflow_run_id, node_run_id, origin_attempt_id, last_attempt_id,
       operation_namespace, operation, effect_class, idempotency_key, semantic_input_digest,
       state, prepared_at, dispatched_at, resolved_at, provider_ref, result_digest,
       error_class, error_message, reconcile_count, last_reconciled_at
FROM effect_intents`

func (t *transaction) PutEffectIntent(ctx context.Context, intent harnessmodel.EffectIntent) (harnessmodel.EffectIntent, bool, error) {
	if err := validateNewEffectIntent(intent); err != nil {
		return harnessmodel.EffectIntent{}, false, err
	}
	if err := t.validateEffectAttempt(ctx, intent.NodeRunID, intent.WorkflowRunID, intent.OriginAttemptID); err != nil {
		return harnessmodel.EffectIntent{}, false, err
	}
	if err := t.validateEffectAttempt(ctx, intent.NodeRunID, intent.WorkflowRunID, intent.LastAttemptID); err != nil {
		return harnessmodel.EffectIntent{}, false, err
	}

	res, err := t.tx.ExecContext(ctx, `
INSERT INTO effect_intents(
    effect_intent_id, workflow_run_id, node_run_id, origin_attempt_id, last_attempt_id,
    operation_namespace, operation, effect_class, idempotency_key, semantic_input_digest,
    state, prepared_at, dispatched_at, resolved_at, provider_ref, result_digest,
    error_class, error_message, reconcile_count, last_reconciled_at
) VALUES(?,?,?,?,?,?,?,?,?,?,'PREPARED',?,NULL,NULL,'','','','',0,NULL)
ON CONFLICT(idempotency_key) DO NOTHING`,
		string(intent.ID), string(intent.WorkflowRunID), string(intent.NodeRunID), string(intent.OriginAttemptID), string(intent.LastAttemptID),
		intent.OperationNamespace, intent.Operation, string(intent.Class), intent.IdempotencyKey, intent.SemanticInputDigest,
		formatTime(intent.PreparedAt))
	if err != nil {
		return harnessmodel.EffectIntent{}, false, fmt.Errorf("put effect intent: %w", err)
	}
	created := false
	if n, rowsErr := res.RowsAffected(); rowsErr != nil {
		return harnessmodel.EffectIntent{}, false, fmt.Errorf("read effect insert count: %w", rowsErr)
	} else {
		created = n == 1
	}

	if created {
		if err := t.bindEffectAttempt(ctx, intent.ID, intent.OriginAttemptID, intent.PreparedAt); err != nil {
			return harnessmodel.EffectIntent{}, false, err
		}
		if intent.LastAttemptID != intent.OriginAttemptID {
			if err := t.bindEffectAttempt(ctx, intent.ID, intent.LastAttemptID, intent.PreparedAt); err != nil {
				return harnessmodel.EffectIntent{}, false, err
			}
		}
		return intent, true, nil
	}

	existing, err := t.GetEffectIntentByKey(ctx, intent.IdempotencyKey)
	if err != nil {
		return harnessmodel.EffectIntent{}, false, err
	}
	if existing.WorkflowRunID != intent.WorkflowRunID || existing.NodeRunID != intent.NodeRunID ||
		existing.OperationNamespace != intent.OperationNamespace || existing.Operation != intent.Operation ||
		existing.Class != intent.Class || existing.SemanticInputDigest != intent.SemanticInputDigest {
		return existing, false, fmt.Errorf("effect idempotency key %q was reused for different semantics: %w", intent.IdempotencyKey, harnessstore.ErrConflict)
	}
	if err := t.bindEffectAttempt(ctx, existing.ID, intent.LastAttemptID, intent.PreparedAt); err != nil {
		return harnessmodel.EffectIntent{}, false, err
	}
	if existing.LastAttemptID != intent.LastAttemptID {
		res, err := t.tx.ExecContext(ctx, `UPDATE effect_intents SET last_attempt_id=? WHERE effect_intent_id=?`, string(intent.LastAttemptID), string(existing.ID))
		if err != nil {
			return harnessmodel.EffectIntent{}, false, fmt.Errorf("bind latest effect attempt: %w", err)
		}
		if err := requireOneAffected(res); err != nil {
			return harnessmodel.EffectIntent{}, false, err
		}
	}
	return t.GetEffectIntent(ctx, existing.ID)
}

func validateNewEffectIntent(intent harnessmodel.EffectIntent) error {
	if intent.ID == "" || intent.WorkflowRunID == "" || intent.NodeRunID == "" || intent.OriginAttemptID == "" || intent.LastAttemptID == "" ||
		strings.TrimSpace(intent.OperationNamespace) == "" || strings.TrimSpace(intent.Operation) == "" || !intent.Class.Valid() ||
		strings.TrimSpace(intent.IdempotencyKey) == "" || strings.TrimSpace(intent.SemanticInputDigest) == "" || intent.State != harnessmodel.EffectPrepared || intent.PreparedAt.IsZero() {
		return fmt.Errorf("invalid new effect intent")
	}
	if !intent.DispatchedAt.IsZero() || !intent.ResolvedAt.IsZero() || intent.ProviderRef != "" || intent.ResultDigest != "" || intent.ErrorClass != "" || intent.ErrorMessage != "" || intent.ReconcileCount != 0 || !intent.LastReconciledAt.IsZero() {
		return fmt.Errorf("new effect intent must be pristine PREPARED state")
	}
	if !strings.HasPrefix(intent.IdempotencyKey, "effk_v1_") || !strings.HasPrefix(intent.SemanticInputDigest, "sha256:") {
		return fmt.Errorf("effect intent key/digest format is not supported")
	}
	return nil
}

func (t *transaction) validateEffectAttempt(ctx context.Context, nodeRunID harnessmodel.NodeRunID, workflowRunID harnessmodel.WorkflowRunID, attemptID harnessmodel.AttemptID) error {
	node, err := t.GetNodeRun(ctx, nodeRunID)
	if err != nil {
		return err
	}
	if node.WorkflowRunID != workflowRunID {
		return fmt.Errorf("effect node %s belongs to workflow %s, not %s: %w", nodeRunID, node.WorkflowRunID, workflowRunID, harnessstore.ErrConflict)
	}
	attempt, err := t.GetAttempt(ctx, attemptID)
	if err != nil {
		return err
	}
	if attempt.NodeRunID != nodeRunID {
		return fmt.Errorf("effect attempt %s belongs to node %s, not %s: %w", attemptID, attempt.NodeRunID, nodeRunID, harnessstore.ErrConflict)
	}
	return nil
}

func (t *transaction) bindEffectAttempt(ctx context.Context, effectID harnessmodel.EffectIntentID, attemptID harnessmodel.AttemptID, at time.Time) error {
	_, err := t.tx.ExecContext(ctx, `
INSERT INTO effect_attempt_bindings(effect_intent_id, attempt_id, bound_at)
VALUES(?,?,?) ON CONFLICT(effect_intent_id, attempt_id) DO NOTHING`, string(effectID), string(attemptID), formatTime(at))
	if err != nil {
		return fmt.Errorf("bind effect attempt: %w", err)
	}
	return nil
}

func (t *transaction) GetEffectIntent(ctx context.Context, id harnessmodel.EffectIntentID) (harnessmodel.EffectIntent, error) {
	return scanEffectIntent(t.tx.QueryRowContext(ctx, effectSelect+` WHERE effect_intent_id=?`, string(id)))
}

func (t *transaction) GetEffectIntentByKey(ctx context.Context, key string) (harnessmodel.EffectIntent, error) {
	if strings.TrimSpace(key) == "" {
		return harnessmodel.EffectIntent{}, fmt.Errorf("effect idempotency key is required")
	}
	return scanEffectIntent(t.tx.QueryRowContext(ctx, effectSelect+` WHERE idempotency_key=?`, key))
}

func (t *transaction) ListUncertainEffects(ctx context.Context, runID harnessmodel.WorkflowRunID, limit int) ([]harnessmodel.EffectIntent, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	query := effectSelect + ` WHERE state IN ('DISPATCHED','IN_DOUBT')`
	args := make([]any, 0, 2)
	if runID != "" {
		query += ` AND workflow_run_id=?`
		args = append(args, string(runID))
	}
	query += ` ORDER BY prepared_at, effect_intent_id LIMIT ?`
	args = append(args, limit)
	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list uncertain effects: %w", err)
	}
	defer rows.Close()
	out := make([]harnessmodel.EffectIntent, 0)
	for rows.Next() {
		intent, err := scanEffectIntent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, intent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate uncertain effects: %w", err)
	}
	return out, nil
}

func (t *transaction) CompareAndSwapEffectIntent(ctx context.Context, expected harnessmodel.EffectState, intent harnessmodel.EffectIntent) error {
	if intent.ID == "" || !expected.Valid() || !intent.State.Valid() || !validEffectTransition(expected, intent.State, intent.Class) {
		return fmt.Errorf("invalid effect transition %s -> %s", expected, intent.State)
	}
	current, err := t.GetEffectIntent(ctx, intent.ID)
	if err != nil {
		return err
	}
	if current.State != expected {
		return harnessstore.ErrConflict
	}
	if !sameEffectIdentity(current, intent) || current.OriginAttemptID != intent.OriginAttemptID {
		return fmt.Errorf("effect immutable identity changed: %w", harnessstore.ErrConflict)
	}
	if err := validateEffectTransitionFields(current, intent); err != nil {
		return err
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE effect_intents
SET last_attempt_id=?, state=?, dispatched_at=?, resolved_at=?, provider_ref=?, result_digest=?, error_class=?, error_message=?, reconcile_count=?, last_reconciled_at=?
WHERE effect_intent_id=? AND state=?`,
		string(intent.LastAttemptID), string(intent.State), nullableTime(intent.DispatchedAt), nullableTime(intent.ResolvedAt),
		intent.ProviderRef, intent.ResultDigest, intent.ErrorClass, intent.ErrorMessage, intent.ReconcileCount, nullableTime(intent.LastReconciledAt),
		string(intent.ID), string(expected))
	if err != nil {
		return fmt.Errorf("CAS effect intent: %w", err)
	}
	if err := requireOneAffected(res); err != nil {
		return harnessstore.ErrConflict
	}
	return nil
}

func sameEffectIdentity(a, b harnessmodel.EffectIntent) bool {
	return a.ID == b.ID && a.WorkflowRunID == b.WorkflowRunID && a.NodeRunID == b.NodeRunID &&
		a.OperationNamespace == b.OperationNamespace && a.Operation == b.Operation && a.Class == b.Class &&
		a.IdempotencyKey == b.IdempotencyKey && a.SemanticInputDigest == b.SemanticInputDigest && a.PreparedAt.Equal(b.PreparedAt)
}

func validEffectTransition(from, to harnessmodel.EffectState, class harnessmodel.EffectClass) bool {
	switch from {
	case harnessmodel.EffectPrepared:
		return to == harnessmodel.EffectDispatched || to == harnessmodel.EffectFailed
	case harnessmodel.EffectDispatched:
		return to == harnessmodel.EffectConfirmed || to == harnessmodel.EffectFailed || to == harnessmodel.EffectInDoubt
	case harnessmodel.EffectInDoubt:
		return to == harnessmodel.EffectConfirmed || to == harnessmodel.EffectFailed || (to == harnessmodel.EffectCompensated && class == harnessmodel.EffectCompensatable)
	case harnessmodel.EffectConfirmed:
		return to == harnessmodel.EffectCompensated && class == harnessmodel.EffectCompensatable
	default:
		return false
	}
}

func validateEffectTransitionFields(current, next harnessmodel.EffectIntent) error {
	if next.LastAttemptID == "" {
		return fmt.Errorf("effect last attempt id is required")
	}
	if !next.DispatchedAt.IsZero() && next.DispatchedAt.Before(next.PreparedAt) {
		return fmt.Errorf("effect dispatched_at precedes prepared_at")
	}
	if !next.ResolvedAt.IsZero() && next.ResolvedAt.Before(next.PreparedAt) {
		return fmt.Errorf("effect resolved_at precedes prepared_at")
	}
	switch next.State {
	case harnessmodel.EffectDispatched:
		if next.DispatchedAt.IsZero() || !next.ResolvedAt.IsZero() {
			return fmt.Errorf("DISPATCHED effect requires dispatched_at and no resolved_at")
		}
	case harnessmodel.EffectInDoubt:
		if next.DispatchedAt.IsZero() || !next.ResolvedAt.IsZero() {
			return fmt.Errorf("IN_DOUBT effect requires dispatch evidence and no resolved_at")
		}
	case harnessmodel.EffectConfirmed, harnessmodel.EffectCompensated:
		if next.ResolvedAt.IsZero() {
			return fmt.Errorf("resolved effect requires resolved_at")
		}
	case harnessmodel.EffectFailed:
		if next.ResolvedAt.IsZero() || strings.TrimSpace(next.ErrorClass) == "" {
			return fmt.Errorf("FAILED effect requires resolved_at and error class")
		}
	}
	_ = current
	return nil
}

func (t *transaction) RecordEffectReconciliation(ctx context.Context, id harnessmodel.EffectIntentID, expected harnessmodel.EffectState, at time.Time) (harnessmodel.EffectIntent, error) {
	if id == "" || (expected != harnessmodel.EffectDispatched && expected != harnessmodel.EffectInDoubt) || at.IsZero() {
		return harnessmodel.EffectIntent{}, fmt.Errorf("invalid reconciliation record")
	}
	row := t.tx.QueryRowContext(ctx, `
UPDATE effect_intents
SET reconcile_count=reconcile_count+1, last_reconciled_at=?
WHERE effect_intent_id=? AND state=?
RETURNING effect_intent_id, workflow_run_id, node_run_id, origin_attempt_id, last_attempt_id,
          operation_namespace, operation, effect_class, idempotency_key, semantic_input_digest,
          state, prepared_at, dispatched_at, resolved_at, provider_ref, result_digest,
          error_class, error_message, reconcile_count, last_reconciled_at`, formatTime(at), string(id), string(expected))
	intent, err := scanEffectIntent(row)
	if err != nil {
		if err == harnessstore.ErrNotFound {
			return harnessmodel.EffectIntent{}, harnessstore.ErrConflict
		}
		return harnessmodel.EffectIntent{}, err
	}
	return intent, nil
}

func scanEffectIntent(row interface{ Scan(...any) error }) (harnessmodel.EffectIntent, error) {
	var intent harnessmodel.EffectIntent
	var id, workflowID, nodeID, originAttempt, lastAttempt, class, state, preparedAt string
	var dispatchedAt, resolvedAt, lastReconciledAt sql.NullString
	if err := row.Scan(&id, &workflowID, &nodeID, &originAttempt, &lastAttempt,
		&intent.OperationNamespace, &intent.Operation, &class, &intent.IdempotencyKey, &intent.SemanticInputDigest,
		&state, &preparedAt, &dispatchedAt, &resolvedAt, &intent.ProviderRef, &intent.ResultDigest,
		&intent.ErrorClass, &intent.ErrorMessage, &intent.ReconcileCount, &lastReconciledAt); err != nil {
		return harnessmodel.EffectIntent{}, mapNotFound(err)
	}
	intent.ID = harnessmodel.EffectIntentID(id)
	intent.WorkflowRunID = harnessmodel.WorkflowRunID(workflowID)
	intent.NodeRunID = harnessmodel.NodeRunID(nodeID)
	intent.OriginAttemptID = harnessmodel.AttemptID(originAttempt)
	intent.LastAttemptID = harnessmodel.AttemptID(lastAttempt)
	intent.Class = harnessmodel.EffectClass(class)
	intent.State = harnessmodel.EffectState(state)
	var err error
	if intent.PreparedAt, err = parseTime(preparedAt); err != nil {
		return harnessmodel.EffectIntent{}, fmt.Errorf("parse effect prepared_at: %w", err)
	}
	if dispatchedAt.Valid {
		if intent.DispatchedAt, err = parseTime(dispatchedAt.String); err != nil { return harnessmodel.EffectIntent{}, fmt.Errorf("parse effect dispatched_at: %w", err) }
	}
	if resolvedAt.Valid {
		if intent.ResolvedAt, err = parseTime(resolvedAt.String); err != nil { return harnessmodel.EffectIntent{}, fmt.Errorf("parse effect resolved_at: %w", err) }
	}
	if lastReconciledAt.Valid {
		if intent.LastReconciledAt, err = parseTime(lastReconciledAt.String); err != nil { return harnessmodel.EffectIntent{}, fmt.Errorf("parse effect last_reconciled_at: %w", err) }
	}
	return intent, nil
}
