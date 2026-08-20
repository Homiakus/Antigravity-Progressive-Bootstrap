package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

func backfillReadyDeadlineNS(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `
SELECT node_run_id, not_before
FROM ready_queue
WHERE not_before IS NOT NULL AND not_before<>'' AND not_before_ns IS NULL`)
	if err != nil {
		return fmt.Errorf("read ready deadline backfill: %w", err)
	}
	type item struct {
		id string
		ns int64
	}
	items := make([]item, 0)
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan ready deadline backfill: %w", err)
		}
		at, err := parseTime(raw)
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("parse ready deadline for %s: %w", id, err)
		}
		items = append(items, item{id: id, ns: at.UnixNano()})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate ready deadline backfill: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close ready deadline backfill rows: %w", err)
	}
	if len(items) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ready deadline backfill: %w", err)
	}
	defer tx.Rollback()
	for _, item := range items {
		res, err := tx.ExecContext(ctx, `UPDATE ready_queue SET not_before_ns=? WHERE node_run_id=? AND not_before_ns IS NULL`, item.ns, item.id)
		if err != nil {
			return fmt.Errorf("backfill ready deadline %s: %w", item.id, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("read ready deadline backfill count: %w", err)
		}
		if n != 1 {
			return fmt.Errorf("ready deadline %s changed concurrently during startup backfill", item.id)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit ready deadline backfill: %w", err)
	}
	return nil
}
