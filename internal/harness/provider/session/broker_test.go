package session

import (
	"context"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

var testNow = time.Date(2026, 8, 29, 7, 0, 0, 0, time.UTC)

func TestEvaluateSessionPolicyMatrix(t *testing.T) {
	policy := Policy{AcquireReuseHeadroomFraction: 0.30, RetainReuseHeadroomFraction: 0.15}

	tests := []struct {
		name     string
		mutate   func(*Snapshot, *Request)
		want     Action
		wantID   harnessmodel.ProviderSessionID
		wantWhy  Reason
	}{
		{
			name: "healthy same workspace reuses",
			want: ActionReuse, wantID: "s1", wantWhy: ReasonReusableSession,
		},
		{
			name: "different workspace starts new without stealing context",
			mutate: func(snapshot *Snapshot, request *Request) { request.WorkspaceFingerprint = "sha256:other" },
			want: ActionNew, wantWhy: ReasonNoReusableSession,
		},
		{
			name: "preferred session is retained inside hysteresis band",
			mutate: func(snapshot *Snapshot, request *Request) {
				snapshot.Sessions[0].ContextUsed = 800
				request.PreferredSessionID = "s1"
			},
			want: ActionReuse, wantID: "s1", wantWhy: ReasonRetainedByHysteresis,
		},
		{
			name: "non preferred session in hysteresis band rotates",
			mutate: func(snapshot *Snapshot, request *Request) { snapshot.Sessions[0].ContextUsed = 800 },
			want: ActionCheckpointAndNew, wantWhy: ReasonContextRotation,
		},
		{
			name: "preferred session below retain threshold rotates",
			mutate: func(snapshot *Snapshot, request *Request) {
				snapshot.Sessions[0].ContextUsed = 900
				request.PreferredSessionID = "s1"
			},
			want: ActionCheckpointAndNew, wantWhy: ReasonContextRotation,
		},
		{
			name: "insufficient absolute context rotates",
			mutate: func(snapshot *Snapshot, request *Request) {
				snapshot.Sessions[0].ContextUsed = 700
				request.RequiredContext = 350
			},
			want: ActionCheckpointAndNew, wantWhy: ReasonContextRotation,
		},
		{
			name: "unknown session context rotates safely",
			mutate: func(snapshot *Snapshot, request *Request) {
				snapshot.Sessions[0].ContextLimit = 0
				snapshot.Sessions[0].ContextUsed = 0
			},
			want: ActionCheckpointAndNew, wantWhy: ReasonContextRotation,
		},
		{
			name: "exhausted session rotates",
			mutate: func(snapshot *Snapshot, request *Request) { snapshot.Sessions[0].State = harnessmodel.ProviderSessionExhausted },
			want: ActionCheckpointAndNew, wantWhy: ReasonContextRotation,
		},
		{
			name: "draining account may finish reusable session",
			mutate: func(snapshot *Snapshot, request *Request) { snapshot.Account.State = harnessmodel.ProviderAccountDraining },
			want: ActionReuse, wantID: "s1", wantWhy: ReasonReusableSession,
		},
		{
			name: "draining account cannot acquire replacement",
			mutate: func(snapshot *Snapshot, request *Request) {
				snapshot.Account.State = harnessmodel.ProviderAccountDraining
				snapshot.Sessions = nil
			},
			want: ActionUnavailable, wantWhy: ReasonAccountDraining,
		},
		{
			name: "disabled account unavailable",
			mutate: func(snapshot *Snapshot, request *Request) { snapshot.Account.State = harnessmodel.ProviderAccountDisabled },
			want: ActionUnavailable, wantWhy: ReasonAccountDisabled,
		},
		{
			name: "provider unavailable is hard block",
			mutate: func(snapshot *Snapshot, request *Request) { snapshot.Health = harnessmodel.ProviderHealthUnavailable },
			want: ActionUnavailable, wantWhy: ReasonProviderUnavailable,
		},
		{
			name: "provider exhausted is hard block",
			mutate: func(snapshot *Snapshot, request *Request) { snapshot.Health = harnessmodel.ProviderHealthExhausted },
			want: ActionUnavailable, wantWhy: ReasonProviderExhausted,
		},
		{
			name: "disabled model unavailable",
			mutate: func(snapshot *Snapshot, request *Request) { snapshot.Models[0].Enabled = false },
			want: ActionUnavailable, wantWhy: ReasonModelDisabled,
		},
		{
			name: "missing model unavailable",
			mutate: func(snapshot *Snapshot, request *Request) { request.ModelID = "missing" },
			want: ActionUnavailable, wantWhy: ReasonModelNotFound,
		},
		{
			name: "capability mismatch unavailable",
			mutate: func(snapshot *Snapshot, request *Request) { request.RequiredCapabilities = []string{"vision"} },
			want: ActionUnavailable, wantWhy: ReasonCapabilityMismatch,
		},
		{
			name: "required context larger than model window unavailable",
			mutate: func(snapshot *Snapshot, request *Request) { request.RequiredContext = 1001 },
			want: ActionUnavailable, wantWhy: ReasonModelContextTooSmall,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot, request := baseSnapshot(), baseRequest()
			if tt.mutate != nil {
				tt.mutate(&snapshot, &request)
			}
			got, err := Evaluate(snapshot, request, policy)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if got.Action != tt.want {
				t.Fatalf("action = %s, want %s; decision=%+v", got.Action, tt.want, got)
			}
			if got.SessionID != tt.wantID {
				t.Fatalf("session = %s, want %s", got.SessionID, tt.wantID)
			}
			if len(got.Reasons) != 1 || got.Reasons[0] != tt.wantWhy {
				t.Fatalf("reasons = %v, want [%s]", got.Reasons, tt.wantWhy)
			}
		})
	}
}

func TestEvaluateCodexWithoutAuthoritativeSessionModelStartsNew(t *testing.T) {
	snapshot := baseSnapshot()
	snapshot.Account.Provider = harnessmodel.ProviderCodex
	snapshot.Models[0].Provider = harnessmodel.ProviderCodex
	// Codex observation intentionally supplies no ProviderSessionSnapshot until
	// the upstream API authoritatively links a thread to a selected model.
	snapshot.Sessions = nil

	decision, err := Evaluate(snapshot, baseRequest(), Policy{AcquireReuseHeadroomFraction: 0.30, RetainReuseHeadroomFraction: 0.15})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionNew || decision.SessionID != "" {
		t.Fatalf("decision = %+v, want NEW without inferred session", decision)
	}
}

func TestEvaluatePreferredSessionWinsWithoutSacrificingSafety(t *testing.T) {
	snapshot := baseSnapshot()
	snapshot.Sessions = append(snapshot.Sessions,
		harnessmodel.ProviderSessionSnapshot{
			ID: "s2", Provider: harnessmodel.ProviderAntigravity, AccountID: "account-1", ModelID: "model-1",
			State: harnessmodel.ProviderSessionActive, ContextUsed: 100, ContextLimit: 1000,
			LastUsedAt: testNow.Add(time.Minute), WorkspaceFingerprint: "sha256:repo",
		},
	)
	request := baseRequest()
	request.PreferredSessionID = "s1"

	decision, err := Evaluate(snapshot, request, Policy{AcquireReuseHeadroomFraction: 0.30, RetainReuseHeadroomFraction: 0.15})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionReuse || decision.SessionID != "s1" {
		t.Fatalf("decision = %+v, want preferred s1", decision)
	}
}

func TestEvaluateDeterministicAcrossSessionOrder(t *testing.T) {
	policy := Policy{AcquireReuseHeadroomFraction: 0.30, RetainReuseHeadroomFraction: 0.15}
	request := baseRequest()
	snapshot := baseSnapshot()
	snapshot.Sessions = []harnessmodel.ProviderSessionSnapshot{
		{ID: "c", Provider: harnessmodel.ProviderAntigravity, AccountID: "account-1", ModelID: "model-1", State: harnessmodel.ProviderSessionActive, ContextUsed: 400, ContextLimit: 1000, LastUsedAt: testNow, WorkspaceFingerprint: "sha256:repo"},
		{ID: "a", Provider: harnessmodel.ProviderAntigravity, AccountID: "account-1", ModelID: "model-1", State: harnessmodel.ProviderSessionActive, ContextUsed: 200, ContextLimit: 1000, LastUsedAt: testNow, WorkspaceFingerprint: "sha256:repo"},
		{ID: "b", Provider: harnessmodel.ProviderAntigravity, AccountID: "account-1", ModelID: "model-1", State: harnessmodel.ProviderSessionActive, ContextUsed: 200, ContextLimit: 1000, LastUsedAt: testNow, WorkspaceFingerprint: "sha256:repo"},
	}

	want, err := Evaluate(snapshot, request, policy)
	if err != nil {
		t.Fatal(err)
	}
	if want.SessionID != "a" {
		t.Fatalf("baseline selected %s, want deterministic lexical tie-break a", want.SessionID)
	}

	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		shuffled := append([]harnessmodel.ProviderSessionSnapshot(nil), snapshot.Sessions...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		candidate := snapshot
		candidate.Sessions = shuffled
		got, err := Evaluate(candidate, request, policy)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("shuffle %d decision = %+v, want %+v", i, got, want)
		}
	}
}

func TestEvaluateRejectsMalformedSnapshotInsteadOfRoutingAroundIt(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Snapshot)
		match  string
	}{
		{"session account mismatch", func(s *Snapshot) { s.Sessions[0].AccountID = "other" }, "inconsistent"},
		{"session provider mismatch", func(s *Snapshot) { s.Sessions[0].Provider = harnessmodel.ProviderCodex }, "inconsistent"},
		{"duplicate session", func(s *Snapshot) { s.Sessions = append(s.Sessions, s.Sessions[0]) }, "duplicate provider session"},
		{"duplicate model", func(s *Snapshot) { s.Models = append(s.Models, s.Models[0]) }, "duplicate provider model"},
		{"invalid health", func(s *Snapshot) { s.Health = harnessmodel.ProviderHealth("BROKEN") }, "invalid provider health"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := baseSnapshot()
			tt.mutate(&snapshot)
			_, err := Evaluate(snapshot, baseRequest(), Policy{AcquireReuseHeadroomFraction: 0.30, RetainReuseHeadroomFraction: 0.15})
			if err == nil || !strings.Contains(err.Error(), tt.match) {
				t.Fatalf("error = %v, want substring %q", err, tt.match)
			}
		})
	}
}

func TestBrokerUsesSourceAndValidatesPolicy(t *testing.T) {
	source := staticSource{snapshot: baseSnapshot()}
	broker := Broker{Source: source, Policy: Policy{AcquireReuseHeadroomFraction: 0.30, RetainReuseHeadroomFraction: 0.15}}
	decision, err := broker.Decide(context.Background(), "account-1", baseRequest())
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionReuse {
		t.Fatalf("decision = %+v, want REUSE", decision)
	}

	broker.Policy = Policy{AcquireReuseHeadroomFraction: 0.10, RetainReuseHeadroomFraction: 0.20}
	if _, err := broker.Decide(context.Background(), "account-1", baseRequest()); err == nil {
		t.Fatal("expected invalid hysteresis policy to fail")
	}
}

func TestEvaluateHeadroomBoundary(t *testing.T) {
	policy := Policy{AcquireReuseHeadroomFraction: 0.30, RetainReuseHeadroomFraction: 0.15}
	snapshot := baseSnapshot()
	request := baseRequest()

	snapshot.Sessions[0].ContextUsed = 700 // exactly 30% remains
	decision, err := Evaluate(snapshot, request, policy)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionReuse {
		t.Fatalf("exact acquire boundary decision = %+v, want REUSE", decision)
	}

	snapshot.Sessions[0].ContextUsed = 701
	decision, err = Evaluate(snapshot, request, policy)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionCheckpointAndNew {
		t.Fatalf("below acquire boundary decision = %+v, want CHECKPOINT_AND_NEW", decision)
	}
}

func BenchmarkEvaluate100Sessions(b *testing.B) {
	policy := Policy{AcquireReuseHeadroomFraction: 0.30, RetainReuseHeadroomFraction: 0.15}
	request := baseRequest()
	snapshot := baseSnapshot()
	snapshot.Sessions = snapshot.Sessions[:0]
	for i := 0; i < 100; i++ {
		snapshot.Sessions = append(snapshot.Sessions, harnessmodel.ProviderSessionSnapshot{
			ID: harnessmodel.ProviderSessionID(strings.Repeat("x", i%7) + string(rune('a'+i%26))),
			Provider: harnessmodel.ProviderAntigravity, AccountID: "account-1", ModelID: "model-1",
			State: harnessmodel.ProviderSessionActive, ContextUsed: int64(i * 5), ContextLimit: 1000,
			LastUsedAt: testNow.Add(time.Duration(i) * time.Second), WorkspaceFingerprint: "sha256:repo",
		})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Evaluate(snapshot, request, policy); err != nil {
			b.Fatal(err)
		}
	}
}

type staticSource struct {
	snapshot Snapshot
	err      error
}

func (s staticSource) Snapshot(context.Context, harnessmodel.ProviderAccountID) (Snapshot, error) {
	return s.snapshot, s.err
}

func baseSnapshot() Snapshot {
	return Snapshot{
		Account: harnessmodel.ProviderAccount{
			ID: "account-1", Provider: harnessmodel.ProviderAntigravity, State: harnessmodel.ProviderAccountActive,
			CreatedAt: testNow.Add(-time.Hour), UpdatedAt: testNow,
		},
		Models: []harnessmodel.ProviderModelDescriptor{{
			AccountID: "account-1", Provider: harnessmodel.ProviderAntigravity, ID: "model-1",
			Capabilities: []string{"tools", "code"}, ContextLimit: 1000, Enabled: true,
		}},
		Sessions: []harnessmodel.ProviderSessionSnapshot{{
			ID: "s1", Provider: harnessmodel.ProviderAntigravity, AccountID: "account-1", ModelID: "model-1",
			State: harnessmodel.ProviderSessionActive, ContextUsed: 500, ContextLimit: 1000,
			LastUsedAt: testNow, WorkspaceFingerprint: "sha256:repo",
		}},
		Health: harnessmodel.ProviderHealthHealthy,
	}
}

func baseRequest() Request {
	return Request{
		ModelID: "model-1", RequiredCapabilities: []string{"code"}, WorkspaceFingerprint: "sha256:repo", RequiredContext: 100,
	}
}
