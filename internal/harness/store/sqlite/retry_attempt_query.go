package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func (t *transaction) GetFirstAttemptCreatedAt(ctx context.Context, nodeRunID harnessmodel.NodeRunID) (time.Time, error) {
	if nodeRunID == "" {
		return time.Time{}, fmt.Errorf("node run id is required")
	}
	var raw sql.NullString
	if err := t.tx.QueryRowContext(ctx, `SELECT MIN(created_at) FROM attempts WHERE node_run_id=?`, string(nodeRunID)).Scan(&raw); err != nil {
		return time.Time{}, fmt.Errorf("query first attempt time: %w", err)
	}
	if !raw.Valid || raw.String == "" {
		return time.Time{}, harnessstore.ErrNotFound
	}
	at, err := parseTime(raw.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse first attempt created_at: %w", err)
	}
	return at, nil
}
