package selector

import (
	"context"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/provider/demand"
	"github.com/homiakus/agctl/internal/harness/provider/session"
)

func TestMutationsKillDefects(t *testing.T) {
	now := time.Now().UTC()
	policy := DefaultPolicy()
	req := Request{
		TaskClass:            "codegen",
		RequiredCapabilities: []string{"tools"},
	}

	t.Run("mutant: disabled accounts admitted", func(t *testing.T) {
		// A mutant policy or evaluator that permits disabled accounts must fail the test
		cDisabled := baseCandidate(harnessmodel.ProviderAntigravity, "acc_mut_disabled", "model1")
		cDisabled.Account.State = harnessmodel.ProviderAccountDisabled

		dec, err := Evaluate(context.Background(), req, []Candidate{cDisabled}, now, policy)
		if err != nil {
			t.Fatal(err)
		}
		if dec.Selected != nil {
			t.Fatal("mutant survival: disabled account was selected")
		}
		if len(dec.Evaluations) != 1 || dec.Evaluations[0].EliminationReason != FilterAccountDisabled {
			t.Fatalf("mutant survival: expected FilterAccountDisabled, got %v", dec.Evaluations[0].EliminationReason)
		}
	})

	t.Run("mutant: open circuit breaker ignored", func(t *testing.T) {
		cOpen := baseCandidate(harnessmodel.ProviderAntigravity, "acc_mut_circuit", "model1")
		cOpen.Circuit = &harnessmodel.ProviderCircuitState{
			AccountID:           cOpen.Account.ID,
			ModelID:             cOpen.Model.ID,
			Revision:            1,
			State:               harnessmodel.CircuitOpen,
			ConsecutiveFailures: 3,
			UpdatedAt:           now,
		}

		dec, err := Evaluate(context.Background(), req, []Candidate{cOpen}, now, policy)
		if err != nil {
			t.Fatal(err)
		}
		if dec.Selected != nil {
			t.Fatal("mutant survival: candidate with open circuit was selected")
		}
		if dec.Evaluations[0].EliminationReason != FilterCircuitOpen {
			t.Fatalf("mutant survival: expected FilterCircuitOpen, got %v", dec.Evaluations[0].EliminationReason)
		}
	})

	t.Run("mutant: switch penalty ignored", func(t *testing.T) {
		// cPreferred has 0.70 headroom, cOther has 0.72 headroom.
		// Without switch penalty, cOther wins.
		// With switch penalty (0.10 weight on 0.5+ penalty), cPreferred MUST win.
		prefReq := req
		prefReq.PreferredModelID = "model_pref"
		prefReq.PreferredProvider = harnessmodel.ProviderAntigravity

		cPreferred := baseCandidate(harnessmodel.ProviderAntigravity, "acc_pref", "model_pref")
		cPreferred.Capacity.Windows[0].RemainingFraction = floatPtr(0.70)

		cOther := baseCandidate(harnessmodel.ProviderAntigravity, "acc_other", "model_other")
		cOther.Capacity.Windows[0].RemainingFraction = floatPtr(0.72)

		dec, err := Evaluate(context.Background(), prefReq, []Candidate{cPreferred, cOther}, now, policy)
		if err != nil {
			t.Fatal(err)
		}
		if dec.Selected == nil || dec.Selected.ModelID != "model_pref" {
			t.Fatalf("mutant survival: expected preferred model to win under switch penalty, got %v", dec.Selected)
		}
	})

	t.Run("mutant: insufficient headroom admitted", func(t *testing.T) {
		cShort := baseCandidate(harnessmodel.ProviderAntigravity, "acc_short", "model1")
		cShort.Capacity.Windows = []harnessmodel.QuotaWindow{
			{
				ID:         "win_short",
				Metric:     harnessmodel.QuotaMetricTokens,
				Remaining:  floatPtr(500),
				Confidence: 1.0,
				ObservedAt: now,
			},
		}
		cShort.DemandEstimates = []demand.Estimate{
			{
				Key: demand.Key{
					TaskClass: "codegen",
					Provider:  harnessmodel.ProviderAntigravity,
					ModelID:   "model1",
					Metric:    harnessmodel.QuotaMetricTokens,
				},
				Reservation: 1000,
			},
		}

		dec, err := Evaluate(context.Background(), req, []Candidate{cShort}, now, policy)
		if err != nil {
			t.Fatal(err)
		}
		if dec.Selected != nil {
			t.Fatal("mutant survival: candidate with insufficient headroom was selected")
		}
		if dec.Evaluations[0].EliminationReason != FilterInsufficientHeadroom {
			t.Fatalf("mutant survival: expected FilterInsufficientHeadroom, got %v", dec.Evaluations[0].EliminationReason)
		}
	})

	t.Run("mutant: non-authoritative session reused", func(t *testing.T) {
		// Session has mismatched workspace fingerprint
		cMismatch := baseCandidate(harnessmodel.ProviderAntigravity, "acc_mismatch", "model1")
		cMismatch.Sessions = []harnessmodel.ProviderSessionSnapshot{
			{
				ID:                   "sess_mismatch",
				Provider:             harnessmodel.ProviderAntigravity,
				AccountID:            "acc_mismatch",
				ModelID:              "model1",
				State:                harnessmodel.ProviderSessionActive,
				ContextUsed:          1000,
				ContextLimit:         128000,
				WorkspaceFingerprint: "fingerprint:repo_a",
				LastUsedAt:           now,
			},
		}

		wsReq := req
		wsReq.WorkspaceFingerprint = "fingerprint:repo_b" // different fingerprint

		dec, err := Evaluate(context.Background(), wsReq, []Candidate{cMismatch}, now, policy)
		if err != nil {
			t.Fatal(err)
		}
		if dec.Selected == nil {
			t.Fatal("expected candidate to remain eligible with NEW session")
		}
		if dec.Selected.SessionDecision.Action == session.ActionReuse {
			t.Fatal("mutant survival: session was reused despite workspace fingerprint mismatch")
		}
		if dec.Selected.SessionDecision.Action != session.ActionNew && dec.Selected.SessionDecision.Action != session.ActionCheckpointAndNew {
			t.Fatalf("expected ActionNew or ActionCheckpointAndNew, got %v", dec.Selected.SessionDecision.Action)
		}
	})
}
