package router

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnesssqlite "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

func BenchmarkReadOnlyRouterDispatch100Operations(b *testing.B) {
	ctx := context.Background()
	db, err := harnesssqlite.Open(ctx, filepath.Join(b.TempDir(), "bench_router.db"), harnesssqlite.Options{})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	now := time.Unix(2000, 0).UTC()
	seedTestAccounts(b, db, now)

	planText := []byte("# MASTER PLAN\n\nBenchmark dispatch...")
	planDigest := harnessmodel.ComputePlanDigest(planText)

	router := NewRouter(db, Options{
		Now: func() time.Time { return now },
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			env := makeReadOnlyEnvelope(planDigest)
			env.ID = harnessmodel.TaskEnvelopeID(fmt.Sprintf("tenv_bench_%d_%d", i, j))
			env.AttemptID = harnessmodel.AttemptID(fmt.Sprintf("att_bench_%d_%d", i, j))

			route, err := router.Route(ctx, env, planText)
			if err != nil || route.Assignment.ID == "" {
				b.Fatalf("Route failed: %v", err)
			}
		}
	}
}
