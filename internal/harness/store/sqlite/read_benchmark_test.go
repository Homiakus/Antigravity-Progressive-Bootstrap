package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func BenchmarkReadWorkflow1KNodes(b *testing.B)   { benchmarkReadWorkflow(b, 1_000) }
func BenchmarkReadWorkflow100KNodes(b *testing.B) { benchmarkReadWorkflow(b, 100_000) }

func benchmarkReadWorkflow(b *testing.B, n int) {
	db, err := Open(context.Background(), filepath.Join(b.TempDir(), "state.db"), Options{})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	now := time.Unix(900, 0).UTC()
	nodes := make([]harnessmodel.NodeSpec, n)
	for i := range nodes {
		nodes[i] = harnessmodel.NodeSpec{ID: harnessmodel.NodeID(fmt.Sprintf("n-%d", i)), Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess}
		if i > 0 {
			nodes[i].Dependencies = []harnessmodel.NodeID{harnessmodel.NodeID(fmt.Sprintf("n-%d", i-1))}
		}
	}
	def := harnessmodel.WorkflowDefinition{ID: "wfd_bench", Version: 1, Name: "bench", CreatedAt: now, CompilerVersion: "bench", Nodes: nodes}
	run := harnessmodel.WorkflowRun{ID: "wfr_bench", DefinitionID: def.ID, DefinitionVersion: 1, State: harnessmodel.WorkflowCreated, CreatedAt: now, UpdatedAt: now}
	if err := db.Update(context.Background(), func(tx harnessstore.Tx) error {
		if err := tx.CreateWorkflowDefinition(context.Background(), def); err != nil {
			return err
		}
		return tx.CreateWorkflowRun(context.Background(), run)
	}); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := db.View(context.Background(), func(r harnessstore.Reader) error {
			_, err := r.GetWorkflowDefinition(context.Background(), def.ID, 1)
			return err
		}); err != nil {
			b.Fatal(err)
		}
	}
}
