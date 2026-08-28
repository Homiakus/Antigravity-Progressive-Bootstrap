package sqlite

import (
	"context"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestProviderAssignmentHistoryAllowsOneActiveAfterSupersede(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(2100, 0).UTC()
	seedProviderRuntimeParents(t, db, now)

	first := harnessmodel.ProviderAssignment{
		ID: "pasn_history_1", AttemptID: testProviderAttemptID, AccountID: testProviderAccountID, ModelID: "model-a",
		State: harnessmodel.ProviderAssignmentActive, Revision: 1, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateProviderAssignment(ctx, first) }); err != nil {
		t.Fatal(err)
	}
	superseded := first
	superseded.State = harnessmodel.ProviderAssignmentSuperseded
	superseded.Revision = 2
	superseded.UpdatedAt = now.Add(2 * time.Second)
	second := harnessmodel.ProviderAssignment{
		ID: "pasn_history_2", AttemptID: testProviderAttemptID, AccountID: testProviderAccountID, ModelID: "model-b",
		State: harnessmodel.ProviderAssignmentActive, Revision: 1, CreatedAt: now.Add(3 * time.Second), UpdatedAt: now.Add(3 * time.Second),
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CompareAndSwapProviderAssignment(ctx, 1, superseded); err != nil {
			return err
		}
		return tx.CreateProviderAssignment(ctx, second)
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.View(ctx, func(r harnessstore.Reader) error {
		history, err := r.ListProviderAssignmentsByAttempt(ctx, testProviderAttemptID)
		if err != nil {
			return err
		}
		if len(history) != 2 || history[0].ID != first.ID || history[0].State != harnessmodel.ProviderAssignmentSuperseded || history[1].ID != second.ID || history[1].State != harnessmodel.ProviderAssignmentActive {
			t.Fatalf("unexpected assignment history: %+v", history)
		}
		active, err := r.GetActiveProviderAssignment(ctx, testProviderAttemptID)
		if err != nil {
			return err
		}
		if active.ID != second.ID {
			t.Fatalf("active assignment=%s want=%s", active.ID, second.ID)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
