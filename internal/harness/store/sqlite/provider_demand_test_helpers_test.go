package sqlite

import (
	"context"
	"fmt"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func createSettledDemandReservation(t *testing.T, db *DB, now time.Time, suffix string, metric harnessmodel.QuotaMetricKind) (harnessmodel.ProviderAssignment, harnessmodel.ProviderReservation) {
	t.Helper()
	ctx := context.Background()
	attemptID := harnessmodel.AttemptID("att_demand_" + suffix)
	assignment := harnessmodel.ProviderAssignment{
		ID: harnessmodel.ProviderAssignmentID("pasn_demand_" + suffix), AttemptID: attemptID,
		AccountID: testProviderAccountID, ModelID: "model-a", State: harnessmodel.ProviderAssignmentActive, Revision: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	amount := 100.0
	if metric == harnessmodel.QuotaMetricFraction {
		amount = 0.5
	}
	reservation := harnessmodel.ProviderReservation{
		ID: harnessmodel.ProviderReservationID("pres_demand_" + suffix), AssignmentID: assignment.ID, AccountID: assignment.AccountID,
		WindowID: "window-" + suffix, ModelID: assignment.ModelID, Metric: metric, Amount: amount,
		State: harnessmodel.ProviderReservationActive, Revision: 1, CreatedAt: now, ExpiresAt: now.Add(time.Hour), UpdatedAt: now,
	}
	settled := reservation
	settled.State = harnessmodel.ProviderReservationSettled
	settled.Revision = 2
	settled.UpdatedAt = now.Add(time.Nanosecond)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if _, err := tx.CreateNextAttempt(ctx, harnessmodel.Attempt{ID: attemptID, NodeRunID: "nr_provider", State: harnessmodel.AttemptCreated, CreatedAt: now}); err != nil {
			return fmt.Errorf("create demand attempt: %w", err)
		}
		if err := tx.CreateProviderAssignment(ctx, assignment); err != nil {
			return err
		}
		if err := tx.CreateProviderReservation(ctx, reservation); err != nil {
			return err
		}
		return tx.CompareAndSwapProviderReservation(ctx, 1, settled)
	}); err != nil {
		t.Fatal(err)
	}
	return assignment, settled
}
