package codex

import (
	"context"
	"testing"
	"time"
)

func TestReconnectBaselineReplacesPriorRollingRateLimitState(t *testing.T) {
	now := time.Date(2026, 8, 28, 15, 10, 0, 0, time.UTC)
	a := newTestAdapter(t, now)
	if err := a.ApplyRateLimitsRead([]byte(`{
		"rateLimits":{"limitId":"codex","primary":{"usedPercent":10}},
		"rateLimitsByLimitId":{"codex":{"limitId":"codex","primary":{"usedPercent":10}},"old":{"limitId":"old","primary":{"usedPercent":20}}}
	}`), now); err != nil {
		t.Fatal(err)
	}
	if err := a.ApplyRateLimitsUpdated([]byte(`{"rateLimits":{"limitId":"codex","primary":{"usedPercent":80}}}`), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	before, err := a.Capacity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Windows) != 2 || *before.Windows[0].RemainingFraction != 0.2 {
		t.Fatalf("unexpected pre-reconnect state: %+v", before)
	}

	// A new account/rateLimits/read after reconnect is authoritative baseline,
	// not another sparse merge. Removed native buckets disappear and the
	// current value replaces pre-disconnect rolling state.
	reconnectedAt := now.Add(2 * time.Minute)
	if err := a.ApplyRateLimitsRead([]byte(`{
		"rateLimits":{"limitId":"codex","primary":{"usedPercent":30}},
		"rateLimitsByLimitId":{"codex":{"limitId":"codex","primary":{"usedPercent":30}}}
	}`), reconnectedAt); err != nil {
		t.Fatal(err)
	}
	after, err := a.Capacity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Windows) != 1 || after.Windows[0].ID != "codex/primary" || *after.Windows[0].RemainingFraction != 0.7 {
		t.Fatalf("reconnect baseline did not replace state: %+v", after)
	}
	if !after.ObservedAt.Equal(reconnectedAt) {
		t.Fatalf("observedAt=%s want=%s", after.ObservedAt, reconnectedAt)
	}
}

func TestJSONRPCErrorResponsesFailClosed(t *testing.T) {
	now := time.Date(2026, 8, 28, 15, 15, 0, 0, time.UTC)
	a := newTestAdapter(t, now)
	rpcError := []byte(`{"jsonrpc":"2.0","id":3,"error":{"code":-32001,"message":"not authenticated","data":{"secret":"must-not-be-copied"}}}`)
	if err := a.ApplyRateLimitsRead(rpcError, now); err == nil {
		t.Fatal("JSON-RPC account/rateLimits/read error unexpectedly accepted")
	}
	if _, err := a.Capacity(context.Background()); err == nil {
		t.Fatal("failed rate-limit RPC unexpectedly created capacity")
	}
	if err := a.ReplaceModelCatalog([][]byte{rpcError}, now); err == nil {
		t.Fatal("JSON-RPC model/list error unexpectedly accepted")
	}
}
