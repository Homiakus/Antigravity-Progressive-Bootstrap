package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVersionFifteenToSixteenCreatesProviderDemandHistory(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	openFixtureAtVersion(t, path, 15)
	db, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	version, err := db.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != 16 {
		t.Fatalf("schema version=%d want=16", version)
	}
	for _, name := range []string{"provider_demand_dimensions", "provider_demand_dimensions_by_classes", "provider_usage_samples_by_model_metric_observed"} {
		var got string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE name=?`, name).Scan(&got); err != nil {
			t.Fatalf("missing v16 object %s: %v", name, err)
		}
	}
}

func TestProviderDemandSchemaRejectsMissingUsageAndInvalidClasses(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_demand_dimensions(usage_key, task_class, repository_class, context_class)
VALUES('missing-usage','code','medium','warm')`); err == nil {
		t.Fatal("demand dimensions without usage unexpectedly accepted")
	}
	// CHECK constraints remain independently enforceable even when callers bypass
	// the Go validation boundary.
	for _, tc := range []struct {
		name string
		task string
		repo string
		ctx  string
	}{
		{name: "empty-task", task: "", repo: "medium", ctx: "warm"},
		{name: "empty-repo", task: "code", repo: "", ctx: "warm"},
		{name: "empty-context", task: "code", repo: "medium", ctx: ""},
		{name: "long-context", task: "code", repo: "medium", ctx: strings.Repeat("x", 129)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Missing usage alone would also fail the FK, so create a minimal valid
			// parent row graph first and exercise class CHECKs through a real usage.
		})
	}

	now := time.Unix(61000, 0).UTC()
	seedProviderRuntimeParents(t, db, now)
	assignmentID := "pasn_v16_schema"
	if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_assignments(id, attempt_id, account_id, model_id, state, revision, created_at, updated_at)
VALUES(?,?,?,'model-a','ACTIVE',1,?,?)`, assignmentID, string(testProviderAttemptID), string(testProviderAccountID), formatTime(now), formatTime(now)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_usage_samples(sample_key, assignment_id, account_id, model_id, metric, amount, observed_at, created_at)
VALUES('usage-v16-schema',?,?, 'model-a','TOKENS',1,?,?)`, assignmentID, string(testProviderAccountID), formatTime(now), formatTime(now)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		task string
		repo string
		ctx  string
	}{
		{name: "empty-task", task: "", repo: "medium", ctx: "warm"},
		{name: "empty-repo", task: "code", repo: "", ctx: "warm"},
		{name: "empty-context", task: "code", repo: "medium", ctx: ""},
		{name: "long-context", task: "code", repo: "medium", ctx: strings.Repeat("x", 129)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_demand_dimensions(usage_key, task_class, repository_class, context_class)
VALUES('usage-v16-schema',?,?,?)`, tc.task, tc.repo, tc.ctx); err == nil {
				t.Fatalf("invalid demand classes unexpectedly accepted: %+v", tc)
			}
		})
	}
}
