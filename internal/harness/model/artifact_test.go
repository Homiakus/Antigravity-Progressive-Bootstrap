package model

import (
	"testing"
	"time"
)

func TestArtifactMetadataValidation(t *testing.T) {
	now := time.Now()
	validDigest := ComputeSHA256Digest([]byte("test content"))

	valid := ArtifactMetadata{
		ID:            "art_1",
		WorkflowRunID: "wfr_1",
		ContentDigest: validDigest,
		Type:          ArtifactLog,
		Name:          "stdout.log",
		URI:           "cas://" + validDigest,
		SizeBytes:     12,
		CreatedAt:     now,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid artifact, got: %v", err)
	}

	invalidType := valid
	invalidType.Type = "INVALID_TYPE"
	if err := invalidType.Validate(); err == nil {
		t.Fatal("expected error on invalid artifact type")
	}

	invalidDigest := valid
	invalidDigest.ContentDigest = "md5:bad"
	if err := invalidDigest.Validate(); err == nil {
		t.Fatal("expected error on invalid digest")
	}

	negativeSize := valid
	negativeSize.SizeBytes = -1
	if err := negativeSize.Validate(); err == nil {
		t.Fatal("expected error on negative size")
	}
}

func TestProvenanceEdgeValidation(t *testing.T) {
	now := time.Now()
	valid := ProvenanceEdge{
		ArtifactID: "art_1",
		NodeRunID:  "nr_1",
		Relation:   ProvenanceProducedBy,
		CreatedAt:  now,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid edge, got: %v", err)
	}

	invalidRelation := valid
	invalidRelation.Relation = "INVALID"
	if err := invalidRelation.Validate(); err == nil {
		t.Fatal("expected error on invalid relation")
	}
}
