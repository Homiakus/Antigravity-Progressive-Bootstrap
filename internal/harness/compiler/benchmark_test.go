package compiler

import (
	"fmt"
	"testing"

	"github.com/homiakus/agctl/internal/harness/ir"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func benchmarkLinear(b *testing.B, n int) {
	nodes := make([]harnessmodel.NodeSpec, n)
	for i := 0; i < n; i++ {
		id := harnessmodel.NodeID(fmt.Sprintf("n-%d", i))
		nodes[i] = action(id)
		if i > 0 {
			nodes[i].Dependencies = []harnessmodel.NodeID{harnessmodel.NodeID(fmt.Sprintf("n-%d", i-1))}
		}
	}
	def := ir.Definition{ID: "wfd_0000000000000_00000000000000000000", Name: "linear", Nodes: nodes}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Compile(def, Options{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileLinear1K(b *testing.B)  { benchmarkLinear(b, 1_000) }
func BenchmarkCompileLinear10K(b *testing.B) { benchmarkLinear(b, 10_000) }

func BenchmarkCompileDAG100K(b *testing.B) {
	const n = 100_000
	nodes := make([]harnessmodel.NodeSpec, n)
	for i := 0; i < n; i++ {
		id := harnessmodel.NodeID(fmt.Sprintf("n-%d", i))
		nodes[i] = action(id)
		if i > 0 {
			deps := []harnessmodel.NodeID{harnessmodel.NodeID(fmt.Sprintf("n-%d", (i-1)/2))}
			nodes[i].Dependencies = deps
		}
	}
	def := ir.Definition{ID: "wfd_0000000000000_00000000000000000000", Name: "dag-100k", Nodes: nodes}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Compile(def, Options{}); err != nil {
			b.Fatal(err)
		}
	}
}
