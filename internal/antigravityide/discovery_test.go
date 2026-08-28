package antigravityide

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestDiscoveryIgnoresStaleRegistrationAndFindsHealthyBridge(t *testing.T) {
	root := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"protocolVersion": 1, "ok": true, "data": map[string]any{"status": "ok", "instanceId": "i1", "bootNonce": "boot-good", "pid": 99}})
	}))
	defer server.Close()
	parsed, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(parsed.Port())
	stale := Registration{ProtocolVersion: 1, InstanceID: "i1", BootNonce: "boot-stale", PID: 1, Port: port + 1, StartedAt: time.Now().UTC().Add(-time.Minute)}
	good := Registration{ProtocolVersion: 1, InstanceID: "i1", BootNonce: "boot-good", PID: 99, Port: port, StartedAt: time.Now().UTC()}
	writeRegistration(t, filepath.Join(root, "stale.json"), stale)
	writeRegistration(t, filepath.Join(root, "good.json"), good)
	discovery := Discovery{Root: root, PollInterval: time.Millisecond, HTTPClient: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	located, err := discovery.Wait(ctx, "i1", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if located.Registration.BootNonce != "boot-good" {
		t.Fatalf("registration=%#v", located.Registration)
	}
}

func TestRegistrationRejectsUnexpectedFields(t *testing.T) {
	file := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(file, []byte(`{"protocolVersion":1,"instanceId":"i","bootNonce":"b","pid":1,"port":1234,"workspaceFolders":[],"startedAt":"2026-08-28T00:00:00Z","bridgeToken":"secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readRegistration(file); err == nil {
		t.Fatal("expected secret/unexpected field to be rejected")
	}
}

func writeRegistration(t *testing.T, file string, registration Registration) {
	t.Helper()
	payload, err := json.Marshal(registration)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, payload, 0o600); err != nil {
		t.Fatal(err)
	}
}
