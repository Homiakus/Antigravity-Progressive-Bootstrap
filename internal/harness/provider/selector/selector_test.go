package selector

import (
	"context"
	"math/rand"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/provider/demand"
	"github.com/homiakus/agctl/internal/harness/provider/session"
)

func floatPtr(v float64) *float64 { return &v }

func timePtr(t time.Time) *time.Time { return &t }

func baseCandidate(provider harnessmodel.ProviderKind, accountID string, modelID string) Candidate {
	return Candidate{
		Account: harnessmodel.ProviderAccount{
			ID:       harnessmodel.ProviderAccountID(accountID),
			Provider: provider,
			State:    harnessmodel.ProviderAccountActive,
		},
		Model: harnessmodel.ProviderModelDescriptor{
			AccountID:    harnessmodel.ProviderAccountID(accountID),
			Provider:     provider,
			ID:           harnessmodel.ProviderModelID(modelID),
			Enabled:      true,
			Capabilities: []string{"tools", "streaming"},
			ContextLimit: 128000,
		},
		Capacity: &harnessmodel.ProviderCapacitySnapshot{
			AccountID:  harnessmodel.ProviderAccountID(accountID),
			Provider:   provider,
			Health:     harnessmodel.ProviderHealthHealthy,
			ObservedAt: time.Now().UTC(),
			Windows: []harnessmodel.QuotaWindow{
				{
					ID:                "win1",
					Metric:            harnessmodel.QuotaMetricFraction,
					RemainingFraction: floatPtr(0.85),
					Confidence:        1.0,
					ObservedAt:        time.Now().UTC(),
				},
			},
		},
	}
}

func TestValidation(t *testing.T) {
	req := Request{}
	if err := req.Validate(); err == nil {
		t.Fatal("expected error for empty task class")
	}

	req = Request{TaskClass: "test", RequiredContext: -1}
	if err := req.Validate(); err == nil {
		t.Fatal("expected error for negative context")
	}

	req = Request{TaskClass: "test", RequiredCapabilities: []string{""}}
	if err := req.Validate(); err == nil {
		t.Fatal("expected error for empty capability")
	}

	req = Request{TaskClass: "test", AllowedProviders: []harnessmodel.ProviderKind{"UNKNOWN"}}
	if err := req.Validate(); err == nil {
		t.Fatal("expected error for invalid allowed provider")
	}

	policy := Policy{
		CapacityPolicy:     DefaultPolicy().CapacityPolicy,
		SessionPolicy:      DefaultPolicy().SessionPolicy,
		HeadroomWeight:     -1,
		ResetHorizonWeight: 0.1,
	}
	if err := policy.Validate(); err == nil {
		t.Fatal("expected error for negative weight")
	}
}

func TestHardFilters(t *testing.T) {
	now := time.Now().UTC()
	policy := DefaultPolicy()
	baseReq := Request{
		TaskClass:            "codegen",
		RequiredContext:      32000,
		RequiredCapabilities: []string{"tools"},
	}

	tests := []struct {
		name           string
		req            Request
		candidate      Candidate
		expectedReason FilterReason
	}{
		{
			name: "account disabled",
			req:  baseReq,
			candidate: func() Candidate {
				c := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model1")
				c.Account.State = harnessmodel.ProviderAccountDisabled
				return c
			}(),
			expectedReason: FilterAccountDisabled,
		},
		{
			name: "account draining without reusable session",
			req:  baseReq,
			candidate: func() Candidate {
				c := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model1")
				c.Account.State = harnessmodel.ProviderAccountDraining
				return c
			}(),
			expectedReason: FilterAccountDrainingNoSession,
		},
		{
			name: "provider not allowed",
			req: func() Request {
				r := baseReq
				r.AllowedProviders = []harnessmodel.ProviderKind{harnessmodel.ProviderCodex}
				return r
			}(),
			candidate:      baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model1"),
			expectedReason: FilterProviderNotAllowed,
		},
		{
			name: "health unavailable",
			req:  baseReq,
			candidate: func() Candidate {
				c := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model1")
				c.Capacity.Health = harnessmodel.ProviderHealthUnavailable
				return c
			}(),
			expectedReason: FilterHealthUnavailable,
		},
		{
			name: "health exhausted",
			req:  baseReq,
			candidate: func() Candidate {
				c := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model1")
				c.Capacity.Health = harnessmodel.ProviderHealthExhausted
				return c
			}(),
			expectedReason: FilterHealthExhausted,
		},
		{
			name: "model disabled",
			req:  baseReq,
			candidate: func() Candidate {
				c := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model1")
				c.Model.Enabled = false
				return c
			}(),
			expectedReason: FilterModelDisabled,
		},
		{
			name: "capability mismatch",
			req:  baseReq,
			candidate: func() Candidate {
				c := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model1")
				c.Model.Capabilities = []string{"vision"} // lacks "tools"
				return c
			}(),
			expectedReason: FilterCapabilityMismatch,
		},
		{
			name: "context limit too small",
			req:  baseReq,
			candidate: func() Candidate {
				c := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model1")
				c.Model.ContextLimit = 16000 // req is 32000
				return c
			}(),
			expectedReason: FilterContextTooSmall,
		},
		{
			name: "circuit breaker open",
			req:  baseReq,
			candidate: func() Candidate {
				c := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model1")
				c.Circuit = &harnessmodel.ProviderCircuitState{
					AccountID:           c.Account.ID,
					ModelID:             c.Model.ID,
					Revision:            1,
					State:               harnessmodel.CircuitOpen,
					ConsecutiveFailures: 5,
					UpdatedAt:           now,
				}
				return c
			}(),
			expectedReason: FilterCircuitOpen,
		},
		{
			name: "insufficient headroom",
			req:  baseReq,
			candidate: func() Candidate {
				c := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model1")
				c.Capacity.Windows = []harnessmodel.QuotaWindow{
					{
						ID:         "win_tokens",
						Metric:     harnessmodel.QuotaMetricTokens,
						Remaining:  floatPtr(1000),
						Confidence: 1.0,
						ObservedAt: now,
					},
				}
				c.DemandEstimates = []demand.Estimate{
					{
						Key: demand.Key{
							TaskClass: "codegen",
							Provider:  harnessmodel.ProviderAntigravity,
							ModelID:   "model1",
							Metric:    harnessmodel.QuotaMetricTokens,
						},
						Reservation: 5000, // 5000 > 1000 remaining
					},
				}
				return c
			}(),
			expectedReason: FilterInsufficientHeadroom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec, err := Evaluate(context.Background(), tt.req, []Candidate{tt.candidate}, now, policy)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dec.Selected != nil {
				t.Fatalf("expected candidate to be eliminated, but got selected: %v", dec.Selected)
			}
			if len(dec.Evaluations) != 1 {
				t.Fatalf("expected 1 evaluation, got %d", len(dec.Evaluations))
			}
			if dec.Evaluations[0].EliminationReason != tt.expectedReason {
				t.Fatalf("expected elimination reason %q, got %q (%s)", tt.expectedReason, dec.Evaluations[0].EliminationReason, dec.Evaluations[0].EliminationDetail)
			}
		})
	}
}

func TestSoftScoring(t *testing.T) {
	now := time.Now().UTC()
	policy := DefaultPolicy()
	req := Request{
		TaskClass:            "codegen",
		RequiredCapabilities: []string{"tools"},
	}

	t.Run("higher headroom fraction wins", func(t *testing.T) {
		cLow := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model_low")
		cLow.Capacity.Windows[0].RemainingFraction = floatPtr(0.20)

		cHigh := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model_high")
		cHigh.Capacity.Windows[0].RemainingFraction = floatPtr(0.90)

		dec, err := Evaluate(context.Background(), req, []Candidate{cLow, cHigh}, now, policy)
		if err != nil {
			t.Fatal(err)
		}
		if dec.Selected == nil || dec.Selected.ModelID != "model_high" {
			t.Fatalf("expected model_high to win, got %v", dec.Selected)
		}
	})

	t.Run("session reuse bonus wins", func(t *testing.T) {
		// Both have identical capacity, but cReuse has an active reusable session
		cCold := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model_cold")

		cReuse := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model_reuse")
		cReuse.Sessions = []harnessmodel.ProviderSessionSnapshot{
			{
				ID:           "sess_1",
				Provider:     harnessmodel.ProviderAntigravity,
				AccountID:    "acc1",
				ModelID:      "model_reuse",
				State:        harnessmodel.ProviderSessionActive,
				ContextUsed:  1000,
				ContextLimit: 128000,
				LastUsedAt:   now,
			},
		}

		dec, err := Evaluate(context.Background(), req, []Candidate{cCold, cReuse}, now, policy)
		if err != nil {
			t.Fatal(err)
		}
		if dec.Selected == nil || dec.Selected.ModelID != "model_reuse" {
			t.Fatalf("expected model_reuse to win, got %v", dec.Selected)
		}
		if dec.Selected.SessionDecision.Action != session.ActionReuse {
			t.Fatalf("expected ActionReuse, got %v", dec.Selected.SessionDecision.Action)
		}
	})

	t.Run("reliability preference healthy over degraded", func(t *testing.T) {
		cDegraded := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model_deg")
		cDegraded.Capacity.Health = harnessmodel.ProviderHealthDegraded

		cHealthy := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model_hlth")
		cHealthy.Capacity.Health = harnessmodel.ProviderHealthHealthy

		dec, err := Evaluate(context.Background(), req, []Candidate{cDegraded, cHealthy}, now, policy)
		if err != nil {
			t.Fatal(err)
		}
		if dec.Selected == nil || dec.Selected.ModelID != "model_hlth" {
			t.Fatalf("expected model_hlth to win, got %v", dec.Selected)
		}
	})

	t.Run("switch penalty preserves preferred model", func(t *testing.T) {
		prefReq := req
		prefReq.PreferredModelID = "model_pref"
		prefReq.PreferredProvider = harnessmodel.ProviderAntigravity

		cPref := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model_pref")
		cPref.Capacity.Windows[0].RemainingFraction = floatPtr(0.70)

		// cOther has slightly higher headroom fraction (0.75 vs 0.70)
		cOther := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model_other")
		cOther.Capacity.Windows[0].RemainingFraction = floatPtr(0.75)

		dec, err := Evaluate(context.Background(), prefReq, []Candidate{cPref, cOther}, now, policy)
		if err != nil {
			t.Fatal(err)
		}
		if dec.Selected == nil || dec.Selected.ModelID != "model_pref" {
			t.Fatalf("expected preferred model to win under switch penalty, got %v", dec.Selected)
		}
	})

	t.Run("draining account with reusable session is eligible", func(t *testing.T) {
		cDraining := baseCandidate(harnessmodel.ProviderAntigravity, "acc_drain", "model_drain")
		cDraining.Account.State = harnessmodel.ProviderAccountDraining
		cDraining.Sessions = []harnessmodel.ProviderSessionSnapshot{
			{
				ID:           "sess_drain",
				Provider:     harnessmodel.ProviderAntigravity,
				AccountID:    "acc_drain",
				ModelID:      "model_drain",
				State:        harnessmodel.ProviderSessionActive,
				ContextUsed:  500,
				ContextLimit: 128000,
				LastUsedAt:   now,
			},
		}

		dec, err := Evaluate(context.Background(), req, []Candidate{cDraining}, now, policy)
		if err != nil {
			t.Fatal(err)
		}
		if dec.Selected == nil || dec.Selected.ModelID != "model_drain" {
			t.Fatalf("expected draining account with reusable session to be selected, got %v", dec.Selected)
		}
	})
}

func TestDeterministicShuffledOrderInvariance(t *testing.T) {
	now := time.Now().UTC()
	policy := DefaultPolicy()
	req := Request{
		TaskClass:            "codegen",
		RequiredCapabilities: []string{"tools"},
		PreferredSessionID:   "sess_antigravity",
	}

	c1 := baseCandidate(harnessmodel.ProviderAntigravity, "acc1", "model_a")
	c1.Capacity.Windows[0].RemainingFraction = floatPtr(0.80)
	c1.Sessions = []harnessmodel.ProviderSessionSnapshot{
		{
			ID:           "sess_antigravity",
			Provider:     harnessmodel.ProviderAntigravity,
			AccountID:    "acc1",
			ModelID:      "model_a",
			State:        harnessmodel.ProviderSessionActive,
			ContextUsed:  2000,
			ContextLimit: 128000,
			LastUsedAt:   now,
		},
	}

	c2 := baseCandidate(harnessmodel.ProviderCodex, "acc2", "model_b")
	c2.Capacity.Windows[0].RemainingFraction = floatPtr(0.85)

	c3 := baseCandidate(harnessmodel.ProviderAntigravity, "acc3", "model_c")
	c3.Capacity.Windows[0].RemainingFraction = floatPtr(0.50)

	c4 := baseCandidate(harnessmodel.ProviderCodex, "acc4", "model_d")
	c4.Model.Enabled = false // eliminated

	originalCandidates := []Candidate{c1, c2, c3, c4}

	// First run to get baseline decision
	baseline, err := Evaluate(context.Background(), req, originalCandidates, now, policy)
	if err != nil {
		t.Fatal(err)
	}
	if baseline.Selected == nil {
		t.Fatal("expected selected candidate in baseline")
	}

	// Run 100 trials with shuffled candidate order
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		shuffled := make([]Candidate, len(originalCandidates))
		copy(shuffled, originalCandidates)
		rng.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})

		dec, err := Evaluate(context.Background(), req, shuffled, now, policy)
		if err != nil {
			t.Fatalf("trial %d failed with error: %v", i, err)
		}
		if dec.Selected == nil {
			t.Fatalf("trial %d returned nil selected", i)
		}
		if dec.Selected.AccountID != baseline.Selected.AccountID || dec.Selected.ModelID != baseline.Selected.ModelID {
			t.Fatalf("trial %d selected candidate %s/%s differed from baseline %s/%s",
				i, dec.Selected.AccountID, dec.Selected.ModelID, baseline.Selected.AccountID, baseline.Selected.ModelID)
		}
		if dec.Selected.Score.CompositeScore != baseline.Selected.Score.CompositeScore {
			t.Fatalf("trial %d composite score %v differed from baseline %v",
				i, dec.Selected.Score.CompositeScore, baseline.Selected.Score.CompositeScore)
		}
	}
}
