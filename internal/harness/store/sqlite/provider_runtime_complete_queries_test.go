package sqlite

import (
	"context"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestCompleteActiveReservationReadCannotBeTruncatedByDiagnosticLimit(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(2200, 0).UTC()
	seedProviderRuntimeParents(t, db, now)
	assignment := harnessmodel.ProviderAssignment{
		ID: "pasn_complete", AttemptID: testProviderAttemptID, AccountID: testProviderAccountID, ModelID: "model-a",
		State: harnessmodel.ProviderAssignmentActive, Revision: 1, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
	}
	reservations := []harnessmodel.ProviderReservation{
		{
			ID: "pres_complete_tokens", AssignmentID: assignment.ID, AccountID: assignment.AccountID, WindowID: "tokens", ModelID: assignment.ModelID,
			Metric: harnessmodel.QuotaMetricTokens, Amount: 100, State: harnessmodel.ProviderReservationActive, Revision: 1,
			CreatedAt: now.Add(2 * time.Second), ExpiresAt: now.Add(time.Minute), UpdatedAt: now.Add(2 * time.Second),
		},
		{
			ID: "pres_complete_requests", AssignmentID: assignment.ID, AccountID: assignment.AccountID, WindowID: "requests", ModelID: assignment.ModelID,
			Metric: harnessmodel.QuotaMetricRequests, Amount: 1, State: harnessmodel.ProviderReservationActive, Revision: 1,
			CreatedAt: now.Add(3 * time.Second), ExpiresAt: now.Add(2 * time.Minute), UpdatedAt: now.Add(3 * time.Second),
		},
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CreateProviderAssignment(ctx, assignment); err != nil {
			return err
		}
		for _, reservation := range reservations {
			if err := tx.CreateProviderReservation(ctx, reservation); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.View(ctx, func(r harnessstore.Reader) error {
		paged, err := r.ListActiveProviderReservations(ctx, testProviderAccountID, now.Add(10*time.Second), 1)
		if err != nil {
			return err
		}
		if len(paged) != 1 {
			t.Fatalf("diagnostic page len=%d want=1", len(paged))
		}
		complete, err := r.ListAllActiveProviderReservations(ctx, testProviderAccountID, now.Add(10*time.Second))
		if err != nil {
			return err
		}
		if len(complete) != 2 {
			t.Fatalf("complete active reservation len=%d want=2: %+v", len(complete), complete)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
