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
	// WorkflowDefinitionID is an opaque durable identifier at this boundary.
	// The built-in compiler emits the stricter generated-ID format, but imported
	// or future external compilers may use another stable representation. Engine
	// correctness only requires that the identity is present and immutable.
	if strings.TrimSpace(string(def.ID)) == "" {
		return fmt.Errorf("workflow definition id is required")
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
