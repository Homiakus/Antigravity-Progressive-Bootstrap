package mirror

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
	"github.com/homiakus/agctl/internal/telegram"
)

const TelegramTransport = "telegram"

type DeliveryStore interface {
	GetRemoteEvent(context.Context, model.RemoteEventID) (model.RemoteEvent, error)
	GetTelegramBindingBySession(context.Context, model.RemoteSessionID) (model.TelegramBinding, error)
	GetTelegramMirrorState(context.Context, model.RemoteSessionID) (model.TelegramMirrorState, error)
	UpsertTelegramMirrorState(context.Context, model.TelegramMirrorState) error
	ListRemoteOutbox(context.Context, string, time.Time, int) ([]model.RemoteOutboxItem, error)
	MarkRemoteOutboxDelivered(context.Context, int64, time.Time) error
	ScheduleRemoteOutboxRetry(context.Context, int64, time.Time) error
}

type ViewAPI interface {
	SendView(context.Context, int64, int64, telegram.View) (telegram.Message, error)
	EditView(context.Context, int64, int64, telegram.View) error
}

type TelegramWorker struct {
	Store DeliveryStore
	API   ViewAPI
	Now   func() time.Time
}

func (w *TelegramWorker) RunOnce(ctx context.Context, limit int) (int, error) {
	if w == nil || w.Store == nil || w.API == nil {
		return 0, fmt.Errorf("Telegram mirror worker requires store and API")
	}
	now := w.now()
	items, err := w.Store.ListRemoteOutbox(ctx, TelegramTransport, now, limit)
	if err != nil {
		return 0, err
	}
	delivered := 0
	var errs []error
	for _, item := range items {
		if err := w.deliver(ctx, item, now); err != nil {
			retryAt := now.Add(retryDelay(item.AttemptCount))
			if retryErr := w.Store.ScheduleRemoteOutboxRetry(ctx, item.ID, retryAt); retryErr != nil && !errors.Is(retryErr, remotestore.ErrConflict) {
				err = errors.Join(err, retryErr)
			}
			errs = append(errs, err)
			continue
		}
		delivered++
	}
	return delivered, errors.Join(errs...)
}

func (w *TelegramWorker) deliver(ctx context.Context, item model.RemoteOutboxItem, now time.Time) error {
	event, err := w.Store.GetRemoteEvent(ctx, item.EventID)
	if err != nil {
		return fmt.Errorf("load remote event %s: %w", item.EventID, err)
	}
	if event.Type != model.EventAgentDelta && event.Type != model.EventAgentMessage {
		return w.markDelivered(ctx, item.ID, now)
	}
	var payload EventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode Telegram mirror payload: %w", err)
	}
	payload.Text = strings.TrimSpace(payload.Text)
	payload.StreamKey = strings.TrimSpace(payload.StreamKey)
	if payload.Text == "" || payload.StreamKey == "" {
		return fmt.Errorf("Telegram mirror payload requires text and stream key")
	}
	binding, err := w.Store.GetTelegramBindingBySession(ctx, event.SessionID)
	if errors.Is(err, remotestore.ErrNotFound) {
		return w.markDelivered(ctx, item.ID, now)
	}
	if err != nil {
		return fmt.Errorf("load Telegram binding: %w", err)
	}

	state, err := w.Store.GetTelegramMirrorState(ctx, event.SessionID)
	if errors.Is(err, remotestore.ErrNotFound) {
		state = model.TelegramMirrorState{SessionID: event.SessionID, ChatID: binding.ChatID, ThreadID: binding.ThreadID, UpdatedAt: now}
	} else if err != nil {
		return fmt.Errorf("load Telegram mirror state: %w", err)
	}
	if event.Seq <= state.LastEventSeq {
		return w.markDelivered(ctx, item.ID, now)
	}

	text := renderTelegramText(payload, event.Type)
	newStream := state.MessageID == 0 || state.StreamKey != payload.StreamKey || state.ChatID != binding.ChatID || state.ThreadID != binding.ThreadID
	if newStream {
		message, err := w.API.SendView(ctx, binding.ChatID, binding.ThreadID, telegram.View{Text: text})
		if err != nil {
			return fmt.Errorf("send Telegram mirror message: %w", err)
		}
		if message.MessageID == 0 {
			return fmt.Errorf("Telegram returned empty mirror message id")
		}
		state.MessageID = message.MessageID
		state.StreamKey = payload.StreamKey
		state.ChatID = binding.ChatID
		state.ThreadID = binding.ThreadID
	} else if state.RenderedText != text {
		if err := w.API.EditView(ctx, binding.ChatID, state.MessageID, telegram.View{Text: text}); err != nil {
			return fmt.Errorf("edit Telegram mirror message: %w", err)
		}
	}
	state.LastEventSeq = event.Seq
	state.RenderedText = text
	state.UpdatedAt = now
	if err := w.Store.UpsertTelegramMirrorState(ctx, state); err != nil {
		return fmt.Errorf("persist Telegram mirror state: %w", err)
	}
	return w.markDelivered(ctx, item.ID, now)
}

func (w *TelegramWorker) markDelivered(ctx context.Context, id int64, now time.Time) error {
	err := w.Store.MarkRemoteOutboxDelivered(ctx, id, now)
	if errors.Is(err, remotestore.ErrConflict) {
		return nil
	}
	return err
}

func (w *TelegramWorker) now() time.Time {
	if w.Now != nil {
		return w.Now().UTC()
	}
	return time.Now().UTC()
}

func renderTelegramText(payload EventPayload, eventType model.EventType) string {
	text := payload.Text
	if eventType == model.EventAgentDelta && !payload.Final {
		text += " ▌"
	}
	runes := []rune(text)
	if len(runes) > 3900 {
		runes = runes[:3899]
		text = string(runes) + "…"
	}
	return text
}

func retryDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := 2 * time.Second
	for i := 0; i < attempt && delay < 2*time.Minute; i++ {
		delay *= 2
	}
	if delay > 2*time.Minute {
		return 2 * time.Minute
	}
	return delay
}
