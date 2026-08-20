package jsonx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteAndBackup(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "a.json")
	backs := filepath.Join(root, "backups")
	if err := WriteAtomic(p, map[string]any{"x": 1}, backs); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(p, map[string]any{"x": 2}, backs); err != nil {
		t.Fatal(err)
	}
	xs, err := ListBackups(backs)
	if err != nil {
		t.Fatal(err)
	}
	if len(xs) != 1 {
		t.Fatalf("expected 1 backup got %d", len(xs))
	}
	if _, err := os.Stat(xs[0]); err != nil {
		t.Fatal(err)
	}
}
