package store

import "testing"

func TestErrNotFoundStable(t *testing.T) {
	if ErrNotFound == nil || ErrNotFound.Error() == "" {
		t.Fatal("ErrNotFound must be a stable sentinel")
	}
}
