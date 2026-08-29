package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	providerreservation "github.com/homiakus/agctl/internal/harness/provider/reservation"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func BenchmarkAtomicProviderReservation10KActiveClaims(b *testing.B) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(b.TempDir(), "state.db"), Options{})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	now := time.Unix(40_000, 0).UTC()
	seedBenchmarkRun(b, db, now)

	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CreateGraphRevision(ctx, harnessmodel.GraphRevision{
			WorkflowRunID: "wfr_test", Number: 1, CreatedAt: now, Reason: "provider-reservation-benchmark",
		}); err != nil {
			return err
		}
		if err := tx.CreateNodeRun(ctx, harnessmodel.NodeRun{
			ID: "nr_provider_reservation_bench", WorkflowRunID: "wfr_test", NodeID: "a",
			GraphRevision: 1, Generation: 1, State: harnessmodel.NodeReady,
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return err
		}
		if _, err := tx.CreateNextAttempt(ctx, harnessmodel.Attempt{
			ID: "att_reservation_background", NodeRunID: "nr_provider_reservation_bench",
			State: harnessmodel.AttemptCreated, CreatedAt: now,
		}); err != nil {
			return err
		}
		if err := tx.UpsertProviderAccount(ctx, harnessmodel.ProviderAccount{
			ID: testProviderAccountID, Provider: harnessmodel.ProviderCodex, Name: "reservation-benchmark",
			State: harnessmodel.ProviderAccountActive, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return err
		}
		if err := tx.UpsertProviderModel(ctx, harnessmodel.ProviderModelDescriptor{
			AccountID: testProviderAccountID, Provider: harnessmodel.ProviderCodex,
			ID: reservationTestModel, Enabled: true,
		}, now); err != nil {
			return err
		}
		remaining := 20_000.0
		if err := tx.AppendProviderCapacity(ctx, harnessmodel.ProviderCapacitySnapshot{
			AccountID: testProviderAccountID, Provider: harnessmodel.ProviderCodex,
			Health: harnessmodel.ProviderHealthHealthy, ObservedAt: now,
			Windows: []harnessmodel.QuotaWindow{{
				ID: "tokens/benchmark-target", Metric: harnessmodel.QuotaMetricTokens,
				Limit: &remaining, Remaining: &remaining, ObservedAt: now, Confidence: 1,
			}},
		}); err != nil {
			return err
		}
		background := harnessmodel.ProviderAssignment{
			ID: "pasn_reservation_background", AttemptID: "att_reservation_background",
			AccountID: testProviderAccountID, ModelID: reservationTestModel,
			State: harnessmodel.ProviderAssignmentActive, Revision: 1,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.CreateProviderAssignment(ctx, background); err != nil {
			return err
		}
		for i := 0; i < 10_000; i++ {
			claim := harnessmodel.ProviderReservation{
				ID: harnessmodel.ProviderReservationID(fmt.Sprintf("pres_background_%05d", i)),
				AssignmentID: background.ID, AccountID: background.AccountID,
				WindowID: fmt.Sprintf("tokens/background/%05d", i), Metric: harnessmodel.QuotaMetricTokens,
				Amount: 1, State: harnessmodel.ProviderReservationActive, Revision: 1,
				CreatedAt: now, ExpiresAt: now.Add(4 * time.Minute), UpdatedAt: now,
			}
			if err := tx.CreateProviderReservation(ctx, claim); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		b.Fatal(err)
	}

	service := providerreservation.Service{Store: db, Policy: reservationPolicy()}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		attemptID := harnessmodel.AttemptID(fmt.Sprintf("att_reservation_bench_%d", i))
		if err := db.Update(ctx, func(tx harnessstore.Tx) error {
			_, err := tx.CreateNextAttempt(ctx, harnessmodel.Attempt{
				ID: attemptID, NodeRunID: "nr_provider_reservation_bench",
				State: harnessmodel.AttemptCreated, CreatedAt: now.Add(time.Duration(i+1) * time.Nanosecond),
			})
			return err
		}); err != nil {
			b.Fatal(err)
		}
		assignmentID := harnessmodel.ProviderAssignmentID(fmt.Sprintf("pasn_reservation_bench_%d", i))
		b.StartTimer()
		if _, err := service.Reserve(ctx, reservationRequest(attemptID, assignmentID, 1, now.Add(time.Duration(i+1)*time.Nanosecond))); err != nil {
			b.Fatal(err)
		}
	}
}
