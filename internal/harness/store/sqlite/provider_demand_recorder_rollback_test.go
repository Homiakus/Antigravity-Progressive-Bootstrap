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

func TestProviderDemandRecorderRollsBackUsageWhenDimensionsCannotBind(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(66000, 0).UTC()
	seedProviderRuntimeParents(t, db, now)
	assignment := harnessmodel.ProviderAssignment{
		ID: "pasn_demand_rollback", AttemptID: testProviderAttemptID, AccountID: testProviderAccountID, ModelID: "model-a",
		State: harnessmodel.ProviderAssignmentActive, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { return tx.CreateProviderAssignment(ctx, assignment) }); err != nil {
		t.Fatal(err)
	}
	usage := harnessmodel.ProviderUsageSample{
		Key: "usage-demand-rollback", AssignmentID: assignment.ID, AccountID: assignment.AccountID,
		Metric: harnessmodel.QuotaMetricTokens, Amount: 5, ObservedAt: now.Add(time.Second), CreatedAt: now.Add(time.Second),
	}
	dims := harnessmodel.ProviderDemandDimensions{UsageKey: usage.Key, TaskClass: "code", RepositoryClass: "small", ContextClass: "warm"}
	if _, err := (providerdemand.Recorder{Store: db}).Record(ctx, usage, dims); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("record without authoritative model error=%v want ErrConflict", err)
	}
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		_, err := r.GetProviderUsageSample(ctx, usage.Key)
		return err
	}); !errors.Is(err, harnessstore.ErrNotFound) {
		t.Fatalf("rolled-back usage lookup error=%v want ErrNotFound", err)
	}
}
