package sqlite

import (
	"context"
	"encoding/json"
	"fmt"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func (t *transaction) AddWorkflowNode(ctx context.Context, defID harnessmodel.WorkflowDefinitionID, defVersion int, node harnessmodel.NodeSpec) error {
	if defID == "" || defVersion < 1 || node.ID == "" {
		return fmt.Errorf("invalid definition id, version or node spec")
	}
	spec, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("marshal node %s: %w", node.ID, err)
	}
	_, err = t.tx.ExecContext(ctx, `
INSERT INTO nodes(definition_id, definition_version, node_id, spec_json)
VALUES(?, ?, ?, ?)
ON CONFLICT(definition_id, definition_version, node_id) DO UPDATE SET spec_json=excluded.spec_json`,
		string(defID), defVersion, string(node.ID), spec)
	if err != nil {
		return fmt.Errorf("insert/update dynamic node %s: %w", node.ID, err)
	}
	return nil
}

func (t *transaction) AddWorkflowDependency(ctx context.Context, defID harnessmodel.WorkflowDefinitionID, defVersion int, nodeID, depID harnessmodel.NodeID) error {
	if defID == "" || defVersion < 1 || nodeID == "" || depID == "" || nodeID == depID {
		return fmt.Errorf("invalid dependency identity")
	}
	_, err := t.tx.ExecContext(ctx, `
INSERT INTO dependencies(definition_id, definition_version, node_id, depends_on_node_id, required)
VALUES(?, ?, ?, ?, 1)
ON CONFLICT(definition_id, definition_version, node_id, depends_on_node_id) DO NOTHING`,
		string(defID), defVersion, string(nodeID), string(depID))
	if err != nil {
		return fmt.Errorf("insert dynamic dependency %s -> %s: %w", nodeID, depID, err)
	}
	return nil
}

func (t *transaction) RemoveWorkflowDependency(ctx context.Context, defID harnessmodel.WorkflowDefinitionID, defVersion int, nodeID, depID harnessmodel.NodeID) error {
	if defID == "" || defVersion < 1 || nodeID == "" || depID == "" {
		return fmt.Errorf("invalid dependency identity")
	}
	_, err := t.tx.ExecContext(ctx, `
DELETE FROM dependencies
WHERE definition_id=? AND definition_version=? AND node_id=? AND depends_on_node_id=?`,
		string(defID), defVersion, string(nodeID), string(depID))
	if err != nil {
		return fmt.Errorf("delete dynamic dependency %s -> %s: %w", nodeID, depID, err)
	}
	return nil
}

func (t *transaction) UpdateWorkflowProgressTotalNodes(ctx context.Context, runID harnessmodel.WorkflowRunID, delta int) error {
	if runID == "" {
		return fmt.Errorf("workflow run id is required")
	}
	_, err := t.tx.ExecContext(ctx, `
UPDATE workflow_progress
SET total_nodes = total_nodes + ?
WHERE workflow_run_id=?`, delta, string(runID))
	if err != nil {
		return fmt.Errorf("update workflow progress total nodes: %w", err)
	}
	return nil
}
