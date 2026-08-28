package store

import (
	"context"
	"errors"

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

type Store interface {
	RepositoryStore
}
