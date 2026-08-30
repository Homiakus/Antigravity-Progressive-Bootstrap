package router

import (
	"context"
	"errors"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestRouterIsolatedWriteMutationSentinels(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestStore(t)
	defer cleanup()

	now := time.Unix(5000, 0).UTC()
	seedTestAccounts(t, db, now)

	planText := []byte("# MASTER PLAN\n\nTask details...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	router := NewRouter(db, Options{
		Now: func() time.Time { return now },
	})

	t.Run("mutant: unisolated write envelope admitted without error", func(t *testing.T) {
		env := makeIsolatedWriteEnvelope(planDigest)
		env.Workspace.Isolated = false
		env.Workspace.IsolationRoot = ""

		_, err := router.RouteIsolatedWrite(ctx, env, planText)
		if err == nil {
			t.Fatal("mutant survived: unisolated write envelope was admitted")
		}
	})

	t.Run("mutant: empty isolation root and worktree admitted for isolated write", func(t *testing.T) {
		env := makeIsolatedWriteEnvelope(planDigest)
		env.Workspace.Isolated = true
		env.Workspace.IsolationRoot = ""
		env.Workspace.WorktreeID = ""

		_, err := router.RouteIsolatedWrite(ctx, env, planText)
		if err == nil {
			t.Fatal("mutant survived: isolated write with empty isolation root was admitted")
		}
	})

	t.Run("mutant: read-only envelope accepted in RouteIsolatedWrite", func(t *testing.T) {
		env := makeReadOnlyEnvelope(planDigest)
		_, err := router.RouteIsolatedWrite(ctx, env, planText)
		if err == nil {
			t.Fatal("mutant survived: read-only envelope admitted in RouteIsolatedWrite")
		}
		if !errors.Is(err, ErrIsolatedWriteRequired) {
			t.Fatalf("expected ErrIsolatedWriteRequired, got %v", err)
		}
	})

	t.Run("mutant: plan drift admitted by a single character", func(t *testing.T) {
		env := makeIsolatedWriteEnvelope(planDigest)
		tamperedPlan := append([]byte(nil), planText...)
		tamperedPlan[len(tamperedPlan)-1] = '!'

		_, err := router.RouteIsolatedWrite(ctx, env, tamperedPlan)
		if err == nil {
			t.Fatal("mutant survived: drifted plan admitted in write routing")
		}
		if !errors.Is(err, harnessmodel.ErrStalePlan) {
			t.Fatalf("expected ErrStalePlan, got %v", err)
		}
	})

	t.Run("mutant: leaked reservation on execution failure", func(t *testing.T) {
		env := makeIsolatedWriteEnvelope(planDigest)
		env.ID = "tenv_write_fail_test"
		env.AttemptID = "att_write_fail_test"

		route, err := router.RouteIsolatedWrite(ctx, env, planText)
		if err != nil {
			t.Fatalf("route failed: %v", err)
		}

		failFn := func(ctx context.Context, e harnessmodel.TaskEnvelope, a harnessmodel.ProviderAssignment) (IsolatedWriteOutput, error) {
			return IsolatedWriteOutput{}, errors.New("upstream failure")
		}

		_, _ = router.ExecuteIsolatedWrite(ctx, route, failFn)

		// Check reservation was released
		if err := db.View(ctx, func(r harnessstore.Reader) error {
			res, err := r.GetProviderReservation(ctx, route.Reservation.ID)
			if err != nil {
				return err
			}
			if res.State != harnessmodel.ProviderReservationReleased {
				t.Fatalf("mutant survived: reservation leaked with state %s", res.State)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("mutant: double execution of settled write route allowed", func(t *testing.T) {
		env := makeIsolatedWriteEnvelope(planDigest)
		env.ID = "tenv_write_double_exec"
		env.AttemptID = "att_write_double_exec"

		route, err := router.RouteIsolatedWrite(ctx, env, planText)
		if err != nil {
			t.Fatalf("route failed: %v", err)
		}

		successFn := func(ctx context.Context, e harnessmodel.TaskEnvelope, a harnessmodel.ProviderAssignment) (IsolatedWriteOutput, error) {
			return IsolatedWriteOutput{Output: "done", TokensUsed: 100}, nil
		}

		_, err = router.ExecuteIsolatedWrite(ctx, route, successFn)
		if err != nil {
			t.Fatalf("first exec failed: %v", err)
		}

		_, err = router.ExecuteIsolatedWrite(ctx, route, successFn)
		if err == nil {
			t.Fatal("mutant survived: double execution of write route succeeded")
		}
	})
}
