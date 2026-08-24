package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
	harnesslease "github.com/homiakus/agctl/internal/harness/lease"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type mockReconciler struct {
	result harnessexecutor.EffectReconcileResult
	err    error
}

func (m *mockReconciler) ReconcileEffect(ctx context.Context, req harnessexecutor.EffectReconcileRequest) (harnessexecutor.EffectReconcileResult, error) {
	return m.result, m.err
}

func TestEffectPrepareDispatchConfirmFenced(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-eff-1", clock.current)

	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("effect-lifecycle"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-eff-1", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, "worker-eff-1", 1)
	if err != nil {
		t.Fatal(err)
	}

	semanticInput := []byte(`{"repo":"test/repo","action":"deploy"}`)

	// Stale worker/epoch should be rejected
	if _, err := eng.PrepareEffectFenced(ctx, attempt.ID, "worker-stale", 1, "deployer", "deploy", harnessmodel.EffectQueryable, semanticInput); !errors.Is(err, harnesslease.ErrStaleFence) {
		t.Fatalf("expected ErrStaleFence for invalid worker, got %v", err)
	}
	if _, err := eng.PrepareEffectFenced(ctx, attempt.ID, "worker-eff-1", 99, "deployer", "deploy", harnessmodel.EffectQueryable, semanticInput); !errors.Is(err, harnesslease.ErrStaleFence) {
		t.Fatalf("expected ErrStaleFence for invalid epoch, got %v", err)
	}

	// Prepare effect with valid fence
	prep, err := eng.PrepareEffectFenced(ctx, attempt.ID, "worker-eff-1", 1, "deployer", "deploy", harnessmodel.EffectQueryable, semanticInput)
	if err != nil {
		t.Fatal(err)
	}
	if !prep.Created || !prep.DispatchAllowed || prep.Intent.State != harnessmodel.EffectPrepared {
		t.Fatalf("unexpected prepare result: %+v", prep)
	}

	// Mark dispatched
	dispatched, err := eng.MarkEffectDispatchedFenced(ctx, prep.Intent.ID, attempt.ID, "worker-eff-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched.State != harnessmodel.EffectDispatched {
		t.Fatalf("expected DISPATCHED state, got %s", dispatched.State)
	}

	// Confirm effect
	confirmed, err := eng.ConfirmEffectFenced(ctx, prep.Intent.ID, attempt.ID, "worker-eff-1", 1, "deploy:12345", "sha256:deployok")
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.State != harnessmodel.EffectConfirmed || confirmed.ProviderRef != "deploy:12345" {
		t.Fatalf("unexpected confirmed effect: %+v", confirmed)
	}

	// Completing attempt afterwards
	comp, err := eng.CompleteAttemptSuccessFenced(ctx, attempt.ID, "worker-eff-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if comp.Attempt.State != harnessmodel.AttemptSucceeded || comp.WorkflowRun.State != harnessmodel.WorkflowSucceeded {
		t.Fatalf("unexpected workflow completion: %+v", comp)
	}
}

func TestLeaseReclaimWithDispatchedUncertainEffectEntersInDoubt(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-crashed", clock.current)
	addActiveWorker(t, ctx, db, "worker-recover", clock.current)

	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("effect-crash-window"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-crashed", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, "worker-crashed", 1)
	if err != nil {
		t.Fatal(err)
	}

	semanticInput := []byte(`{"charge_id":"ch_123"}`)
	prep, err := eng.PrepareEffectFenced(ctx, attempt.ID, "worker-crashed", 1, "payment", "charge", harnessmodel.EffectQueryable, semanticInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.MarkEffectDispatchedFenced(ctx, prep.Intent.ID, attempt.ID, "worker-crashed", 1); err != nil {
		t.Fatal(err)
	}

	// Advance clock past lease expiration (simulating worker crash / silent death)
	clock.current = claim.Lease.ExpiresAt.Add(time.Second)

	// ReclaimExpiredAttempt must NOT blindly pass this attempt to worker-recover!
	// It must detect the in-flight uncertain effect and move attempt + node + effect to IN_DOUBT.
	_, err = eng.ReclaimExpiredAttempt(ctx, attempt.ID, "worker-recover", 30*time.Second)
	if !errors.Is(err, harnesslease.ErrUncertainEffect) {
		t.Fatalf("expected ErrUncertainEffect on reclaiming attempt with in-doubt effect, got %v", err)
	}

	// Verify durable states
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		nr, err := r.GetNodeRun(ctx, node.ID)
		if err != nil {
			return err
		}
		if nr.State != harnessmodel.NodeInDoubt {
			t.Fatalf("expected node state IN_DOUBT, got %s", nr.State)
		}
		att, err := r.GetAttempt(ctx, attempt.ID)
		if err != nil {
			return err
		}
		if att.State != harnessmodel.AttemptInDoubt {
			t.Fatalf("expected attempt state IN_DOUBT, got %s", att.State)
		}
		eff, err := r.GetEffectIntent(ctx, prep.Intent.ID)
		if err != nil {
			return err
		}
		if eff.State != harnessmodel.EffectInDoubt {
			t.Fatalf("expected effect state IN_DOUBT, got %s", eff.State)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLeaseReclaimWithSafeIdempotentEffectReclaimsNormally(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-safe-1", clock.current)
	addActiveWorker(t, ctx, db, "worker-safe-2", clock.current)

	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("effect-safe-reclaim"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-safe-1", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, "worker-safe-1", 1)
	if err != nil {
		t.Fatal(err)
	}

	semanticInput := []byte(`{"read":"status"}`)
	prep, err := eng.PrepareEffectFenced(ctx, attempt.ID, "worker-safe-1", 1, "api", "get_status", harnessmodel.EffectIdempotentWithKey, semanticInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.MarkEffectDispatchedFenced(ctx, prep.Intent.ID, attempt.ID, "worker-safe-1", 1); err != nil {
		t.Fatal(err)
	}

	// Advance clock past expiration
	clock.current = claim.Lease.ExpiresAt.Add(time.Second)

	// Since EffectIdempotentWithKey is BlindRetrySafe(), reclaim succeeds
	newLease, err := eng.ReclaimExpiredAttempt(ctx, attempt.ID, "worker-safe-2", 30*time.Second)
	if err != nil {
		t.Fatalf("reclaim failed for blind-retry-safe effect: %v", err)
	}
	if newLease.Epoch != 2 || newLease.WorkerID != "worker-safe-2" {
		t.Fatalf("unexpected new lease: %+v", newLease)
	}
}

func TestReconcileInDoubtEffectConfirmedCompletesWorkflow(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-rec-1", clock.current)

	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("reconcile-confirmed"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-rec-1", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, "worker-rec-1", 1)
	if err != nil {
		t.Fatal(err)
	}

	semanticInput := []byte(`{"resource":"github_issue"}`)
	prep, err := eng.PrepareEffectFenced(ctx, attempt.ID, "worker-rec-1", 1, "github", "create_issue", harnessmodel.EffectQueryable, semanticInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.MarkEffectDispatchedFenced(ctx, prep.Intent.ID, attempt.ID, "worker-rec-1", 1); err != nil {
		t.Fatal(err)
	}

	// Simulate unknown disconnection / in doubt
	if _, err := eng.MarkEffectInDoubtFenced(ctx, prep.Intent.ID, attempt.ID, "worker-rec-1", 1, "network dropped before ack"); err != nil {
		t.Fatal(err)
	}

	// Provider reconciler confirms that the issue was indeed created on GitHub!
	reconciler := &mockReconciler{
		result: harnessexecutor.EffectReconcileResult{
			Status:       harnessexecutor.EffectReconcileConfirmed,
			ProviderRef:  "issue:101",
			ResultDigest: "sha256:issue101hash",
		},
	}

	decision, err := eng.ReconcileEffect(ctx, prep.Intent.ID, reconciler)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Intent.State != harnessmodel.EffectConfirmed || decision.ProviderResult.ProviderRef != "issue:101" {
		t.Fatalf("unexpected reconcile decision: %+v", decision)
	}

	// Verify node and workflow succeeded automatically without rerunning the action
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		nr, err := r.GetNodeRun(ctx, node.ID)
		if err != nil {
			return err
		}
		if nr.State != harnessmodel.NodeSucceeded {
			t.Fatalf("expected node state SUCCEEDED after reconcile, got %s", nr.State)
		}
		wfr, err := r.GetWorkflowRun(ctx, run.ID)
		if err != nil {
			return err
		}
		if wfr.State != harnessmodel.WorkflowSucceeded {
			t.Fatalf("expected workflow state SUCCEEDED after reconcile, got %s", wfr.State)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileInDoubtEffectAbsentEnablesControlledRetryWithSameKey(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-abs-1", clock.current)

	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("reconcile-absent-retry"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim1, err := eng.ClaimNode(ctx, node.ID, "worker-abs-1", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	attempt1, err := eng.StartClaimedAttempt(ctx, claim1.Attempt.ID, "worker-abs-1", 1)
	if err != nil {
		t.Fatal(err)
	}

	semanticInput := []byte(`{"resource":"db_mutation"}`)
	prep1, err := eng.PrepareEffectFenced(ctx, attempt1.ID, "worker-abs-1", 1, "db", "mutate", harnessmodel.EffectQueryable, semanticInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.MarkEffectDispatchedFenced(ctx, prep1.Intent.ID, attempt1.ID, "worker-abs-1", 1); err != nil {
		t.Fatal(err)
	}

	// Move to IN_DOUBT
	if _, err := eng.MarkEffectInDoubtFenced(ctx, prep1.Intent.ID, attempt1.ID, "worker-abs-1", 1, "timeout"); err != nil {
		t.Fatal(err)
	}

	// Reconciler checks DB and confirms the effect was ABSENT (never applied)
	reconciler := &mockReconciler{
		result: harnessexecutor.EffectReconcileResult{
			Status: harnessexecutor.EffectReconcileAbsent,
		},
	}

	decision, err := eng.ReconcileEffect(ctx, prep1.Intent.ID, reconciler)
	if err != nil {
		t.Fatal(err)
	}
	if !decision.RetrySafe || decision.Intent.State != harnessmodel.EffectFailed {
		t.Fatalf("expected RetrySafe decision, got %+v", decision)
	}

	// Re-arm node for controlled retry
	rearmedNode, err := eng.ResolveReconciledRetry(ctx, prep1.Intent.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rearmedNode.State != harnessmodel.NodeReady {
		t.Fatalf("expected node state READY, got %s", rearmedNode.State)
	}

	// Second attempt executes
	claim2, err := eng.ClaimNode(ctx, rearmedNode.ID, "worker-abs-1", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if claim2.Attempt.ID == attempt1.ID {
		t.Fatal("retry must create a new Attempt, not reuse old attempt ID")
	}
	attempt2, err := eng.StartClaimedAttempt(ctx, claim2.Attempt.ID, "worker-abs-1", 1)
	if err != nil {
		t.Fatal(err)
	}

	// PrepareEffect with the same semantic inputs must yield the SAME logical idempotency key
	prep2, err := eng.PrepareEffectFenced(ctx, attempt2.ID, "worker-abs-1", 1, "db", "mutate", harnessmodel.EffectQueryable, semanticInput)
	if err != nil {
		t.Fatal(err)
	}
	if prep2.Intent.IdempotencyKey != prep1.Intent.IdempotencyKey {
		t.Fatalf("retry did not reuse stable idempotency key: %s vs %s", prep2.Intent.IdempotencyKey, prep1.Intent.IdempotencyKey)
	}
	if prep2.Intent.ID != prep1.Intent.ID {
		t.Fatalf("retry did not reuse logical effect intent ID: %s vs %s", prep2.Intent.ID, prep1.Intent.ID)
	}
}

func TestResolveReconciledFailure(t *testing.T) {
	ctx := context.Background()
	eng, db, _, clock := newTestEngine(t)
	addActiveWorker(t, ctx, db, "worker-fail-1", clock.current)

	run, err := eng.StartWorkflow(ctx, oneNodeDefinition("reconcile-failure"))
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRunFor(t, db, run.ID, "a")
	claim, err := eng.ClaimNode(ctx, node.ID, "worker-fail-1", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := eng.StartClaimedAttempt(ctx, claim.Attempt.ID, "worker-fail-1", 1)
	if err != nil {
		t.Fatal(err)
	}

	prep, err := eng.PrepareEffectFenced(ctx, attempt.ID, "worker-fail-1", 1, "payment", "charge", harnessmodel.EffectNonIdempotentUnknown, []byte(`{"amt":100}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.MarkEffectDispatchedFenced(ctx, prep.Intent.ID, attempt.ID, "worker-fail-1", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.MarkEffectInDoubtFenced(ctx, prep.Intent.ID, attempt.ID, "worker-fail-1", 1, "lost connection"); err != nil {
		t.Fatal(err)
	}

	// Explicitly resolve failure
	failedNode, err := eng.ResolveReconciledFailure(ctx, prep.Intent.ID, "MANUAL_ABORT", "operator cancelled uncertain charge")
	if err != nil {
		t.Fatal(err)
	}
	if failedNode.State != harnessmodel.NodeFailed {
		t.Fatalf("expected node state FAILED, got %s", failedNode.State)
	}

	// Verify workflow failed
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		wfr, err := r.GetWorkflowRun(ctx, run.ID)
		if err != nil {
			return err
		}
		if wfr.State != harnessmodel.WorkflowFailed {
			t.Fatalf("expected workflow state FAILED, got %s", wfr.State)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
