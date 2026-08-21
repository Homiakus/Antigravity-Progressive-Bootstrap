package sqlite

import (
	"context"
	"fmt"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func (t *transaction) ListEffectIntentsByAttempt(ctx context.Context, attemptID harnessmodel.AttemptID, limit int) ([]harnessmodel.EffectIntent, error) {
	if attemptID == "" {
		return nil, fmt.Errorf("attempt id is required")
	}
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	rows, err := t.tx.QueryContext(ctx, effectSelect+`
WHERE EXISTS (
    SELECT 1 FROM effect_attempt_bindings eab
    WHERE eab.effect_intent_id=effect_intents.effect_intent_id
      AND eab.attempt_id=?
)
ORDER BY effect_intents.prepared_at, effect_intents.effect_intent_id
LIMIT ?`, string(attemptID), limit)
	if err != nil {
		return nil, fmt.Errorf("list effect intents by attempt: %w", err)
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
		return nil, fmt.Errorf("iterate attempt effect intents: %w", err)
	}
	return out, nil
}
