package mcpprobe

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/model"
)

func TestHTTPProbeModernStatelessAndTools(t *testing.T) {
	var sawInitialize bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if req.Method == "initialize" {
			sawInitialize = true
		}
		if got := r.Header.Get("MCP-Protocol-Version"); got != CurrentProtocolVersion {
			t.Fatalf("protocol header=%q", got)
		}
		if got := r.Header.Get("Mcp-Method"); got != req.Method {
			t.Fatalf("method header=%q body=%q", got, req.Method)
		}
		params, _ := req.Params.(map[string]any)
		meta, _ := params["_meta"].(map[string]any)
		if meta["io.modelcontextprotocol/protocolVersion"] != CurrentProtocolVersion {
			t.Fatalf("missing modern _meta: %#v", params)
		}
		w.Header().Set("Content-Type", "application/json")
		result := map[string]any{"resultType": "complete", "ttlMs": float64(1000), "cacheScope": "private"}
		switch req.Method {
		case "server/discover":
			result = map[string]any{
				"resultType":        "complete",
				"ttlMs":             float64(1000),
				"cacheScope":        "public",
				"supportedVersions": []any{CurrentProtocolVersion},
				"capabilities":      map[string]any{"tools": map[string]any{}, "resources": map[string]any{}, "prompts": map[string]any{}},
				"_meta":             map[string]any{"io.modelcontextprotocol/serverInfo": map[string]any{"name": "modern-fake", "version": "2"}},
			}
		case "tools/list":
			result["tools"] = []any{map[string]any{"name": "ping"}}
		case "resources/list":
			result["resources"] = []any{map[string]any{"uri": "fake://r"}}
		case "prompts/list":
			result["prompts"] = []any{map[string]any{"name": "review"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer srv.Close()

	r, err := Probe("fake", model.MCPServer{ServerURL: srv.URL}, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if sawInitialize {
		t.Fatal("modern probe must not use initialize")
	}
	if !r.OK || r.Protocol != CurrentProtocolVersion || r.ServerName != "modern-fake" || len(r.Tools) != 1 || r.Tools[0] != "ping" {
		t.Fatalf("report=%+v", r)
	}
}

func TestHTTPProbeFallsBackToLegacyInitialize(t *testing.T) {
	var initialized bool
	var initializedNotification bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "notifications/initialized" {
			initializedNotification = true
			if r.Header.Get("MCP-Protocol-Version") == "" || r.Header.Get("Mcp-Session-Id") != "legacy-session" {
				t.Fatalf("legacy notification missing negotiated headers")
			}
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if req.Method == "server/discover" {
			// Legacy HTTP+streamable-era endpoint: no recognized modern JSON-RPC
			// error body, so current clients may fall back to initialize.
			http.Error(w, "legacy endpoint", http.StatusNotFound)
			return
		}
		result := map[string]any{}
		switch req.Method {
		case "initialize":
			initialized = true
			w.Header().Set("Mcp-Session-Id", "legacy-session")
			result = map[string]any{"protocolVersion": FallbackProtocolVersion, "serverInfo": map[string]any{"name": "legacy-fake", "version": "1"}}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "ping"}}}
		case "resources/list":
			result = map[string]any{"resources": []any{map[string]any{"uri": "fake://r"}}}
		case "prompts/list":
			result = map[string]any{"prompts": []any{map[string]any{"name": "review"}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer srv.Close()

	r, err := Probe("legacy", model.MCPServer{ServerURL: srv.URL}, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !initialized || !initializedNotification || r.Protocol != FallbackProtocolVersion || len(r.Tools) != 1 {
		t.Fatalf("initialized=%v initializedNotification=%v report=%+v", initialized, initializedNotification, r)
	}
}

func TestHTTPProbeDoesNotFallbackOnRecognizedModernMethodError(t *testing.T) {
	var sawInitialize bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method == "initialize" {
			sawInitialize = true
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": req.ID,
			"error": map[string]any{"code": -32601, "message": "Method not found"},
		})
	}))
	defer srv.Close()

	_, err := Probe("modern-broken", model.MCPServer{ServerURL: srv.URL}, 2*time.Second)
	if err == nil {
		t.Fatal("expected modern method error")
	}
	if sawInitialize {
		t.Fatal("recognized modern HTTP error must not fall back to initialize")
	}
}

func TestHTTPProbeRejectsModernResultWithoutResultType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": req.ID,
			"result": map[string]any{
				"supportedVersions": []any{CurrentProtocolVersion},
				"capabilities":      map[string]any{},
				"ttlMs":             float64(1000),
				"cacheScope":        "public",
			},
		})
	}))
	defer srv.Close()

	_, err := Probe("noncompliant", model.MCPServer{ServerURL: srv.URL}, 2*time.Second)
	if err == nil || !strings.Contains(err.Error(), "resultType") {
		t.Fatalf("expected resultType compliance error, got %v", err)
	}
}
