package model

import "testing"

func TestBuildEffectIdentityStableAcrossAttemptsByConstruction(t *testing.T) {
	key1, digest1, err := BuildEffectIdentity("wfr_same", "nr_same", "github.create_issue", []byte(`{"title":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	key2, digest2, err := BuildEffectIdentity("wfr_same", "nr_same", "github.create_issue", []byte(`{"title":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if key1 != key2 || digest1 != digest2 {
		t.Fatalf("same logical effect changed identity: %q/%q vs %q/%q", key1, digest1, key2, digest2)
	}
	changed, _, err := BuildEffectIdentity("wfr_same", "nr_same", "github.create_issue", []byte(`{"title":"y"}`))
	if err != nil {
		t.Fatal(err)
	}
	if changed == key1 {
		t.Fatal("semantic input change did not change effect key")
	}
}

func TestBuildEffectIdentitySeparatesNamespacesAndNodes(t *testing.T) {
	base, _, err := BuildEffectIdentity("wfr_a", "nr_a", "github.create_issue", []byte("same"))
	if err != nil {
		t.Fatal(err)
	}
	otherNode, _, _ := BuildEffectIdentity("wfr_a", "nr_b", "github.create_issue", []byte("same"))
	otherNamespace, _, _ := BuildEffectIdentity("wfr_a", "nr_a", "github.create_pr", []byte("same"))
	otherRun, _, _ := BuildEffectIdentity("wfr_b", "nr_a", "github.create_issue", []byte("same"))
	for name, got := range map[string]string{"node": otherNode, "namespace": otherNamespace, "run": otherRun} {
		if got == base {
			t.Fatalf("%s change did not separate effect identity", name)
		}
	}
}

func TestEffectClassBlindRetrySafetyIsConservative(t *testing.T) {
	safe := []EffectClass{EffectPure, EffectIdempotent, EffectIdempotentWithKey}
	for _, class := range safe {
		if !class.BlindRetrySafe() {
			t.Errorf("class %s should be blind-retry safe", class)
		}
	}
	unsafe := []EffectClass{EffectQueryable, EffectCompensatable, EffectNonIdempotentUnknown}
	for _, class := range unsafe {
		if class.BlindRetrySafe() {
			t.Errorf("class %s must require reconciliation/policy before retry", class)
		}
	}
}
