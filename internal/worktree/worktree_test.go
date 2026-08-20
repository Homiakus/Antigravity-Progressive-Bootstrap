package worktree

import "testing"

func TestSafe(t *testing.T) {
	if safe("feature") != "feature" {
		t.Fatal()
	}
	if safe("../bad") != "" {
		t.Fatal("unsafe accepted")
	}
}
