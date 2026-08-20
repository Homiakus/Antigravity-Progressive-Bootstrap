package sqlite

import (
	"context"
	"fmt"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func (t *transaction) CreateNodeRun(ctx context.Context, nr harnessmodel.NodeRun) error {
	if nr.ID == "" || nr.WorkflowRunID == "" || nr.NodeID == "" || nr.GraphRevision < 1 || nr.Generation < 1 || nr.State == "" || nr.CreatedAt.IsZero() || nr.UpdatedAt.IsZero() || nr.RemainingDependencies < 0 {
		return fmt.Errorf("invalid node run")
	}
	res, err := t.tx.ExecContext(ctx, `
INSERT INTO node_runs(
    id, workflow_run_id, definition_id, definition_version, node_id,
    graph_revision, generation, state, remaining_dependencies,
    priority, effective_priority, created_at, updated_at
)
SELECT ?, wr.id, wr.definition_id, wr.definition_version, ?, ?, ?, ?, ?, ?, ?, ?, ?
FROM workflow_runs wr
WHERE wr.id=?`, string(nr.ID), string(nr.NodeID), nr.GraphRevision, nr.Generation, string(nr.State), nr.RemainingDependencies, nr.Priority, nr.EffectivePriority, formatTime(nr.CreatedAt), formatTime(nr.UpdatedAt), string(nr.WorkflowRunID))
	if err != nil {
		return fmt.Errorf("insert node run: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read node run insert count: %w", err)
	}
	if n != 1 {
		return harnessstore.ErrNotFound
	}
	return nil
}
