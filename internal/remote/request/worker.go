package request

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
	remotesession "github.com/homiakus/agctl/internal/remote/session"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

type Store interface {
	ListSessionRequests(context.Context, []model.SessionRequestState, int) ([]model.RemoteSessionRequest, error)
	ClaimPendingSessionRequest(context.Context, time.Time) (model.RemoteSessionRequest, error)
	AttachSessionToRequest(context.Context, model.RemoteSessionRequestID, model.RemoteSessionID) error
	CompleteSessionRequest(context.Context, model.RemoteSessionRequestID, time.Time) error
	FailSessionRequest(context.Context, model.RemoteSessionRequestID, string, time.Time) error
	ReplaceTelegramBinding(context.Context, model.TelegramBinding) error
}

type Provisioner interface {
	Provision(context.Context, remotesession.Spec) (model.RemoteSession, error)
}

type Worker struct {
	Store       Store
	Provisioner Provisioner
	IDs         model.IDGenerator
	Now         func() time.Time
}

func (w *Worker) RunOnce(ctx context.Context, limit int) (int, error) {
	if w == nil || w.Store == nil || w.Provisioner == nil {
		return 0, fmt.Errorf("remote session request worker requires store and provisioner")
	}
	if limit <= 0 || limit > 1000 {
		limit = 25
	}
	ids := w.IDs
	if ids == nil {
		generator := model.NewIDGenerator()
		ids = generator
	}

	processed := 0
	var errs []error
	bindings, err := w.Store.ListSessionRequests(ctx, []model.SessionRequestState{model.SessionRequestBinding}, limit)
	if err != nil {
		return 0, fmt.Errorf("list binding session requests: %w", err)
	}
	for _, request := range bindings {
		if processed >= limit {
			break
		}
		processed++
		if err := w.finishBinding(ctx, ids, request); err != nil {
			errs = append(errs, fmt.Errorf("session request %s binding: %w", request.ID, err))
		}
	}
	if processed >= limit {
		return processed, errors.Join(errs...)
	}

	request, err := w.Store.ClaimPendingSessionRequest(ctx, w.now())
	if errors.Is(err, remotestore.ErrNotFound) {
		return processed, errors.Join(errs...)
	}
	if err != nil {
		errs = append(errs, fmt.Errorf("claim session request: %w", err))
		return processed, errors.Join(errs...)
	}
	processed++

	session, err := w.Provisioner.Provision(ctx, toSessionSpec(request))
	if err != nil {
		if failErr := w.Store.FailSessionRequest(ctx, request.ID, err.Error(), w.now()); failErr != nil {
			errs = append(errs, fmt.Errorf("session request %s provision failed: %v; persist failure: %w", request.ID, err, failErr))
		} else {
			errs = append(errs, fmt.Errorf("session request %s provision: %w", request.ID, err))
		}
		return processed, errors.Join(errs...)
	}

	// This write is the durable correlation boundary after provisioning. If it
	// fails, do not mark the request failed and never provision it again in this
	// worker: the side effect may already exist and blind retry could open a
	// duplicate IDE instance/conversation.
	if err := w.Store.AttachSessionToRequest(ctx, request.ID, session.ID); err != nil {
		errs = append(errs, fmt.Errorf("session request %s attach durable session %s: %w", request.ID, session.ID, err))
		return processed, errors.Join(errs...)
	}
	request.State = model.SessionRequestBinding
	request.SessionID = session.ID
	if err := w.finishBinding(ctx, ids, request); err != nil {
		errs = append(errs, fmt.Errorf("session request %s binding: %w", request.ID, err))
	}
	return processed, errors.Join(errs...)
}

func (w *Worker) finishBinding(ctx context.Context, ids model.IDGenerator, request model.RemoteSessionRequest) error {
	if request.SessionID == "" {
		return fmt.Errorf("binding session request has no session id")
	}
	id, err := ids.New(model.IDTelegramBinding)
	if err != nil {
		return fmt.Errorf("generate Telegram binding id: %w", err)
	}
	binding := model.TelegramBinding{
		ID:          model.TelegramBindingID(id),
		SessionID:   request.SessionID,
		ChatID:      request.ChatID,
		ThreadID:    request.ThreadID,
		OwnerUserID: request.RequesterUserID,
		Enabled:     true,
		CreatedAt:   w.now(),
	}
	if err := w.Store.ReplaceTelegramBinding(ctx, binding); err != nil {
		return fmt.Errorf("replace Telegram topic binding: %w", err)
	}
	if err := w.Store.CompleteSessionRequest(ctx, request.ID, w.now()); err != nil {
		return fmt.Errorf("complete remote session request: %w", err)
	}
	return nil
}

func toSessionSpec(request model.RemoteSessionRequest) remotesession.Spec {
	return remotesession.Spec{
		RepositoryID:           request.RepositoryID,
		AccountID:              request.AccountID,
		InstanceStrategy:       remotesession.InstanceStrategy(request.InstanceStrategy),
		InstanceID:             request.InstanceID,
		ConversationStrategy:   remotesession.ConversationStrategy(request.ConversationStrategy),
		ProviderConversationID: request.ProviderConversationID,
		IsolationMode:          request.IsolationMode,
	}
}

func (w *Worker) now() time.Time {
	if w.Now != nil {
		return w.Now().UTC()
	}
	return time.Now().UTC()
}
