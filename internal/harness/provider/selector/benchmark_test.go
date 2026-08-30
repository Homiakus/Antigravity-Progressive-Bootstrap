package selector

import (
	"context"
	"fmt"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func BenchmarkSelectorEvaluate100Candidates(b *testing.B) {
	now := time.Now().UTC()
	policy := DefaultPolicy()
	req := Request{
		TaskClass:            "codegen",
		RequiredCapabilities: []string{"tools"},
		PreferredSessionID:   "sess_preferred",
		PreferredModelID:     "model_preferred",
	}

	candidates := make([]Candidate, 100)
	for i := 0; i < 100; i++ {
		provider := harnessmodel.ProviderAntigravity
		if i%2 == 1 {
			provider = harnessmodel.ProviderCodex
		}
		c := baseCandidate(provider, fmt.Sprintf("acc_%d", i), fmt.Sprintf("model_%d", i))
		c.Capacity.Windows[0].RemainingFraction = floatPtr(float64(i%100) / 100.0)
		if i == 42 {
			c.Model.ID = "model_preferred"
			c.Sessions = []harnessmodel.ProviderSessionSnapshot{
				{
					ID:           "sess_preferred",
					Provider:     provider,
					AccountID:    c.Account.ID,
					ModelID:      "model_preferred",
					State:        harnessmodel.ProviderSessionActive,
					ContextUsed:  1000,
					ContextLimit: 128000,
					LastUsedAt:   now,
				},
			}
		}
		candidates[i] = c
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec, err := Evaluate(ctx, req, candidates, now, policy)
		if err != nil {
			b.Fatalf("evaluate failed: %v", err)
		}
		if dec.Selected == nil {
			b.Fatal("expected candidate to be selected")
		}
	}
}
