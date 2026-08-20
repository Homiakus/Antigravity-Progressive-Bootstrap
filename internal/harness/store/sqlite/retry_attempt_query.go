package sqlite

import (
	"context"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func (t *transaction) GetFirstAttemptCreatedAt(ctx context.Context, nodeRunID harnessmodel.NodeRunID) (time.Time, error) {
	if nodeRunID == "" {
		return time.Time{}, fmt.Errorf("node run id is required")
	}
	var raw string
	if err := t.tx.QueryRowContext(ctx, `
SELECT created_at FROM attempts
WHERE node_run_id=?
ORDER BY attempt_number
LIMIT 1`, string(nodeRunID)).Scan(&raw); err != nil {
		return time.Time{}, mapNotFound(err)
	}
	at, err := parseTime(raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse first attempt created_at: %w", err)
	}
	return at, nil
}
