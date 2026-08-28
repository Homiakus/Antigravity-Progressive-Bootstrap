package sqlite

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func demandTx(tx harnessstore.Tx) (harnessstore.ProviderDemandTx, error) {
	dtx, ok := tx.(harnessstore.ProviderDemandTx)
	if !ok {
		return nil, fmt.Errorf("store transaction lacks provider demand capability")
	}
	return dtx, nil
}

func demandReader(r harnessstore.Reader) (harnessstore.ProviderDemandReader, error) {
	dr, ok := r.(harnessstore.ProviderDemandReader)
	if !ok {
		return nil, fmt.Errorf("store reader lacks provider demand capability")
	}
	return dr, nil
}

func TestProviderDemandStoreRoundTripIdempotencyAndHistoryFilters(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(62000, 0).UTC()
	seedProviderRuntimeParents(t, db, now)

	type inserted struct {
		assignment  harnessmodel.ProviderAssignment
		reservation harnessmodel.ProviderReservation
	}
	fixtures := map[string]inserted{}
	insert := func(suffix, key string, metric harnessmodel.QuotaMetricKind, amount float64, observed time.Time, task, repo, contextClass string) {
		t.Helper()
		assignment, reservation := createSettledDemandReservation(t, db, observed.Add(-time.Second), suffix, metric)
		usage := harnessmodel.ProviderUsageSample{
			Key: key, AssignmentID: assignment.ID, ReservationID: reservation.ID, AccountID: assignment.AccountID, ModelID: assignment.ModelID,
			Metric: metric, Amount: amount, ObservedAt: observed, CreatedAt: observed.Add(time.Second),
		}
		dims := harnessmodel.ProviderDemandDimensions{UsageKey: key, TaskClass: task, RepositoryClass: repo, ContextClass: contextClass}
		if err := db.Update(ctx, func(tx harnessstore.Tx) error {
			if _, _, err := tx.PutProviderUsageSample(ctx, usage); err != nil {
				return err
			}
			dtx, err := demandTx(tx)
			if err != nil {
				return err
			}
			_, _, err = dtx.PutProviderDemandDimensions(ctx, dims)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		fixtures[key] = inserted{assignment: assignment, reservation: reservation}
	}

	insert("one", "usage-1", harnessmodel.QuotaMetricTokens, 10, now.Add(time.Minute), "code", "medium", "warm")
	insert("two", "usage-2", harnessmodel.QuotaMetricTokens, 20, now.Add(2*time.Minute), "code", "medium", "cold")
	insert("three", "usage-3", harnessmodel.QuotaMetricTokens, 30, now.Add(3*time.Minute), "review", "medium", "warm")
	insert("four", "usage-4", harnessmodel.QuotaMetricRequests, 99, now.Add(4*time.Minute), "code", "medium", "warm")

	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		dtx, err := demandTx(tx)
		if err != nil {
			return err
		}
		dims := harnessmodel.ProviderDemandDimensions{UsageKey: "usage-1", TaskClass: "code", RepositoryClass: "medium", ContextClass: "warm"}
		stored, created, err := dtx.PutProviderDemandDimensions(ctx, dims)
		if err != nil {
			return err
		}
		if created || stored != dims {
			t.Fatalf("idempotent dimensions replay created=%v stored=%+v", created, stored)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		dtx, err := demandTx(tx)
		if err != nil {
			return err
		}
		_, _, err = dtx.PutProviderDemandDimensions(ctx, harnessmodel.ProviderDemandDimensions{UsageKey: "usage-1", TaskClass: "code", RepositoryClass: "large", ContextClass: "warm"})
		return err
	}); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("conflicting dimensions replay error=%v want ErrConflict", err)
	}

	// A second provider event for the same assignment/metric is raw telemetry,
	// not another independent task-demand sample.
	first := fixtures["usage-1"]
	secondUsage := harnessmodel.ProviderUsageSample{
		Key: "usage-1-second-event", AssignmentID: first.assignment.ID, ReservationID: first.reservation.ID,
		AccountID: first.assignment.AccountID, ModelID: first.assignment.ModelID, Metric: harnessmodel.QuotaMetricTokens,
		Amount: 11, ObservedAt: now.Add(90 * time.Second), CreatedAt: now.Add(90 * time.Second),
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if _, _, err := tx.PutProviderUsageSample(ctx, secondUsage); err != nil {
			return err
		}
		dtx, err := demandTx(tx)
		if err != nil {
			return err
		}
		_, _, err = dtx.PutProviderDemandDimensions(ctx, harnessmodel.ProviderDemandDimensions{
			UsageKey: secondUsage.Key, TaskClass: "code", RepositoryClass: "medium", ContextClass: "warm",
		})
		return err
	}); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("second canonical sample error=%v want ErrConflict", err)
	}

	if err := db.View(ctx, func(r harnessstore.Reader) error {
		dr, err := demandReader(r)
		if err != nil {
			return err
		}
		got, err := dr.GetProviderDemandDimensions(ctx, "usage-1")
		if err != nil {
			return err
		}
		if got.RepositoryClass != "medium" || got.ContextClass != "warm" {
			t.Fatalf("unexpected dimensions: %+v", got)
		}
		exact, err := dr.ListProviderDemandHistory(ctx, harnessmodel.ProviderDemandHistoryQuery{
			Provider: harnessmodel.ProviderCodex, ModelID: "model-a", Metric: harnessmodel.QuotaMetricTokens,
			TaskClass: "code", RepositoryClass: "medium", ContextClass: "warm", Since: now, Limit: 10,
		})
		if err != nil {
			return err
		}
		if len(exact) != 1 || exact[0].UsageKey != "usage-1" || exact[0].Amount != 10 {
			t.Fatalf("unexpected exact history: %+v", exact)
		}
		repo, err := dr.ListProviderDemandHistory(ctx, harnessmodel.ProviderDemandHistoryQuery{
			Provider: harnessmodel.ProviderCodex, ModelID: "model-a", Metric: harnessmodel.QuotaMetricTokens,
			TaskClass: "code", RepositoryClass: "medium", Since: now, Limit: 10,
		})
		if err != nil {
			return err
		}
		if len(repo) != 2 || repo[0].UsageKey != "usage-2" || repo[1].UsageKey != "usage-1" {
			t.Fatalf("unexpected repository history/order: %+v", repo)
		}
		baseline, err := dr.ListProviderDemandHistory(ctx, harnessmodel.ProviderDemandHistoryQuery{
			Provider: harnessmodel.ProviderCodex, ModelID: "model-a", Metric: harnessmodel.QuotaMetricTokens,
			Since: now, Limit: 2,
		})
		if err != nil {
			return err
		}
		if len(baseline) != 2 || baseline[0].UsageKey != "usage-3" || baseline[1].UsageKey != "usage-2" {
			t.Fatalf("unexpected baseline history/limit: %+v", baseline)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestProviderDemandDimensionsRequireAuthoritativeUsageModel(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(63000, 0).UTC()
	seedProviderRuntimeParents(t, db, now)
	assignment, reservation := createSettledDemandReservation(t, db, now, "missing-model", harnessmodel.QuotaMetricTokens)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		_, _, err := tx.PutProviderUsageSample(ctx, harnessmodel.ProviderUsageSample{
			Key: "usage-no-model", AssignmentID: assignment.ID, ReservationID: reservation.ID, AccountID: assignment.AccountID,
			Metric: harnessmodel.QuotaMetricTokens, Amount: 1, ObservedAt: now, CreatedAt: now,
		})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		dtx, err := demandTx(tx)
		if err != nil {
			return err
		}
		_, _, err = dtx.PutProviderDemandDimensions(ctx, harnessmodel.ProviderDemandDimensions{
			UsageKey: "usage-no-model", TaskClass: "code", RepositoryClass: "small", ContextClass: "cold",
		})
		return err
	}); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("missing model binding error=%v want ErrConflict", err)
	}
}
