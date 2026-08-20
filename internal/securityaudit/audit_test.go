package securityaudit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
)

func TestAuditPenalizesInsecureRemoteAndLatest(t *testing.T) {
	root := t.TempDir()
	p := paths.Paths{AppRoot: filepath.Join(root, "app"), ConfigRoot: filepath.Join(root, "config"), GlobalSkillsRoot: filepath.Join(root, "skills"), GlobalPluginsRoot: filepath.Join(root, "plugins"), GlobalMCP: filepath.Join(root, "mcp.json"), LocksRoot: filepath.Join(root, "locks"), SecurityRoot: filepath.Join(root, "security")}
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	cfg := model.MCPConfig{MCPServers: map[string]model.MCPServer{"bad": {ServerURL: "http://example.com/mcp"}, "latest": {Command: "npx", Args: []string{"-y", "pkg@latest"}}}}
	if err := jsonx.WriteAtomic(p.GlobalMCP, cfg, ""); err != nil {
		t.Fatal(err)
	}
	rep, err := Audit(p, "")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Score >= 90 {
		t.Fatalf("expected penalties, score=%d", rep.Score)
	}
	if rep.Grade == "A" {
		t.Fatalf("expected grade below A")
	}
	_ = os.RemoveAll(root)
}

func TestAssessRegistryServerRejectsInsecureRemote(t *testing.T) {
	raw := map[string]any{"server": map[string]any{"remotes": []any{map[string]any{"url": "http://example.com/mcp"}}}}
	rep := AssessRegistryServer("example", "active", raw)
	if rep.Score >= 80 {
		t.Fatalf("expected insecure remote penalty, got %d", rep.Score)
	}
}
