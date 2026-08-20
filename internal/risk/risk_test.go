package risk

import (
	"path/filepath"
	"testing"

	"github.com/homiakus/agctl/internal/model"
)

func TestSemanticDestructiveMCPDenied(t *testing.T) {
	d := Classify(model.ToolCall{Name: "mcp__database__drop_database", Args: map[string]any{}}, Context{PermissionMode: "guarded"})
	if d.Decision != "deny" || d.Risk != RiskCritical {
		t.Fatalf("expected critical deny, got %+v", d)
	}
}

func TestNormalReadAllowed(t *testing.T) {
	d := Classify(model.ToolCall{Name: "mcp__github__list_issues", Args: map[string]any{}}, Context{PermissionMode: "guarded"})
	if d.Decision != "allow" || d.Risk != RiskReadLow {
		t.Fatalf("expected read allow, got %+v", d)
	}
}

func TestSensitiveWriteDenied(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(home, ".ssh", "config")
	d := Classify(model.ToolCall{Name: "write_to_file", Args: map[string]any{"TargetFile": target}}, Context{PermissionMode: "guarded", Home: home, Workspaces: []string{filepath.Join(home, "project")}})
	if d.Decision != "deny" {
		t.Fatalf("expected sensitive write deny, got %+v", d)
	}
}

func TestUnrestrictedAllowsExternalWriteButStillClassifies(t *testing.T) {
	d := Classify(model.ToolCall{Name: "mcp__github__merge_pull_request", Args: map[string]any{}}, Context{PermissionMode: "unrestricted"})
	if d.Decision != "allow" || d.Risk != RiskExternalHigh {
		t.Fatalf("expected unrestricted external allow, got %+v", d)
	}
}
