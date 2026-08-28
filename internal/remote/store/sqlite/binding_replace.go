package sqlite

import (
	"context"
	"fmt"

	"github.com/homiakus/agctl/internal/remote/model"
)

// ReplaceTelegramBinding atomically rebinds one Telegram topic to a session.
// The partial unique index permits exactly one enabled binding per topic, so
// disabling the old row and enabling the new row must happen in one tx.
func (s *Store) ReplaceTelegramBinding(ctx context.Context, binding model.TelegramBinding) error {
	if err := binding.Validate(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE telegram_bindings SET enabled=0 WHERE chat_id=? AND thread_id=? AND enabled=1 AND id<>?`, binding.ChatID, binding.ThreadID, binding.ID); err != nil {
		return fmt.Errorf("disable previous Telegram topic binding: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO telegram_bindings(id,session_id,chat_id,thread_id,owner_user_id,enabled,created_at) VALUES(?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET session_id=excluded.session_id,chat_id=excluded.chat_id,thread_id=excluded.thread_id,owner_user_id=excluded.owner_user_id,enabled=excluded.enabled`, binding.ID, binding.SessionID, binding.ChatID, binding.ThreadID, binding.OwnerUserID, boolInt(binding.Enabled), formatTime(binding.CreatedAt)); err != nil {
		return mapWriteError("replace telegram binding", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Telegram topic binding: %w", err)
	}
	return nil
}
