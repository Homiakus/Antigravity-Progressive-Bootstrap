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

func (s *Store) GetTelegramBindingBySession(ctx context.Context, sessionID model.RemoteSessionID) (model.TelegramBinding, error) {
	var item model.TelegramBinding
	var enabled int
	var created string
	err := s.db.QueryRowContext(ctx, `SELECT id,session_id,chat_id,thread_id,owner_user_id,enabled,created_at FROM telegram_bindings WHERE session_id=? AND enabled=1 ORDER BY created_at DESC,id LIMIT 1`, sessionID).Scan(&item.ID,&item.SessionID,&item.ChatID,&item.ThreadID,&item.OwnerUserID,&enabled,&created)
	if errors.Is(err, sql.ErrNoRows) { return model.TelegramBinding{}, remotestore.ErrNotFound }
	if err != nil { return model.TelegramBinding{}, err }
	item.Enabled = enabled != 0
	item.CreatedAt, err = parseTime(created)
	return item, err
}

func (s *Store) GetTelegramMirrorState(ctx context.Context, sessionID model.RemoteSessionID) (model.TelegramMirrorState, error) {
	var item model.TelegramMirrorState
	var updated string
	err := s.db.QueryRowContext(ctx, `SELECT session_id,chat_id,thread_id,message_id,last_event_seq,rendered_text,updated_at FROM telegram_mirror_state WHERE session_id=?`, sessionID).Scan(&item.SessionID,&item.ChatID,&item.ThreadID,&item.MessageID,&item.LastEventSeq,&item.RenderedText,&updated)
	if errors.Is(err, sql.ErrNoRows) { return model.TelegramMirrorState{}, remotestore.ErrNotFound }
	if err != nil { return model.TelegramMirrorState{}, err }
	item.UpdatedAt, err = parseTime(updated)
	return item, err
}

func (s *Store) UpsertTelegramMirrorState(ctx context.Context, state model.TelegramMirrorState) error {
	if err := state.Validate(); err != nil { return err }
	_, err := s.db.ExecContext(ctx, `INSERT INTO telegram_mirror_state(session_id,chat_id,thread_id,message_id,last_event_seq,rendered_text,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(session_id) DO UPDATE SET chat_id=excluded.chat_id,thread_id=excluded.thread_id,message_id=excluded.message_id,last_event_seq=CASE WHEN excluded.last_event_seq >= telegram_mirror_state.last_event_seq THEN excluded.last_event_seq ELSE telegram_mirror_state.last_event_seq END,rendered_text=CASE WHEN excluded.last_event_seq >= telegram_mirror_state.last_event_seq THEN excluded.rendered_text ELSE telegram_mirror_state.rendered_text END,updated_at=CASE WHEN excluded.last_event_seq >= telegram_mirror_state.last_event_seq THEN excluded.updated_at ELSE telegram_mirror_state.updated_at END`, state.SessionID,state.ChatID,state.ThreadID,state.MessageID,state.LastEventSeq,state.RenderedText,formatTime(state.UpdatedAt))
	return err
}

func (s *Store) AppendRemoteEventWithOutbox(ctx context.Context, event model.RemoteEvent, transport string, outboxPayload []byte) (model.RemoteEvent, bool, error) {
	transport = strings.TrimSpace(transport)
	if err := validateRemoteEventDraft(event); err != nil { return model.RemoteEvent{}, false, err }
	if transport == "" { return model.RemoteEvent{}, false, fmt.Errorf("remote outbox transport is required") }
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil { return model.RemoteEvent{}, false, err }
	defer tx.Rollback()

	if event.SourceEventID != "" {
		existing, err := scanRemoteEvent(tx.QueryRowContext(ctx, remoteEventSelect+` WHERE source=? AND source_event_id=?`, event.Source, event.SourceEventID))
		if err == nil {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO remote_outbox(event_id,transport,payload,created_at) VALUES(?,?,?,?)`, existing.ID,transport,outboxPayload,formatTime(existing.Timestamp)); err != nil { return model.RemoteEvent{}, false, err }
			if err := tx.Commit(); err != nil { return model.RemoteEvent{}, false, err }
			return existing, false, nil
		}
		if !errors.Is(err, remotestore.ErrNotFound) { return model.RemoteEvent{}, false, err }
	}

	var seq uint64
	err = tx.QueryRowContext(ctx, `INSERT INTO remote_events(event_id,session_id,session_seq,source,type,source_event_id,payload,timestamp) SELECT ?,?,COALESCE(MAX(session_seq),0)+1,?,?,?,?,? FROM remote_events WHERE session_id=? RETURNING session_seq`, event.ID,event.SessionID,event.Source,event.Type,event.SourceEventID,[]byte(event.Payload),formatTime(event.Timestamp),event.SessionID).Scan(&seq)
	if err != nil { return model.RemoteEvent{}, false, mapWriteError("append remote event with outbox", err) }
	event.Seq = seq
	if err := event.Validate(); err != nil { return model.RemoteEvent{}, false, err }
	if _, err := tx.ExecContext(ctx, `INSERT INTO remote_outbox(event_id,transport,payload,created_at) VALUES(?,?,?,?)`, event.ID,transport,outboxPayload,formatTime(event.Timestamp)); err != nil { return model.RemoteEvent{}, false, mapWriteError("enqueue remote outbox", err) }
	if err := tx.Commit(); err != nil { return model.RemoteEvent{}, false, err }
	return event, true, nil
}

func validateRemoteEventDraft(event model.RemoteEvent) error {
	if err := model.ValidateGeneratedID(string(event.ID), model.IDRemoteEvent); err != nil { return err }
	if err := model.ValidateGeneratedID(string(event.SessionID), model.IDRemoteSession); err != nil { return fmt.Errorf("remote event session: %w", err) }
	if event.Source == "" || event.Type == "" || event.Timestamp.IsZero() { return fmt.Errorf("remote event source, type and timestamp are required") }
	return nil
}

func (s *Store) ListRemoteOutbox(ctx context.Context, transport string, now time.Time, limit int) ([]model.RemoteOutboxItem, error) {
	if strings.TrimSpace(transport)=="" || now.IsZero() { return nil, fmt.Errorf("remote outbox transport and time are required") }
	if limit <= 0 || limit > 1000 { limit = 100 }
	rows, err := s.db.QueryContext(ctx, `SELECT outbox_id,event_id,transport,payload,created_at,delivered_at,attempt_count,next_attempt_at FROM remote_outbox WHERE transport=? AND delivered_at IS NULL AND (next_attempt_at IS NULL OR next_attempt_at<=?) ORDER BY outbox_id LIMIT ?`, transport,formatTime(now),limit)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []model.RemoteOutboxItem
	for rows.Next() { item,err:=scanRemoteOutbox(rows);if err!=nil{return nil,err};out=append(out,item) }
	return out,rows.Err()
}

func scanRemoteOutbox(row scanner) (model.RemoteOutboxItem,error) {
	var item model.RemoteOutboxItem
	var payload []byte
	var created string
	var delivered,next sql.NullString
	if err:=row.Scan(&item.ID,&item.EventID,&item.Transport,&payload,&created,&delivered,&item.AttemptCount,&next);err!=nil{if errors.Is(err,sql.ErrNoRows){return model.RemoteOutboxItem{},remotestore.ErrNotFound};return model.RemoteOutboxItem{},err}
	item.Payload=append(item.Payload[:0],payload...)
	var err error
	if item.CreatedAt,err=parseTime(created);err!=nil{return model.RemoteOutboxItem{},err}
	if delivered.Valid{v,e:=parseTime(delivered.String);if e!=nil{return model.RemoteOutboxItem{},e};item.DeliveredAt=&v}
	if next.Valid{v,e:=parseTime(next.String);if e!=nil{return model.RemoteOutboxItem{},e};item.NextAttemptAt=&v}
	return item,nil
}

func (s *Store) MarkRemoteOutboxDelivered(ctx context.Context, id int64, deliveredAt time.Time) error {
	if id<=0||deliveredAt.IsZero(){return fmt.Errorf("remote outbox id and delivered time are required")}
	result,err:=s.db.ExecContext(ctx,`UPDATE remote_outbox SET delivered_at=?,next_attempt_at=NULL WHERE outbox_id=? AND delivered_at IS NULL`,formatTime(deliveredAt),id);if err!=nil{return err};rows,err:=result.RowsAffected();if err!=nil{return err};if rows==0{return remotestore.ErrConflict};return nil
}

func (s *Store) ScheduleRemoteOutboxRetry(ctx context.Context, id int64, nextAttempt time.Time) error {
	if id<=0||nextAttempt.IsZero(){return fmt.Errorf("remote outbox id and retry time are required")}
	result,err:=s.db.ExecContext(ctx,`UPDATE remote_outbox SET attempt_count=attempt_count+1,next_attempt_at=? WHERE outbox_id=? AND delivered_at IS NULL`,formatTime(nextAttempt),id);if err!=nil{return err};rows,err:=result.RowsAffected();if err!=nil{return err};if rows==0{return remotestore.ErrConflict};return nil
}
