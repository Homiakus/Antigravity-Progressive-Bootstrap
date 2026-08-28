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

func (s *Store) AdmitRemoteCommand(ctx context.Context, command model.RemoteCommand) (model.RemoteCommand, bool, error) {
	if err := command.Validate(); err != nil { return model.RemoteCommand{}, false, err }
	_, err := s.db.ExecContext(ctx, `INSERT INTO remote_commands(id,source,source_message_id,session_id,kind,payload,state,requested_at,started_at,completed_at,error) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, command.ID,command.Source,command.SourceMessageID,command.SessionID,command.Kind,[]byte(command.Payload),command.State,formatTime(command.RequestedAt),nil,nil,command.Error)
	if err == nil { return command, true, nil }
	existing, getErr := s.getRemoteCommandBySource(ctx, command.Source, command.SourceMessageID)
	if getErr == nil { return existing, false, nil }
	return model.RemoteCommand{}, false, mapWriteError("admit remote command", err)
}

func (s *Store) GetRemoteCommand(ctx context.Context, id model.RemoteCommandID) (model.RemoteCommand, error) {
	return scanRemoteCommand(s.db.QueryRowContext(ctx, remoteCommandSelect+` WHERE id=?`, id))
}

func (s *Store) getRemoteCommandBySource(ctx context.Context, source, messageID string) (model.RemoteCommand, error) {
	return scanRemoteCommand(s.db.QueryRowContext(ctx, remoteCommandSelect+` WHERE source=? AND source_message_id=?`, source,messageID))
}

func (s *Store) ListPendingRemoteCommands(ctx context.Context, limit int) ([]model.RemoteCommand, error) {
	if limit <= 0 || limit > 1000 { limit = 100 }
	rows, err := s.db.QueryContext(ctx, remoteCommandSelect+` WHERE state='PENDING' ORDER BY requested_at,id LIMIT ?`, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []model.RemoteCommand
	for rows.Next() { item, err := scanRemoteCommand(rows); if err != nil { return nil, err }; out=append(out,item) }
	return out, rows.Err()
}

func (s *Store) UpdateRemoteCommandState(ctx context.Context, id model.RemoteCommandID, state model.CommandState, message string, at time.Time) error {
	if !state.Valid() || at.IsZero() { return fmt.Errorf("invalid remote command transition") }
	var result sql.Result
	var err error
	switch state {
	case model.CommandRunning:
		result, err = s.db.ExecContext(ctx, `UPDATE remote_commands SET state=?,started_at=?,error=? WHERE id=? AND state='PENDING'`,state,formatTime(at),message,id)
	case model.CommandSucceeded, model.CommandFailed, model.CommandCancelled:
		result, err = s.db.ExecContext(ctx, `UPDATE remote_commands SET state=?,completed_at=?,error=? WHERE id=? AND state IN ('PENDING','RUNNING')`,state,formatTime(at),message,id)
	default:
		return fmt.Errorf("unsupported remote command transition to %s", state)
	}
	if err != nil { return err }
	rows, err := result.RowsAffected(); if err != nil { return err }; if rows==0 { return remotestore.ErrConflict }; return nil
}

const remoteCommandSelect = `SELECT id,source,source_message_id,session_id,kind,payload,state,requested_at,started_at,completed_at,error FROM remote_commands`

func scanRemoteCommand(row scanner) (model.RemoteCommand,error) {
	var item model.RemoteCommand
	var payload []byte
	var requested string
	var started, completed sql.NullString
	if err := row.Scan(&item.ID,&item.Source,&item.SourceMessageID,&item.SessionID,&item.Kind,&payload,&item.State,&requested,&started,&completed,&item.Error); err != nil {
		if errors.Is(err,sql.ErrNoRows) { return model.RemoteCommand{},remotestore.ErrNotFound }; return model.RemoteCommand{},err
	}
	item.Payload=append(item.Payload[:0],payload...)
	var err error
	if item.RequestedAt,err=parseTime(requested);err!=nil{return model.RemoteCommand{},err}
	if started.Valid { t,e:=parseTime(started.String);if e!=nil{return model.RemoteCommand{},e};item.StartedAt=&t }
	if completed.Valid { t,e:=parseTime(completed.String);if e!=nil{return model.RemoteCommand{},e};item.CompletedAt=&t }
	return item,nil
}

func (s *Store) AppendRemoteEvent(ctx context.Context, event model.RemoteEvent) (model.RemoteEvent, bool, error) {
	if strings.TrimSpace(string(event.ID))=="" || strings.TrimSpace(string(event.SessionID))=="" || event.Source=="" || event.Type=="" || event.Timestamp.IsZero() { return model.RemoteEvent{},false,fmt.Errorf("invalid remote event draft") }
	if event.SourceEventID!="" {
		if existing,err:=s.getRemoteEventBySource(ctx,event.Source,event.SourceEventID);err==nil{return existing,false,nil}else if !errors.Is(err,remotestore.ErrNotFound){return model.RemoteEvent{},false,err}
	}
	var seq uint64
	err := s.db.QueryRowContext(ctx, `INSERT INTO remote_events(event_id,session_id,session_seq,source,type,source_event_id,payload,timestamp)
SELECT ?,?,COALESCE(MAX(session_seq),0)+1,?,?,?,?,? FROM remote_events WHERE session_id=? RETURNING session_seq`, event.ID,event.SessionID,event.Source,event.Type,event.SourceEventID,[]byte(event.Payload),formatTime(event.Timestamp),event.SessionID).Scan(&seq)
	if err != nil {
		if event.SourceEventID!="" { if existing,getErr:=s.getRemoteEventBySource(ctx,event.Source,event.SourceEventID);getErr==nil{return existing,false,nil} }
		return model.RemoteEvent{},false,mapWriteError("append remote event",err)
	}
	event.Seq=seq
	if err:=event.Validate();err!=nil{return model.RemoteEvent{},false,err}
	return event,true,nil
}

func (s *Store) getRemoteEventBySource(ctx context.Context, source model.EventSource, sourceEventID string)(model.RemoteEvent,error){
	return scanRemoteEvent(s.db.QueryRowContext(ctx,remoteEventSelect+` WHERE source=? AND source_event_id=?`,source,sourceEventID))
}

func (s *Store) ListRemoteEventsAfter(ctx context.Context, sessionID model.RemoteSessionID, after uint64, limit int)([]model.RemoteEvent,error){
	if limit<=0||limit>5000{limit=500}
	rows,err:=s.db.QueryContext(ctx,remoteEventSelect+` WHERE session_id=? AND session_seq>? ORDER BY session_seq LIMIT ?`,sessionID,after,limit);if err!=nil{return nil,err};defer rows.Close()
	var out []model.RemoteEvent
	for rows.Next(){item,err:=scanRemoteEvent(rows);if err!=nil{return nil,err};out=append(out,item)}
	return out,rows.Err()
}

const remoteEventSelect=`SELECT event_id,session_id,session_seq,source,type,source_event_id,payload,timestamp FROM remote_events`
func scanRemoteEvent(row scanner)(model.RemoteEvent,error){var item model.RemoteEvent;var payload []byte;var stamp string;if err:=row.Scan(&item.ID,&item.SessionID,&item.Seq,&item.Source,&item.Type,&item.SourceEventID,&payload,&stamp);err!=nil{if errors.Is(err,sql.ErrNoRows){return model.RemoteEvent{},remotestore.ErrNotFound};return model.RemoteEvent{},err};item.Payload=append(item.Payload[:0],payload...);var err error;item.Timestamp,err=parseTime(stamp);return item,err}
