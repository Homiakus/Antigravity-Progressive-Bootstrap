package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

func (s *Store) UpdateSessionAccount(ctx context.Context, id model.RemoteSessionID, accountID string, updated time.Time) error {
	if strings.TrimSpace(string(id)) == "" || strings.TrimSpace(accountID) == "" || updated.IsZero() {
		return fmt.Errorf("invalid remote session account update")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE remote_sessions SET cockpit_account_id=?,updated_at=? WHERE id=?`, accountID, formatTime(updated), id)
	if err != nil {
		return mapWriteError("update remote session account", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return remotestore.ErrNotFound
	}
	return nil
}
