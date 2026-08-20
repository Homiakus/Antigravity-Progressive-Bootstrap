package dashboard

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/homiakus/agctl/internal/paths"
)

func TestAdaptiveEndpointsAreRegistered(t *testing.T) {
	root := t.TempDir()
	p := paths.Paths{AppRoot: filepath.Join(root, "app")}
	p.TasksRoot = filepath.Join(p.AppRoot, "tasks")
	p.PlansRoot = filepath.Join(p.AppRoot, "plans")
	p.ReplanRoot = filepath.Join(p.AppRoot, "replan")
	p.ReplanConfig = filepath.Join(p.ReplanRoot, "config.json")
	p.ReplanInbox = filepath.Join(p.ReplanRoot, "inbox")
	p.ReplanArchive = filepath.Join(p.ReplanRoot, "archive")
	p.TelemetryRoot = filepath.Join(p.AppRoot, "telemetry")
	p.SecurityRoot = filepath.Join(p.AppRoot, "security")
	p.LocksRoot = filepath.Join(p.AppRoot, "locks")
	p.GlobalMCP = filepath.Join(root, "mcp.json")
	p.GlobalPluginsRoot = filepath.Join(root, "plugins")
	p.GlobalSkillsRoot = filepath.Join(root, "skills")
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	h := newHandler(p, root, "127.0.0.1:8787")

	r := httptest.NewRequest("GET", "/api/replan", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("/api/replan status=%d body=%s", w.Code, w.Body.String())
	}
	var x map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &x); err != nil {
		t.Fatal(err)
	}
	if _, ok := x["config"]; !ok {
		t.Fatalf("missing config: %v", x)
	}

	r = httptest.NewRequest("GET", "/metrics", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("/metrics status=%d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"agctl_plan_revisions_total", "agctl_dynamic_nodes_total", "agctl_replan_inbox"} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics missing %s: %s", want, body)
		}
	}
}
