package demand

import (
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestRequestRequiresCompleteClassification(t *testing.T) {
	base := Request{
		Provider: harnessmodel.ProviderCodex, ModelID: "model-a", Metric: harnessmodel.QuotaMetricTokens,
		TaskClass: "code", RepositoryClass: "medium", ContextClass: "warm", Now: time.Unix(70000, 0).UTC(),
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	for _, mutate := range []func(*Request){
		func(r *Request) { r.TaskClass = "" },
		func(r *Request) { r.RepositoryClass = "" },
		func(r *Request) { r.ContextClass = "" },
	} {
		r := base
		mutate(&r)
		if err := r.Validate(); err == nil {
			t.Fatalf("incomplete classification unexpectedly valid: %+v", r)
		}
	}
}
