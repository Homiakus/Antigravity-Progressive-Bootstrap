package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

const SchemaVersion = 14

type migration struct {
	Version int
	Name    string
	SQL     string
}

var migrations = []migration{
	{
		Version: 1,
		Name:    "durable_harness_core",
		SQL: `
CREATE TABLE workflow_definitions (
    id TEXT NOT NULL,
    version INTEGER NOT NULL CHECK(version > 0),
    name TEXT NOT NULL,
    compiler_version TEXT NOT NULL,
    definition_json BLOB NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY(id, version)
);

CREATE TABLE workflow_runs (
    id TEXT PRIMARY KEY,
    definition_id TEXT NOT NULL,
    definition_version INTEGER NOT NULL CHECK(definition_version > 0),
    state TEXT NOT NULL,
    current_graph_revision INTEGER NOT NULL DEFAULT 0 CHECK(current_graph_revision >= 0),
    next_event_seq INTEGER NOT NULL DEFAULT 1 CHECK(next_event_seq > 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(id, definition_id, definition_version),
    FOREIGN KEY(definition_id, definition_version)
        REFERENCES workflow_definitions(id, version)
        ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE TABLE graph_revisions (
    workflow_run_id TEXT NOT NULL,
    number INTEGER NOT NULL CHECK(number > 0),
    parent_number INTEGER,
    reason TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    PRIMARY KEY(workflow_run_id, number),
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE,
    FOREIGN KEY(workflow_run_id, parent_number)
        REFERENCES graph_revisions(workflow_run_id, number)
        ON DELETE RESTRICT,
    CHECK(parent_number IS NULL OR parent_number < number)
);

CREATE TABLE nodes (
    definition_id TEXT NOT NULL,
    definition_version INTEGER NOT NULL,
    node_id TEXT NOT NULL,
    spec_json BLOB NOT NULL,
    PRIMARY KEY(definition_id, definition_version, node_id),
    FOREIGN KEY(definition_id, definition_version)
        REFERENCES workflow_definitions(id, version)
        ON DELETE CASCADE
);

CREATE TABLE dependencies (
    definition_id TEXT NOT NULL,
    definition_version INTEGER NOT NULL,
    node_id TEXT NOT NULL,
    depends_on_node_id TEXT NOT NULL,
    required INTEGER NOT NULL DEFAULT 1 CHECK(required IN (0,1)),
    PRIMARY KEY(definition_id, definition_version, node_id, depends_on_node_id),
    CHECK(node_id <> depends_on_node_id),
    FOREIGN KEY(definition_id, definition_version, node_id)
        REFERENCES nodes(definition_id, definition_version, node_id)
        ON DELETE CASCADE,
    FOREIGN KEY(definition_id, definition_version, depends_on_node_id)
        REFERENCES nodes(definition_id, definition_version, node_id)
        ON DELETE CASCADE
);

CREATE INDEX dependencies_by_parent
    ON dependencies(definition_id, definition_version, depends_on_node_id);

CREATE TABLE node_runs (
    id TEXT PRIMARY KEY,
    workflow_run_id TEXT NOT NULL,
    definition_id TEXT NOT NULL,
    definition_version INTEGER NOT NULL,
    node_id TEXT NOT NULL,
    graph_revision INTEGER NOT NULL CHECK(graph_revision > 0),
    generation INTEGER NOT NULL DEFAULT 1 CHECK(generation > 0),
    state TEXT NOT NULL,
    remaining_dependencies INTEGER NOT NULL DEFAULT 0 CHECK(remaining_dependencies >= 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(workflow_run_id, node_id, generation),
    FOREIGN KEY(workflow_run_id, definition_id, definition_version)
        REFERENCES workflow_runs(id, definition_id, definition_version)
        ON DELETE CASCADE,
    FOREIGN KEY(definition_id, definition_version, node_id)
        REFERENCES nodes(definition_id, definition_version, node_id)
        ON DELETE RESTRICT,
    FOREIGN KEY(workflow_run_id, graph_revision)
        REFERENCES graph_revisions(workflow_run_id, number)
        ON DELETE RESTRICT
);

CREATE INDEX node_runs_by_state ON node_runs(state, updated_at);
CREATE INDEX node_runs_by_workflow ON node_runs(workflow_run_id, state);

CREATE TABLE attempts (
    id TEXT PRIMARY KEY,
    node_run_id TEXT NOT NULL,
    attempt_number INTEGER NOT NULL CHECK(attempt_number > 0),
    state TEXT NOT NULL,
    worker_id TEXT,
    lease_epoch INTEGER NOT NULL DEFAULT 0 CHECK(lease_epoch >= 0),
    created_at TEXT NOT NULL,
    started_at TEXT,
    finished_at TEXT,
    error_class TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    UNIQUE(node_run_id, attempt_number),
    FOREIGN KEY(node_run_id) REFERENCES node_runs(id) ON DELETE CASCADE
);

CREATE INDEX attempts_by_node_run ON attempts(node_run_id, attempt_number);
CREATE INDEX attempts_by_state ON attempts(state, created_at);

CREATE TABLE events (
    event_id TEXT PRIMARY KEY,
    workflow_run_id TEXT NOT NULL,
    workflow_seq INTEGER NOT NULL CHECK(workflow_seq > 0),
    type TEXT NOT NULL,
    timestamp TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    payload_version INTEGER NOT NULL CHECK(payload_version > 0),
    payload BLOB NOT NULL,
    UNIQUE(workflow_run_id, workflow_seq),
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE
);

CREATE INDEX events_by_workflow_seq ON events(workflow_run_id, workflow_seq);
CREATE INDEX events_by_entity ON events(entity_type, entity_id, timestamp);

CREATE TABLE outbox (
    outbox_id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL UNIQUE,
    topic TEXT NOT NULL,
    payload BLOB NOT NULL,
    created_at TEXT NOT NULL,
    delivered_at TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK(attempt_count >= 0),
    next_attempt_at TEXT,
    FOREIGN KEY(event_id) REFERENCES events(event_id) ON DELETE CASCADE
);

CREATE INDEX outbox_pending
    ON outbox(delivered_at, next_attempt_at, outbox_id);
`,
	},
	{
		Version: 2,
		Name:    "workflow_progress_counters",
		SQL: `
CREATE TABLE workflow_progress (
    workflow_run_id TEXT PRIMARY KEY,
    total_nodes INTEGER NOT NULL DEFAULT 0 CHECK(total_nodes >= 0),
    terminal_nodes INTEGER NOT NULL DEFAULT 0 CHECK(terminal_nodes >= 0),
    failed_nodes INTEGER NOT NULL DEFAULT 0 CHECK(failed_nodes >= 0),
    updated_at TEXT NOT NULL,
    CHECK(terminal_nodes <= total_nodes),
    CHECK(failed_nodes <= terminal_nodes),
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE
);
`,
	},
}

func migrate(ctx context.Context, db *sql.DB) error {
	if err := ensureMigrationTable(ctx, db); err != nil {
		return err
	}
	applied, err := loadAppliedMigrations(ctx, db)
	if err != nil {
		return err
	}
	for version, record := range applied {
		if version > SchemaVersion {
			return fmt.Errorf("SQLite schema version %d is newer than supported version %d", version, SchemaVersion)
		}
		m, ok := migrationByVersion(version)
		if !ok {
			return fmt.Errorf("SQLite contains unknown migration version %d", version)
		}
		if record.Name != m.Name || record.Checksum != migrationChecksum(m) {
			return fmt.Errorf("SQLite migration %d checksum/name mismatch; released migrations are immutable", version)
		}
	}
	for version := 1; version <= SchemaVersion; version++ {
		if _, ok := applied[version]; ok {
			continue
		}
		m, ok := migrationByVersion(version)
		if !ok {
			return fmt.Errorf("missing compiled SQLite migration version %d", version)
		}
		if err := applyMigration(ctx, db, m); err != nil {
			return err
		}
	}
	return nil
}

type appliedMigration struct {
	Name     string
	Checksum string
}

func ensureMigrationTable(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration bootstrap: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY CHECK(version > 0),
    name TEXT NOT NULL,
    checksum TEXT NOT NULL,
    applied_at TEXT NOT NULL
)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration bootstrap: %w", err)
	}
	return nil
}

func loadAppliedMigrations(ctx context.Context, db *sql.DB) (map[int]appliedMigration, error) {
	rows, err := db.QueryContext(ctx, `SELECT version, name, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("read schema migrations: %w", err)
	}
	defer rows.Close()
	out := map[int]appliedMigration{}
	for rows.Next() {
		var version int
		var rec appliedMigration
		if err := rows.Scan(&version, &rec.Name, &rec.Checksum); err != nil {
			return nil, fmt.Errorf("scan schema migration: %w", err)
		}
		out[version] = rec
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema migrations: %w", err)
	}
	return out, nil
}

func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", m.Version, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
		return fmt.Errorf("apply migration %d %s: %w", m.Version, m.Name, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations(version,name,checksum,applied_at) VALUES(?,?,?,?)`,
		m.Version, m.Name, migrationChecksum(m), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("record migration %d: %w", m.Version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", m.Version, err)
	}
	return nil
}

func migrationByVersion(version int) (migration, bool) {
	for _, m := range migrations {
		if m.Version == version {
			return m, true
		}
	}
	return migration{}, false
}

func migrationChecksum(m migration) string {
	sum := sha256.Sum256([]byte(m.SQL))
	return hex.EncodeToString(sum[:])
}

func schemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}
