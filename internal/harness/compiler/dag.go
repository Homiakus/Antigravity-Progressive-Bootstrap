package compiler

import (
	"fmt"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func validateDAG(nodes []harnessmodel.NodeSpec) error {
	indegree := make(map[harnessmodel.NodeID]int, len(nodes))
	outgoing := make(map[harnessmodel.NodeID][]harnessmodel.NodeID, len(nodes))
	for _, n := range nodes {
		indegree[n.ID] = len(n.Dependencies)
		for _, dep := range n.Dependencies {
			outgoing[dep] = append(outgoing[dep], n.ID)
		}
	}
	q := make([]harnessmodel.NodeID, 0, len(nodes))
	for _, n := range nodes {
		if indegree[n.ID] == 0 {
			q = append(q, n.ID)
		}
	}
	visited := 0
	for head := 0; head < len(q); head++ {
		cur := q[head]
		visited++
		for _, child := range outgoing[cur] {
			indegree[child]--
			if indegree[child] == 0 {
				q = append(q, child)
			}
		}
	}
	if visited != len(nodes) {
		return fmt.Errorf("workflow graph contains a cycle")
	}
	return nil
}

func validateReachability(nodes []harnessmodel.NodeSpec, explicitEntries []harnessmodel.NodeID) error {
	byID := make(map[harnessmodel.NodeID]harnessmodel.NodeSpec, len(nodes))
	outgoing := make(map[harnessmodel.NodeID][]harnessmodel.NodeID, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
		for _, dep := range n.Dependencies {
			outgoing[dep] = append(outgoing[dep], n.ID)
		}
	}
	entries := append([]harnessmodel.NodeID(nil), explicitEntries...)
	if len(entries) == 0 {
		for _, n := range nodes {
			if len(n.Dependencies) == 0 {
				entries = append(entries, n.ID)
			}
		}
	}
	seen := map[harnessmodel.NodeID]struct{}{}
	queue := append([]harnessmodel.NodeID(nil), entries...)
	for head := 0; head < len(queue); head++ {
		cur := queue[head]
		if _, ok := seen[cur]; ok {
			continue
		}
		seen[cur] = struct{}{}
		queue = append(queue, outgoing[cur]...)
	}
	if len(seen) == len(byID) {
		return nil
	}
	for _, n := range nodes {
		if _, ok := seen[n.ID]; !ok {
			return fmt.Errorf("node %q is unreachable from workflow entry nodes", n.ID)
		}
	}
	return nil
}
