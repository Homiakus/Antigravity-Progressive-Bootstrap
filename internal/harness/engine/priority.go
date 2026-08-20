package engine

import harnessmodel "github.com/homiakus/agctl/internal/harness/model"

// effectivePriorities propagates a descendant's priority to unfinished
// ancestors. The input is already validated as a DAG by the compiler boundary,
// so a single reverse topological pass is sufficient and costs O(V+E).
func effectivePriorities(nodes []harnessmodel.NodeSpec) map[harnessmodel.NodeID]int {
	byID := make(map[harnessmodel.NodeID]harnessmodel.NodeSpec, len(nodes))
	indegree := make(map[harnessmodel.NodeID]int, len(nodes))
	children := make(map[harnessmodel.NodeID][]harnessmodel.NodeID, len(nodes))
	effective := make(map[harnessmodel.NodeID]int, len(nodes))
	for _, node := range nodes {
		byID[node.ID] = node
		indegree[node.ID] = len(node.Dependencies)
		effective[node.ID] = node.Priority
		for _, dep := range node.Dependencies {
			children[dep] = append(children[dep], node.ID)
		}
	}
	queue := make([]harnessmodel.NodeID, 0, len(nodes))
	for _, node := range nodes {
		if indegree[node.ID] == 0 {
			queue = append(queue, node.ID)
		}
	}
	order := make([]harnessmodel.NodeID, 0, len(nodes))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)
		for _, child := range children[id] {
			indegree[child]--
			if indegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}
	for i := len(order) - 1; i >= 0; i-- {
		id := order[i]
		for _, dep := range byID[id].Dependencies {
			if effective[id] > effective[dep] {
				effective[dep] = effective[id]
			}
		}
	}
	return effective
}
