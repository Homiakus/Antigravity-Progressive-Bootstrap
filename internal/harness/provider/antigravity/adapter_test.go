package antigravity

import (
	"context"
	"strings"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func testAccount(now time.Time) harnessmodel.ProviderAccount {
	return harnessmodel.ProviderAccount{
		ID:        "pacc_antigravity",
		Provider:  harnessmodel.ProviderAntigravity,
		Name:      "local-antigravity",
		State:     harnessmodel.ProviderAccountActive,
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Minute),
	}
}

func TestParseStatusLineNormalizesOfficialSignals(t *testing.T) {
	observedAt := time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC)
	payload := []byte(`{
		"cwd":"/home/user/my-project",
		"session_id":"12345678-abcd-ef01-2345-6789abcdef01",
		"conversation_id":"12345678-abcd-ef01-2345-6789abcdef01",
		"model":{"id":"Gemini 3.5 Flash (High)","display_name":"Gemini 3.5 Flash (High)","future_model_field":true},
		"workspace":{"current_dir":"/home/user/my-project","project_dir":"/home/user/my-project"},
		"context_window":{"total_input_tokens":88244,"total_output_tokens":61074,"context_window_size":1048576,"used_percentage":14.24,"remaining_percentage":85.76},
		"product":"antigravity",
		"quota":{"gemini-weekly":{"remaining_fraction":0.9378,"reset_time":"2026-09-03T07:50:32Z","reset_in_seconds":560580}},
		"email":"developer@example.invalid",
		"plan_tier":"Pro",
		"future_top_level":{"ignored":true}
	}`)

	obs, err := ParseStatusLine(testAccount(observedAt), payload, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	if obs.Capacity.AccountID != "pacc_antigravity" || obs.Capacity.Provider != harnessmodel.ProviderAntigravity {
		t.Fatalf("unexpected capacity identity: %+v", obs.Capacity)
	}
	if obs.Capacity.Health != harnessmodel.ProviderHealthUnknown {
		t.Fatalf("health=%s want UNKNOWN: valid status-line telemetry is not a service-health proof", obs.Capacity.Health)
	}
	if len(obs.Capacity.Windows) != 1 {
		t.Fatalf("quota windows=%d want=1", len(obs.Capacity.Windows))
	}
	window := obs.Capacity.Windows[0]
	if window.ID != "gemini-weekly" || window.Metric != harnessmodel.QuotaMetricFraction || window.ModelID != "" {
		t.Fatalf("unexpected quota window: %+v", window)
	}
	if window.RemainingFraction == nil || *window.RemainingFraction != 0.9378 || window.Confidence != 1 {
		t.Fatalf("unexpected normalized fraction: %+v", window)
	}
	if window.ResetAt == nil || !window.ResetAt.Equal(time.Date(2026, 9, 3, 7, 50, 32, 0, time.UTC)) {
		t.Fatalf("unexpected reset time: %v", window.ResetAt)
	}

	if len(obs.Models) != 1 {
		t.Fatalf("models=%d want=1", len(obs.Models))
	}
	model := obs.Models[0]
	if model.ID != "Gemini 3.5 Flash (High)" || model.ContextLimit != 1048576 || !model.Enabled {
		t.Fatalf("unexpected model: %+v", model)
	}

	if len(obs.Sessions) != 1 {
		t.Fatalf("sessions=%d want=1", len(obs.Sessions))
	}
	session := obs.Sessions[0]
	if session.ID != "12345678-abcd-ef01-2345-6789abcdef01" || session.ModelID != model.ID {
		t.Fatalf("unexpected session identity: %+v", session)
	}
	if session.ContextLimit != 1048576 || session.ContextUsed != 149317 {
		t.Fatalf("context=%d/%d want=149317/1048576", session.ContextUsed, session.ContextLimit)
	}
	if session.WorkspaceFingerprint == "" || strings.Contains(session.WorkspaceFingerprint, "/home/user") {
		t.Fatalf("workspace path leaked instead of fingerprint: %q", session.WorkspaceFingerprint)
	}
}

func TestParseStatusLinePreservesOpaqueAndSortsNativeQuotaBuckets(t *testing.T) {
	now := time.Date(2026, 8, 28, 13, 5, 0, 0, time.UTC)
	obs, err := ParseStatusLine(testAccount(now), []byte(`{
		"product":"antigravity",
		"quota":{
			"z-native":{"remaining_fraction":0.4},
			"a-future":{"reset_time":"2026-08-29T00:00:00Z"}
		}
	}`), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs.Capacity.Windows) != 2 {
		t.Fatalf("windows=%d want=2", len(obs.Capacity.Windows))
	}
	if obs.Capacity.Windows[0].ID != "a-future" || obs.Capacity.Windows[1].ID != "z-native" {
		t.Fatalf("quota order is not deterministic: %+v", obs.Capacity.Windows)
	}
	if obs.Capacity.Windows[0].Metric != harnessmodel.QuotaMetricOpaque || obs.Capacity.Windows[0].Confidence != 0 {
		t.Fatalf("missing fraction must remain opaque: %+v", obs.Capacity.Windows[0])
	}
	if obs.Capacity.Windows[1].ModelID != "" {
		t.Fatalf("native bucket was guessed as model mapping: %+v", obs.Capacity.Windows[1])
	}
	if obs.Capacity.Health != harnessmodel.ProviderHealthUnknown {
		t.Fatalf("health=%s want UNKNOWN with opaque bucket", obs.Capacity.Health)
	}
}

func TestParseStatusLineMarksOnlyProvenQuotaExhaustion(t *testing.T) {
	now := time.Date(2026, 8, 28, 13, 10, 0, 0, time.UTC)
	obs, err := ParseStatusLine(testAccount(now), []byte(`{
		"product":"antigravity",
		"quota":{"bucket-a":{"remaining_fraction":0},"bucket-b":{"remaining_fraction":0}}
	}`), now)
	if err != nil {
		t.Fatal(err)
	}
	if obs.Capacity.Health != harnessmodel.ProviderHealthExhausted {
		t.Fatalf("health=%s want EXHAUSTED", obs.Capacity.Health)
	}

	obs, err = ParseStatusLine(testAccount(now), []byte(`{
		"product":"antigravity",
		"quota":{"bucket-a":{"remaining_fraction":0},"bucket-unknown":{}}
	}`), now)
	if err != nil {
		t.Fatal(err)
	}
	if obs.Capacity.Health != harnessmodel.ProviderHealthUnknown {
		t.Fatalf("health=%s want UNKNOWN when one bucket is opaque", obs.Capacity.Health)
	}
}

func TestParseStatusLineMissingOptionalFieldsIsUsableUnknownObservation(t *testing.T) {
	now := time.Date(2026, 8, 28, 13, 15, 0, 0, time.UTC)
	obs, err := ParseStatusLine(testAccount(now), []byte(`{"product":"antigravity","email":"sensitive@example.invalid","new_field":123}`), now)
	if err != nil {
		t.Fatal(err)
	}
	if obs.Capacity.Health != harnessmodel.ProviderHealthUnknown || len(obs.Capacity.Windows) != 0 || len(obs.Models) != 0 || len(obs.Sessions) != 0 {
		t.Fatalf("unexpected sparse observation: %+v", obs)
	}
}

func TestParseStatusLineRejectsMalformedKnownFields(t *testing.T) {
	now := time.Date(2026, 8, 28, 13, 20, 0, 0, time.UTC)
	tests := []struct {
		name    string
		payload string
	}{
		{"invalid-json", `{`},
		{"wrong-product", `{"product":"codex"}`},
		{"conversation-alias-mismatch", `{"conversation_id":"a","session_id":"b"}`},
		{"empty-model-id", `{"model":{"id":"   "}}`},
		{"negative-context", `{"context_window":{"context_window_size":-1}}`},
		{"bad-used-percent", `{"context_window":{"context_window_size":100,"used_percentage":101}}`},
		{"bad-remaining-percent", `{"context_window":{"context_window_size":100,"remaining_percentage":-1}}`},
		{"bad-quota-fraction", `{"quota":{"q":{"remaining_fraction":1.1}}}`},
		{"bad-reset-time", `{"quota":{"q":{"remaining_fraction":0.5,"reset_time":"tomorrow"}}}`},
		{"empty-quota-id", `{"quota":{" ":{"remaining_fraction":0.5}}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseStatusLine(testAccount(now), []byte(tc.payload), now); err == nil {
				t.Fatal("malformed known field unexpectedly accepted")
			}
		})
	}
}

func TestParseStatusLineCapsPayloadSize(t *testing.T) {
	now := time.Date(2026, 8, 28, 13, 25, 0, 0, time.UTC)
	if _, err := ParseStatusLine(testAccount(now), make([]byte, maxObservationBytes+1), now); err == nil {
		t.Fatal("oversized status-line payload unexpectedly accepted")
	}
}

func TestAdapterImplementsProviderObservationContracts(t *testing.T) {
	now := time.Date(2026, 8, 28, 13, 30, 0, 0, time.UTC)
	payload := []byte(`{
		"product":"antigravity",
		"conversation_id":"conv-1",
		"model":{"id":"model-1","display_name":"Model One"},
		"context_window":{"context_window_size":1000,"remaining_percentage":75},
		"quota":{"native":{"remaining_fraction":0.75}}
	}`)
	source := StatusLineSourceFunc(func(context.Context) ([]byte, time.Time, error) {
		return payload, now, nil
	})
	adapter, err := NewAdapter(testAccount(now), source)
	if err != nil {
		t.Fatal(err)
	}
	if adapter.Kind() != harnessmodel.ProviderAntigravity || adapter.Account().ID != "pacc_antigravity" {
		t.Fatalf("unexpected adapter identity")
	}
	capacity, err := adapter.Capacity(context.Background())
	if err != nil || len(capacity.Windows) != 1 {
		t.Fatalf("capacity: %+v err=%v", capacity, err)
	}
	models, err := adapter.Models(context.Background())
	if err != nil || len(models) != 1 || models[0].ID != "model-1" {
		t.Fatalf("models: %+v err=%v", models, err)
	}
	sessions, err := adapter.Sessions(context.Background())
	if err != nil || len(sessions) != 1 || sessions[0].ContextUsed != 250 {
		t.Fatalf("sessions: %+v err=%v", sessions, err)
	}
}

func TestParseHeadlessResultDirectAndStreamJSON(t *testing.T) {
	now := time.Date(2026, 8, 28, 13, 35, 0, 0, time.UTC)
	direct := []byte(`{
		"conversation_id":"055a398f-db14-4c5f-abbb-1bf03f8120a7",
		"status":"SUCCESS",
		"response":"ok",
		"future_field":true,
		"usage":{"input_tokens":10415,"output_tokens":657,"thinking_tokens":616,"cache_read_tokens":8113,"total_tokens":11072}
	}`)
	obs, err := ParseHeadlessResult(direct, now)
	if err != nil {
		t.Fatal(err)
	}
	if obs.ConversationID != "055a398f-db14-4c5f-abbb-1bf03f8120a7" || obs.Status != "SUCCESS" || obs.Usage.TotalTokens != 11072 || obs.Usage.CacheReadTokens != 8113 {
		t.Fatalf("unexpected direct headless observation: %+v", obs)
	}

	stream := []byte(`{"event":"result","result":{"conversation_id":"conv-stream","status":"SUCCESS","usage":{"input_tokens":10,"output_tokens":2,"thinking_tokens":1,"cache_read_tokens":5,"total_tokens":12}},"future":true}`)
	obs, err = ParseHeadlessResult(stream, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if obs.ConversationID != "conv-stream" || obs.Usage.InputTokens != 10 || obs.Usage.TotalTokens != 12 {
		t.Fatalf("unexpected stream headless observation: %+v", obs)
	}
}

func TestParseHeadlessResultRejectsNonTerminalAndInvalidCounts(t *testing.T) {
	now := time.Date(2026, 8, 28, 13, 40, 0, 0, time.UTC)
	for _, payload := range []string{
		`{"event":"step_update","step_update":{"state":"DONE"}}`,
		`{"conversation_id":"x","usage":{}}`,
		`{"conversation_id":"x","status":"SUCCESS","usage":{"input_tokens":-1}}`,
	} {
		if _, err := ParseHeadlessResult([]byte(payload), now); err == nil {
			t.Fatalf("invalid headless payload accepted: %s", payload)
		}
	}
}
