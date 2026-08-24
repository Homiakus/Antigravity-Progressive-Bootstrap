package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type ArtifactType string

const (
	ArtifactLog    ArtifactType = "LOG"
	ArtifactOutput ArtifactType = "OUTPUT"
	ArtifactFile   ArtifactType = "FILE"
	ArtifactJSON   ArtifactType = "JSON"
	ArtifactDiff   ArtifactType = "DIFF"
	ArtifactCustom ArtifactType = "CUSTOM"
)

func (t ArtifactType) Valid() bool {
	switch t {
	case ArtifactLog, ArtifactOutput, ArtifactFile, ArtifactJSON, ArtifactDiff, ArtifactCustom:
		return true
	default:
		return false
	}
}

type ArtifactMetadata struct {
	ID                ArtifactID        `json:"id"`
	WorkflowRunID     WorkflowRunID     `json:"workflowRunId"`
	ProducerNodeRunID NodeRunID         `json:"producerNodeRunId,omitempty"`
	ProducerAttemptID AttemptID         `json:"producerAttemptId,omitempty"`
	ContentDigest     string            `json:"contentDigest"` // e.g. "sha256:abcd..."
	Type              ArtifactType      `json:"type"`
	Name              string            `json:"name"`
	URI               string            `json:"uri"`
	SizeBytes         int64             `json:"sizeBytes"`
	CreatedAt         time.Time         `json:"createdAt"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

func (a ArtifactMetadata) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("artifact id is required")
	}
	if a.WorkflowRunID == "" {
		return fmt.Errorf("artifact workflow run id is required")
	}
	if !a.Type.Valid() {
		return fmt.Errorf("invalid artifact type %q", a.Type)
	}
	if strings.TrimSpace(a.Name) == "" {
		return fmt.Errorf("artifact name is required")
	}
	if !ValidateDigest(a.ContentDigest) {
		return fmt.Errorf("invalid content digest %q", a.ContentDigest)
	}
	if a.SizeBytes < 0 {
		return fmt.Errorf("artifact size cannot be negative")
	}
	if a.CreatedAt.IsZero() {
		return fmt.Errorf("artifact created_at is required")
	}
	return nil
}

type ProvenanceRelation string

const (
	ProvenanceProducedBy ProvenanceRelation = "PRODUCED_BY"
	ProvenanceConsumedBy ProvenanceRelation = "CONSUMED_BY"
)

func (r ProvenanceRelation) Valid() bool {
	return r == ProvenanceProducedBy || r == ProvenanceConsumedBy
}

type ProvenanceEdge struct {
	ArtifactID ArtifactID         `json:"artifactId"`
	NodeRunID  NodeRunID          `json:"nodeRunId"`
	Relation   ProvenanceRelation `json:"relation"`
	CreatedAt  time.Time          `json:"createdAt"`
}

func (e ProvenanceEdge) Validate() error {
	if e.ArtifactID == "" || e.NodeRunID == "" {
		return fmt.Errorf("artifact id and node run id are required for provenance edge")
	}
	if !e.Relation.Valid() {
		return fmt.Errorf("invalid provenance relation %q", e.Relation)
	}
	if e.CreatedAt.IsZero() {
		return fmt.Errorf("provenance edge created_at is required")
	}
	return nil
}

func ValidateDigest(digest string) bool {
	if !strings.HasPrefix(digest, "sha256:") {
		return false
	}
	hexPart := strings.TrimPrefix(digest, "sha256:")
	if len(hexPart) != 64 {
		return false
	}
	_, err := hex.DecodeString(hexPart)
	return err == nil
}

func ComputeSHA256Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
