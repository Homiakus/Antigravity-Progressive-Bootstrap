package sqlite

import (
	"context"

	"github.com/homiakus/agctl/internal/remote/model"
)

func (s *Store) GetRemoteEvent(ctx context.Context, id model.RemoteEventID) (model.RemoteEvent, error) {
	return scanRemoteEvent(s.db.QueryRowContext(ctx, remoteEventSelect+` WHERE event_id=?`, id))
}
