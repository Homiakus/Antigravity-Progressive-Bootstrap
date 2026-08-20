package compiler

import (
	"fmt"
	"time"

	"github.com/homiakus/agctl/internal/harness/ir"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

const Version = "harness-compiler/0.1"

type Options struct {
	IDs harnessmodel.IDGenerator
	Now func() time.Time
}

func Compile(def ir.Definition, opts Options) (harnessmodel.WorkflowDefinition, error) {
	normalized, err := normalize(def)
	if err != nil {
		return harnessmodel.WorkflowDefinition{}, err
	}
	if err := Validate(normalized); err != nil {
		return harnessmodel.WorkflowDefinition{}, err
	}

	id := normalized.ID
	if id == "" {
		gen := opts.IDs
		if gen == nil {
			g := harnessmodel.NewIDGenerator()
			gen = g
		}
		raw, err := gen.New(harnessmodel.IDWorkflowDefinition)
		if err != nil {
			return harnessmodel.WorkflowDefinition{}, err
		}
		id = harnessmodel.WorkflowDefinitionID(raw)
	}
	if err := harnessmodel.ValidateGeneratedID(string(id), harnessmodel.IDWorkflowDefinition); err != nil {
		return harnessmodel.WorkflowDefinition{}, err
	}

	createdAt := normalized.CreatedAt
	if createdAt.IsZero() {
		now := opts.Now
		if now == nil {
			now = time.Now
		}
		createdAt = now().UTC()
	}
	version := normalized.Version
	if version == 0 {
		version = 1
	}
	if version < 1 {
		return harnessmodel.WorkflowDefinition{}, fmt.Errorf("workflow definition version must be >= 1")
	}

	return harnessmodel.WorkflowDefinition{
		ID:              id,
		Version:         version,
		Name:            normalized.Name,
		CreatedAt:       createdAt,
		CompilerVersion: Version,
		Nodes:           cloneNodes(normalized.Nodes),
		EntryNodes:      append([]harnessmodel.NodeID(nil), normalized.EntryNodes...),
		RetryPolicies:   cloneRetryPolicies(normalized.RetryPolicies),
		Metadata:        cloneStringMap(normalized.Metadata),
	}, nil
}
