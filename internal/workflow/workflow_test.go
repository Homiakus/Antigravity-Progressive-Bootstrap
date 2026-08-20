package workflow

import "testing"

func TestInstallAndList(t *testing.T) {
	w := t.TempDir()
	if err := InstallEmbedded(w, "verified-goal"); err != nil {
		t.Fatal(err)
	}
	xs, err := List(w)
	if err != nil {
		t.Fatal(err)
	}
	if len(xs) != 1 || xs[0].Name != "verified-goal" {
		t.Fatalf("got %+v", xs)
	}
}
