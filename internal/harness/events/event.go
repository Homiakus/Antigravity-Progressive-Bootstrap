package events

import (
	"encoding/json"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

type Event struct {
	ID             harnessmodel.EventID       `json:"eventId"`
	WorkflowRunID  harnessmodel.WorkflowRunID `json:"workflowRunId"`
	WorkflowSeq    int64                      `json:"workflowSeq"`
	Type           string                     `json:"type"`
	Timestamp      time.Time                  `json:"timestamp"`
	EntityType     string                     `json:"entityType"`
	EntityID       string                     `json:"entityId"`
	PayloadVersion int                        `json:"payloadVersion"`
	Payload        json.RawMessage            `json:"payload"`
}

func (e Event) ValidateForAppend() error {
	if e.ID == "" {
		return fmt.Errorf("event id is required")
	}
	if e.WorkflowRunID == "" {
		return fmt.Errorf("workflow run id is required")
	}
	if e.WorkflowSeq != 0 {
		return fmt.Errorf("workflow sequence is assigned by the durable store")
	}
	if e.Type == "" {
		return fmt.Errorf("event type is required")
	}
	if e.Timestamp.IsZero() {
		return fmt.Errorf("event timestamp is required")
	}
	if e.EntityType == "" || e.EntityID == "" {
		return fmt.Errorf("event entity type and id are required")
	}
	if e.PayloadVersion < 1 {
		return fmt.Errorf("event payload version must be >= 1")
	}
	if len(e.Payload) == 0 {
		e.Payload = json.RawMessage(`{}`)
	}
	if !json.Valid(e.Payload) {
		return fmt.Errorf("event payload must be valid JSON")
	}
	return nil
}

type OutboxMessage struct {
	Topic   string          `json:"topic"`
	Payload json.RawMessage `json:"payload"`
}

func (m OutboxMessage) Validate() error {
	if m.Topic == "" {
		return fmt.Errorf("outbox topic is required")
	}
	if len(m.Payload) > 0 && !json.Valid(m.Payload) {
		return fmt.Errorf("outbox payload must be valid JSON")
	}
	return nil
}
