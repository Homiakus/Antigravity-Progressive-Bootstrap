package model

import (
	"bytes"
	"testing"
	"time"
)

func TestTimeSortableIDGeneratorDeterministicInjection(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 123000000, time.UTC)
	g := TimeSortableIDGenerator{Now: func() time.Time { return now }, Random: bytes.NewReader(make([]byte, 10))}
	got, err := g.New(IDWorkflowDefinition)
	if err != nil {
		t.Fatal(err)
	}
	want := "wfd_1787227200123_00000000000000000000"
	if got != want {
		t.Fatalf("id = %q, want %q", got, want)
	}
	if err := ValidateGeneratedID(got, IDWorkflowDefinition); err != nil {
		t.Fatalf("generated id did not validate: %v", err)
	}
}

func TestValidateNodeID(t *testing.T) {
	good := []NodeID{"inspect", "r2-repair-verify", "stage/child:1", "A.B_C"}
	for _, id := range good {
		if err := ValidateNodeID(id); err != nil {
			t.Errorf("ValidateNodeID(%q): %v", id, err)
		}
	}
	bad := []NodeID{"", " space", "bad space", "@bad"}
	for _, id := range bad {
		if err := ValidateNodeID(id); err == nil {
			t.Errorf("ValidateNodeID(%q) unexpectedly succeeded", id)
		}
	}
}
