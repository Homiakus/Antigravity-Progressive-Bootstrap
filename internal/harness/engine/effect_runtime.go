package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

const (
	effectUnknownClass = "UNKNOWN_EFFECT"
	effectAbsentClass  = "EFFECT_ABSENT"
)

type EffectPrepareResult struct {
	Intent            harnessmodel.EffectIntent `json:"intent"`
	Created           bool                      `json:"created"`
	DispatchAllowed   bool                      `json:"dispatchAllowed"`
	AlreadyConfirmed  bool                      `json:"alreadyConfirmed,omitempty"`
	RequiresReconcile bool                      `json:"requiresReconcile,omitempty"`
}

type EffectReconcileDecision struct {
	Intent         harnessmodel.EffectIntent              `json:"intent"`
	ProviderResult harnessexecutor.EffectReconcileResult `json:"providerResult"`
	NodeRun        harnessmodel.NodeRun                   `json:"nodeRun,omitempty"`
	WorkflowRun    harnessmodel.WorkflowRun               `json:"workflowRun,omitempty"`
	RetrySafe      bool                                   `json:"retrySafe,omitempty"`
	RequiresManual bool                                   `json:"requiresManual,omitempty"`
	Idempotent     bool                                   `json:"idempotent,omitempty"`
}

func (e *Engine) PrepareEffect(ctx context.Context, attemptID harnessmodel.AttemptID, namespace, operation string, class harnessmodel.EffectClass, semanticInput []byte) (EffectPrepareResult, error) {
	return e.prepareEffect(ctx, attemptID, namespace, operation, class, semanticInput, nil)
}

func (e *Engine) PrepareEffectFenced(ctx context.Context, attemptID harnessmodel.AttemptID, workerID harnessmodel.WorkerID, epoch uint64, namespace, operation string, class harnessmodel.EffectClass, semanticInput []byte) (EffectPrepareResult, error) {
	if workerID == "" || epoch == 0 {
		return EffectPrepareResult{}, fmt.Errorf("worker id and lease epoch are required")
	}
	return e.prepareEffect(ctx, attemptID, namespace, operation, class, semanticInput, &completionFence{WorkerID: workerID, Epoch: epoch})
}

func (e *Engine) prepareEffect(ctx context.Context, attemptID harnessmodel.AttemptID, namespace, operation string, class harnessmodel.EffectClass, semanticInput []byte, fence *completionFence) (EffectPrepareResult, error) {
	if attemptID == "" || !class.Valid() || strings.TrimSpace(namespace) == "" || strings.TrimSpace(operation) == "" {
		return EffectPrepareResult{}, fmt.Errorf("attempt, namespace, operation and valid effect class are required")
	}
	now := e.now().UTC()
	rawEffectID, err := e.nextID(harnessmodel.IDEffectIntent)
	if err != nil {
		return EffectPrepareResult{}, err
	}
	var result EffectPrepareResult
	err = e.store.Update(ctx, func(tx harnessstore.Tx) error {
		attempt, nr, run, err := e.effectAttemptContext(ctx, tx, attemptID, fence, now)
		if err != nil {
			return err
		}
		key, inputDigest, err := harnessmodel.BuildEffectIdentity(run.ID, nr.ID, strings.TrimSpace(namespace), strings.TrimSpace(operation), semanticInput)
		if err != nil {
			return err
		}
		candidate := harnessmodel.EffectIntent{
			ID: harnessmodel.EffectIntentID(rawEffectID), WorkflowRunID: run.ID, NodeRunID: nr.ID,
			OriginAttemptID: attempt.ID, LastAttemptID: attempt.ID,
			OperationNamespace: strings.TrimSpace(namespace), Operation: strings.TrimSpace(operation), Class: class,
			IdempotencyKey: key, SemanticInputDigest: inputDigest, State: harnessmodel.EffectPrepared, PreparedAt: now,
		}
		stored, created, err := tx.PutEffectIntent(ctx, candidate)
		if err != nil {
			return err
		}
		result = EffectPrepareResult{Intent: stored, Created: created}
		switch stored.State {
		case harnessmodel.EffectPrepared:
			result.DispatchAllowed = true
		case harnessmodel.EffectConfirmed:
			result.AlreadyConfirmed = true
		case harnessmodel.EffectDispatched:
			result.DispatchAllowed = stored.Class.BlindRetrySafe()
			result.RequiresReconcile = !result.DispatchAllowed
		case harnessmodel.EffectInDoubt:
			result.RequiresReconcile = true
		case harnessmodel.EffectFailed:
			result.DispatchAllowed = false
		default:
			return fmt.Errorf("effect %s is in non-dispatchable state %s", stored.ID, stored.State)
		}
		if created {
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "EffectPrepared", "effect_intent", string(stored.ID), map[string]any{
				"nodeRunId": nr.ID, "attemptId": attempt.ID, "namespace": stored.OperationNamespace,
				"operation": stored.Operation, "class": stored.Class, "idempotencyKey": stored.IdempotencyKey,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}

func (e *Engine) MarkEffectDispatched(ctx context.Context, effectID harnessmodel.EffectIntentID, attemptID harnessmodel.AttemptID) (harnessmodel.EffectIntent, error) {
	return e.markEffectDispatched(ctx, effectID, attemptID, nil)
}

func (e *Engine) MarkEffectDispatchedFenced(ctx context.Context, effectID harnessmodel.EffectIntentID, attemptID harnessmodel.AttemptID, workerID harnessmodel.WorkerID, epoch uint64) (harnessmodel.EffectIntent, error) {
	if workerID == "" || epoch == 0 {
		return harnessmodel.EffectIntent{}, fmt.Errorf("worker id and lease epoch are required")
	}
	return e.markEffectDispatched(ctx, effectID, attemptID, &completionFence{WorkerID: workerID, Epoch: epoch})
}

func (e *Engine) markEffectDispatched(ctx context.Context, effectID harnessmodel.EffectIntentID, attemptID harnessmodel.AttemptID, fence *completionFence) (harnessmodel.EffectIntent, error) {
	if effectID == "" || attemptID == "" {
		return harnessmodel.EffectIntent{}, fmt.Errorf("effect intent id and attempt id are required")
	}
	now := e.now().UTC()
	var result harnessmodel.EffectIntent
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		intent, err := tx.GetEffectIntent(ctx, effectID)
		if err != nil { return err }
		if intent.LastAttemptID != attemptID { return fmt.Errorf("effect %s is bound to attempt %s, not %s: %w", intent.ID, intent.LastAttemptID, attemptID, harnessstore.ErrConflict) }
		if intent.State == harnessmodel.EffectDispatched {
			result = intent
			return nil
		}
		attempt, _, run, err := e.effectAttemptContext(ctx, tx, attemptID, fence, now)
		if err != nil { return err }
		if intent.State != harnessmodel.EffectPrepared {
			return fmt.Errorf("cannot dispatch effect %s from state %s", intent.ID, intent.State)
		}
		intent.State = harnessmodel.EffectDispatched
		intent.DispatchedAt = now
		if err := tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectPrepared, intent); err != nil { return err }
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "EffectDispatched", "effect_intent", string(intent.ID), map[string]any{"attemptId": attempt.ID, "idempotencyKey": intent.IdempotencyKey}); err != nil { return err }
		result = intent
		return nil
	})
	return result, err
}

func (e *Engine) ConfirmEffect(ctx context.Context, effectID harnessmodel.EffectIntentID, attemptID harnessmodel.AttemptID, providerRef, resultDigest string) (harnessmodel.EffectIntent, error) {
	return e.confirmEffect(ctx, effectID, attemptID, providerRef, resultDigest, nil)
}

func (e *Engine) ConfirmEffectFenced(ctx context.Context, effectID harnessmodel.EffectIntentID, attemptID harnessmodel.AttemptID, workerID harnessmodel.WorkerID, epoch uint64, providerRef, resultDigest string) (harnessmodel.EffectIntent, error) {
	if workerID == "" || epoch == 0 { return harnessmodel.EffectIntent{}, fmt.Errorf("worker id and lease epoch are required") }
	return e.confirmEffect(ctx, effectID, attemptID, providerRef, resultDigest, &completionFence{WorkerID: workerID, Epoch: epoch})
}

func (e *Engine) confirmEffect(ctx context.Context, effectID harnessmodel.EffectIntentID, attemptID harnessmodel.AttemptID, providerRef, resultDigest string, fence *completionFence) (harnessmodel.EffectIntent, error) {
	now := e.now().UTC()
	var result harnessmodel.EffectIntent
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		intent, err := tx.GetEffectIntent(ctx, effectID)
		if err != nil { return err }
		if intent.State == harnessmodel.EffectConfirmed {
			result = intent
			return nil
		}
		if intent.LastAttemptID != attemptID || intent.State != harnessmodel.EffectDispatched {
			return fmt.Errorf("effect %s cannot confirm from state %s / attempt %s: %w", intent.ID, intent.State, attemptID, harnessstore.ErrConflict)
		}
		_, _, run, err := e.effectAttemptContext(ctx, tx, attemptID, fence, now)
		if err != nil { return err }
		intent.State = harnessmodel.EffectConfirmed
		intent.ResolvedAt = now
		intent.ProviderRef = providerRef
		intent.ResultDigest = resultDigest
		intent.ErrorClass = ""
		intent.ErrorMessage = ""
		if err := tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectDispatched, intent); err != nil { return err }
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "EffectConfirmed", "effect_intent", string(intent.ID), map[string]any{"attemptId": attemptID, "providerRef": providerRef, "resultDigest": resultDigest}); err != nil { return err }
		result = intent
		return nil
	})
	return result, err
}

func (e *Engine) FailEffect(ctx context.Context, effectID harnessmodel.EffectIntentID, attemptID harnessmodel.AttemptID, errorClass, errorMessage string) (harnessmodel.EffectIntent, error) {
	return e.failEffect(ctx, effectID, attemptID, errorClass, errorMessage, nil)
}

func (e *Engine) FailEffectFenced(ctx context.Context, effectID harnessmodel.EffectIntentID, attemptID harnessmodel.AttemptID, workerID harnessmodel.WorkerID, epoch uint64, errorClass, errorMessage string) (harnessmodel.EffectIntent, error) {
	if workerID == "" || epoch == 0 { return harnessmodel.EffectIntent{}, fmt.Errorf("worker id and lease epoch are required") }
	return e.failEffect(ctx, effectID, attemptID, errorClass, errorMessage, &completionFence{WorkerID: workerID, Epoch: epoch})
}

func (e *Engine) failEffect(ctx context.Context, effectID harnessmodel.EffectIntentID, attemptID harnessmodel.AttemptID, errorClass, errorMessage string, fence *completionFence) (harnessmodel.EffectIntent, error) {
	if strings.TrimSpace(errorClass) == "" { return harnessmodel.EffectIntent{}, fmt.Errorf("effect error class is required") }
	now := e.now().UTC()
	var result harnessmodel.EffectIntent
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		intent, err := tx.GetEffectIntent(ctx, effectID)
		if err != nil { return err }
		if intent.State == harnessmodel.EffectFailed { result = intent; return nil }
		if intent.LastAttemptID != attemptID || intent.State != harnessmodel.EffectDispatched { return fmt.Errorf("effect %s cannot fail from state %s: %w", intent.ID, intent.State, harnessstore.ErrConflict) }
		_, _, run, err := e.effectAttemptContext(ctx, tx, attemptID, fence, now)
		if err != nil { return err }
		intent.State = harnessmodel.EffectFailed
		intent.ResolvedAt = now
		intent.ErrorClass = strings.TrimSpace(errorClass)
		intent.ErrorMessage = errorMessage
		if err := tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectDispatched, intent); err != nil { return err }
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "EffectFailed", "effect_intent", string(intent.ID), map[string]any{"attemptId": attemptID, "errorClass": intent.ErrorClass, "error": errorMessage}); err != nil { return err }
		result = intent
		return nil
	})
	return result, err
}

func (e *Engine) MarkEffectInDoubt(ctx context.Context, effectID harnessmodel.EffectIntentID, attemptID harnessmodel.AttemptID, detail string) (harnessmodel.EffectIntent, error) {
	return e.markEffectInDoubt(ctx, effectID, attemptID, detail, nil, harnessmodel.LeaseReleased)
}

func (e *Engine) MarkEffectInDoubtFenced(ctx context.Context, effectID harnessmodel.EffectIntentID, attemptID harnessmodel.AttemptID, workerID harnessmodel.WorkerID, epoch uint64, detail string) (harnessmodel.EffectIntent, error) {
	if workerID == "" || epoch == 0 { return harnessmodel.EffectIntent{}, fmt.Errorf("worker id and lease epoch are required") }
	return e.markEffectInDoubt(ctx, effectID, attemptID, detail, &completionFence{WorkerID: workerID, Epoch: epoch}, harnessmodel.LeaseReleased)
}

func (e *Engine) markEffectInDoubt(ctx context.Context, effectID harnessmodel.EffectIntentID, attemptID harnessmodel.AttemptID, detail string, fence *completionFence, closeState harnessmodel.LeaseState) (harnessmodel.EffectIntent, error) {
	if effectID == "" || attemptID == "" { return harnessmodel.EffectIntent{}, fmt.Errorf("effect intent id and attempt id are required") }
	now := e.now().UTC()
	var result harnessmodel.EffectIntent
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		intent, err := tx.GetEffectIntent(ctx, effectID)
		if err != nil { return err }
		attempt, err := tx.GetAttempt(ctx, attemptID)
		if err != nil { return err }
		nr, err := tx.GetNodeRun(ctx, attempt.NodeRunID)
		if err != nil { return err }
		run, err := tx.GetWorkflowRun(ctx, nr.WorkflowRunID)
		if err != nil { return err }
		if intent.State == harnessmodel.EffectInDoubt && attempt.State == harnessmodel.AttemptInDoubt && nr.State == harnessmodel.NodeInDoubt { result = intent; return nil }
		if intent.LastAttemptID != attemptID || intent.State != harnessmodel.EffectDispatched || attempt.State != harnessmodel.AttemptRunning || nr.State != harnessmodel.NodeRunning {
			return fmt.Errorf("cannot mark effect %s in doubt from effect=%s attempt=%s node=%s: %w", intent.ID, intent.State, attempt.State, nr.State, harnessstore.ErrConflict)
		}
		if err := authorizeCompletion(ctx, tx, attempt, fence, now); err != nil { return err }
		intent.State = harnessmodel.EffectInDoubt
		intent.ErrorClass = effectUnknownClass
		intent.ErrorMessage = detail
		if err := tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectDispatched, intent); err != nil { return err }
		attempt.ErrorClass = effectUnknownClass
		attempt.ErrorMessage = detail
		attempt.FinishedAt = now
		if err := transitionAttempt(ctx, tx, &attempt, harnessmodel.AttemptInDoubt); err != nil { return err }
		if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeInDoubt, now); err != nil { return err }
		if fence != nil {
			if err := tx.CloseLease(ctx, attempt.ID, fence.WorkerID, fence.Epoch, closeState, now); err != nil { return err }
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "EffectInDoubt", "effect_intent", string(intent.ID), map[string]any{"attemptId": attempt.ID, "detail": detail}); err != nil { return err }
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "AttemptInDoubt", "attempt", string(attempt.ID), map[string]any{"nodeRunId": nr.ID, "effectIntentId": intent.ID}); err != nil { return err }
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "NodeInDoubt", "node_run", string(nr.ID), map[string]any{"nodeId": nr.NodeID, "effectIntentId": intent.ID}); err != nil { return err }
		if _, err := e.finalizePauseIfDrained(ctx, tx, &run, now); err != nil { return err }
		result = intent
		return nil
	})
	return result, err
}

// ReconcileEffect asks a provider adapter for evidence about an already
// IN_DOUBT effect. It never blindly repeats the external operation.
func (e *Engine) ReconcileEffect(ctx context.Context, effectID harnessmodel.EffectIntentID, reconciler harnessexecutor.EffectReconciler) (EffectReconcileDecision, error) {
	if effectID == "" || reconciler == nil { return EffectReconcileDecision{}, fmt.Errorf("effect id and reconciler are required") }
	now := e.now().UTC()
	var snapshot harnessmodel.EffectIntent
	if err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		intent, err := tx.GetEffectIntent(ctx, effectID)
		if err != nil { return err }
		if intent.State == harnessmodel.EffectConfirmed || intent.State == harnessmodel.EffectFailed {
			snapshot = intent
			return nil
		}
		if intent.State != harnessmodel.EffectInDoubt { return fmt.Errorf("effect %s must be IN_DOUBT before reconciliation, got %s", intent.ID, intent.State) }
		snapshot, err = tx.RecordEffectReconciliation(ctx, intent.ID, harnessmodel.EffectInDoubt, now)
		return err
	}); err != nil { return EffectReconcileDecision{}, err }
	if snapshot.State == harnessmodel.EffectConfirmed || snapshot.State == harnessmodel.EffectFailed {
		return EffectReconcileDecision{Intent: snapshot, Idempotent: true, RetrySafe: snapshot.State == harnessmodel.EffectFailed && snapshot.Class.BlindRetrySafe()}, nil
	}

	request := harnessexecutor.EffectReconcileRequest{
		EffectIntentID: snapshot.ID, WorkflowRunID: snapshot.WorkflowRunID, NodeRunID: snapshot.NodeRunID,
		OperationNamespace: snapshot.OperationNamespace, Operation: snapshot.Operation, Class: snapshot.Class,
		IdempotencyKey: snapshot.IdempotencyKey, SemanticInputDigest: snapshot.SemanticInputDigest, ProviderRef: snapshot.ProviderRef,
	}
	if err := request.Validate(); err != nil { return EffectReconcileDecision{}, err }
	providerResult, err := reconciler.ReconcileEffect(ctx, request)
	if err != nil { return EffectReconcileDecision{}, fmt.Errorf("reconcile effect %s: %w", snapshot.ID, err) }
	if err := providerResult.Validate(); err != nil { return EffectReconcileDecision{}, err }

	decision := EffectReconcileDecision{ProviderResult: providerResult}
	err = e.store.Update(ctx, func(tx harnessstore.Tx) error {
		intent, err := tx.GetEffectIntent(ctx, effectID)
		if err != nil { return err }
		if intent.State != harnessmodel.EffectInDoubt {
			decision.Intent = intent
			decision.Idempotent = true
			return nil
		}
		nr, err := tx.GetNodeRun(ctx, intent.NodeRunID)
		if err != nil { return err }
		run, err := tx.GetWorkflowRun(ctx, intent.WorkflowRunID)
		if err != nil { return err }
		switch providerResult.Status {
		case harnessexecutor.EffectReconcileConfirmed:
			intent.State = harnessmodel.EffectConfirmed
			intent.ResolvedAt = now
			intent.ProviderRef = providerResult.ProviderRef
			intent.ResultDigest = providerResult.ResultDigest
			intent.ErrorClass, intent.ErrorMessage = "", ""
			if err := tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectInDoubt, intent); err != nil { return err }
			if nr.State == harnessmodel.NodeInDoubt {
				if err := e.resolveReconciledNodeSuccess(ctx, tx, &run, &nr, now, intent.ID); err != nil { return err }
			}
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "EffectReconciledConfirmed", "effect_intent", string(intent.ID), map[string]any{"providerRef": providerResult.ProviderRef, "resultDigest": providerResult.ResultDigest}); err != nil { return err }
		case harnessexecutor.EffectReconcileAbsent:
			intent.State = harnessmodel.EffectFailed
			intent.ResolvedAt = now
			intent.ErrorClass = effectAbsentClass
			intent.ErrorMessage = "provider reconciliation proved the effect is absent"
			if err := tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectInDoubt, intent); err != nil { return err }
			decision.RetrySafe = true
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "EffectReconciledAbsent", "effect_intent", string(intent.ID), nil); err != nil { return err }
		case harnessexecutor.EffectReconcileFailed:
			intent.State = harnessmodel.EffectFailed
			intent.ResolvedAt = now
			intent.ErrorClass = providerResult.ErrorClass
			intent.ErrorMessage = providerResult.ErrorMessage
			if err := tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectInDoubt, intent); err != nil { return err }
			decision.RetrySafe = intent.Class.BlindRetrySafe()
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "EffectReconciledFailed", "effect_intent", string(intent.ID), map[string]any{"errorClass": intent.ErrorClass, "error": intent.ErrorMessage}); err != nil { return err }
		case harnessexecutor.EffectReconcileUnknown:
			decision.RetrySafe = intent.Class.BlindRetrySafe()
			decision.RequiresManual = !decision.RetrySafe
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "EffectReconciliationUnknown", "effect_intent", string(intent.ID), map[string]any{"retrySafe": decision.RetrySafe}); err != nil { return err }
		default:
			return fmt.Errorf("unsupported reconciliation status %s", providerResult.Status)
		}
		decision.Intent = intent
		decision.NodeRun = nr
		decision.WorkflowRun = run
		return nil
	})
	return decision, err
}

func (e *Engine) effectAttemptContext(ctx context.Context, tx harnessstore.Tx, attemptID harnessmodel.AttemptID, fence *completionFence, now time.Time) (harnessmodel.Attempt, harnessmodel.NodeRun, harnessmodel.WorkflowRun, error) {
	attempt, err := tx.GetAttempt(ctx, attemptID)
	if err != nil { return harnessmodel.Attempt{}, harnessmodel.NodeRun{}, harnessmodel.WorkflowRun{}, err }
	if err := authorizeCompletion(ctx, tx, attempt, fence, now); err != nil { return harnessmodel.Attempt{}, harnessmodel.NodeRun{}, harnessmodel.WorkflowRun{}, err }
	if attempt.State != harnessmodel.AttemptRunning { return harnessmodel.Attempt{}, harnessmodel.NodeRun{}, harnessmodel.WorkflowRun{}, fmt.Errorf("effect operation requires RUNNING attempt, got %s", attempt.State) }
	nr, err := tx.GetNodeRun(ctx, attempt.NodeRunID)
	if err != nil { return harnessmodel.Attempt{}, harnessmodel.NodeRun{}, harnessmodel.WorkflowRun{}, err }
	if nr.State != harnessmodel.NodeRunning { return harnessmodel.Attempt{}, harnessmodel.NodeRun{}, harnessmodel.WorkflowRun{}, fmt.Errorf("effect operation requires RUNNING node, got %s", nr.State) }
	run, err := tx.GetWorkflowRun(ctx, nr.WorkflowRunID)
	if err != nil { return harnessmodel.Attempt{}, harnessmodel.NodeRun{}, harnessmodel.WorkflowRun{}, err }
	if run.State != harnessmodel.WorkflowRunning && run.State != harnessmodel.WorkflowPausing { return harnessmodel.Attempt{}, harnessmodel.NodeRun{}, harnessmodel.WorkflowRun{}, fmt.Errorf("effect operation cannot run while workflow %s is %s", run.ID, run.State) }
	return attempt, nr, run, nil
}

func (e *Engine) resolveReconciledNodeSuccess(ctx context.Context, tx harnessstore.Tx, run *harnessmodel.WorkflowRun, nr *harnessmodel.NodeRun, now time.Time, effectID harnessmodel.EffectIntentID) error {
	if nr.State != harnessmodel.NodeInDoubt { return nil }
	if err := transitionNode(ctx, tx, nr, harnessmodel.NodeSucceeded, now); err != nil { return err }
	progress, err := tx.IncrementWorkflowProgress(ctx, run.ID, false, now)
	if err != nil { return err }
	if _, err := e.appendEvent(ctx, tx, run.ID, now, "NodeSucceeded", "node_run", string(nr.ID), map[string]any{"nodeId": nr.NodeID, "reconciledEffectId": effectID}); err != nil { return err }

	if run.State == harnessmodel.WorkflowRunning || run.State == harnessmodel.WorkflowPausing || run.State == harnessmodel.WorkflowPaused {
		dependents, err := tx.ListDependentNodeRuns(ctx, run.ID, nr.NodeID)
		if err != nil { return err }
		for _, child := range dependents {
			remaining, err := tx.DecrementNodeRemainingDependencies(ctx, child.ID, now)
			if err != nil { return err }
			if remaining != 0 { continue }
			child, err = tx.GetNodeRun(ctx, child.ID)
			if err != nil { return err }
			if err := transitionNode(ctx, tx, &child, harnessmodel.NodeReady, now); err != nil { return err }
			if err := tx.EnqueueReadyNode(ctx, child.ID, now, time.Time{}, ""); err != nil { return err }
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "NodeReady", "node_run", string(child.ID), map[string]any{"nodeId": child.NodeID, "remainingDependencies": 0, "reconciledParent": nr.ID}); err != nil { return err }
		}
	}
	if progress.TerminalNodes == progress.TotalNodes && progress.FailedNodes == 0 && (run.State == harnessmodel.WorkflowRunning || run.State == harnessmodel.WorkflowPausing || run.State == harnessmodel.WorkflowPaused) {
		if err := transitionWorkflow(ctx, tx, run, harnessmodel.WorkflowSucceeded, now); err != nil { return err }
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "WorkflowSucceeded", "workflow_run", string(run.ID), map[string]any{"terminalNodes": progress.TerminalNodes, "totalNodes": progress.TotalNodes, "reconciledEffectId": effectID}); err != nil { return err }
	}
	return nil
}

// ResolveReconciledRetry re-arms a NodeRun in IN_DOUBT state when provider
// reconciliation has proved the effect was ABSENT or is safe to retry.
// The new Attempt will reuse the same stable effect idempotency key.
func (e *Engine) ResolveReconciledRetry(ctx context.Context, effectID harnessmodel.EffectIntentID, delay time.Duration) (harnessmodel.NodeRun, error) {
	if effectID == "" {
		return harnessmodel.NodeRun{}, fmt.Errorf("effect id is required")
	}
	now := e.now().UTC()
	var result harnessmodel.NodeRun
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		intent, err := tx.GetEffectIntent(ctx, effectID)
		if err != nil {
			return err
		}
		if intent.State != harnessmodel.EffectFailed && intent.State != harnessmodel.EffectInDoubt {
			return fmt.Errorf("effect %s must be FAILED or IN_DOUBT to re-arm retry, got %s", intent.ID, intent.State)
		}
		if intent.State == harnessmodel.EffectFailed && intent.ErrorClass != effectAbsentClass && !intent.Class.BlindRetrySafe() {
			return fmt.Errorf("effect %s failed with non-retryable class %s", intent.ID, intent.ErrorClass)
		}
		nr, err := tx.GetNodeRun(ctx, intent.NodeRunID)
		if err != nil {
			return err
		}
		if nr.State != harnessmodel.NodeInDoubt {
			return fmt.Errorf("node %s is in state %s, not IN_DOUBT", nr.ID, nr.State)
		}
		run, err := tx.GetWorkflowRun(ctx, intent.WorkflowRunID)
		if err != nil {
			return err
		}
		if run.State != harnessmodel.WorkflowRunning && run.State != harnessmodel.WorkflowPausing && run.State != harnessmodel.WorkflowPaused {
			return fmt.Errorf("cannot retry node %s while workflow %s is %s", nr.ID, run.ID, run.State)
		}

		errClass := harnessmodel.ErrorClass(intent.ErrorClass)
		if !errClass.Valid() {
			errClass = harnessmodel.ErrorApplicationTransient
		}
		if delay > 0 {
			if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeRetryWait, now); err != nil {
				return err
			}
			sched := harnessmodel.RetrySchedule{
				NodeRunID:       nr.ID,
				WorkflowRunID:   run.ID,
				FailedAttemptID: intent.LastAttemptID,
				AttemptNumber:   1,
				FailureClass:    errClass,
				ScheduledAt:     now,
				NotBefore:       now.Add(delay),
			}
			if err := tx.CreateRetrySchedule(ctx, sched); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "NodeRetryScheduled", "node_run", string(nr.ID), map[string]any{
				"nodeId": nr.NodeID, "notBefore": sched.NotBefore, "reconciledEffectId": intent.ID,
			}); err != nil {
				return err
			}
		} else {
			if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeRetryWait, now); err != nil {
				return err
			}
			if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeReady, now); err != nil {
				return err
			}
			if err := tx.EnqueueReadyNode(ctx, nr.ID, now, time.Time{}, ""); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "NodeReady", "node_run", string(nr.ID), map[string]any{
				"nodeId": nr.NodeID, "reconciledEffectId": intent.ID,
			}); err != nil {
				return err
			}
		}
		result = nr
		return nil
	})
	return result, err
}

// ResolveReconciledFailure transitions an IN_DOUBT node to FAILED when reconciliation
// confirms failure or manual decision rejects execution.
func (e *Engine) ResolveReconciledFailure(ctx context.Context, effectID harnessmodel.EffectIntentID, errorClass, errorMessage string) (harnessmodel.NodeRun, error) {
	if effectID == "" || strings.TrimSpace(errorClass) == "" {
		return harnessmodel.NodeRun{}, fmt.Errorf("effect id and error class are required")
	}
	now := e.now().UTC()
	var result harnessmodel.NodeRun
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		intent, err := tx.GetEffectIntent(ctx, effectID)
		if err != nil {
			return err
		}
		nr, err := tx.GetNodeRun(ctx, intent.NodeRunID)
		if err != nil {
			return err
		}
		if nr.State != harnessmodel.NodeInDoubt {
			return fmt.Errorf("node %s is in state %s, not IN_DOUBT", nr.ID, nr.State)
		}
		run, err := tx.GetWorkflowRun(ctx, intent.WorkflowRunID)
		if err != nil {
			return err
		}
		if intent.State == harnessmodel.EffectInDoubt {
			intent.State = harnessmodel.EffectFailed
			intent.ResolvedAt = now
			intent.ErrorClass = strings.TrimSpace(errorClass)
			intent.ErrorMessage = errorMessage
			if err := tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectInDoubt, intent); err != nil {
				return err
			}
		}
		if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeFailed, now); err != nil {
			return err
		}
		progress, err := tx.IncrementWorkflowProgress(ctx, run.ID, true, now)
		if err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "NodeFailed", "node_run", string(nr.ID), map[string]any{
			"nodeId": nr.NodeID, "errorClass": errorClass, "error": errorMessage, "reconciledEffectId": intent.ID,
		}); err != nil {
			return err
		}
		if progress.TerminalNodes == progress.TotalNodes && (run.State == harnessmodel.WorkflowRunning || run.State == harnessmodel.WorkflowPausing || run.State == harnessmodel.WorkflowPaused) {
			if err := transitionWorkflow(ctx, tx, &run, harnessmodel.WorkflowFailed, now); err != nil {
				return err
			}
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "WorkflowFailed", "workflow_run", string(run.ID), map[string]any{
				"failedNodes": progress.FailedNodes, "totalNodes": progress.TotalNodes, "reconciledEffectId": intent.ID,
			}); err != nil {
				return err
			}
		}
		result = nr
		return nil
	})
	return result, err
}

var _ = errors.Is