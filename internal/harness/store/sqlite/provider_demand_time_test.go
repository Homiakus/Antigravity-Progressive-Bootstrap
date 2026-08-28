package sqlite

import (
	"context"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestProviderDemandHistoryUsesNanosecondHorizonAndOrdering(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	base := time.Unix(64000, 0).UTC()
	seedProviderRuntimeParents(t, db, base)
	for i, tc := range []struct {
		key  string
		when time.Time
	}{
		{key: "usage-whole-second", when: base},
		{key: "usage-fractional", when: base.Add(100 * time.Millisecond)},
	} {
		assignment, reservation := createSettledDemandReservation(t, db, tc.when.Add(-time.Second), tc.key, harnessmodel.QuotaMetricTokens)
		if err := db.Update(ctx, func(tx harnessstore.Tx) error {
			if _, _, err := tx.PutProviderUsageSample(ctx, harnessmodel.ProviderUsageSample{
				Key: tc.key, AssignmentID: assignment.ID, ReservationID: reservation.ID, AccountID: assignment.AccountID, ModelID: assignment.ModelID,
				Metric: harnessmodel.QuotaMetricTokens, Amount: float64(i + 1), ObservedAt: tc.when, CreatedAt: tc.when,
			}); err != nil {
				return err
			}
			dtx, err := demandTx(tx)
			if err != nil {
				return err
			}
			_, _, err = dtx.PutProviderDemandDimensions(ctx, harnessmodel.ProviderDemandDimensions{
				UsageKey: tc.key, TaskClass: "code", RepositoryClass: "small", ContextClass: "warm",
			})
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}

	if err := db.View(ctx, func(r harnessstore.Reader) error {
		dr, err := demandReader(r)
		if err != nil {
			return err
		}
		history, err := dr.ListProviderDemandHistory(ctx, harnessmodel.ProviderDemandHistoryQuery{
			Provider: harnessmodel.ProviderCodex, ModelID: "model-a", Metric: harnessmodel.QuotaMetricTokens,
			TaskClass: "code", RepositoryClass: "small", ContextClass: "warm",
			Since: base.Add(50 * time.Millisecond), Limit: 10,
		})
		if err != nil {
			return err
		}
		if len(history) != 1 || history[0].UsageKey != "usage-fractional" {
			t.Fatalf("history=%+v want only fractional sample", history)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
