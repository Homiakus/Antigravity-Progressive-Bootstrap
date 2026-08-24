package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/homiakus/agctl/internal/paths"
)

func setupTestPaths(t *testing.T) paths.Paths {
	t.Helper()
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	_ = os.MkdirAll(home, 0o755)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	t.Setenv("AGCTL_CONFIG_ROOT", filepath.Join(home, ".gemini", "config"))
	t.Setenv("AGCTL_STATE_ROOT", filepath.Join(home, ".gemini", "state"))

	p, err := paths.Detect()
	if err != nil {
		t.Fatalf("detect failed: %v", err)
	}
	if err := p.Ensure(); err != nil {
		t.Fatalf("ensure failed: %v", err)
	}
	return p
}

func TestWebEndpoints(t *testing.T) {
	p := setupTestPaths(t)
	ws := t.TempDir()

	handler := newRouter(p, ws, "127.0.0.1:8787", "http://127.0.0.1:8787")

	// 1. Test Index HTML
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for index, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "agctl Web Control Plane") {
		t.Fatalf("index response missing brand title")
	}

	// 2. Test Snapshot API
	req = httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for snapshot, got %d", rec.Code)
	}
	var snap SystemSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("failed to decode snapshot: %v", err)
	}
	if snap.Version != AppVersion {
		t.Fatalf("expected version %s, got %s", AppVersion, snap.Version)
	}

	// 3. Test Loop API (Get & Post)
	req = httptest.NewRequest(http.MethodGet, "/api/loop", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for get loop, got %d", rec.Code)
	}

	loopBody := `{"action":"enable","profile":"deep"}`
	req = httptest.NewRequest(http.MethodPost, "/api/loop", strings.NewReader(loopBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for post loop, got %d", rec.Code)
	}

	// 4. Test Tasks Add & List API
	taskBody := `{"description":"Test Task 1","priority":"high","kind":"feature"}`
	req = httptest.NewRequest(http.MethodPost, "/api/tasks/add", strings.NewReader(taskBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for task add, got %d: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for task list, got %d", rec.Code)
	}

	// 5. Test Doctor API
	req = httptest.NewRequest(http.MethodGet, "/api/doctor", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for doctor, got %d", rec.Code)
	}
}

func TestBroker(t *testing.T) {
	b := newBroker()
	ch := make(chan []byte, 10)
	b.addClient(ch)

	b.Broadcast("INFO", "test", "hello world")
	select {
	case msg := <-ch:
		if !strings.Contains(string(msg), "hello world") {
			t.Fatalf("unexpected message: %s", string(msg))
		}
	default:
		t.Fatalf("client did not receive broadcast")
	}

	b.removeClient(ch)
}
