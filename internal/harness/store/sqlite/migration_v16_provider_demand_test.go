package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
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
	if version != SchemaVersion {
		t.Fatalf("schema version=%d want=%d", version, SchemaVersion)
	}
	for _, name := range []string{
		"provider_demand_dimensions",
		"provider_demand_dimensions_require_settled_usage",
		"provider_demand_dimensions_by_classes_time",
		"provider_usage_samples_by_model_metric_account",
	} {
		var got string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE name=?`, name).Scan(&got); err != nil {
			t.Fatalf("missing v16 object %s: %v", name, err)
		}
	}
}

func TestProviderDemandSchemaRejectsMissingUsageAndInvalidClasses(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(61000, 0).UTC()
	seedProviderRuntimeParents(t, db, now)
	assignment, reservation := createSettledDemandReservation(t, db, now, "v16-schema", harnessmodel.QuotaMetricTokens)

	if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_demand_dimensions(
 usage_key, assignment_id, metric, task_class, repository_class, context_class, usage_observed_at_ns
) VALUES('missing-usage',?,'TOKENS','code','medium','warm',?)`, string(assignment.ID), now.UnixNano()); err == nil {
		t.Fatal("demand dimensions without settled usage unexpectedly accepted")
	}
	if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_usage_samples(sample_key, assignment_id, reservation_id, account_id, model_id, metric, amount, observed_at, created_at)
VALUES('usage-v16-schema',?,?,?,'model-a','TOKENS',1,?,?)`, string(assignment.ID), string(reservation.ID), string(testProviderAccountID), formatTime(now), formatTime(now)); err != nil {
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
INSERT INTO provider_demand_dimensions(
 usage_key, assignment_id, metric, task_class, repository_class, context_class, usage_observed_at_ns
) VALUES('usage-v16-schema',?,'TOKENS',?,?,?,?)`, string(assignment.ID), tc.task, tc.repo, tc.ctx, now.UnixNano()); err == nil {
				t.Fatalf("invalid demand classes unexpectedly accepted: %+v", tc)
			}
		})
	}
}
