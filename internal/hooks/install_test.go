package hooks

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/loop"
)

func TestInstallPreservesUnknownHooksAndReflectsLoopDisabled(t *testing.T) {
	p := hookPaths(t)
	original := map[string]any{
		"user-hook": map[string]any{
			"enabled":     true,
			"CustomEvent": []any{map[string]any{"type": "command", "command": "echo user"}},
		},
		"unknownTopLevel": "keep-me",
	}
	if err := jsonx.WriteAtomic(p.GlobalHooks, original, ""); err != nil {
		t.Fatal(err)
	}

	cfg := loop.DefaultConfig()
	cfg.Enabled = false
	if err := loop.Save(p, cfg); err != nil {
		t.Fatal(err)
	}

	if err := Install(p, `C:\\Program Files\\agctl\\agctl.exe`); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(p.GlobalHooks)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}

	if got["unknownTopLevel"] != "keep-me" {
		t.Fatalf("unknown top-level field lost: %#v", got["unknownTopLevel"])
	}
	if _, ok := got["user-hook"]; !ok {
		t.Fatal("user hook was removed")
	}

	loopHook, ok := got[LoopHookName].(map[string]any)
	if !ok {
		t.Fatalf("missing loop hook: %#v", got[LoopHookName])
	}
	if enabled, _ := loopHook["enabled"].(bool); enabled {
		t.Fatalf("loop hook unexpectedly enabled while loop config is disabled: %#v", loopHook)
	}
	toolHook, ok := got[ToolHookName].(map[string]any)
	if !ok {
		t.Fatalf("missing tool hook: %#v", got[ToolHookName])
	}
	if enabled, _ := toolHook["enabled"].(bool); enabled {
		t.Fatalf("tool hook unexpectedly enabled while loop config is disabled: %#v", toolHook)
	}
}

func TestInstallReflectsLoopEnabled(t *testing.T) {
	p := hookPaths(t)
	if _, err := loop.EnableProfile(p, "deep"); err != nil {
		t.Fatal(err)
	}
	if err := Install(p, "agctl"); err != nil {
		t.Fatal(err)
	}
	got, err := jsonx.ReadMap(p.GlobalHooks)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{LoopHookName, ToolHookName} {
		hook, ok := got[name].(map[string]any)
		if !ok {
			t.Fatalf("missing managed hook %s", name)
		}
		enabled, _ := hook["enabled"].(bool)
		if !enabled {
			t.Fatalf("managed hook %s should be enabled", name)
		}
	}
}
