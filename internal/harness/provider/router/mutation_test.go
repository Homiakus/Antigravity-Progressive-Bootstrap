package router

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
	seedTestAccounts(t, db, now)

	planText := []byte("# MASTER PLAN\n\nMutation test...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	router := NewRouter(db, Options{
		Now: func() time.Time { return now },
	})

	t.Run("mutant: write envelope admitted to router", func(t *testing.T) {
		env := makeReadOnlyEnvelope(planDigest)
		env.Workspace.ReadOnly = false // write allowed
		_, err := router.Route(ctx, env, planText)
		if err == nil || !errors.Is(err, ErrReadOnlyRequired) {
			t.Fatal("mutant survival: non-read-only envelope was admitted to read-only router")
		}
	})

	t.Run("mutant: plan drift by 1 character admitted to router", func(t *testing.T) {
		env := makeReadOnlyEnvelope(planDigest)
		tamperedPlan := append([]byte(nil), planText...)
		tamperedPlan = append(tamperedPlan, '!')
		_, err := router.Route(ctx, env, tamperedPlan)
		if err == nil || !errors.Is(err, harnessmodel.ErrStalePlan) {
			t.Fatal("mutant survival: 1-character plan drift was not rejected")
		}
	})

	t.Run("mutant: invalid envelope without title admitted", func(t *testing.T) {
		env := makeReadOnlyEnvelope(planDigest)
		env.Title = ""
		_, err := router.Route(ctx, env, planText)
		if err == nil {
			t.Fatal("mutant survival: invalid envelope was admitted")
		}
	})

	t.Run("mutant: reservation not released when executor fails", func(t *testing.T) {
		env := makeReadOnlyEnvelope(planDigest)
		env.ID = "tenv_mut_fail"
		env.AttemptID = "att_mut_fail"

		route, err := router.Route(ctx, env, planText)
		if err != nil {
			t.Fatalf("Route failed: %v", err)
		}

		// Fail executor
		_, _ = router.Execute(ctx, route, func(ctx context.Context, e harnessmodel.TaskEnvelope, a harnessmodel.ProviderAssignment) (string, int64, error) {
			return "", 0, errors.New("aborted")
		})

		// Verify reservation was released in store despite failure
		if err := db.View(ctx, func(r harnessstore.Reader) error {
			res, err := r.GetProviderReservation(ctx, route.Reservation.ID)
			if err != nil {
				return err
			}
			if res.State != harnessmodel.ProviderReservationReleased {
				t.Fatalf("mutant survival: reservation state = %s, want RELEASED", res.State)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("mutant: settled route can be executed a second time", func(t *testing.T) {
		env := makeReadOnlyEnvelope(planDigest)
		env.ID = "tenv_mut_reexec"
		env.AttemptID = "att_mut_reexec"

		route, err := router.Route(ctx, env, planText)
		if err != nil {
			t.Fatalf("Route failed: %v", err)
		}

		// First execution succeeds
		_, err = router.Execute(ctx, route, func(ctx context.Context, e harnessmodel.TaskEnvelope, a harnessmodel.ProviderAssignment) (string, int64, error) {
			return "done", 50, nil
		})
		if err != nil {
			t.Fatalf("first execution failed: %v", err)
		}

		// Second execution with settled route should fail
		route.Assignment.State = harnessmodel.ProviderAssignmentCompleted
		_, err = router.Execute(ctx, route, func(ctx context.Context, e harnessmodel.TaskEnvelope, a harnessmodel.ProviderAssignment) (string, int64, error) {
			return "second", 50, nil
		})
		if err == nil || !errors.Is(err, ErrInvalidRouteState) {
			t.Fatal("mutant survival: settled route was re-executed without error")
		}
	})
}
