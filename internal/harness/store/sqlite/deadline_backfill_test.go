package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func seedV4ReadyDeadline(t *testing.T, path, deadline string) time.Time {
	t.Helper()
	openFixtureAtVersion(t, path, 4)
	ctx := context.Background()
	raw, err := sql.Open("sqlite", buildDSN(path, Options{}))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	now := time.Date(2026, 8, 20, 19, 0, 0, 0, time.UTC)
	def := testDefinition(now)
	payload, err := json.Marshal(def)
	if err != nil {
		t.Fatal(err)
	}
	nodeJSON, err := json.Marshal(def.Nodes[0])
	if err != nil {
		t.Fatal(err)
	}
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO workflow_definitions(id,version,name,compiler_version,definition_json,created_at) VALUES(?,?,?,?,?,?)`, []any{string(def.ID), def.Version, def.Name, def.CompilerVersion, payload, formatTime(now)}},
		{`INSERT INTO nodes(definition_id,definition_version,node_id,spec_json) VALUES(?,?,?,?)`, []any{string(def.ID), def.Version, string(def.Nodes[0].ID), nodeJSON}},
		{`INSERT INTO workflow_runs(id,definition_id,definition_version,state,current_graph_revision,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, []any{"wfr_deadline_v4", string(def.ID), def.Version, string(harnessmodel.WorkflowRunning), 1, formatTime(now), formatTime(now)}},
		{`INSERT INTO graph_revisions(workflow_run_id,number,reason,created_at) VALUES(?,?,?,?)`, []any{"wfr_deadline_v4", 1, "deadline-backfill-test", formatTime(now)}},
		{`INSERT INTO node_runs(id,workflow_run_id,definition_id,definition_version,node_id,graph_revision,generation,state,remaining_dependencies,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, []any{"nr_deadline_v4", "wfr_deadline_v4", string(def.ID), def.Version, string(def.Nodes[0].ID), 1, 1, string(harnessmodel.NodeReady), 0, formatTime(now), formatTime(now)}},
		{`INSERT INTO ready_queue(node_run_id,workflow_run_id,priority,effective_priority,ready_at,not_before,resource_class,wait_reason,wait_detail,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, []any{"nr_deadline_v4", "wfr_deadline_v4", 0, 0, formatTime(now), deadline, "", "", "", formatTime(now)}},
	}
	for _, statement := range statements {
		if _, err := raw.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed v4 deadline fixture: %v", err)
		}
	}
	return now
}

func TestVersionFourReadyDeadlineBackfillsToIntegerAndGatesExactly(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	deadline := time.Date(2026, 8, 20, 19, 0, 0, 100_000_000, time.UTC)
	seedV4ReadyDeadline(t, path, formatTime(deadline))

	db, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var gotNS int64
	if err := db.SQLDB().QueryRowContext(ctx, `SELECT not_before_ns FROM ready_queue WHERE node_run_id='nr_deadline_v4'`).Scan(&gotNS); err != nil {
		t.Fatal(err)
	}
	if gotNS != deadline.UnixNano() {
		t.Fatalf("backfilled deadline ns=%d want=%d", gotNS, deadline.UnixNano())
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		early, err := reader.ListReadyNodes(ctx, "wfr_deadline_v4", deadline.Add(-time.Nanosecond), 10)
		if err != nil {
			return err
		}
		if len(early) != 0 {
			t.Fatalf("READY node became schedulable before integer deadline: %+v", early)
		}
		due, err := reader.ListReadyNodes(ctx, "wfr_deadline_v4", deadline, 10)
		if err != nil {
			return err
		}
		if len(due) != 1 || due[0].NodeRunID != "nr_deadline_v4" || !due[0].NotBefore.Equal(deadline) {
			t.Fatalf("READY node missing at exact deadline: %+v", due)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestVersionFourMalformedReadyDeadlineFailsOpenInsteadOfSchedulingEarly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	seedV4ReadyDeadline(t, path, "not-a-rfc3339-time")
	if _, err := Open(context.Background(), path, Options{}); err == nil || !strings.Contains(err.Error(), "backfill SQLite deadlines") {
		t.Fatalf("malformed historical deadline did not fail startup: %v", err)
	}
}

func TestVersionFourOutOfRangeReadyDeadlineFailsOpenInsteadOfWrapping(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	seedV4ReadyDeadline(t, path, formatTime(maxUnixNanoTime.Add(time.Nanosecond)))
	if _, err := Open(context.Background(), path, Options{}); err == nil || !strings.Contains(err.Error(), "not representable durably") {
		t.Fatalf("out-of-range historical deadline did not fail startup: %v", err)
	}
}
