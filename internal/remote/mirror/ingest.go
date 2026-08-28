package mirror

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/homiakus/agctl/internal/antigravityide"
	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

var ErrAgentEventsUnavailable = errors.New("remote mirror: Antigravity agent events unavailable")

type EventClient interface {
	Capabilities(context.Context) (antigravityide.Capabilities, error)
	Events(context.Context, string, uint64) ([]antigravityide.BridgeEvent, error)
}

type IngestStore interface {
	GetSession(context.Context, model.RemoteSessionID) (model.RemoteSession, error)
	GetConversation(context.Context, model.ConversationID) (model.Conversation, error)
	GetTelegramBindingBySession(context.Context, model.RemoteSessionID) (model.TelegramBinding, error)
	AppendRemoteEvent(context.Context, model.RemoteEvent) (model.RemoteEvent, bool, error)
	AppendRemoteEventWithOutbox(context.Context, model.RemoteEvent, string, []byte) (model.RemoteEvent, bool, error)
}

type EventPayload struct {
	ConversationID string `json:"conversationId"`
	StepIndex      int    `json:"stepIndex"`
	Text           string `json:"text"`
	Final          bool   `json:"final"`
	StreamKey      string `json:"streamKey"`
}

type Ingestor struct {
	Store IngestStore
	IDs   model.IDGenerator
}

func (i *Ingestor) PollSession(ctx context.Context, client EventClient, sessionID model.RemoteSessionID) (int, error) {
	if i == nil || i.Store == nil || client == nil {
		return 0, fmt.Errorf("remote mirror ingestor requires store and Bridge client")
	}
	ids := i.IDs
	if ids == nil {
		generator := model.NewIDGenerator()
		ids = generator
	}
	session, err := i.Store.GetSession(ctx, sessionID)
	if err != nil {
		return 0, fmt.Errorf("load mirror session: %w", err)
	}
	conversation, err := i.Store.GetConversation(ctx, session.ConversationID)
	if err != nil {
		return 0, fmt.Errorf("load mirror conversation: %w", err)
	}
	caps, err := client.Capabilities(ctx)
	if err != nil {
		return 0, fmt.Errorf("read Bridge capabilities: %w", err)
	}
	if !caps.AgentEvents {
		return 0, ErrAgentEventsUnavailable
	}
	providerID := strings.TrimSpace(conversation.ProviderConversationID)
	if providerID == "" {
		return 0, fmt.Errorf("mirror conversation has empty provider id")
	}

	// Bridge event sequence is process-local and resets after extension restart.
	// Always replay from zero and let durable source_event_id deduplication decide
	// what is new; this prevents a restart from skipping the current final answer.
	bridgeEvents, err := client.Events(ctx, providerID, 0)
	if err != nil {
		return 0, fmt.Errorf("read Bridge events: %w", err)
	}
	_, bindingErr := i.Store.GetTelegramBindingBySession(ctx, sessionID)
	bound := bindingErr == nil
	if bindingErr != nil && !errors.Is(bindingErr, remotestore.ErrNotFound) {
		return 0, fmt.Errorf("read Telegram binding: %w", bindingErr)
	}

	inserted := 0
	for _, bridgeEvent := range bridgeEvents {
		eventType, ok := mapEventType(bridgeEvent.Type)
		if !ok {
			continue
		}
		var source struct {
			ConversationID string `json:"conversationId"`
			StepIndex      int    `json:"stepIndex"`
			Text           string `json:"text"`
			Final          bool   `json:"final"`
		}
		if err := json.Unmarshal(bridgeEvent.Payload, &source); err != nil {
			return inserted, fmt.Errorf("decode Bridge event %s: %w", bridgeEvent.SourceEventID, err)
		}
		if source.ConversationID != providerID {
			return inserted, fmt.Errorf("Bridge event conversation %q does not match %q", source.ConversationID, providerID)
		}
		if strings.TrimSpace(source.Text) == "" || strings.TrimSpace(bridgeEvent.StreamKey) == "" {
			continue
		}
		payload, err := json.Marshal(EventPayload{ConversationID: source.ConversationID, StepIndex: source.StepIndex, Text: source.Text, Final: source.Final, StreamKey: bridgeEvent.StreamKey})
		if err != nil {
			return inserted, err
		}
		id, err := ids.New(model.IDRemoteEvent)
		if err != nil {
			return inserted, err
		}
		draft := model.RemoteEvent{ID: model.RemoteEventID(id), SessionID: sessionID, Source: model.EventSourceIDE, Type: eventType, SourceEventID: bridgeEvent.SourceEventID, Payload: payload, Timestamp: bridgeEvent.Timestamp}
		var created bool
		if bound {
			_, created, err = i.Store.AppendRemoteEventWithOutbox(ctx, draft, "telegram", payload)
		} else {
			_, created, err = i.Store.AppendRemoteEvent(ctx, draft)
		}
		if err != nil {
			return inserted, fmt.Errorf("persist Bridge event %s: %w", bridgeEvent.SourceEventID, err)
		}
		if created {
			inserted++
		}
	}
	return inserted, nil
}

func mapEventType(value string) (model.EventType, bool) {
	switch value {
	case "agent_delta":
		return model.EventAgentDelta, true
	case "agent_message":
		return model.EventAgentMessage, true
	default:
		return "", false
	}
}
