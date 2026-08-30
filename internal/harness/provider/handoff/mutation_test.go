package handoff

import (
	"context"
	"errors"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestMutationsKillDefects(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	now := time.Unix(2000, 0).UTC()
	acc1, acc2 := seedTestData(t, db, now)

	planText := []byte("# MASTER PLAN\n\nOriginal plan content")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	mgr := NewManager(db, Options{
		Now: func() time.Time { return now },
	})

	t.Run("mutant: non-idempotent IN_DOUBT effect bypassed without reconciliation", func(t *testing.T) {
		key, dig, err := harnessmodel.BuildEffectIdentity("wfr_test", "nr_test", "aws", "launch_instance", []byte(`{"type":"t3.micro"}`))
		if err != nil {
			t.Fatal(err)
		}
		err = db.Update(ctx, func(tx harnessstore.Tx) error {
			eff := harnessmodel.EffectIntent{
				ID:                  "eff_mut_indoubt",
				WorkflowRunID:       "wfr_test",
				NodeRunID:           "nr_test",
				OriginAttemptID:     "att_test",
				LastAttemptID:       "att_test",
				OperationNamespace:  "aws",
				Operation:           "launch_instance",
				Class:               harnessmodel.EffectNonIdempotentUnknown,
				IdempotencyKey:      key,
				SemanticInputDigest: dig,
				State:               harnessmodel.EffectPrepared,
				PreparedAt:          now,
			}
			if _, _, err := tx.PutEffectIntent(ctx, eff); err != nil {
				return err
			}
			eff.State = harnessmodel.EffectDispatched
			eff.DispatchedAt = now
			if err := tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectPrepared, eff); err != nil {
				return err
			}
			eff.State = harnessmodel.EffectInDoubt
			return tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectDispatched, eff)
		})
		if err != nil {
			t.Fatal(err)
		}

		env := makeEnvelope(planDigest)
		req := HandoffRequest{
			Envelope:          env,
			PlanText:          planText,
			PriorAssignmentID: "pasn_init",
		}

		_, err = mgr.Handoff(ctx, req)
		if err == nil {
			t.Fatal("mutant survival: un-reconciled IN_DOUBT effect was admitted to handoff")
		}
		if !errors.Is(err, ErrHandoffUnsafeInDoubt) {
			t.Fatalf("mutant survival: unexpected error %v", err)
		}
	})

	t.Run("mutant: plan drift by 1 character bypassed", func(t *testing.T) {
		driftedPlan := []byte("# MASTER PLAN\n\nOriginal plan content!")
		env := makeEnvelope(planDigest)
		req := HandoffRequest{
			Envelope:          env,
			PlanText:          driftedPlan,
			PriorAssignmentID: "pasn_init",
		}

		_, err := mgr.Handoff(ctx, req)
		if err == nil {
			t.Fatal("mutant survival: 1-char plan drift was bypassed during handoff")
		}
		if !errors.Is(err, harnessmodel.ErrStalePlan) {
			t.Fatalf("mutant survival: expected ErrStalePlan, got %v", err)
		}
	})

	t.Run("mutant: prior assignment remains active or missing", func(t *testing.T) {
		env := makeEnvelope(planDigest)
		req := HandoffRequest{
			Envelope:          env,
			PlanText:          planText,
			PriorAssignmentID: "pasn_nonexistent",
		}

		_, err := mgr.Handoff(ctx, req)
		if err == nil {
			t.Fatal("mutant survival: nonexistent prior assignment admitted to handoff")
		}
	})

	t.Run("mutant: failed provider account reused during handoff", func(t *testing.T) {
		// Clean the in-doubt effect to allow clean handoff check
		_ = db.Update(ctx, func(tx harnessstore.Tx) error {
			eff, _ := tx.GetEffectIntent(ctx, "eff_mut_indoubt")
			eff.State = harnessmodel.EffectConfirmed
			eff.ResolvedAt = now
			return tx.CompareAndSwapEffectIntent(ctx, harnessmodel.EffectInDoubt, eff)
		})

		env := makeEnvelope(planDigest)
		req := HandoffRequest{
			Envelope:          env,
			PlanText:          planText,
			PriorAssignmentID: "pasn_init",
			ExcludeAccountIDs: []harnessmodel.ProviderAccountID{acc1},
		}

		res, err := mgr.Handoff(ctx, req)
		if err != nil {
			t.Fatalf("handoff failed: %v", err)
		}
		if res.ReplacementAssignment.AccountID == acc1 {
			t.Fatalf("mutant survival: excluded prior account %s was reused for replacement", acc1)
		}
		if res.ReplacementAssignment.AccountID != acc2 {
			t.Fatalf("expected backup account %s, got %s", acc2, res.ReplacementAssignment.AccountID)
		}
	})
}
