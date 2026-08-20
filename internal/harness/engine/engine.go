package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/homiakus/agctl/internal/harness/events"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type Options struct {
	IDs harnessmodel.IDGenerator
	Now func() time.Time
}

type Engine struct {
	store harnessstore.Store
	ids   harnessmodel.IDGenerator
	now   func() time.Time
}

func New(store harnessstore.Store, opts Options) (*Engine, error) {
	if store == nil {
		return nil, fmt.Errorf("harness store is required")
	}
	ids := opts.IDs
	if ids == nil {
		g := harnessmodel.NewIDGenerator()
		ids = g
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Engine{store: store, ids: ids, now: now}, nil
}

func (e *Engine) nextID(kind harnessmodel.IDKind) (string, error) {
	id, err := e.ids.New(kind)
	if err != nil {
		return "", fmt.Errorf("generate %s id: %w", kind, err)
	}
	return id, nil
}

func (e *Engine) appendEvent(ctx context.Context, tx harnessstore.Tx, runID harnessmodel.WorkflowRunID, at time.Time, typ, entityType, entityID string, payload any) (events.Event, error) {
	id, err := e.nextID(harnessmodel.IDEvent)
	if err != nil {
		return events.Event{}, err
	}
	raw := json.RawMessage(`{}`)
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return events.Event{}, fmt.Errorf("marshal %s event payload: %w", typ, err)
		}
		raw = b
	}
	return tx.AppendEvent(ctx, events.Event{
		ID:             harnessmodel.EventID(id),
		WorkflowRunID:  runID,
		Type:           typ,
		Timestamp:      at.UTC(),
		EntityType:     entityType,
		EntityID:       entityID,
		PayloadVersion: 1,
		Payload:        raw,
	}, nil)
}
