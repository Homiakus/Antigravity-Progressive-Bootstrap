package codex

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func codexAccount(now time.Time) harnessmodel.ProviderAccount {
	return harnessmodel.ProviderAccount{
		ID:        "pacc_codex",
		Provider:  harnessmodel.ProviderCodex,
		Name:      "codex-local",
		State:     harnessmodel.ProviderAccountActive,
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Minute),
	}
}

func newTestAdapter(t *testing.T, now time.Time) *Adapter {
	t.Helper()
	a, err := NewAdapter(codexAccount(now))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestRateLimitsReadNormalizesMultiBucketSnapshot(t *testing.T) {
	now := time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC)
	a := newTestAdapter(t, now)
	payload := []byte(`{
		"jsonrpc":"2.0","id":7,
		"result":{
			"rateLimits":{"limitId":"codex","limitName":"Codex","primary":{"usedPercent":25,"windowDurationMins":300,"resetsAt":1787929200},"secondary":null,"spendControlReached":null},
			"rateLimitsByLimitId":{
				"codex":{"limitId":"codex","limitName":"Codex","primary":{"usedPercent":25,"windowDurationMins":300,"resetsAt":1787929200},"secondary":{"usedPercent":50,"windowDurationMins":10080,"resetsAt":1788500000}},
				"review":{"limitId":"review","primary":{"usedPercent":100,"windowDurationMins":300,"resetsAt":1787930000},"secondary":null,"future":true}
			},
			"rateLimitResetCredits":null,
			"futureField":true
		}
	}`)
	if err := a.ApplyRateLimitsRead(payload, now); err != nil {
		t.Fatal(err)
	}
	capacity, err := a.Capacity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if capacity.Health != harnessmodel.ProviderHealthUnknown {
		t.Fatalf("health=%s want UNKNOWN because one native bucket still has headroom", capacity.Health)
	}
	if len(capacity.Windows) != 3 {
		t.Fatalf("windows=%d want=3: %+v", len(capacity.Windows), capacity.Windows)
	}
	wantIDs := []string{"codex/primary", "codex/secondary", "review/primary"}
	wantRemaining := []float64{0.75, 0.50, 0}
	for i, window := range capacity.Windows {
		if window.ID != wantIDs[i] {
			t.Fatalf("window[%d].ID=%q want=%q", i, window.ID, wantIDs[i])
		}
		if window.Metric != harnessmodel.QuotaMetricFraction || window.ModelID != "" || window.RemainingFraction == nil || *window.RemainingFraction != wantRemaining[i] {
			t.Fatalf("unexpected window[%d]: %+v", i, window)
		}
		if window.Confidence != 1 {
			t.Fatalf("window[%d] confidence=%v want=1", i, window.Confidence)
		}
	}
	if capacity.Windows[0].ResetAt == nil || capacity.Windows[0].ResetAt.Unix() != 1787929200 {
		t.Fatalf("unexpected primary reset: %v", capacity.Windows[0].ResetAt)
	}
}

func TestRateLimitsReadFallsBackToLegacySingleBucket(t *testing.T) {
	now := time.Date(2026, 8, 28, 14, 5, 0, 0, time.UTC)
	a := newTestAdapter(t, now)
	if err := a.ApplyRateLimitsRead([]byte(`{
		"rateLimits":{"limitId":null,"primary":{"usedPercent":40,"windowDurationMins":300,"resetsAt":1787929200},"secondary":null},
		"rateLimitsByLimitId":null
	}`), now); err != nil {
		t.Fatal(err)
	}
	capacity, err := a.Capacity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(capacity.Windows) != 1 || capacity.Windows[0].ID != "legacy/primary" || *capacity.Windows[0].RemainingFraction != 0.6 {
		t.Fatalf("unexpected legacy capacity: %+v", capacity)
	}
}

func TestSparseRateLimitUpdatePreservesUnavailableFields(t *testing.T) {
	now := time.Date(2026, 8, 28, 14, 10, 0, 0, time.UTC)
	a := newTestAdapter(t, now)
	if err := a.ApplyRateLimitsRead([]byte(`{
		"rateLimits":{"limitId":"codex","primary":{"usedPercent":10,"windowDurationMins":300,"resetsAt":1787929200},"secondary":{"usedPercent":20,"windowDurationMins":10080,"resetsAt":1788500000},"spendControlReached":false},
		"rateLimitsByLimitId":{"codex":{"limitId":"codex","primary":{"usedPercent":10,"windowDurationMins":300,"resetsAt":1787929200},"secondary":{"usedPercent":20,"windowDurationMins":10080,"resetsAt":1788500000},"spendControlReached":false}}
	}`), now); err != nil {
		t.Fatal(err)
	}
	updateAt := now.Add(time.Minute)
	if err := a.ApplyRateLimitsUpdated([]byte(`{
		"jsonrpc":"2.0","method":"account/rateLimits/updated","params":{
			"rateLimits":{"limitId":"codex","limitName":null,"primary":null,"secondary":{"usedPercent":70,"windowDurationMins":10080,"resetsAt":1788600000},"spendControlReached":null,"planType":null}
		}
	}`), updateAt); err != nil {
		t.Fatal(err)
	}
	capacity, err := a.Capacity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(capacity.Windows) != 2 {
		t.Fatalf("windows=%d want=2", len(capacity.Windows))
	}
	if got := *capacity.Windows[0].RemainingFraction; got != 0.9 {
		t.Fatalf("primary was cleared/replaced by sparse null: remaining=%v want=.9", got)
	}
	if got := *capacity.Windows[1].RemainingFraction; got != 0.3 {
		t.Fatalf("secondary remaining=%v want=.3", got)
	}
	if !capacity.ObservedAt.Equal(updateAt) {
		t.Fatalf("observedAt=%s want=%s", capacity.ObservedAt, updateAt)
	}
}

func TestSparseRateLimitUpdateRejectsAmbiguousBucketAndStaleDelivery(t *testing.T) {
	now := time.Date(2026, 8, 28, 14, 15, 0, 0, time.UTC)
	a := newTestAdapter(t, now)
	baseline := []byte(`{
		"rateLimits":{"limitId":"a","primary":{"usedPercent":10}},
		"rateLimitsByLimitId":{"a":{"limitId":"a","primary":{"usedPercent":10}},"b":{"limitId":"b","primary":{"usedPercent":20}}}
	}`)
	if err := a.ApplyRateLimitsRead(baseline, now); err != nil {
		t.Fatal(err)
	}
	if err := a.ApplyRateLimitsUpdated([]byte(`{"rateLimits":{"limitId":null,"primary":{"usedPercent":50}}}`), now.Add(time.Second)); err == nil {
		t.Fatal("ambiguous sparse update without limitId unexpectedly accepted")
	}
	if err := a.ApplyRateLimitsUpdated([]byte(`{"rateLimits":{"limitId":"a","primary":{"usedPercent":50}}}`), now.Add(-time.Second)); !errors.Is(err, ErrStaleObservation) {
		t.Fatalf("stale update err=%v want ErrStaleObservation", err)
	}
}

func TestSparseRateLimitUpdateRequiresBaseline(t *testing.T) {
	now := time.Date(2026, 8, 28, 14, 20, 0, 0, time.UTC)
	a := newTestAdapter(t, now)
	err := a.ApplyRateLimitsUpdated([]byte(`{"rateLimits":{"limitId":"codex","primary":{"usedPercent":50}}}`), now)
	if !errors.Is(err, ErrRateLimitsNotObserved) {
		t.Fatalf("err=%v want ErrRateLimitsNotObserved", err)
	}
}

func TestCapacityOnlyMarksAccountExhaustedWhenAllBucketsProveExhaustion(t *testing.T) {
	now := time.Date(2026, 8, 28, 14, 25, 0, 0, time.UTC)
	a := newTestAdapter(t, now)
	allExhausted := []byte(`{
		"rateLimits":{"limitId":"a","primary":{"usedPercent":100}},
		"rateLimitsByLimitId":{
			"a":{"limitId":"a","primary":{"usedPercent":100}},
			"b":{"limitId":"b","primary":null,"secondary":null,"rateLimitReachedType":"workspace_member_usage_limit_reached"}
		}
	}`)
	if err := a.ApplyRateLimitsRead(allExhausted, now); err != nil {
		t.Fatal(err)
	}
	capacity, err := a.Capacity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if capacity.Health != harnessmodel.ProviderHealthExhausted {
		t.Fatalf("health=%s want EXHAUSTED", capacity.Health)
	}

	mixed := []byte(`{
		"rateLimits":{"limitId":"a","primary":{"usedPercent":100}},
		"rateLimitsByLimitId":{"a":{"limitId":"a","primary":{"usedPercent":100}},"b":{"limitId":"b","primary":{"usedPercent":99}}}
	}`)
	if err := a.ApplyRateLimitsRead(mixed, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	capacity, err = a.Capacity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if capacity.Health != harnessmodel.ProviderHealthUnknown {
		t.Fatalf("health=%s want UNKNOWN when any opaque-mapped bucket has headroom", capacity.Health)
	}
}

func TestRateLimitKnownFieldBoundsAndIdentityFailClosed(t *testing.T) {
	now := time.Date(2026, 8, 28, 14, 30, 0, 0, time.UTC)
	tests := []string{
		`{"rateLimits":{"primary":{"usedPercent":101}}}`,
		`{"rateLimits":{"primary":{"usedPercent":-1}}}`,
		`{"rateLimits":{"primary":{"usedPercent":10,"windowDurationMins":-1}}}`,
		`{"rateLimits":{"primary":{"usedPercent":10,"resetsAt":-1}}}`,
		`{"rateLimits":{"limitId":"a","primary":{"usedPercent":10}},"rateLimitsByLimitId":{"b":{"limitId":"a","primary":{"usedPercent":10}}}}`,
	}
	for i, payload := range tests {
		a := newTestAdapter(t, now)
		if err := a.ApplyRateLimitsRead([]byte(payload), now); err == nil {
			t.Fatalf("case %d malformed rate limit unexpectedly accepted", i)
		}
	}
}

func TestReplaceModelCatalogRequiresCompletePaginationAndNormalizesCapabilities(t *testing.T) {
	now := time.Date(2026, 8, 28, 14, 35, 0, 0, time.UTC)
	a := newTestAdapter(t, now)
	page1 := []byte(`{"jsonrpc":"2.0","id":1,"result":{"data":[{
		"id":"gpt-codex-a","model":"gpt-codex-a","displayName":"Codex A","hidden":false,
		"inputModalities":["text","image","text"],"supportsPersonality":true,"multiAgentVersion":"v1","future":1
	}],"nextCursor":"cursor-2"}}`)
	page2 := []byte(`{"data":[{
		"id":"gpt-codex-b","model":"gpt-codex-b","displayName":"","hidden":true,
		"inputModalities":["text"],"supportsPersonality":false,"multiAgentVersion":null
	}],"nextCursor":null}`)
	if err := a.ReplaceModelCatalog([][]byte{page1, page2}, now); err != nil {
		t.Fatal(err)
	}
	models, err := a.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].ID != "gpt-codex-a" || models[1].ID != "gpt-codex-b" {
		t.Fatalf("unexpected model catalog: %+v", models)
	}
	if got := fmt.Sprint(models[0].Capabilities); got != "[input:image input:text multi-agent personality]" {
		t.Fatalf("capabilities=%s", got)
	}
	if !models[0].Enabled || models[1].Enabled || models[1].DisplayName != "gpt-codex-b" {
		t.Fatalf("unexpected enabled/display normalization: %+v", models)
	}

	if err := a.ReplaceModelCatalog([][]byte{page1}, now.Add(time.Minute)); err == nil {
		t.Fatal("incomplete paginated catalog unexpectedly replaced full catalog")
	}
	modelsAfter, _ := a.Models(context.Background())
	if len(modelsAfter) != 2 {
		t.Fatalf("failed catalog replacement mutated prior catalog: %+v", modelsAfter)
	}
}

func TestReplaceModelCatalogRejectsDuplicateAndStaleCatalog(t *testing.T) {
	now := time.Date(2026, 8, 28, 14, 40, 0, 0, time.UTC)
	a := newTestAdapter(t, now)
	if err := a.ReplaceModelCatalog([][]byte{[]byte(`{"data":[{"id":"m","displayName":"M","hidden":false}],"nextCursor":null}`)}, now); err != nil {
		t.Fatal(err)
	}
	if err := a.ReplaceModelCatalog([][]byte{[]byte(`{"data":[{"id":"m2","hidden":false}],"nextCursor":null}`)}, now.Add(-time.Second)); !errors.Is(err, ErrStaleObservation) {
		t.Fatalf("stale model catalog err=%v", err)
	}
	dup1 := []byte(`{"data":[{"id":"dup","hidden":false}],"nextCursor":"x"}`)
	dup2 := []byte(`{"data":[{"id":"dup","hidden":false}],"nextCursor":null}`)
	if err := a.ReplaceModelCatalog([][]byte{dup1, dup2}, now.Add(time.Minute)); err == nil {
		t.Fatal("duplicate model id across pages unexpectedly accepted")
	}
}

func TestObserveIsProviderNeutralAndDoesNotInventCodexSessionModelBinding(t *testing.T) {
	now := time.Date(2026, 8, 28, 14, 45, 0, 0, time.UTC)
	a := newTestAdapter(t, now)
	if err := a.ApplyRateLimitsRead([]byte(`{"rateLimits":{"limitId":"codex","primary":{"usedPercent":25}}}`), now); err != nil {
		t.Fatal(err)
	}
	if err := a.ReplaceModelCatalog([][]byte{[]byte(`{"data":[{"id":"model-a","displayName":"A","hidden":false}],"nextCursor":null}`)}, now); err != nil {
		t.Fatal(err)
	}
	obs, err := a.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if obs.Capacity.Provider != harnessmodel.ProviderCodex || len(obs.Models) != 1 {
		t.Fatalf("unexpected observation: %+v", obs)
	}
	if len(obs.Sessions) != 0 {
		t.Fatalf("Codex sessions unexpectedly inferred without thread->model linkage: %+v", obs.Sessions)
	}
	if obs.Capacity.Windows[0].ModelID != "" {
		t.Fatalf("quota bucket unexpectedly mapped to model: %+v", obs.Capacity.Windows[0])
	}
}

func TestParseThreadTokenUsagePreservesContextSignalWithoutModelGuess(t *testing.T) {
	now := time.Date(2026, 8, 28, 14, 50, 0, 0, time.UTC)
	payload := []byte(`{
		"jsonrpc":"2.0","method":"thread/tokenUsage/updated","params":{
			"threadId":"0198-thread","turnId":"turn-7","tokenUsage":{
				"total":{"totalTokens":1200,"inputTokens":900,"cachedInputTokens":300,"cacheWriteInputTokens":100,"outputTokens":300,"reasoningOutputTokens":90},
				"last":{"totalTokens":200,"inputTokens":150,"cachedInputTokens":50,"cacheWriteInputTokens":10,"outputTokens":50,"reasoningOutputTokens":20},
				"modelContextWindow":200000
			},"future":true
		}
	}`)
	obs, err := ParseThreadTokenUsageUpdated(payload, now)
	if err != nil {
		t.Fatal(err)
	}
	if obs.ThreadID != "0198-thread" || obs.TurnID != "turn-7" || obs.TotalTokens != 1200 || obs.CachedInputTokens != 300 || obs.ModelContextWindow != 200000 {
		t.Fatalf("unexpected thread usage: %+v", obs)
	}
}

func TestParseThreadTokenUsageRejectsMalformedKnownFields(t *testing.T) {
	now := time.Date(2026, 8, 28, 14, 55, 0, 0, time.UTC)
	for _, payload := range []string{
		`{"threadId":"","turnId":"x","tokenUsage":{"total":{},"last":{}}}`,
		`{"threadId":"t","turnId":"x","tokenUsage":{"total":{"totalTokens":-1},"last":{}}}`,
		`{"threadId":"t","turnId":"x","tokenUsage":{"total":{},"last":{},"modelContextWindow":-1}}`,
		`{"jsonrpc":"2.0","method":"other","params":{"threadId":"t","turnId":"x","tokenUsage":{"total":{},"last":{}}}}`,
	} {
		if _, err := ParseThreadTokenUsageUpdated([]byte(payload), now); err == nil {
			t.Fatalf("invalid thread usage accepted: %s", payload)
		}
	}
}

func TestProtocolPayloadSizeIsBounded(t *testing.T) {
	now := time.Date(2026, 8, 28, 15, 0, 0, 0, time.UTC)
	a := newTestAdapter(t, now)
	payload := make([]byte, maxProtocolBytes+1)
	if err := a.ApplyRateLimitsRead(payload, now); err == nil {
		t.Fatal("oversized rate-limit payload unexpectedly accepted")
	}
	if _, err := ParseThreadTokenUsageUpdated(payload, now); err == nil {
		t.Fatal("oversized token-usage payload unexpectedly accepted")
	}
}

func TestConcurrentSparseUpdatesAndObservationReads(t *testing.T) {
	now := time.Date(2026, 8, 28, 15, 5, 0, 0, time.UTC)
	a := newTestAdapter(t, now)
	if err := a.ApplyRateLimitsRead([]byte(`{"rateLimits":{"limitId":"codex","primary":{"usedPercent":0}}}`), now); err != nil {
		t.Fatal(err)
	}
	if err := a.ReplaceModelCatalog([][]byte{[]byte(`{"data":[{"id":"model-a","hidden":false}],"nextCursor":null}`)}, now); err != nil {
		t.Fatal(err)
	}

	const n = 32
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		i := i
		wg.Add(2)
		go func() {
			defer wg.Done()
			payload := []byte(fmt.Sprintf(`{"rateLimits":{"limitId":"codex","primary":{"usedPercent":%d}}}`, i%101))
			// Use the same timestamp to exercise lock safety without stale-order
			// nondeterminism between goroutines.
			if err := a.ApplyRateLimitsUpdated(payload, now.Add(time.Minute)); err != nil {
				t.Errorf("update %d: %v", i, err)
			}
		}()
		go func() {
			defer wg.Done()
			obs, err := a.Observe(context.Background())
			if err != nil {
				t.Errorf("observe: %v", err)
				return
			}
			if len(obs.Models) != 1 || len(obs.Capacity.Windows) != 1 {
				t.Errorf("torn/invalid in-memory observation: %+v", obs)
			}
		}()
	}
	wg.Wait()
}
