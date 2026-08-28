package antigravityide

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestHTTPClientRequiresLoopbackAndToken(t *testing.T) {
	if _, err := NewHTTPClient("http://example.com:1234", "token", nil); err == nil {
		t.Fatal("expected non-loopback bridge to be rejected")
	}
	if _, err := NewHTTPClient("http://127.0.0.1:1234", "", nil); err == nil {
		t.Fatal("expected empty token to be rejected")
	}
}

func TestHTTPClientConversationDispatch(t *testing.T) {
	var mu sync.Mutex
	var focused, message string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"protocolVersion": 1, "ok": false, "error": map[string]string{"code": "UNAUTHORIZED", "message": "bad token"}})
			return
		}
		var data any = map[string]any{}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/capabilities":
			data = map[string]any{"protocolVersion": 1, "workspaceOpen": true, "conversationList": true, "conversationCreate": true, "conversationFocus": true, "conversationSend": true, "conversationDirectSend": false, "messageHistory": false, "agentEvents": false, "cancel": false, "approvalEvents": false, "approvalDecision": false, "nativeFork": false, "conversationCreateMode": "command-fallback", "conversationDispatchMode": "focus-then-send"}
		case r.Method == http.MethodGet && r.URL.Path == "/v1/conversations":
			data = []map[string]any{{"id": "c1", "title": "test"}}
		case r.Method == http.MethodPost && r.URL.Path == "/v1/conversations/c1/focus":
			mu.Lock(); focused = "c1"; mu.Unlock()
		case r.Method == http.MethodPost && r.URL.Path == "/v1/conversations/c1/messages":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock(); message = body["text"]; mu.Unlock()
		default:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"protocolVersion": 1, "ok": false, "error": map[string]string{"code": "NOT_FOUND", "message": r.URL.Path}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"protocolVersion": 1, "ok": true, "data": data})
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, "secret", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	caps, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if caps.ConversationDirectSend || caps.ConversationDispatchMode != "focus-then-send" {
		t.Fatalf("capabilities=%#v", caps)
	}
	conversations, err := client.ListConversations(context.Background())
	if err != nil || len(conversations) != 1 || conversations[0].ID != "c1" {
		t.Fatalf("conversations=%#v err=%v", conversations, err)
	}
	if err := client.FocusConversation(context.Background(), "c1"); err != nil {
		t.Fatal(err)
	}
	if err := client.SendMessage(context.Background(), "c1", "hello"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if focused != "c1" || message != "hello" {
		t.Fatalf("focused=%q message=%q", focused, message)
	}
}

func TestHTTPClientRejectsUnknownSecretLikeResponseFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"protocolVersion": 1, "ok": true, "data": map[string]any{"status": "ok", "instanceId": "i", "bootNonce": "b", "pid": 1, "token": "must-not-leak"}})
	}))
	defer server.Close()
	client, _ := NewHTTPClient(server.URL, "secret", server.Client())
	_, err := client.Health(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected strict decode failure, got %v", err)
	}
}
