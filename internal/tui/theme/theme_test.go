package theme

import (
	"os"
	"testing"
)

func TestThemeDarkLight(t *testing.T) {
	dark := NewTheme(true)
	if dark == nil {
		t.Fatal("expected dark theme to initialize")
	}

	light := NewTheme(false)
	if light == nil {
		t.Fatal("expected light theme to initialize")
	}
}

func TestNoColorMode(t *testing.T) {
	_ = os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	th := NewTheme(true)
	if !th.NoColor {
		t.Errorf("expected NoColor to be true when NO_COLOR env var is set")
	}
}
