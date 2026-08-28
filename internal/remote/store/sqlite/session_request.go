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

func (s *Store) AdmitSessionRequest(ctx context.Context, request model.RemoteSessionRequest) (model.RemoteSessionRequest, bool, error) {
	if request.State == "" {
		request.State = model.SessionRequestPending
	}
	if err := request.Validate(); err != nil {
		return model.RemoteSessionRequest{}, false, err
	}
	if request.State != model.SessionRequestPending || request.SessionID != "" || request.StartedAt != nil || request.CompletedAt != nil {
		return model.RemoteSessionRequest{}, false, fmt.Errorf("new remote session request must be pristine PENDING")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO remote_session_requests(
		id,source,source_message_id,repository_id,account_id,chat_id,thread_id,requester_user_id,
		instance_strategy,instance_id,conversation_strategy,provider_conversation_id,isolation_mode,state,session_id,
		requested_at,started_at,completed_at,error) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		request.ID, request.Source, request.SourceMessageID, request.RepositoryID, request.AccountID, request.ChatID, request.ThreadID,
		request.RequesterUserID, request.InstanceStrategy, request.InstanceID, request.ConversationStrategy, request.ProviderConversationID,
		request.IsolationMode, request.State, nil, formatTime(request.RequestedAt), nil, nil, request.Error)
	if err == nil {
		return request, true, nil
	}
	existing, getErr := s.getSessionRequestBySource(ctx, request.Source, request.SourceMessageID)
	if getErr == nil {
		return existing, false, nil
	}
	return model.RemoteSessionRequest{}, false, mapWriteError("admit remote session request", err)
}

func (s *Store) GetSessionRequest(ctx context.Context, id model.RemoteSessionRequestID) (model.RemoteSessionRequest, error) {
	return scanSessionRequest(s.db.QueryRowContext(ctx, sessionRequestSelect+` WHERE id=?`, id))
}

func (s *Store) getSessionRequestBySource(ctx context.Context, source, sourceMessageID string) (model.RemoteSessionRequest, error) {
	return scanSessionRequest(s.db.QueryRowContext(ctx, sessionRequestSelect+` WHERE source=? AND source_message_id=?`, source, sourceMessageID))
}

func (s *Store) ClaimPendingSessionRequest(ctx context.Context, at time.Time) (model.RemoteSessionRequest, error) {
	if at.IsZero() {
		return model.RemoteSessionRequest{}, fmt.Errorf("claim remote session request requires time")
	}
	row := s.db.QueryRowContext(ctx, `UPDATE remote_session_requests
SET state='PROVISIONING', started_at=COALESCE(started_at,?), error=''
WHERE id=(SELECT id FROM remote_session_requests WHERE state='PENDING' ORDER BY requested_at,id LIMIT 1)
RETURNING id,source,source_message_id,repository_id,account_id,chat_id,thread_id,requester_user_id,
instance_strategy,instance_id,conversation_strategy,provider_conversation_id,isolation_mode,state,session_id,
requested_at,started_at,completed_at,error`, formatTime(at))
	return scanSessionRequest(row)
}

func (s *Store) ListSessionRequests(ctx context.Context, states []model.SessionRequestState, limit int) ([]model.RemoteSessionRequest, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if len(states) == 0 {
		return nil, fmt.Errorf("remote session request states are required")
	}
	placeholders := make([]string, 0, len(states))
	args := make([]any, 0, len(states)+1)
	for _, state := range states {
		if !state.Valid() {
			return nil, fmt.Errorf("invalid remote session request state %q", state)
		}
		placeholders = append(placeholders, "?")
		args = append(args, state)
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, sessionRequestSelect+` WHERE state IN (`+strings.Join(placeholders, ",")+`) ORDER BY requested_at,id LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.RemoteSessionRequest
	for rows.Next() {
		item, err := scanSessionRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) AttachSessionToRequest(ctx context.Context, id model.RemoteSessionRequestID, sessionID model.RemoteSessionID) error {
	if err := model.ValidateGeneratedID(string(id), model.IDRemoteSessionRequest); err != nil {
		return err
	}
	if err := model.ValidateGeneratedID(string(sessionID), model.IDRemoteSession); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE remote_session_requests SET state='BINDING',session_id=?,error='' WHERE id=? AND state='PROVISIONING' AND session_id IS NULL`, sessionID, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 1 {
		return nil
	}
	existing, getErr := s.GetSessionRequest(ctx, id)
	if getErr == nil && existing.SessionID == sessionID && (existing.State == model.SessionRequestBinding || existing.State == model.SessionRequestSucceeded) {
		return nil
	}
	return remotestore.ErrConflict
}

func (s *Store) CompleteSessionRequest(ctx context.Context, id model.RemoteSessionRequestID, at time.Time) error {
	if at.IsZero() {
		return fmt.Errorf("complete remote session request requires time")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE remote_session_requests SET state='SUCCEEDED',completed_at=?,error='' WHERE id=? AND state='BINDING' AND session_id IS NOT NULL`, formatTime(at), id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 1 {
		return nil
	}
	existing, getErr := s.GetSessionRequest(ctx, id)
	if getErr == nil && existing.State == model.SessionRequestSucceeded {
		return nil
	}
	return remotestore.ErrConflict
}

func (s *Store) FailSessionRequest(ctx context.Context, id model.RemoteSessionRequestID, message string, at time.Time) error {
	if at.IsZero() {
		return fmt.Errorf("fail remote session request requires time")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE remote_session_requests SET state='FAILED',completed_at=?,error=? WHERE id=? AND state IN ('PENDING','PROVISIONING','BINDING')`, formatTime(at), strings.TrimSpace(message), id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return remotestore.ErrConflict
	}
	return nil
}

const sessionRequestSelect = `SELECT id,source,source_message_id,repository_id,account_id,chat_id,thread_id,requester_user_id,instance_strategy,instance_id,conversation_strategy,provider_conversation_id,isolation_mode,state,session_id,requested_at,started_at,completed_at,error FROM remote_session_requests`

func scanSessionRequest(row scanner) (model.RemoteSessionRequest, error) {
	var item model.RemoteSessionRequest
	var session sql.NullString
	var requested string
	var started, completed sql.NullString
	if err := row.Scan(&item.ID, &item.Source, &item.SourceMessageID, &item.RepositoryID, &item.AccountID, &item.ChatID, &item.ThreadID,
		&item.RequesterUserID, &item.InstanceStrategy, &item.InstanceID, &item.ConversationStrategy, &item.ProviderConversationID,
		&item.IsolationMode, &item.State, &session, &requested, &started, &completed, &item.Error); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.RemoteSessionRequest{}, remotestore.ErrNotFound
		}
		return model.RemoteSessionRequest{}, err
	}
	if session.Valid {
		item.SessionID = model.RemoteSessionID(session.String)
	}
	var err error
	if item.RequestedAt, err = parseTime(requested); err != nil {
		return model.RemoteSessionRequest{}, err
	}
	if started.Valid {
		t, parseErr := parseTime(started.String)
		if parseErr != nil {
			return model.RemoteSessionRequest{}, parseErr
		}
		item.StartedAt = &t
	}
	if completed.Valid {
		t, parseErr := parseTime(completed.String)
		if parseErr != nil {
			return model.RemoteSessionRequest{}, parseErr
		}
		item.CompletedAt = &t
	}
	return item, nil
}
