package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("remote sqlite store requires database")
	}
	return &Store{db: db}, nil
}

func (s *Store) UpsertRepository(ctx context.Context, repository model.Repository) error {
	if err := repository.Validate(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO remote_repositories(
    id,name,canonical_path,git_root,git_remote,default_branch,enabled,created_at,last_seen_at
) VALUES(?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET
    name=excluded.name,
    canonical_path=excluded.canonical_path,
    git_root=excluded.git_root,
    git_remote=excluded.git_remote,
    default_branch=excluded.default_branch,
    enabled=excluded.enabled,
    last_seen_at=excluded.last_seen_at`,
		repository.ID, repository.Name, repository.CanonicalPath, repository.GitRoot,
		repository.GitRemote, repository.DefaultBranch, boolInt(repository.Enabled),
		formatTime(repository.CreatedAt), formatTime(repository.LastSeenAt))
	if err != nil {
		return mapWriteError("upsert repository", err)
	}
	return nil
}

func (s *Store) GetRepository(ctx context.Context, id model.RepositoryID) (model.Repository, error) {
	return scanRepository(s.db.QueryRowContext(ctx, `
SELECT id,name,canonical_path,git_root,git_remote,default_branch,enabled,created_at,last_seen_at
FROM remote_repositories WHERE id=?`, id))
}

func (s *Store) GetRepositoryByPath(ctx context.Context, canonicalPath string) (model.Repository, error) {
	return scanRepository(s.db.QueryRowContext(ctx, `
SELECT id,name,canonical_path,git_root,git_remote,default_branch,enabled,created_at,last_seen_at
FROM remote_repositories WHERE canonical_path=?`, canonicalPath))
}

func (s *Store) ListRepositories(ctx context.Context, enabledOnly bool) ([]model.Repository, error) {
	query := `
SELECT id,name,canonical_path,git_root,git_remote,default_branch,enabled,created_at,last_seen_at
FROM remote_repositories`
	if enabledOnly {
		query += ` WHERE enabled=1`
	}
	query += ` ORDER BY name COLLATE NOCASE, id`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list remote repositories: %w", err)
	}
	defer rows.Close()
	out := make([]model.Repository, 0)
	for rows.Next() {
		repository, err := scanRepository(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, repository)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate remote repositories: %w", err)
	}
	return out, nil
}

func (s *Store) SetRepositoryEnabled(ctx context.Context, id model.RepositoryID, enabled bool) error {
	result, err := s.db.ExecContext(ctx, `UPDATE remote_repositories SET enabled=? WHERE id=?`, boolInt(enabled), id)
	if err != nil {
		return fmt.Errorf("set repository enabled: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("repository enabled rows affected: %w", err)
	}
	if rows == 0 {
		return remotestore.ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(...any) error
}

func scanRepository(row scanner) (model.Repository, error) {
	var repository model.Repository
	var enabled int
	var createdAt, lastSeenAt string
	if err := row.Scan(
		&repository.ID, &repository.Name, &repository.CanonicalPath, &repository.GitRoot,
		&repository.GitRemote, &repository.DefaultBranch, &enabled, &createdAt, &lastSeenAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Repository{}, remotestore.ErrNotFound
		}
		return model.Repository{}, fmt.Errorf("scan remote repository: %w", err)
	}
	var err error
	repository.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return model.Repository{}, fmt.Errorf("parse repository created_at: %w", err)
	}
	repository.LastSeenAt, err = parseTime(lastSeenAt)
	if err != nil {
		return model.Repository{}, fmt.Errorf("parse repository last_seen_at: %w", err)
	}
	repository.Enabled = enabled != 0
	return repository, nil
}

func mapWriteError(op string, err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint") || strings.Contains(message, "constraint failed") {
		return fmt.Errorf("%s: %w: %v", op, remotestore.ErrConflict, err)
	}
	return fmt.Errorf("%s: %w", op, err)
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}
