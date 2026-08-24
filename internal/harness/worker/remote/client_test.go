package remote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRemoteWorkerRegistrationAndHeartbeat(t *testing.T) {
	var registered, heartbeated bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/workers/register":
			registered = true
			w.WriteHeader(http.StatusOK)
		case "/v1/workers/heartbeat":
			heartbeated = true
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	ctx := context.Background()
	client := NewClient(WorkerConfig{
		WorkerID:     "remote-worker-1",
		BaseURL:      ts.URL,
		Capabilities: []string{"process.execute", "filesystem.read"},
		Trust:        "TRUSTED_REMOTE",
		Heartbeat:    5 * time.Second,
	}, nil)

	// 1. Register
	if err := client.Register(ctx); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if !registered {
		t.Fatal("expected server to receive register request")
	}

	// 2. Heartbeat
	if err := client.Heartbeat(ctx); err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	if !heartbeated {
		t.Fatal("expected server to receive heartbeat request")
	}
}
