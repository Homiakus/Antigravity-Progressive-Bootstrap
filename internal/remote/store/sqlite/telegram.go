package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

func (s *Store) GetTelegramCursor(ctx context.Context, botKey string) (model.TelegramCursor, error) {
	var item model.TelegramCursor
	var updated string
	err := s.db.QueryRowContext(ctx, `SELECT bot_key,next_update_id,updated_at FROM telegram_runtime WHERE bot_key=?`, botKey).Scan(&item.BotKey, &item.NextUpdateID, &updated)
	if errors.Is(err, sql.ErrNoRows) { return model.TelegramCursor{}, remotestore.ErrNotFound }
	if err != nil { return model.TelegramCursor{}, err }
	item.UpdatedAt, err = parseTime(updated)
	return item, err
}

func (s *Store) AdvanceTelegramCursor(ctx context.Context, cursor model.TelegramCursor) error {
	if err := cursor.Validate(); err != nil { return err }
	_, err := s.db.ExecContext(ctx, `INSERT INTO telegram_runtime(bot_key,next_update_id,updated_at) VALUES(?,?,?)
ON CONFLICT(bot_key) DO UPDATE SET next_update_id=CASE WHEN excluded.next_update_id > telegram_runtime.next_update_id THEN excluded.next_update_id ELSE telegram_runtime.next_update_id END, updated_at=CASE WHEN excluded.next_update_id > telegram_runtime.next_update_id THEN excluded.updated_at ELSE telegram_runtime.updated_at END`, cursor.BotKey, cursor.NextUpdateID, formatTime(cursor.UpdatedAt))
	return err
}

func (s *Store) UpsertTelegramPrincipal(ctx context.Context, principal model.TelegramPrincipal) error {
	if err := principal.Validate(); err != nil { return err }
	_, err := s.db.ExecContext(ctx, `INSERT INTO telegram_principals(user_id,role,enabled,paired_at) VALUES(?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET role=excluded.role,enabled=excluded.enabled,paired_at=excluded.paired_at`, principal.UserID, principal.Role, boolInt(principal.Enabled), formatTime(principal.PairedAt))
	return err
}

func (s *Store) GetTelegramPrincipal(ctx context.Context, userID int64) (model.TelegramPrincipal, error) {
	return scanTelegramPrincipal(s.db.QueryRowContext(ctx, `SELECT user_id,role,enabled,paired_at FROM telegram_principals WHERE user_id=?`, userID))
}

func scanTelegramPrincipal(row scanner) (model.TelegramPrincipal,error) {
	var item model.TelegramPrincipal
	var enabled int
	var paired string
	if err:=row.Scan(&item.UserID,&item.Role,&enabled,&paired);err!=nil { if errors.Is(err,sql.ErrNoRows){return model.TelegramPrincipal{},remotestore.ErrNotFound};return model.TelegramPrincipal{},err }
	item.Enabled=enabled!=0
	var err error
	item.PairedAt,err=parseTime(paired)
	return item,err
}

func (s *Store) CreateTelegramPairing(ctx context.Context, pairing model.TelegramPairing) error {
	if err := pairing.Validate(); err != nil { return err }
	_, err := s.db.ExecContext(ctx, `INSERT INTO telegram_pairings(token_hash,role,intended_chat_id,created_at,expires_at,consumed_at,consumed_by_user_id,consumed_chat_id) VALUES(?,?,?,?,?,?,?,?,?)`, pairing.TokenHash, pairing.Role, pairing.IntendedChatID, formatTime(pairing.CreatedAt), formatTime(pairing.ExpiresAt), nil, nil, nil)
	if err != nil { return mapWriteError("create telegram pairing", err) }
	return nil
}

func (s *Store) ConsumeTelegramPairing(ctx context.Context, tokenHash string, userID, chatID int64, now time.Time) (model.TelegramPrincipal, error) {
	if strings.TrimSpace(tokenHash)==""||userID==0||chatID==0||now.IsZero(){return model.TelegramPrincipal{},fmt.Errorf("telegram pairing hash, user, chat and time are required")}
	tx,err:=s.db.BeginTx(ctx,nil);if err!=nil{return model.TelegramPrincipal{},err};defer tx.Rollback()
	var role model.TelegramRole;var intended int64;var expires string;var consumed sql.NullString;var consumedUser,consumedChat sql.NullInt64
	err=tx.QueryRowContext(ctx,`SELECT role,intended_chat_id,expires_at,consumed_at,consumed_by_user_id,consumed_chat_id FROM telegram_pairings WHERE token_hash=?`,tokenHash).Scan(&role,&intended,&expires,&consumed,&consumedUser,&consumedChat)
	if errors.Is(err,sql.ErrNoRows){return model.TelegramPrincipal{},remotestore.ErrNotFound};if err!=nil{return model.TelegramPrincipal{},err}
	if intended!=0&&intended!=chatID{return model.TelegramPrincipal{},fmt.Errorf("telegram pairing chat mismatch: %w",remotestore.ErrConflict)}
	if consumed.Valid {
		if consumedUser.Valid&&consumedChat.Valid&&consumedUser.Int64==userID&&consumedChat.Int64==chatID {
			principal,err:=scanTelegramPrincipal(tx.QueryRowContext(ctx,`SELECT user_id,role,enabled,paired_at FROM telegram_principals WHERE user_id=?`,userID));if err!=nil{return model.TelegramPrincipal{},err};return principal,nil
		}
		return model.TelegramPrincipal{},remotestore.ErrConflict
	}
	expiresAt,err:=parseTime(expires);if err!=nil{return model.TelegramPrincipal{},err};if !now.Before(expiresAt){return model.TelegramPrincipal{},fmt.Errorf("telegram pairing expired: %w",remotestore.ErrConflict)}
	result,err:=tx.ExecContext(ctx,`UPDATE telegram_pairings SET consumed_at=?,consumed_by_user_id=?,consumed_chat_id=? WHERE token_hash=? AND consumed_at IS NULL`,formatTime(now),userID,chatID,tokenHash);if err!=nil{return model.TelegramPrincipal{},err};rows,err:=result.RowsAffected();if err!=nil{return model.TelegramPrincipal{},err};if rows!=1{return model.TelegramPrincipal{},remotestore.ErrConflict}
	principal:=model.TelegramPrincipal{UserID:userID,Role:role,Enabled:true,PairedAt:now.UTC()}
	if _,err:=tx.ExecContext(ctx,`INSERT INTO telegram_principals(user_id,role,enabled,paired_at) VALUES(?,?,1,?) ON CONFLICT(user_id) DO UPDATE SET role=excluded.role,enabled=1,paired_at=excluded.paired_at`,principal.UserID,principal.Role,formatTime(principal.PairedAt));err!=nil{return model.TelegramPrincipal{},err}
	if err:=tx.Commit();err!=nil{return model.TelegramPrincipal{},err};return principal,nil
}

func (s *Store) ReserveTelegramCallback(ctx context.Context, callbackID string, userID, chatID int64, receivedAt time.Time) (bool, error) {
	if strings.TrimSpace(callbackID)==""||userID==0||chatID==0||receivedAt.IsZero(){return false,fmt.Errorf("telegram callback id, user, chat and time are required")}
	_,err:=s.db.ExecContext(ctx,`INSERT INTO telegram_callback_replay(callback_query_id,user_id,chat_id,received_at) VALUES(?,?,?,?)`,callbackID,userID,chatID,formatTime(receivedAt));if err==nil{return true,nil}
	var count int;if qerr:=s.db.QueryRowContext(ctx,`SELECT COUNT(*) FROM telegram_callback_replay WHERE callback_query_id=?`,callbackID).Scan(&count);qerr==nil&&count==1{return false,nil};return false,err
}

func (s *Store) UpsertTelegramBinding(ctx context.Context, binding model.TelegramBinding) error {
	if err:=binding.Validate();err!=nil{return err};_,err:=s.db.ExecContext(ctx,`INSERT INTO telegram_bindings(id,session_id,chat_id,thread_id,owner_user_id,enabled,created_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET session_id=excluded.session_id,chat_id=excluded.chat_id,thread_id=excluded.thread_id,owner_user_id=excluded.owner_user_id,enabled=excluded.enabled`,binding.ID,binding.SessionID,binding.ChatID,binding.ThreadID,binding.OwnerUserID,boolInt(binding.Enabled),formatTime(binding.CreatedAt));if err!=nil{return mapWriteError("upsert telegram binding",err)};return nil
}

func (s *Store) GetTelegramBindingByTopic(ctx context.Context, chatID, threadID int64) (model.TelegramBinding, error) {
	var item model.TelegramBinding;var enabled int;var created string;err:=s.db.QueryRowContext(ctx,`SELECT id,session_id,chat_id,thread_id,owner_user_id,enabled,created_at FROM telegram_bindings WHERE chat_id=? AND thread_id=? AND enabled=1`,chatID,threadID).Scan(&item.ID,&item.SessionID,&item.ChatID,&item.ThreadID,&item.OwnerUserID,&enabled,&created);if errors.Is(err,sql.ErrNoRows){return model.TelegramBinding{},remotestore.ErrNotFound};if err!=nil{return model.TelegramBinding{},err};item.Enabled=enabled!=0;item.CreatedAt,err=parseTime(created);return item,err
}
