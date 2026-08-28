package store

import (
	"context"
	"errors"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
)

var (
	ErrNotFound = errors.New("remote store: not found")
	ErrConflict = errors.New("remote store: conflict")
)

type RepositoryStore interface {
	UpsertRepository(context.Context, model.Repository) error
	GetRepository(context.Context, model.RepositoryID) (model.Repository, error)
	GetRepositoryByPath(context.Context, string) (model.Repository, error)
	ListRepositories(context.Context, bool) ([]model.Repository, error)
	SetRepositoryEnabled(context.Context, model.RepositoryID, bool) error
}

type InstanceStore interface {
	UpsertInstance(context.Context, model.InstanceMirror) error
	GetInstance(context.Context, model.InstanceID) (model.InstanceMirror, error)
	ListInstances(context.Context) ([]model.InstanceMirror, error)
}

type ConversationStore interface {
	UpsertConversation(context.Context, model.Conversation) error
	GetConversation(context.Context, model.ConversationID) (model.Conversation, error)
	GetConversationByProvider(context.Context, model.InstanceID, string) (model.Conversation, error)
	ListConversationsByInstance(context.Context, model.InstanceID) ([]model.Conversation, error)
}

type SessionStore interface {
	CreateSession(context.Context, model.RemoteSession) error
	GetSession(context.Context, model.RemoteSessionID) (model.RemoteSession, error)
	UpdateSessionStates(context.Context, model.RemoteSessionID, model.SessionDesiredState, model.SessionObservedState, time.Time) error
	UpdateSessionAccount(context.Context, model.RemoteSessionID, string, time.Time) error
	ListSessionsByInstance(context.Context, model.InstanceID, bool) ([]model.RemoteSession, error)
}

type TelegramStore interface {
	GetTelegramCursor(context.Context, string) (model.TelegramCursor, error)
	AdvanceTelegramCursor(context.Context, model.TelegramCursor) error
	UpsertTelegramPrincipal(context.Context, model.TelegramPrincipal) error
	GetTelegramPrincipal(context.Context, int64) (model.TelegramPrincipal, error)
	CreateTelegramPairing(context.Context, model.TelegramPairing) error
	ConsumeTelegramPairing(context.Context, string, int64, int64, time.Time) (model.TelegramPrincipal, error)
	ReserveTelegramCallback(context.Context, string, int64, int64, time.Time) (bool, error)
	UpsertTelegramBinding(context.Context, model.TelegramBinding) error
	GetTelegramBindingByTopic(context.Context, int64, int64) (model.TelegramBinding, error)
	GetTelegramBindingBySession(context.Context, model.RemoteSessionID) (model.TelegramBinding, error)
	GetTelegramMirrorState(context.Context, model.RemoteSessionID) (model.TelegramMirrorState, error)
	UpsertTelegramMirrorState(context.Context, model.TelegramMirrorState) error
}

type RemoteCommandStore interface {
	AdmitRemoteCommand(context.Context, model.RemoteCommand) (model.RemoteCommand, bool, error)
	GetRemoteCommand(context.Context, model.RemoteCommandID) (model.RemoteCommand, error)
	ListPendingRemoteCommands(context.Context, int) ([]model.RemoteCommand, error)
	UpdateRemoteCommandState(context.Context, model.RemoteCommandID, model.CommandState, string, time.Time) error
}

type RemoteEventStore interface {
	AppendRemoteEvent(context.Context, model.RemoteEvent) (model.RemoteEvent, bool, error)
	AppendRemoteEventWithOutbox(context.Context, model.RemoteEvent, string, []byte) (model.RemoteEvent, bool, error)
	ListRemoteEventsAfter(context.Context, model.RemoteSessionID, uint64, int) ([]model.RemoteEvent, error)
	ListRemoteOutbox(context.Context, string, time.Time, int) ([]model.RemoteOutboxItem, error)
	MarkRemoteOutboxDelivered(context.Context, int64, time.Time) error
	ScheduleRemoteOutboxRetry(context.Context, int64, time.Time) error
}

type Store interface {
	RepositoryStore
	InstanceStore
	ConversationStore
	SessionStore
	TelegramStore
	RemoteCommandStore
	RemoteEventStore
}
