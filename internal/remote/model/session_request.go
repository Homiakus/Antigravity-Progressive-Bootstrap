package model

import (
	"fmt"
	"strings"
	"time"
)

type SessionRequestState string

const (
	SessionRequestPending      SessionRequestState = "PENDING"
	SessionRequestProvisioning SessionRequestState = "PROVISIONING"
	SessionRequestBinding      SessionRequestState = "BINDING"
	SessionRequestSucceeded    SessionRequestState = "SUCCEEDED"
	SessionRequestFailed       SessionRequestState = "FAILED"
	SessionRequestCancelled    SessionRequestState = "CANCELLED"
)

func (s SessionRequestState) Valid() bool {
	switch s {
	case SessionRequestPending, SessionRequestProvisioning, SessionRequestBinding,
		SessionRequestSucceeded, SessionRequestFailed, SessionRequestCancelled:
		return true
	default:
		return false
	}
}

type RemoteSessionRequest struct {
	ID                     RemoteSessionRequestID
	Source                 string
	SourceMessageID        string
	RepositoryID           RepositoryID
	AccountID              string
	ChatID                 int64
	ThreadID               int64
	RequesterUserID        int64
	InstanceStrategy       string
	InstanceID             string
	ConversationStrategy   string
	ProviderConversationID string
	IsolationMode          IsolationMode
	State                  SessionRequestState
	SessionID              RemoteSessionID
	RequestedAt            time.Time
	StartedAt              *time.Time
	CompletedAt            *time.Time
	Error                  string
}

func (r RemoteSessionRequest) Validate() error {
	if err := ValidateGeneratedID(string(r.ID), IDRemoteSessionRequest); err != nil {
		return err
	}
	if strings.TrimSpace(r.Source) == "" || strings.TrimSpace(r.SourceMessageID) == "" {
		return fmt.Errorf("remote session request source and source message id are required")
	}
	if err := ValidateGeneratedID(string(r.RepositoryID), IDRepository); err != nil {
		return err
	}
	if strings.TrimSpace(r.AccountID) == "" || r.ChatID == 0 || r.RequesterUserID == 0 {
		return fmt.Errorf("remote session request account, chat and requester are required")
	}
	switch r.InstanceStrategy {
	case "AUTO", "DEDICATED":
	case "EXISTING":
		if strings.TrimSpace(r.InstanceID) == "" {
			return fmt.Errorf("EXISTING instance strategy requires instance id")
		}
	default:
		return fmt.Errorf("invalid remote session request instance strategy %q", r.InstanceStrategy)
	}
	switch r.ConversationStrategy {
	case "NEW":
	case "EXISTING":
		if strings.TrimSpace(r.ProviderConversationID) == "" {
			return fmt.Errorf("EXISTING conversation strategy requires provider conversation id")
		}
	default:
		return fmt.Errorf("invalid remote session request conversation strategy %q", r.ConversationStrategy)
	}
	if !r.IsolationMode.Valid() {
		return fmt.Errorf("invalid remote session request isolation mode %q", r.IsolationMode)
	}
	if !r.State.Valid() || r.RequestedAt.IsZero() {
		return fmt.Errorf("invalid remote session request state or requested time")
	}
	if r.State == SessionRequestBinding || r.State == SessionRequestSucceeded {
		if err := ValidateGeneratedID(string(r.SessionID), IDRemoteSession); err != nil {
			return fmt.Errorf("remote session request %s requires result session: %w", r.State, err)
		}
	}
	return nil
}
