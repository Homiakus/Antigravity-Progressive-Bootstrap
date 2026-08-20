package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/homiakus/agctl/internal/paths"
)

func testPaths(t *testing.T) paths.Paths {
	t.Helper()
	root := t.TempDir()
	return paths.Paths{
		GlobalAgentsRoot: filepath.Join(root, "global-agents"),
		BackupsRoot:      filepath.Join(root, "backups"),
	}
}

func TestEmbeddedAgentsUseDocumentedFrontmatter(t *testing.T) {
	p := testPaths(t)
	if err := InstallEmbedded(p, ""); err != nil {
		t.Fatal(err)
	}
	for _, d := range Embedded {
		path := filepath.Join(p.GlobalAgentsRoot, d.Name, "agent.md")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		if strings.Contains(text, "permissionMode:") {
			t.Fatalf("%s contains unsupported permissionMode", d.Name)
		}
		for _, want := range []string{"mainAgent: false", "subagent: true", "commandExecutionPolicy: auto"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q\n%s", d.Name, want, text)
			}
		}
	}
	for _, line := range Doctor(p, "") {
		if !strings.Contains(line, "OK") {
			t.Fatalf("doctor rejected embedded agent: %s", line)
		}
	}
}

func TestListSupportsFlatAndFolderAgents(t *testing.T) {
	p := testPaths(t)
	if err := os.MkdirAll(p.GlobalAgentsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	flat := `---
name: flat-agent
description: flat layout
model: inherit
subagent: true
---
`
	if err := os.WriteFile(filepath.Join(p.GlobalAgentsRoot, "flat-agent.md"), []byte(flat), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(p.GlobalAgentsRoot, "folder-agent")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	folder := `---
name: folder-agent
description: folder layout
model: flash
subagent: true
---
`
	if err := os.WriteFile(filepath.Join(dir, "agent.md"), []byte(folder), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := List(p, "")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, it := range items {
		seen[it.Name] = true
	}
	if !seen["flat-agent"] || !seen["folder-agent"] {
		t.Fatalf("expected both documented layouts, got %#v", seen)
	}
}

func TestDoctorRejectsUnsupportedAgentFieldsAndTools(t *testing.T) {
	p := testPaths(t)
	if err := os.MkdirAll(p.GlobalAgentsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := `---
name: bad
description: invalid agent
model: magic
permissionMode: acceptEdits
commandExecutionPolicy: invalid
tools:
  - definitely_not_a_tool
---
`
	if err := os.WriteFile(filepath.Join(p.GlobalAgentsRoot, "bad.md"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	lines := Doctor(p, "")
	if len(lines) != 1 || strings.Contains(lines[0], " OK") {
		t.Fatalf("doctor should reject bad agent: %v", lines)
	}
}
