package model

import (
	"strings"
	"testing"
	"time"
)

func TestProviderDemandDimensionsValidation(t *testing.T) {
	valid := ProviderDemandDimensions{UsageKey: "usage-1", TaskClass: "code", RepositoryClass: "medium", ContextClass: "warm"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid dimensions: %v", err)
	}
	cases := []ProviderDemandDimensions{
		{UsageKey: "", TaskClass: "code", RepositoryClass: "medium", ContextClass: "warm"},
		{UsageKey: "usage", TaskClass: " code", RepositoryClass: "medium", ContextClass: "warm"},
		{UsageKey: "usage", TaskClass: "code", RepositoryClass: "", ContextClass: "warm"},
		{UsageKey: "usage", TaskClass: "code", RepositoryClass: "medium", ContextClass: strings.Repeat("x", MaxProviderDemandClassLength+1)},
	}
	for i, tc := range cases {
		if err := tc.Validate(); err == nil {
			t.Fatalf("case %d unexpectedly valid: %+v", i, tc)
		}
	}
}

func TestProviderDemandHistoryQueryRequiresHierarchicalFilters(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	base := ProviderDemandHistoryQuery{Provider: ProviderCodex, ModelID: "model", Metric: QuotaMetricTokens, Since: now, Limit: 10}
	if err := base.Validate(); err != nil {
		t.Fatalf("baseline query: %v", err)
	}
	q := base
	q.RepositoryClass = "repo"
	if err := q.Validate(); err == nil {
		t.Fatal("repository filter without task unexpectedly valid")
	}
	q = base
	q.TaskClass = "code"
	q.ContextClass = "warm"
	if err := q.Validate(); err == nil {
		t.Fatal("context filter without repository unexpectedly valid")
	}
	q.RepositoryClass = "repo"
	if err := q.Validate(); err != nil {
		t.Fatalf("complete hierarchical filter: %v", err)
	}
	q = base
	q.Metric = QuotaMetricOpaque
	if err := q.Validate(); err == nil {
		t.Fatal("OPAQUE demand history unexpectedly valid")
	}
}
