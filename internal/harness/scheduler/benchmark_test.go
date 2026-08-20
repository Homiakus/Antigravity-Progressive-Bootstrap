package scheduler

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/harness/engine"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/resource"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

func benchmarkReadyDefinition(count int) harnessmodel.WorkflowDefinition {
	nodes := make([]harnessmodel.NodeSpec, 0, count)
	for i := 0; i < count; i++ {
		nodes = append(nodes, harnessmodel.NodeSpec{
			ID: harnessmodel.NodeID(fmt.Sprintf("n-%06d", i)),
			Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess,
			Priority: i % 32,
		})
	}
	return harnessmodel.WorkflowDefinition{
		ID: harnessmodel.WorkflowDefinitionID(fmt.Sprintf("bench-ready-%d", count)),
		Version: 1, Name: "scheduler-benchmark", CompilerVersion: "benchmark",
		CreatedAt: time.Unix(1000, 0).UTC(), Nodes: nodes,
	}
}

func benchmarkSchedulerNext(b *testing.B, ready int) {
	ctx := context.Background()
	db, err := sqlitestore.Open(ctx, filepath.Join(b.TempDir(), "state.db"), sqlitestore.Options{})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	eng, err := engine.New(db, engine.Options{})
	if err != nil {
		b.Fatal(err)
	}
	if _, err := eng.StartWorkflow(ctx, benchmarkReadyDefinition(ready)); err != nil {
		b.Fatal(err)
	}
	sched, err := New(db, Options{Capacity: resource.Capacity{CPUWeight: 1000, MemoryBytes: 1 << 40, DiskBytes: 1 << 40, BuildSlots: 64, BrowserSlots: 64}})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok, err := sched.Next(ctx); err != nil {
			b.Fatal(err)
		} else if !ok {
			b.Fatal("no READY scheduler decision")
		}
	}
}

func BenchmarkSchedulerNext10KReady(b *testing.B)  { benchmarkSchedulerNext(b, 10_000) }
func BenchmarkSchedulerNext100KReady(b *testing.B) { benchmarkSchedulerNext(b, 100_000) }
