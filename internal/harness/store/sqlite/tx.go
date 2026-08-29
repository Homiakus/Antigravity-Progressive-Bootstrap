package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/homiakus/agctl/internal/harness/events"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type transaction struct {
	tx *sql.Tx
}

var _ harnessstore.Store = (*DB)(nil)
var _ harnessstore.Tx = (*transaction)(nil)
var _ harnessstore.Reader = (*transaction)(nil)

func (d *DB) View(ctx context.Context, fn func(harnessstore.Reader) error) error {
	if d == nil || d.readDB == nil {
		return fmt.Errorf("SQLite database is not open")
	}
	tx, err := d.readDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("begin SQLite view transaction: %w", err)
	}
	defer tx.Rollback()
	if err := fn(&transaction{tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit SQLite view transaction: %w", err)
	}
	return nil
}

func (d *DB) Update(ctx context.Context, fn func(harnessstore.Tx) error) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("SQLite database is not open")
	}
	// d.db is the authoritative single-connection writer pool. That serializes
	// in-process read/modify/write transactions before they enter SQLite; the
	// SQL transaction plus FK/CHECK/UNIQUE/CAS invariants provide durable
	// correctness. No driver-specific _txlock mode is required.
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin SQLite update transaction: %w", err)
	}
	defer tx.Rollback()
	if err := fn(&transaction{tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit SQLite update transaction: %w", err)
	}
	return nil
}

func (t *transaction) CreateWorkflowDefinition(ctx context.Context, def harnessmodel.WorkflowDefinition) error {
	if def.ID == "" || def.Version < 1 || def.Name == "" || def.CreatedAt.IsZero() {
		return fmt.Errorf("invalid workflow definition identity/version/name/time")
	}
	payload, err := json.Marshal(def)
	if err != nil {
		return fmt.Errorf("marshal workflow definition: %w", err)
	}
	if _, err := t.tx.ExecContext(ctx, `
INSERT INTO workflow_definitions(id, version, name, compiler_version, definition_json, created_at)
VALUES(?, ?, ?, ?, ?, ?)`, string(def.ID), def.Version, def.Name, def.CompilerVersion, payload, formatTime(def.CreatedAt)); err != nil {
		return fmt.Errorf("insert workflow definition: %w", err)
	}
	for _, node := range def.Nodes {
		spec, err := json.Marshal(node)
		if err != nil {
			return fmt.Errorf("marshal node %s: %w", node.ID, err)
		}
		if _, err := t.tx.ExecContext(ctx, `
INSERT INTO nodes(definition_id, definition_version, node_id, spec_json)
VALUES(?, ?, ?, ?)`, string(def.ID), def.Version, string(node.ID), spec); err != nil {
			return fmt.Errorf("insert node %s: %w", node.ID, err)
		}
	}
	for _, node := range def.Nodes {
		for _, dep := range node.Dependencies {
			if _, err := t.tx.ExecContext(ctx, `
INSERT INTO dependencies(definition_id, definition_version, node_id, depends_on_node_id, required)
VALUES(?, ?, ?, ?, 1)`, string(def.ID), def.Version, string(node.ID), string(dep)); err != nil {
				return fmt.Errorf("insert dependency %s -> %s: %w", node.ID, dep, err)
			}
		}
	}
	return nil
}

func (t *transaction) GetWorkflowDefinition(ctx context.Context, id harnessmodel.WorkflowDefinitionID, version int) (harnessmodel.WorkflowDefinition, error) {
	var payload []byte
	if err := t.tx.QueryRowContext(ctx, `SELECT definition_json FROM workflow_definitions WHERE id=? AND version=?`, string(id), version).Scan(&payload); err != nil {
		return harnessmodel.WorkflowDefinition{}, mapNotFound(err)
	}
	var def harnessmodel.WorkflowDefinition
	if err := json.Unmarshal(payload, &def); err != nil {
		return harnessmodel.WorkflowDefinition{}, fmt.Errorf("decode workflow definition: %w", err)
	}
	return def, nil
}

func (t *transaction) CreateWorkflowRun(ctx context.Context, run harnessmodel.WorkflowRun) error {
	if run.ID == "" || run.DefinitionID == "" || run.DefinitionVersion < 1 || run.CreatedAt.IsZero() || run.UpdatedAt.IsZero() {
		return fmt.Errorf("invalid workflow run identity/version/time")
	}
	if _, err := t.tx.ExecContext(ctx, `
INSERT INTO workflow_runs(id, definition_id, definition_version, state, current_graph_revision, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?)`, string(run.ID), string(run.DefinitionID), run.DefinitionVersion, string(run.State), run.CurrentGraphRevision, formatTime(run.CreatedAt), formatTime(run.UpdatedAt)); err != nil {
		return fmt.Errorf("insert workflow run: %w", err)
	}
	return nil
}

func (t *transaction) GetWorkflowRun(ctx context.Context, id harnessmodel.WorkflowRunID) (harnessmodel.WorkflowRun, error) {
	var run harnessmodel.WorkflowRun
	var defID string
	var state string
	var created, updated string
	if err := t.tx.QueryRowContext(ctx, `
SELECT id, definition_id, definition_version, state, current_graph_revision, created_at, updated_at
FROM workflow_runs WHERE id=?`, string(id)).Scan(&run.ID, &defID, &run.DefinitionVersion, &state, &run.CurrentGraphRevision, &created, &updated); err != nil {
		return harnessmodel.WorkflowRun{}, mapNotFound(err)
	}
	run.DefinitionID = harnessmodel.WorkflowDefinitionID(defID)
	run.State = harnessmodel.WorkflowState(state)
	var err error
	if run.CreatedAt, err = parseTime(created); err != nil {
		return harnessmodel.WorkflowRun{}, fmt.Errorf("parse workflow run created_at: %w", err)
	}
	if run.UpdatedAt, err = parseTime(updated); err != nil {
		return harnessmodel.WorkflowRun{}, fmt.Errorf("parse workflow run updated_at: %w", err)
	}
	return run, nil
}

func (t *transaction) UpdateWorkflowRunState(ctx context.Context, id harnessmodel.WorkflowRunID, state harnessmodel.WorkflowState, updatedAt time.Time) error {
	if id == "" || state == "" || updatedAt.IsZero() {
		return fmt.Errorf("workflow run id, state and updated time are required")
	}
	res, err := t.tx.ExecContext(ctx, `UPDATE workflow_runs SET state=?, updated_at=? WHERE id=?`, string(state), formatTime(updatedAt), string(id))
	if err != nil {
		return fmt.Errorf("update workflow run state: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read workflow run update count: %w", err)
	}
	if n != 1 {
		return harnessstore.ErrNotFound
	}
	return nil
}

func (t *transaction) CreateGraphRevision(ctx context.Context, rev harnessmodel.GraphRevision) error {
	if rev.WorkflowRunID == "" || rev.Number < 1 || rev.CreatedAt.IsZero() {
		return fmt.Errorf("invalid graph revision identity/number/time")
	}
	var parent any
	if rev.ParentNumber > 0 {
		parent = rev.ParentNumber
	}
	if _, err := t.tx.ExecContext(ctx, `
INSERT INTO graph_revisions(workflow_run_id, number, parent_number, reason, created_at)
VALUES(?, ?, ?, ?, ?)`, string(rev.WorkflowRunID), rev.Number, parent, rev.Reason, formatTime(rev.CreatedAt)); err != nil {
		return fmt.Errorf("insert graph revision: %w", err)
	}
	if _, err := t.tx.ExecContext(ctx, `UPDATE workflow_runs SET current_graph_revision=? WHERE id=?`, rev.Number, string(rev.WorkflowRunID)); err != nil {
		return fmt.Errorf("advance current graph revision: %w", err)
	}
	return nil
}

func (t *transaction) AppendEvent(ctx context.Context, event events.Event, outbox *events.OutboxMessage) (events.Event, error) {
	if err := event.ValidateForAppend(); err != nil {
		return events.Event{}, err
	}
	if outbox != nil {
		if err := outbox.Validate(); err != nil {
			return events.Event{}, err
		}
	}
	var seq int64
	if err := t.tx.QueryRowContext(ctx, `
UPDATE workflow_runs
SET next_event_seq = next_event_seq + 1
WHERE id=?
RETURNING next_event_seq - 1`, string(event.WorkflowRunID)).Scan(&seq); err != nil {
		return events.Event{}, mapNotFound(err)
	}
	payload := event.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if _, err := t.tx.ExecContext(ctx, `
INSERT INTO events(event_id, workflow_run_id, workflow_seq, type, timestamp, entity_type, entity_id, payload_version, payload)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`, string(event.ID), string(event.WorkflowRunID), seq, event.Type, formatTime(event.Timestamp), event.EntityType, event.EntityID, event.PayloadVersion, []byte(payload)); err != nil {
		return events.Event{}, fmt.Errorf("insert event: %w", err)
	}
	event.WorkflowSeq = seq
	event.Payload = payload
	if outbox != nil {
		outPayload := outbox.Payload
		if len(outPayload) == 0 {
			outPayload = payload
		}
		if _, err := t.tx.ExecContext(ctx, `
INSERT INTO outbox(event_id, topic, payload, created_at)
VALUES(?, ?, ?, ?)`, string(event.ID), outbox.Topic, []byte(outPayload), formatTime(event.Timestamp)); err != nil {
			return events.Event{}, fmt.Errorf("insert outbox record: %w", err)
		}
	}
	return event, nil
}

func (t *transaction) ListEvents(ctx context.Context, runID harnessmodel.WorkflowRunID, afterSeq int64, limit int) ([]events.Event, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	rows, err := t.tx.QueryContext(ctx, `
SELECT event_id, workflow_run_id, workflow_seq, type, timestamp, entity_type, entity_id, payload_version, payload
FROM events
WHERE workflow_run_id=? AND workflow_seq>?
ORDER BY workflow_seq
LIMIT ?`, string(runID), afterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("list workflow events: %w", err)
	}
	defer rows.Close()
	out := make([]events.Event, 0)
	for rows.Next() {
		var e events.Event
		var run string
		var ts string
		var payload []byte
		if err := rows.Scan(&e.ID, &run, &e.WorkflowSeq, &e.Type, &ts, &e.EntityType, &e.EntityID, &e.PayloadVersion, &payload); err != nil {
			return nil, fmt.Errorf("scan workflow event: %w", err)
		}
		e.WorkflowRunID = harnessmodel.WorkflowRunID(run)
		if e.Timestamp, err = parseTime(ts); err != nil {
			return nil, fmt.Errorf("parse event timestamp: %w", err)
		}
		e.Payload = append(json.RawMessage(nil), payload...)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow events: %w", err)
	}
	return out, nil
}

func mapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return harnessstore.ErrNotFound
	}
	return err
}

func formatTime(v time.Time) string { return v.UTC().Format(time.RFC3339Nano) }
func parseTime(v string) (time.Time, error) { return time.Parse(time.RFC3339Nano, v) }
