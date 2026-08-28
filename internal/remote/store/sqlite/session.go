package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

func (s *Store) UpsertInstance(ctx context.Context, instance model.InstanceMirror) error {
	if err := instance.Validate(); err != nil { return err }
	_, err := s.db.ExecContext(ctx, `INSERT INTO remote_instances(cockpit_instance_id,name,user_data_dir,working_dir,account_id,pid,desired_state,observed_state,bridge_id,last_reconciled_at,last_error)
VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(cockpit_instance_id) DO UPDATE SET name=excluded.name,user_data_dir=excluded.user_data_dir,working_dir=excluded.working_dir,account_id=excluded.account_id,pid=excluded.pid,desired_state=excluded.desired_state,observed_state=excluded.observed_state,bridge_id=excluded.bridge_id,last_reconciled_at=excluded.last_reconciled_at,last_error=excluded.last_error`,
		instance.ID, instance.Name, instance.UserDataDir, instance.WorkingDir, instance.AccountID, instance.PID, instance.DesiredState, instance.ObservedState, instance.BridgeID, formatTime(instance.LastReconciledAt), instance.LastError)
	if err != nil { return mapWriteError("upsert remote instance", err) }
	return nil
}

func (s *Store) GetInstance(ctx context.Context, id model.InstanceID) (model.InstanceMirror, error) {
	return scanInstance(s.db.QueryRowContext(ctx, `SELECT cockpit_instance_id,name,user_data_dir,working_dir,account_id,pid,desired_state,observed_state,bridge_id,last_reconciled_at,last_error FROM remote_instances WHERE cockpit_instance_id=?`, id))
}

func (s *Store) ListInstances(ctx context.Context) ([]model.InstanceMirror, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT cockpit_instance_id,name,user_data_dir,working_dir,account_id,pid,desired_state,observed_state,bridge_id,last_reconciled_at,last_error FROM remote_instances ORDER BY cockpit_instance_id`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []model.InstanceMirror
	for rows.Next() { item, err := scanInstance(rows); if err != nil { return nil, err }; out = append(out, item) }
	return out, rows.Err()
}

func scanInstance(row scanner) (model.InstanceMirror, error) {
	var item model.InstanceMirror
	var reconciled string
	if err := row.Scan(&item.ID,&item.Name,&item.UserDataDir,&item.WorkingDir,&item.AccountID,&item.PID,&item.DesiredState,&item.ObservedState,&item.BridgeID,&reconciled,&item.LastError); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return model.InstanceMirror{}, remotestore.ErrNotFound }
		return model.InstanceMirror{}, err
	}
	var err error
	item.LastReconciledAt, err = parseTime(reconciled)
	return item, err
}

func (s *Store) UpsertConversation(ctx context.Context, conversation model.Conversation) error {
	if err := conversation.Validate(); err != nil { return err }
	_, err := s.db.ExecContext(ctx, `INSERT INTO remote_conversations(id,provider_conversation_id,cockpit_instance_id,workspace_id,title,state,mirror_mode,last_activity_at,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET provider_conversation_id=excluded.provider_conversation_id,cockpit_instance_id=excluded.cockpit_instance_id,workspace_id=excluded.workspace_id,title=excluded.title,state=excluded.state,mirror_mode=excluded.mirror_mode,last_activity_at=excluded.last_activity_at,updated_at=excluded.updated_at`,
		conversation.ID, conversation.ProviderConversationID, conversation.InstanceID, conversation.WorkspaceID, conversation.Title, conversation.State, conversation.MirrorMode, formatTime(conversation.LastActivityAt), formatTime(conversation.CreatedAt), formatTime(conversation.UpdatedAt))
	if err != nil { return mapWriteError("upsert remote conversation", err) }
	return nil
}

func (s *Store) GetConversation(ctx context.Context, id model.ConversationID) (model.Conversation, error) {
	return scanConversation(s.db.QueryRowContext(ctx, conversationSelect+` WHERE id=?`, id))
}

func (s *Store) GetConversationByProvider(ctx context.Context, instanceID model.InstanceID, providerID string) (model.Conversation, error) {
	return scanConversation(s.db.QueryRowContext(ctx, conversationSelect+` WHERE cockpit_instance_id=? AND provider_conversation_id=?`, instanceID, providerID))
}

func (s *Store) ListConversationsByInstance(ctx context.Context, instanceID model.InstanceID) ([]model.Conversation, error) {
	rows, err := s.db.QueryContext(ctx, conversationSelect+` WHERE cockpit_instance_id=? ORDER BY last_activity_at DESC,id`, instanceID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []model.Conversation
	for rows.Next() { item, err := scanConversation(rows); if err != nil { return nil, err }; out = append(out, item) }
	return out, rows.Err()
}

const conversationSelect = `SELECT id,provider_conversation_id,cockpit_instance_id,workspace_id,title,state,mirror_mode,last_activity_at,created_at,updated_at FROM remote_conversations`

func scanConversation(row scanner) (model.Conversation, error) {
	var item model.Conversation
	var activity, created, updated string
	if err := row.Scan(&item.ID,&item.ProviderConversationID,&item.InstanceID,&item.WorkspaceID,&item.Title,&item.State,&item.MirrorMode,&activity,&created,&updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return model.Conversation{}, remotestore.ErrNotFound }
		return model.Conversation{}, err
	}
	var err error
	if item.LastActivityAt, err = parseTime(activity); err != nil { return model.Conversation{}, err }
	if item.CreatedAt, err = parseTime(created); err != nil { return model.Conversation{}, err }
	if item.UpdatedAt, err = parseTime(updated); err != nil { return model.Conversation{}, err }
	return item, nil
}

func (s *Store) CreateSession(ctx context.Context, session model.RemoteSession) error {
	if err := session.Validate(); err != nil { return err }
	var workflow any
	if session.WorkflowRunID != "" { workflow = session.WorkflowRunID }
	_, err := s.db.ExecContext(ctx, `INSERT INTO remote_sessions(id,host_id,cockpit_instance_id,cockpit_account_id,repository_id,workspace_id,workspace_path,conversation_id,workflow_run_id,desired_state,observed_state,isolation_mode,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		session.ID,session.HostID,session.CockpitInstanceID,session.CockpitAccountID,session.RepositoryID,session.WorkspaceID,session.WorkspacePath,session.ConversationID,workflow,session.DesiredState,session.ObservedState,session.IsolationMode,formatTime(session.CreatedAt),formatTime(session.UpdatedAt))
	if err != nil { return mapWriteError("create remote session", err) }
	return nil
}

func (s *Store) GetSession(ctx context.Context, id model.RemoteSessionID) (model.RemoteSession, error) {
	return scanSession(s.db.QueryRowContext(ctx, sessionSelect+` WHERE id=?`, id))
}

func (s *Store) UpdateSessionStates(ctx context.Context, id model.RemoteSessionID, desired model.SessionDesiredState, observed model.SessionObservedState, updated time.Time) error {
	if !desired.Valid() || !observed.Valid() || updated.IsZero() { return fmt.Errorf("invalid remote session state update") }
	result, err := s.db.ExecContext(ctx, `UPDATE remote_sessions SET desired_state=?,observed_state=?,updated_at=? WHERE id=?`,desired,observed,formatTime(updated),id)
	if err != nil { return err }
	rows, err := result.RowsAffected(); if err != nil { return err }; if rows == 0 { return remotestore.ErrNotFound }; return nil
}

func (s *Store) ListSessionsByInstance(ctx context.Context, instanceID model.InstanceID, includeClosed bool) ([]model.RemoteSession, error) {
	query := sessionSelect+` WHERE cockpit_instance_id=?`
	if !includeClosed { query += ` AND observed_state <> 'CLOSED'` }
	query += ` ORDER BY updated_at DESC,id`
	rows, err := s.db.QueryContext(ctx, query, instanceID); if err != nil { return nil, err }; defer rows.Close()
	var out []model.RemoteSession
	for rows.Next() { item, err := scanSession(rows); if err != nil { return nil, err }; out = append(out,item) }
	return out, rows.Err()
}

const sessionSelect = `SELECT id,host_id,cockpit_instance_id,cockpit_account_id,repository_id,workspace_id,workspace_path,conversation_id,workflow_run_id,desired_state,observed_state,isolation_mode,created_at,updated_at FROM remote_sessions`

func scanSession(row scanner) (model.RemoteSession, error) {
	var item model.RemoteSession
	var workflow sql.NullString
	var created, updated string
	if err := row.Scan(&item.ID,&item.HostID,&item.CockpitInstanceID,&item.CockpitAccountID,&item.RepositoryID,&item.WorkspaceID,&item.WorkspacePath,&item.ConversationID,&workflow,&item.DesiredState,&item.ObservedState,&item.IsolationMode,&created,&updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return model.RemoteSession{}, remotestore.ErrNotFound }
		return model.RemoteSession{}, err
	}
	if workflow.Valid { item.WorkflowRunID = workflow.String }
	var err error
	if item.CreatedAt, err = parseTime(created); err != nil { return model.RemoteSession{}, err }
	if item.UpdatedAt, err = parseTime(updated); err != nil { return model.RemoteSession{}, err }
	return item,nil
}
