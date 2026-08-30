package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestVersionFifteenToSixteenCreatesPlanDigestColumnAndIndex(t *testing.T) {
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

	// Verify index exists
	var indexName string
	if err := db.SQLDB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='index' AND name='provider_assignments_by_plan_digest'`).Scan(&indexName); err != nil {
		t.Fatalf("missing v16 index provider_assignments_by_plan_digest: %v", err)
	}

	// Verify column can be queried directly
	var colCount int
	if err := db.SQLDB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('provider_assignments') WHERE name='plan_digest'`).Scan(&colCount); err != nil {
		t.Fatal(err)
	}
	if colCount != 1 {
		t.Fatalf("expected plan_digest column on provider_assignments, got count=%d", colCount)
	}
}

func TestProviderAssignmentPlanDigestPersistenceAndCAS(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(2000, 0).UTC()
	seedProviderRuntimeParents(t, db, now)

	planDigest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	assignment := harnessmodel.ProviderAssignment{
		ID:         "pasn_digest_test",
		AttemptID:  testProviderAttemptID,
		AccountID:  testProviderAccountID,
		ModelID:    "gpt-test",
		PlanDigest: planDigest,
		State:      harnessmodel.ProviderAssignmentActive,
		Revision:   1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.CreateProviderAssignment(ctx, assignment)
	}); err != nil {
		t.Fatalf("CreateProviderAssignment failed: %v", err)
	}

	// Verify read back
	var loaded harnessmodel.ProviderAssignment
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		var err error
		loaded, err = r.GetProviderAssignment(ctx, assignment.ID)
		return err
	}); err != nil {
		t.Fatalf("GetProviderAssignment failed: %v", err)
	}

	if loaded.PlanDigest != planDigest {
		t.Fatalf("loaded plan digest = %q, want %q", loaded.PlanDigest, planDigest)
	}

	// Test CAS transition to COMPLETED preserves plan digest
	assignment.Revision = 2
	assignment.State = harnessmodel.ProviderAssignmentCompleted
	assignment.UpdatedAt = now.Add(time.Second)

	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.CompareAndSwapProviderAssignment(ctx, 1, assignment)
	}); err != nil {
		t.Fatalf("CompareAndSwapProviderAssignment failed: %v", err)
	}

	// Verify CAS with altered plan digest is rejected
	tampered := assignment
	tampered.Revision = 3
	tampered.PlanDigest = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	tampered.UpdatedAt = now.Add(2 * time.Second)

	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.CompareAndSwapProviderAssignment(ctx, 2, tampered)
	}); err == nil {
		t.Fatal("CAS unexpectedly allowed mutating plan digest")
	}
}
