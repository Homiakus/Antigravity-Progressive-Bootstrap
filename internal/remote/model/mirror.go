package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type RemoteOutboxItem struct {
	ID            int64
	EventID       RemoteEventID
	Transport     string
	Payload       json.RawMessage
	CreatedAt     time.Time
	DeliveredAt   *time.Time
	AttemptCount  int
	NextAttemptAt *time.Time
}

func (o RemoteOutboxItem) Validate() error {
	if o.ID <= 0 {
		return fmt.Errorf("remote outbox id must be positive")
	}
	if err := ValidateGeneratedID(string(o.EventID), IDRemoteEvent); err != nil {
		return fmt.Errorf("remote outbox event: %w", err)
	}
	if strings.TrimSpace(o.Transport) == "" || o.CreatedAt.IsZero() {
		return fmt.Errorf("remote outbox transport and created_at are required")
	}
	if o.AttemptCount < 0 {
		return fmt.Errorf("remote outbox attempt count cannot be negative")
	}
	return nil
}

type TelegramMirrorState struct {
	SessionID    RemoteSessionID
	ChatID       int64
	ThreadID     int64
	StreamKey    string
	MessageID    int64
	LastEventSeq uint64
	RenderedText string
	UpdatedAt    time.Time
}

func (s TelegramMirrorState) Validate() error {
	if err := ValidateGeneratedID(string(s.SessionID), IDRemoteSession); err != nil {
		return fmt.Errorf("telegram mirror session: %w", err)
	}
	if s.ChatID == 0 || s.MessageID < 0 || s.UpdatedAt.IsZero() {
		return fmt.Errorf("telegram mirror chat, non-negative message id and updated_at are required")
	}
	if s.MessageID > 0 && strings.TrimSpace(s.StreamKey) == "" {
		return fmt.Errorf("telegram mirror stream_key is required once a message exists")
	}
	return nil
}
