package executor

import (
	"context"
	"fmt"
	"strings"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

type EffectReconcileStatus string

const (
	EffectReconcileConfirmed EffectReconcileStatus = "CONFIRMED"
	EffectReconcileAbsent    EffectReconcileStatus = "ABSENT"
	EffectReconcileFailed    EffectReconcileStatus = "FAILED"
	EffectReconcileUnknown   EffectReconcileStatus = "UNKNOWN"
)

func (s EffectReconcileStatus) Valid() bool {
	switch s {
	case EffectReconcileConfirmed, EffectReconcileAbsent, EffectReconcileFailed, EffectReconcileUnknown:
		return true
	default:
		return false
	}
}

type EffectReconcileRequest struct {
	EffectIntentID      harnessmodel.EffectIntentID `json:"effectIntentId"`
	WorkflowRunID       harnessmodel.WorkflowRunID  `json:"workflowRunId"`
	NodeRunID           harnessmodel.NodeRunID      `json:"nodeRunId"`
	OperationNamespace  string                      `json:"operationNamespace"`
	Operation           string                      `json:"operation"`
	Class               harnessmodel.EffectClass    `json:"class"`
	IdempotencyKey      string                      `json:"idempotencyKey"`
	SemanticInputDigest string                      `json:"semanticInputDigest"`
	ProviderRef         string                      `json:"providerRef,omitempty"`
}

func (r EffectReconcileRequest) Validate() error {
	if r.EffectIntentID == "" || r.WorkflowRunID == "" || r.NodeRunID == "" ||
		strings.TrimSpace(r.OperationNamespace) == "" || strings.TrimSpace(r.Operation) == "" ||
		!r.Class.Valid() || strings.TrimSpace(r.IdempotencyKey) == "" || strings.TrimSpace(r.SemanticInputDigest) == "" {
		return fmt.Errorf("invalid effect reconciliation request")
	}
	return nil
}

type EffectReconcileResult struct {
	Status       EffectReconcileStatus `json:"status"`
	ProviderRef  string                `json:"providerRef,omitempty"`
	ResultDigest string                `json:"resultDigest,omitempty"`
	ErrorClass   string                `json:"errorClass,omitempty"`
	ErrorMessage string                `json:"errorMessage,omitempty"`
}

func (r EffectReconcileResult) Validate() error {
	if !r.Status.Valid() {
		return fmt.Errorf("invalid effect reconciliation status %q", r.Status)
	}
	if r.Status == EffectReconcileFailed && strings.TrimSpace(r.ErrorClass) == "" {
		return fmt.Errorf("failed effect reconciliation requires error class")
	}
	return nil
}

// EffectReconciler is intentionally optional. Process execution, for example,
// cannot prove whether an arbitrary external API mutation happened. Provider
// adapters implement this only when they have a real query/idempotency contract.
type EffectReconciler interface {
	ReconcileEffect(context.Context, EffectReconcileRequest) (EffectReconcileResult, error)
}
