package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestVersionFourteenToFifteenCreatesProviderRuntimeLedger(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	openFixtureAtVersion(t, path, 14)
	db, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	version, err := db.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != 15 {
		t.Fatalf("schema version=%d want=15", version)
	}
	for _, table := range []string{"provider_assignments", "provider_reservations", "provider_usage_samples", "provider_circuit_state"} {
		var got string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&got); err != nil {
			t.Fatalf("missing v15 table %s: %v", table, err)
		}
	}
	for _, index := range []string{
		"provider_assignments_by_attempt", "provider_assignments_by_account_state", "provider_assignments_one_active_per_attempt",
		"provider_reservations_by_account_state_expiry", "provider_reservations_by_assignment", "provider_reservations_one_active_dimension",
		"provider_usage_samples_by_assignment", "provider_usage_samples_by_account_metric", "provider_circuit_by_state_probe",
	} {
		var got string
		if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='index' AND name=?`, index).Scan(&got); err != nil {
			t.Fatalf("missing v15 index %s: %v", index, err)
		}
	}
}

func TestProviderRuntimeSchemaRejectsBrokenForeignKeysChecksAndDuplicateActiveClaims(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(2000, 0).UTC()
	seedProviderRuntimeParents(t, db, now)

	if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_assignments(id, attempt_id, account_id, model_id, state, revision, created_at, updated_at)
VALUES('pasn_missing','missing-attempt',?,'gpt-test','ACTIVE',1,?,?)`, string(testProviderAccountID), formatTime(now), formatTime(now)); err == nil {
		t.Fatal("assignment with missing attempt unexpectedly accepted")
	}

	assignment := harnessmodel.ProviderAssignment{
		ID: "pasn_schema", AttemptID: testProviderAttemptID, AccountID: testProviderAccountID, ModelID: "gpt-test",
		State: harnessmodel.ProviderAssignmentActive, Revision: 1, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateProviderAssignment(ctx, assignment) }); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_assignments(id, attempt_id, account_id, model_id, state, revision, created_at, updated_at)
VALUES('pasn_duplicate',?,?, 'gpt-test','ACTIVE',1,?,?)`, string(testProviderAttemptID), string(testProviderAccountID), formatTime(now.Add(2*time.Second)), formatTime(now.Add(2*time.Second))); err == nil {
		t.Fatal("second ACTIVE assignment for one attempt unexpectedly accepted")
	}

	for name, metric, amount := range []struct {
		name   string
		metric string
		amount float64
	}{
		{name: "opaque", metric: "OPAQUE", amount: 1},
		{name: "zero", metric: "TOKENS", amount: 0},
		{name: "negative", metric: "TOKENS", amount: -1},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_reservations(
 id, assignment_id, account_id, window_id, metric, amount, state, revision, created_at, expires_at, expires_at_ns, updated_at
) VALUES(?,?,?,?,?,?,'ACTIVE',1,?,?,?,?)`, "pres_bad_"+name, string(assignment.ID), string(testProviderAccountID), "primary", metric, amount,
				formatTime(now), formatTime(now.Add(time.Minute)), now.Add(time.Minute).UnixNano(), formatTime(now)); err == nil {
				t.Fatalf("invalid reservation metric=%s amount=%v unexpectedly accepted", metric, amount)
			}
		})
	}

	if _, err := db.SQLDB().ExecContext(ctx, `
INSERT INTO provider_circuit_state(account_id, model_id, revision, state, consecutive_failures, probe_in_flight, updated_at)
VALUES('missing-account','',1,'CLOSED',0,0,?)`, formatTime(now)); err == nil {
		t.Fatal("provider circuit without account unexpectedly accepted")
	}
}
