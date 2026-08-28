package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/antigravityide"
	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

const ConversationSend = "conversation.send"

type Store interface {
	ListPendingRemoteCommands(context.Context, int) ([]model.RemoteCommand, error)
	UpdateRemoteCommandState(context.Context, model.RemoteCommandID, model.CommandState, string, time.Time) error
	GetSession(context.Context, model.RemoteSessionID) (model.RemoteSession, error)
	GetConversation(context.Context, model.ConversationID) (model.Conversation, error)
}

type BridgeResolver interface {
	Bridge(string) (antigravityide.LocatedBridge, bool)
}

type Worker struct {
	Store   Store
	Bridges BridgeResolver
	Gate    *antigravityide.InstanceCommandGatePool
	Now     func() time.Time
}

func (w *Worker) RunOnce(ctx context.Context, limit int) (int, error) {
	if w == nil || w.Store == nil || w.Bridges == nil {
		return 0, fmt.Errorf("remote command worker requires store and Bridge resolver")
	}
	gate := w.Gate
	if gate == nil {
		gate = antigravityide.NewInstanceCommandGatePool()
	}
	commands, err := w.Store.ListPendingRemoteCommands(ctx, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	var errs []error
	for _, command := range commands {
		if command.Kind != ConversationSend {
			continue
		}
		// A missing live Bridge is transient and, importantly, may simply mean
		// this daemon did not own the IDE boot that produced the persisted
		// session. Do not burn the command into FAILED before a managed Bridge
		// is actually available; leave it pending for a later recovery/start.
		session, sessionErr := w.Store.GetSession(ctx, command.SessionID)
		if sessionErr == nil {
			if bridge, ok := w.Bridges.Bridge(string(session.CockpitInstanceID)); !ok || bridge.Client == nil {
				continue
			}
		}

		now := w.now()
		if err := w.Store.UpdateRemoteCommandState(ctx, command.ID, model.CommandRunning, "", now); err != nil {
			if errors.Is(err, remotestore.ErrConflict) {
				continue
			}
			errs = append(errs, err)
			continue
		}
		processed++
		if err := w.executeSend(ctx, gate, command); err != nil {
			_ = w.Store.UpdateRemoteCommandState(ctx, command.ID, model.CommandFailed, err.Error(), w.now())
			errs = append(errs, fmt.Errorf("command %s: %w", command.ID, err))
			continue
		}
		if err := w.Store.UpdateRemoteCommandState(ctx, command.ID, model.CommandSucceeded, "", w.now()); err != nil && !errors.Is(err, remotestore.ErrConflict) {
			errs = append(errs, err)
		}
	}
	return processed, errors.Join(errs...)
}

func (w *Worker) executeSend(ctx context.Context, gate *antigravityide.InstanceCommandGatePool, command model.RemoteCommand) error {
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(command.Payload, &payload); err != nil {
		return fmt.Errorf("decode conversation.send payload: %w", err)
	}
	payload.Text = strings.TrimSpace(payload.Text)
	if payload.Text == "" {
		return fmt.Errorf("conversation.send text is required")
	}
	session, err := w.Store.GetSession(ctx, command.SessionID)
	if err != nil {
		return fmt.Errorf("load remote session: %w", err)
	}
	conversation, err := w.Store.GetConversation(ctx, session.ConversationID)
	if err != nil {
		return fmt.Errorf("load remote conversation: %w", err)
	}
	bridge, ok := w.Bridges.Bridge(string(session.CockpitInstanceID))
	if !ok || bridge.Client == nil {
		return fmt.Errorf("live Antigravity Bridge unavailable for instance %s", session.CockpitInstanceID)
	}
	caps, err := bridge.Client.Capabilities(ctx)
	if err != nil {
		return fmt.Errorf("read Bridge capabilities: %w", err)
	}
	if err := gate.Send(ctx, string(session.CockpitInstanceID), bridge.Client, caps, conversation.ProviderConversationID, payload.Text); err != nil {
		return fmt.Errorf("send to Antigravity conversation: %w", err)
	}
	return nil
}

func (w *Worker) now() time.Time {
	if w.Now != nil {
		return w.Now().UTC()
	}
	return time.Now().UTC()
}
