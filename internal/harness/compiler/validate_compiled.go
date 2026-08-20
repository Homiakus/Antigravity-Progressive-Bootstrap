package compiler

import (
	"fmt"
	"strings"

	"github.com/homiakus/agctl/internal/harness/ir"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

// ValidateCompiled re-validates a compiled durable definition at the engine
// boundary. The compiler is the normal producer of WorkflowDefinition, but the
// engine must not trust callers to have used it: a manually constructed or
// deserialized definition must satisfy the same DAG/resource invariants before
// any durable state is created.
func ValidateCompiled(def harnessmodel.WorkflowDefinition) error {
	if err := harnessmodel.ValidateGeneratedID(string(def.ID), harnessmodel.IDWorkflowDefinition); err != nil {
		return fmt.Errorf("workflow definition id: %w", err)
	}
	if def.Version < 1 {
		return fmt.Errorf("workflow definition version must be >= 1")
	}
	if strings.TrimSpace(def.Name) == "" {
		return fmt.Errorf("workflow definition name is required")
	}
	if strings.TrimSpace(def.CompilerVersion) == "" {
		return fmt.Errorf("workflow definition compiler version is required")
	}

	return Validate(ir.Definition{
		ID:         def.ID,
		Version:    def.Version,
		Name:       def.Name,
		CreatedAt:  def.CreatedAt,
		Nodes:      def.Nodes,
		EntryNodes: def.EntryNodes,
		Metadata:   def.Metadata,
	})
}
