package demand

import (
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestSampleFromUsagePreservesNativeQuantityAndModel(t *testing.T) {
	observed := time.Date(2026, 8, 29, 5, 10, 0, 0, time.UTC)
	usage := harnessmodel.ProviderUsageSample{
		Key:          "usage-1",
		AssignmentID: harnessmodel.ProviderAssignmentID("assignment-1"),
		AccountID:    harnessmodel.ProviderAccountID("account-1"),
		ModelID:      harnessmodel.ProviderModelID("model-1"),
		Metric:       harnessmodel.QuotaMetricTokens,
		Amount:       1234,
		ObservedAt:   observed,
		CreatedAt:    observed.Add(time.Second),
	}
	got, err := SampleFromUsage(UsageClassification{
		TaskClass: " implement ", Provider: harnessmodel.ProviderCodex,
		RepositoryID: " repo-a ", ContextClass: " medium ",
	}, usage)
	if err != nil {
		t.Fatal(err)
	}
	if got.Key.TaskClass != "implement" || got.Key.Provider != harnessmodel.ProviderCodex ||
		got.Key.ModelID != usage.ModelID || got.Key.RepositoryID != "repo-a" || got.Key.ContextClass != "medium" ||
		got.Key.Metric != usage.Metric || got.Amount != usage.Amount || !got.ObservedAt.Equal(observed) {
		t.Fatalf("durable usage semantics changed during classification: %+v", got)
	}
}

func TestSampleFromUsageDoesNotInventMissingModel(t *testing.T) {
	now := time.Date(2026, 8, 29, 5, 10, 0, 0, time.UTC)
	usage := harnessmodel.ProviderUsageSample{
		Key: "usage-2", AssignmentID: harnessmodel.ProviderAssignmentID("assignment-1"),
		AccountID: harnessmodel.ProviderAccountID("account-1"), Metric: harnessmodel.QuotaMetricRequests,
		Amount: 1, ObservedAt: now, CreatedAt: now,
	}
	got, err := SampleFromUsage(UsageClassification{TaskClass: "test", Provider: harnessmodel.ProviderAntigravity}, usage)
	if err != nil {
		t.Fatal(err)
	}
	if got.Key.ModelID != "" {
		t.Fatalf("model affinity was invented: %+v", got.Key)
	}
}

func TestSampleFromUsageRejectsInvalidClassificationOrUsage(t *testing.T) {
	now := time.Date(2026, 8, 29, 5, 10, 0, 0, time.UTC)
	valid := harnessmodel.ProviderUsageSample{
		Key: "usage-3", AssignmentID: harnessmodel.ProviderAssignmentID("assignment-1"),
		AccountID: harnessmodel.ProviderAccountID("account-1"), Metric: harnessmodel.QuotaMetricCost,
		Amount: 0.25, ObservedAt: now, CreatedAt: now,
	}
	if _, err := SampleFromUsage(UsageClassification{Provider: harnessmodel.ProviderCodex}, valid); err == nil {
		t.Fatal("missing authoritative task class accepted")
	}
	bad := valid
	bad.Metric = harnessmodel.QuotaMetricOpaque
	if _, err := SampleFromUsage(UsageClassification{TaskClass: "review", Provider: harnessmodel.ProviderCodex}, bad); err == nil {
		t.Fatal("opaque durable usage accepted as arithmetic demand")
	}
}
