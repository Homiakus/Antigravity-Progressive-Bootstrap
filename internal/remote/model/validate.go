package model

import (
	"fmt"
	"strings"
)

func (r Repository) Validate() error {
	if err := ValidateGeneratedID(string(r.ID), IDRepository); err != nil { return err }
	if strings.TrimSpace(r.Name) == "" { return fmt.Errorf("repository name is required") }
	if strings.TrimSpace(r.CanonicalPath) == "" { return fmt.Errorf("repository canonical path is required") }
	if r.CreatedAt.IsZero() || r.LastSeenAt.IsZero() { return fmt.Errorf("repository timestamps are required") }
	return nil
}

func (i InstanceMirror) Validate() error {
	if strings.TrimSpace(string(i.ID)) == "" { return fmt.Errorf("instance id is required") }
	if strings.TrimSpace(i.UserDataDir) == "" { return fmt.Errorf("instance user_data_dir is required") }
	if i.PID < 0 { return fmt.Errorf("instance pid cannot be negative") }
	if !i.DesiredState.Valid() { return fmt.Errorf("invalid instance desired state %q", i.DesiredState) }
	if !i.ObservedState.Valid() { return fmt.Errorf("invalid instance observed state %q", i.ObservedState) }
	if i.LastReconciledAt.IsZero() { return fmt.Errorf("instance last_reconciled_at is required") }
	return nil
}

func (c Conversation) Validate() error {
	if err := ValidateGeneratedID(string(c.ID), IDConversation); err != nil { return err }
	if strings.TrimSpace(c.ProviderConversationID) == "" { return fmt.Errorf("provider conversation id is required") }
	if strings.TrimSpace(string(c.InstanceID)) == "" { return fmt.Errorf("conversation instance id is required") }
	if err := ValidateGeneratedID(string(c.WorkspaceID), IDWorkspace); err != nil { return fmt.Errorf("conversation workspace: %w", err) }
	if !c.State.Valid() { return fmt.Errorf("invalid conversation state %q", c.State) }
	if !c.MirrorMode.Valid() { return fmt.Errorf("invalid conversation mirror mode %q", c.MirrorMode) }
	if c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() || c.LastActivityAt.IsZero() { return fmt.Errorf("conversation timestamps are required") }
	return nil
}

func (s RemoteSession) Validate() error {
	if err := ValidateGeneratedID(string(s.ID), IDRemoteSession); err != nil { return err }
	if strings.TrimSpace(string(s.HostID)) == "" { return fmt.Errorf("session host id is required") }
	if strings.TrimSpace(string(s.CockpitInstanceID)) == "" { return fmt.Errorf("session cockpit instance id is required") }
	if err := ValidateGeneratedID(string(s.RepositoryID), IDRepository); err != nil { return fmt.Errorf("session repository: %w", err) }
	if err := ValidateGeneratedID(string(s.WorkspaceID), IDWorkspace); err != nil { return fmt.Errorf("session workspace: %w", err) }
	if strings.TrimSpace(s.WorkspacePath) == "" { return fmt.Errorf("session workspace path is required") }
	if err := ValidateGeneratedID(string(s.ConversationID), IDConversation); err != nil { return fmt.Errorf("session conversation: %w", err) }
	if !s.DesiredState.Valid() { return fmt.Errorf("invalid session desired state %q", s.DesiredState) }
	if !s.ObservedState.Valid() { return fmt.Errorf("invalid session observed state %q", s.ObservedState) }
	if !s.IsolationMode.Valid() { return fmt.Errorf("invalid session isolation mode %q", s.IsolationMode) }
	if s.CreatedAt.IsZero() || s.UpdatedAt.IsZero() { return fmt.Errorf("session timestamps are required") }
	return nil
}

func (b TelegramBinding) Validate() error {
	if err := ValidateGeneratedID(string(b.ID), IDTelegramBinding); err != nil { return err }
	if err := ValidateGeneratedID(string(b.SessionID), IDRemoteSession); err != nil { return fmt.Errorf("telegram binding session: %w", err) }
	if b.ChatID == 0 || b.OwnerUserID == 0 { return fmt.Errorf("telegram chat and owner ids are required") }
	if b.CreatedAt.IsZero() { return fmt.Errorf("telegram binding created_at is required") }
	return nil
}

func (c RemoteCommand) Validate() error {
	if err := ValidateGeneratedID(string(c.ID), IDRemoteCommand); err != nil { return err }
	if strings.TrimSpace(c.Source) == "" || strings.TrimSpace(c.SourceMessageID) == "" { return fmt.Errorf("command source and source message id are required") }
	if err := ValidateGeneratedID(string(c.SessionID), IDRemoteSession); err != nil { return fmt.Errorf("remote command session: %w", err) }
	if strings.TrimSpace(c.Kind) == "" { return fmt.Errorf("command kind is required") }
	if !c.State.Valid() { return fmt.Errorf("invalid command state %q", c.State) }
	if c.RequestedAt.IsZero() { return fmt.Errorf("command requested_at is required") }
	return nil
}

func (e RemoteEvent) Validate() error {
	if err := ValidateGeneratedID(string(e.ID), IDRemoteEvent); err != nil { return err }
	if err := ValidateGeneratedID(string(e.SessionID), IDRemoteSession); err != nil { return fmt.Errorf("remote event session: %w", err) }
	if e.Seq == 0 { return fmt.Errorf("remote event seq must be positive") }
	if e.Source == "" || e.Type == "" || e.Timestamp.IsZero() { return fmt.Errorf("remote event source, type and timestamp are required") }
	return nil
}
