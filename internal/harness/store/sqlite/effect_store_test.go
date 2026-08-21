package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func seedEffectAttempt(t *testing.T, db *DB, now time.Time) (harnessmodel.NodeRun, harnessmodel.Attempt) {
	t.Helper()
	seedRun(t, db, now)
	node := harnessmodel.NodeRun{ID: "nr_effect", WorkflowRunID: "wfr_test", NodeID: "a", GraphRevision: 1, Generation: 1, State: harnessmodel.NodeRunning, CreatedAt: now, UpdatedAt: now}
	var attempt harnessmodel.Attempt
	if err := db.Update(context.Background(), func(tx harnessstore.Tx) error {
		if err := tx.CreateGraphRevision(context.Background(), harnessmodel.GraphRevision{WorkflowRunID: "wfr_test", Number: 1, CreatedAt: now, Reason: "effect fixture"}); err != nil { return err }
		if err := tx.CreateWorkflowProgress(context.Background(), harnessmodel.WorkflowProgress{WorkflowRunID: "wfr_test", TotalNodes: 1, UpdatedAt: now}); err != nil { return err }
		if err := tx.CreateNodeRun(context.Background(), node); err != nil { return err }
		created, err := tx.CreateNextAttempt(context.Background(), harnessmodel.Attempt{ID: "att_effect_1", NodeRunID: node.ID, State: harnessmodel.AttemptCreated, CreatedAt: now})
		if err != nil { return err }
		created.State = harnessmodel.AttemptRunning
		created.StartedAt = now
		if err := tx.CompareAndSwapAttempt(context.Background(), harnessmodel.AttemptCreated, created); err != nil { return err }
		attempt = created
		return nil
	}); err != nil { t.Fatal(err) }
	return node, attempt
}

func newEffectIntent(t *testing.T, node harnessmodel.NodeRun, attempt harnessmodel.Attempt, id harnessmodel.EffectIntentID, at time.Time) harnessmodel.EffectIntent {
	t.Helper()
	key, digest, err := harnessmodel.BuildEffectIdentity(node.WorkflowRunID, node.ID, "github", "create_issue", []byte(`{"title":"stable"}`))
	if err != nil { t.Fatal(err) }
	return harnessmodel.EffectIntent{
		ID: id, WorkflowRunID: node.WorkflowRunID, NodeRunID: node.ID,
		OriginAttemptID: attempt.ID, LastAttemptID: attempt.ID,
		OperationNamespace: "github", Operation: "create_issue", Class: harnessmodel.EffectIdempotentWithKey,
		IdempotencyKey: key, SemanticInputDigest: digest, State: harnessmodel.EffectPrepared, PreparedAt: at,
	}
}

func TestEffectIntentStableKeyDeduplicatesAcrossAttempts(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(130_000, 0).UTC()
	node, attempt1 := seedEffectAttempt(t, db, now)
	intent := newEffectIntent(t, node, attempt1, "eff_one", now)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		stored, created, err := tx.PutEffectIntent(ctx, intent)
		if err != nil { return err }
		if !created || stored.ID != intent.ID { t.Fatalf("first effect not created: created=%v stored=%+v", created, stored) }
		return nil
	}); err != nil { t.Fatal(err) }

	var attempt2 harnessmodel.Attempt
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		created, err := tx.CreateNextAttempt(ctx, harnessmodel.Attempt{ID: "att_effect_2", NodeRunID: node.ID, State: harnessmodel.AttemptCreated, CreatedAt: now.Add(time.Minute)})
		if err != nil { return err }
		created.State = harnessmodel.AttemptRunning
		created.StartedAt = now.Add(time.Minute)
		if err := tx.CompareAndSwapAttempt(ctx, harnessmodel.AttemptCreated, created); err != nil { return err }
		attempt2 = created
		retryIntent := newEffectIntent(t, node, attempt2, "eff_retry_should_not_win", now.Add(time.Minute))
		stored, createdEffect, err := tx.PutEffectIntent(ctx, retryIntent)
		if err != nil { return err }
		if createdEffect || stored.ID != intent.ID || stored.OriginAttemptID != attempt1.ID || stored.LastAttemptID != attempt2.ID {
			t.Fatalf("retry did not reuse effect intent: created=%v stored=%+v", createdEffect, stored)
		}
		return nil
	}); err != nil { t.Fatal(err) }

	var bindings int
	if err := db.SQLDB().QueryRow(`SELECT COUNT(*) FROM effect_attempt_bindings WHERE effect_intent_id=?`, string(intent.ID)).Scan(&bindings); err != nil { t.Fatal(err) }
	if bindings != 2 { t.Fatalf("effect attempt bindings=%d want=2", bindings) }
}

func TestEffectIntentRejectsKeyReplayWithDifferentSemantics(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(131_000, 0).UTC()
	node, attempt := seedEffectAttempt(t, db, now)
	intent := newEffectIntent(t, node, attempt, "eff_semantics", now)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { _, _, err := tx.PutEffectIntent(ctx, intent); return err }); err != nil { t.Fatal(err) }
	bad := intent
	bad.ID = "eff_bad"
	bad.Operation = "delete_issue"
	if err := db.Update(ctx, func(tx harnessstore.Tx) error { _, _, err := tx.PutEffectIntent(ctx, bad); return err }); !errors.Is(err, harnessstore.ErrConflict) {
		t.Fatalf("semantic key replay error=%v want ErrConflict", err)
	}
}

func TestEffectIntentTransitionAndReconciliationCounter(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(132_000, 0).UTC()
	node, attempt := seedEffectAttempt(t, db, now)
	intent := newEffectIntent(t, node, attempt, "eff_transition", now)
	if err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if _, _, err := tx.PutEffectIntent(ctx, intent); err != nil { return err }
		intent.State = harnessmodel.EffectDispatched
		intent.DispatchedAt = now.Add(time.Second)
		if err := tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectPrepared, intent); err != nil { return err }
		reconciled, err := tx.RecordEffectReconciliation(ctx, intent.ID, harnessmodel.EffectDispatched, now.Add(2*time.Second))
		if err != nil { return err }
		if reconciled.ReconcileCount != 1 { t.Fatalf("reconcile count=%d want=1", reconciled.ReconcileCount) }
		intent = reconciled
		intent.State = harnessmodel.EffectInDoubt
		intent.ErrorClass = "UNKNOWN_EFFECT"
		intent.ErrorMessage = "connection lost after dispatch"
		if err := tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectDispatched, intent); err != nil { return err }
		intent.State = harnessmodel.EffectConfirmed
		intent.ProviderRef = "issue:42"
		intent.ResultDigest = "sha256:result"
		intent.ResolvedAt = now.Add(3*time.Second)
		return tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectInDoubt, intent)
	}); err != nil { t.Fatal(err) }
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		got, err := r.GetEffectIntent(ctx, intent.ID)
		if err != nil { return err }
		if got.State != harnessmodel.EffectConfirmed || got.ReconcileCount != 1 || got.ProviderRef != "issue:42" { t.Fatalf("unexpected confirmed effect: %+v", got) }
		return nil
	}); err != nil { t.Fatal(err) }
}

func TestUncertainEffectSurvivesDatabaseReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	now := time.Unix(133_000, 0).UTC()
	first, err := Open(ctx, path, Options{})
	if err != nil { t.Fatal(err) }
	node, attempt := seedEffectAttempt(t, first, now)
	intent := newEffectIntent(t, node, attempt, "eff_reopen", now)
	if err := first.Update(ctx, func(tx harnessstore.Tx) error {
		if _, _, err := tx.PutEffectIntent(ctx, intent); err != nil { return err }
		intent.State = harnessmodel.EffectDispatched
		intent.DispatchedAt = now.Add(time.Second)
		return tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectPrepared, intent)
	}); err != nil { t.Fatal(err) }
	if err := first.Close(); err != nil { t.Fatal(err) }
	second, err := Open(ctx, path, Options{})
	if err != nil { t.Fatal(err) }
	defer second.Close()
	if err := second.View(ctx, func(r harnessstore.Reader) error {
		uncertain, err := r.ListUncertainEffects(ctx, node.WorkflowRunID, 10)
		if err != nil { return err }
		if len(uncertain) != 1 || uncertain[0].ID != intent.ID || uncertain[0].State != harnessmodel.EffectDispatched { t.Fatalf("uncertain effect lost on reopen: %+v", uncertain) }
		return nil
	}); err != nil { t.Fatal(err) }
}
