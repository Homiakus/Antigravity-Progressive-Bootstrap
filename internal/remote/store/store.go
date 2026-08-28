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

type Store interface {
	RepositoryStore
	InstanceStore
	ConversationStore
	SessionStore
}
