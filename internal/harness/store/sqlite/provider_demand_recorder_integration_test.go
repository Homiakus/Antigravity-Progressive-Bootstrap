package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	providerdemand "github.com/homiakus/agctl/internal/harness/provider/demand"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestProviderDemandRecorderAtomicallyRecordsAndReplays(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(65000, 0).UTC()
	seedProviderRuntimeParents(t, db, now)
	assignment, reservation := createSettledDemandReservation(t, db, now, "recorder", harnessmodel.QuotaMetricTokens)
	usage := harnessmodel.ProviderUsageSample{
		Key: "usage-recorder", AssignmentID: assignment.ID, ReservationID: reservation.ID, AccountID: assignment.AccountID, ModelID: assignment.ModelID,
		Metric: harnessmodel.QuotaMetricTokens, Amount: 77, ObservedAt: now.Add(time.Second), CreatedAt: now.Add(2 * time.Second),
	}
	dims := harnessmodel.ProviderDemandDimensions{UsageKey: usage.Key, TaskClass: "code", RepositoryClass: "large", ContextClass: "warm"}
	recorder := providerdemand.Recorder{Store: db}
	first, err := recorder.Record(ctx, usage, dims)
	if err != nil {
		t.Fatal(err)
	}
	if !first.UsageCreated || !first.DimensionsCreated {
		t.Fatalf("first record=%+v want both created", first)
	}
	replayUsage := usage
	replayUsage.CreatedAt = now.Add(time.Hour)
	replay, err := recorder.Record(ctx, replayUsage, dims)
	if err != nil {
		t.Fatal(err)
	}
	if replay.UsageCreated || replay.DimensionsCreated {
		t.Fatalf("replay record=%+v want no creates", replay)
	}

	badDims := dims
	badDims.ContextClass = "cold"
	if _, err := recorder.Record(ctx, replayUsage, badDims); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("conflicting replay error=%v want ErrConflict", err)
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		dr, err := demandReader(r)
		if err != nil {
			return err
		}
		history, err := dr.ListProviderDemandHistory(ctx, harnessmodel.ProviderDemandHistoryQuery{
			Provider: harnessmodel.ProviderCodex, ModelID: "model-a", Metric: harnessmodel.QuotaMetricTokens,
			TaskClass: "code", RepositoryClass: "large", ContextClass: "warm", Since: now, Limit: 10,
		})
		if err != nil {
			return err
		}
		if len(history) != 1 || history[0].Amount != 77 {
			t.Fatalf("history after recorder replay=%+v", history)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
