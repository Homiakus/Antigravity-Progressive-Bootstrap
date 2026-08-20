package goal

import "testing"

func TestNativeAndHeadless(t *testing.T) {
	n, _ := NativeCommand("build it")
	if n != "/goal build it" {
		t.Fatal(n)
	}
	h, _ := HeadlessPrompt("build it")
	if h == "" {
		t.Fatal("empty")
	}
}
