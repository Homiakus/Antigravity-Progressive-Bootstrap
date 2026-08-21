package process

import (
	"os"
	"testing"
)

// TestMain removes the race runtime's default one-second exit sleep from child
// Go test binaries spawned by the process executor tests. The current package
// has already initialized its race runtime before this executes; the setting is
// therefore only inherited by subprocess helpers. Without this, a helper that
// has completed all useful work remains alive and silent for one second, which
// correctly trips production idle-timeout semantics and turns the test into a
// race-runtime timing test instead of an executor activity test.
//
// GORACE is ignored by non-race binaries.
func TestMain(m *testing.M) {
	_ = os.Setenv("GORACE", "atexit_sleep_ms=0")
	os.Exit(m.Run())
}
