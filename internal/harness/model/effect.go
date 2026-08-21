package model

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type EffectClass string

const (
	EffectPure                 EffectClass = "PURE"
	EffectIdempotent           EffectClass = "IDEMPOTENT"
	EffectIdempotentWithKey    EffectClass = "IDEMPOTENT_WITH_KEY"
	EffectQueryable            EffectClass = "QUERYABLE"
	EffectCompensatable        EffectClass = "COMPENSATABLE"
	EffectNonIdempotentUnknown EffectClass = "NON_IDEMPOTENT_UNKNOWN"
)

func (c EffectClass) Valid() bool {
	switch c {
	case EffectPure, EffectIdempotent, EffectIdempotentWithKey, EffectQueryable, EffectCompensatable, EffectNonIdempotentUnknown:
		return true
	default:
		return false
	}
}

func (c EffectClass) BlindRetrySafe() bool {
	return c == EffectPure || c == EffectIdempotent || c == EffectIdempotentWithKey
}

type EffectState string

const (
	EffectPrepared    EffectState = "PREPARED"
	EffectDispatched  EffectState = "DISPATCHED"
	EffectConfirmed   EffectState = "CONFIRMED"
	EffectFailed      EffectState = "FAILED"
	EffectInDoubt     EffectState = "IN_DOUBT"
	EffectCompensated EffectState = "COMPENSATED"
)

func (s EffectState) Valid() bool {
	switch s {
	case EffectPrepared, EffectDispatched, EffectConfirmed, EffectFailed, EffectInDoubt, EffectCompensated:
		return true
	default:
		return false
	}
}

func (s EffectState) Terminal() bool {
	return s == EffectConfirmed || s == EffectFailed || s == EffectCompensated
}

type EffectIntent struct {
	ID                  EffectIntentID `json:"id"`
	WorkflowRunID       WorkflowRunID  `json:"workflowRunId"`
	NodeRunID           NodeRunID      `json:"nodeRunId"`
	OriginAttemptID     AttemptID      `json:"originAttemptId"`
	LastAttemptID       AttemptID      `json:"lastAttemptId"`
	OperationNamespace  string         `json:"operationNamespace"`
	Operation           string         `json:"operation"`
	Class               EffectClass    `json:"class"`
	IdempotencyKey      string         `json:"idempotencyKey"`
	SemanticInputDigest string         `json:"semanticInputDigest"`
	State               EffectState    `json:"state"`
	PreparedAt          time.Time      `json:"preparedAt"`
	DispatchedAt        time.Time      `json:"dispatchedAt,omitempty"`
	ResolvedAt          time.Time      `json:"resolvedAt,omitempty"`
	ProviderRef         string         `json:"providerRef,omitempty"`
	ResultDigest        string         `json:"resultDigest,omitempty"`
	ErrorClass          string         `json:"errorClass,omitempty"`
	ErrorMessage        string         `json:"errorMessage,omitempty"`
	ReconcileCount      int            `json:"reconcileCount,omitempty"`
	LastReconciledAt    time.Time      `json:"lastReconciledAt,omitempty"`
}

// BuildEffectIdentity creates a stable identity for one logical side effect.
// AttemptID is deliberately absent: retries of the same node operation and
// semantic input reuse the same idempotency key.
func BuildEffectIdentity(runID WorkflowRunID, nodeRunID NodeRunID, namespace, operation string, semanticInput []byte) (key, inputDigest string, err error) {
	namespace = strings.TrimSpace(namespace)
	operation = strings.TrimSpace(operation)
	if runID == "" || nodeRunID == "" || namespace == "" || operation == "" {
		return "", "", fmt.Errorf("workflow run id, node run id, operation namespace and operation are required")
	}
	inputSum := sha256.Sum256(semanticInput)
	inputDigest = "sha256:" + hex.EncodeToString(inputSum[:])

	h := sha256.New()
	writeEffectKeyPart(h, "harness-effect-key-v1")
	writeEffectKeyPart(h, string(runID))
	writeEffectKeyPart(h, string(nodeRunID))
	writeEffectKeyPart(h, namespace)
	writeEffectKeyPart(h, operation)
	writeEffectKeyPart(h, inputDigest)
	return "effk_v1_" + hex.EncodeToString(h.Sum(nil)), inputDigest, nil
}

type effectKeyWriter interface{ Write([]byte) (int, error) }

func writeEffectKeyPart(w effectKeyWriter, value string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = w.Write(size[:])
	_, _ = w.Write([]byte(value))
}
