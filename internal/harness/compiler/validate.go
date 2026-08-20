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
	if err := validateRetryPolicies(def.RetryPolicies); err != nil {
		return err
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
		if n.RetryPolicyRef != "" {
			if _, ok := def.RetryPolicies[n.RetryPolicyRef]; !ok {
				return fmt.Errorf("node %q references missing retry policy %q", n.ID, n.RetryPolicyRef)
			}
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

func validateRetryPolicies(policies map[string]harnessmodel.RetryPolicySpec) error {
	for name, policy := range policies {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("retry policy name is required")
		}
		if policy.MaxAttempts < 1 {
			return fmt.Errorf("retry policy %q maxAttempts must be >= 1", name)
		}
		if policy.MaxElapsedTime < 0 || policy.InitialDelay < 0 || policy.MaxDelay < 0 {
			return fmt.Errorf("retry policy %q contains negative durations", name)
		}
		if policy.BackoffFactor != 0 && policy.BackoffFactor < 1 {
			return fmt.Errorf("retry policy %q backoffFactor must be >= 1 when set", name)
		}
		if policy.Jitter < 0 || policy.Jitter > 1 {
			return fmt.Errorf("retry policy %q jitter must be in [0,1]", name)
		}
		seen := make(map[harnessmodel.ErrorClass]bool)
		for _, class := range policy.RetryableClasses {
			if !class.Valid() {
				return fmt.Errorf("retry policy %q has invalid retryable class %q", name, class)
			}
			if seen[class] {
				return fmt.Errorf("retry policy %q repeats class %q", name, class)
			}
			seen[class] = true
		}
		for _, class := range policy.NonRetryableClasses {
			if !class.Valid() {
				return fmt.Errorf("retry policy %q has invalid non-retryable class %q", name, class)
			}
			if seen[class] {
				return fmt.Errorf("retry policy %q class %q is both retryable and non-retryable", name, class)
			}
			seen[class] = true
		}
	}
	return nil
}

func validateResources(nodeID harnessmodel.NodeID, r harnessmodel.ResourceSpec) error {
	if r.CPUWeight < 0 || r.MemoryBytes < 0 || r.GPUCount < 0 || r.MinVRAMBytes < 0 || r.DiskBytes < 0 || r.BuildSlots < 0 || r.BrowserSlots < 0 {
		return fmt.Errorf("node %q has negative resource values", nodeID)
	}
	return nil
}
