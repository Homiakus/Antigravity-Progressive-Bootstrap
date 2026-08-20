package compiler

import (
	"fmt"
	"strings"

	"github.com/homiakus/agctl/internal/harness/ir"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func Validate(def ir.Definition) error {
	if strings.TrimSpace(def.Name) == "" {
		return fmt.Errorf("workflow definition name is required")
	}
	if len(def.Nodes) == 0 {
		return fmt.Errorf("workflow definition must contain at least one node")
	}

	byID := make(map[harnessmodel.NodeID]harnessmodel.NodeSpec, len(def.Nodes))
	for _, n := range def.Nodes {
		if err := harnessmodel.ValidateNodeID(n.ID); err != nil {
			return err
		}
		if _, exists := byID[n.ID]; exists {
			return fmt.Errorf("duplicate node id %q", n.ID)
		}
		if !n.Kind.Valid() {
			return fmt.Errorf("node %q has invalid kind %q", n.ID, n.Kind)
		}
		if n.Kind == harnessmodel.NodeKindAction {
			if !n.ExecutorKind.Valid() {
				return fmt.Errorf("action node %q has invalid executor kind %q", n.ID, n.ExecutorKind)
			}
		} else if n.ExecutorKind != harnessmodel.ExecutorNone {
			return fmt.Errorf("non-action node %q must not declare executor kind %q", n.ID, n.ExecutorKind)
		}
		if !n.CachePolicy.Valid() && n.CachePolicy != "" {
			return fmt.Errorf("node %q has invalid cache policy %q", n.ID, n.CachePolicy)
		}
		if err := validateResources(n.ID, n.Resources); err != nil {
			return err
		}
		seenDeps := map[harnessmodel.NodeID]struct{}{}
		for _, dep := range n.Dependencies {
			if dep == n.ID {
				return fmt.Errorf("node %q depends on itself", n.ID)
			}
			if _, dup := seenDeps[dep]; dup {
				return fmt.Errorf("node %q has duplicate dependency %q", n.ID, dep)
			}
			seenDeps[dep] = struct{}{}
		}
		byID[n.ID] = n
	}

	for _, n := range def.Nodes {
		for _, dep := range n.Dependencies {
			if _, ok := byID[dep]; !ok {
				return fmt.Errorf("node %q depends on missing node %q", n.ID, dep)
			}
		}
	}
	for _, entry := range def.EntryNodes {
		if _, ok := byID[entry]; !ok {
			return fmt.Errorf("entry node %q does not exist", entry)
		}
	}
	if err := validateDAG(def.Nodes); err != nil {
		return err
	}
	if err := validateReachability(def.Nodes, def.EntryNodes); err != nil {
		return err
	}
	return nil
}

func validateResources(nodeID harnessmodel.NodeID, r harnessmodel.ResourceSpec) error {
	if r.CPUWeight < 0 || r.MemoryBytes < 0 || r.GPUCount < 0 || r.MinVRAMBytes < 0 || r.DiskBytes < 0 || r.BuildSlots < 0 || r.BrowserSlots < 0 {
		return fmt.Errorf("node %q has negative resource values", nodeID)
	}
	return nil
}
