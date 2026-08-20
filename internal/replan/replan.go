package replan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/planner"
	"github.com/homiakus/agctl/internal/tasks"
	"github.com/homiakus/agctl/internal/telemetry"
	"github.com/homiakus/agctl/internal/worktree"
)

const GeneratedBy = "agctl-replan/3.2.1"

func DefaultConfig() model.ReplanConfig {
	return model.ReplanConfig{
		Enabled:          true,
		MaxRevisions:     8,
		MaxDynamicNodes:  24,
		MaxRepairDepth:   3,
		MaxSameFailure:   2,
		MinConfidence:    0.65,
		AutoApplyRiskMax: "write-medium",
		PreferWorktrees:  true,
		RequireEvidence:  true,
	}
}

func LoadConfig(p paths.Paths) (model.ReplanConfig, error) {
	cfg := DefaultConfig()
	if p.ReplanConfig == "" {
		return cfg, nil
	}
	if _, err := os.Stat(p.ReplanConfig); errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	loaded, err := jsonx.Read(p.ReplanConfig, cfg)
	if err != nil {
		return cfg, err
	}
	if loaded.MaxRevisions <= 0 {
		loaded.MaxRevisions = cfg.MaxRevisions
	}
	if loaded.MaxDynamicNodes <= 0 {
		loaded.MaxDynamicNodes = cfg.MaxDynamicNodes
	}
	if loaded.MaxRepairDepth <= 0 {
		loaded.MaxRepairDepth = cfg.MaxRepairDepth
	}
	if loaded.MaxSameFailure <= 0 {
		loaded.MaxSameFailure = cfg.MaxSameFailure
	}
	if loaded.MinConfidence <= 0 || loaded.MinConfidence > 1 {
		loaded.MinConfidence = cfg.MinConfidence
	}
	if strings.TrimSpace(loaded.AutoApplyRiskMax) == "" {
		loaded.AutoApplyRiskMax = cfg.AutoApplyRiskMax
	}
	return loaded, nil
}

func SaveConfig(p paths.Paths, cfg model.ReplanConfig) error {
	if cfg.MaxRevisions < 1 || cfg.MaxDynamicNodes < 1 || cfg.MaxRepairDepth < 1 || cfg.MaxSameFailure < 1 {
		return fmt.Errorf("replan limits must be >= 1")
	}
	if cfg.MinConfidence <= 0 || cfg.MinConfidence > 1 {
		return fmt.Errorf("minConfidence must be in (0,1]")
	}
	if _, ok := riskRank(cfg.AutoApplyRiskMax); !ok {
		return fmt.Errorf("unsupported autoApplyRiskMax %q", cfg.AutoApplyRiskMax)
	}
	return jsonx.WriteAtomic(p.ReplanConfig, cfg, p.BackupsRoot)
}

// RunPending runs the task supervisor with an adaptive observer. Final task
// failures that are successfully converted into a repair branch are treated as
// handled, so the supervisor can continue through the revised DAG.
func RunPending(p paths.Paths) ([]model.TaskRecord, error) {
	cfg, err := LoadConfig(p)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return tasks.RunPending(p)
	}
	results, runErr := tasks.RunPendingObserved(p, func(rec model.TaskRecord) (bool, error) {
		res, err := ProcessRecord(p, rec)
		return res.Applied && rec.Status == tasks.StatusFailed, err
	})
	_ = finalizePlans(p)
	return results, runErr
}

func ProcessTask(p paths.Paths, taskID string) (model.ReplanResult, error) {
	rec, err := tasks.Load(p, taskID)
	if err != nil {
		return model.ReplanResult{}, err
	}
	return ProcessRecord(p, rec)
}

func ProcessRecord(p paths.Paths, rec model.TaskRecord) (model.ReplanResult, error) {
	cfg, err := LoadConfig(p)
	if err != nil {
		return model.ReplanResult{}, err
	}
	if !cfg.Enabled || rec.PlanID == "" || rec.NodeID == "" {
		return model.ReplanResult{Applied: false, PlanID: rec.PlanID, Reason: "adaptive replanning not applicable"}, nil
	}
	if rec.Status == tasks.StatusFailed {
		return recoverFailure(p, cfg, rec)
	}
	if rec.Status != tasks.StatusSucceeded {
		return model.ReplanResult{Applied: false, PlanID: rec.PlanID, Reason: "task is not terminal-success/failure"}, nil
	}
	if rec.ReplanProposalPath == "" {
		return model.ReplanResult{Applied: false, PlanID: rec.PlanID, Reason: "no proposal path"}, nil
	}
	if _, err := os.Stat(rec.ReplanProposalPath); errors.Is(err, os.ErrNotExist) {
		return model.ReplanResult{Applied: false, PlanID: rec.PlanID, Reason: "no proposal emitted"}, nil
	}
	proposal, err := jsonx.Read(rec.ReplanProposalPath, model.ReplanProposal{})
	if err != nil {
		_ = archiveProposal(p, rec, "invalid")
		_ = telemetry.Record(p, telemetry.Event{Type: "replan.proposal.invalid", Reason: err.Error(), Data: map[string]any{"taskId": rec.ID, "planId": rec.PlanID}})
		return model.ReplanResult{}, fmt.Errorf("read replan proposal: %w", err)
	}
	return applyProposal(p, cfg, rec, proposal)
}

func applyProposal(p paths.Paths, cfg model.ReplanConfig, parent model.TaskRecord, proposal model.ReplanProposal) (model.ReplanResult, error) {
	plan, err := planner.Load(p, parent.PlanID)
	if err != nil {
		return model.ReplanResult{}, err
	}
	if proposal.PlanID != "" && proposal.PlanID != plan.ID {
		return rejectProposal(p, parent, plan.ID, "proposal planId does not match task")
	}
	if proposal.ParentTaskID != "" && proposal.ParentTaskID != parent.ID {
		return rejectProposal(p, parent, plan.ID, "proposal parentTaskId does not match task")
	}
	if proposal.ParentNodeID != "" && proposal.ParentNodeID != parent.NodeID {
		return rejectProposal(p, parent, plan.ID, "proposal parentNodeId does not match task")
	}
	if proposal.Confidence < cfg.MinConfidence {
		return rejectProposal(p, parent, plan.ID, fmt.Sprintf("confidence %.2f is below threshold %.2f", proposal.Confidence, cfg.MinConfidence))
	}
	if cfg.RequireEvidence && len(nonEmpty(proposal.Evidence)) == 0 {
		return rejectProposal(p, parent, plan.ID, "proposal contains no concrete evidence")
	}
	if len(proposal.Actions) == 0 {
		return rejectProposal(p, parent, plan.ID, "proposal contains no actions")
	}
	plannedGrowth := len(proposal.Actions)
	if cfg.PreferWorktrees && parent.Resources.ReadOnly && countParallelWrites(proposal.Actions) >= 2 {
		plannedGrowth++
	}
	if err := budgetCheck(plan, cfg, plannedGrowth); err != nil {
		return blockPlan(p, plan, parent, "proposal-budget", err.Error(), "")
	}
	for _, a := range proposal.Actions {
		if !riskAllowed(a.Risk, cfg.AutoApplyRiskMax) {
			return rejectProposal(p, parent, plan.ID, fmt.Sprintf("action %s risk %q exceeds auto-apply threshold %q", a.ID, a.Risk, cfg.AutoApplyRiskMax))
		}
	}

	revision := plan.Revision + 1
	prefix := fmt.Sprintf("r%d-", revision)
	actions, err := normalizeActions(proposal.Actions, prefix)
	if err != nil {
		return model.ReplanResult{}, err
	}
	if len(topoActions(actions)) != len(actions) {
		return model.ReplanResult{}, fmt.Errorf("replan proposal contains an action dependency cycle")
	}

	originalNodeCount := len(plan.Nodes)
	created, addedNodes, gateTaskIDs, gateNodeIDs, warnings, err := materializeActions(p, cfg, &plan, parent, revision, actions)
	if err != nil {
		return model.ReplanResult{}, err
	}
	rewiredTasks, err := tasks.ReplaceDependency(p, plan.ID, parent.ID, gateTaskIDs)
	if err != nil {
		return model.ReplanResult{}, err
	}
	rewiredNodes := rewirePlanNodes(&plan, originalNodeCount, parent.NodeID, gateNodeIDs)

	fingerprints := []string{proposalFingerprint(proposal)}
	plan.Revision = revision
	plan.Status = "active"
	plan.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	plan.DynamicNodeCount += len(addedNodes)
	plan.RevisionHistory = append(plan.RevisionHistory, model.PlanRevision{
		Number:        revision,
		CreatedAt:     plan.UpdatedAt,
		TriggerTaskID: parent.ID,
		TriggerNodeID: parent.NodeID,
		Reason:        strings.TrimSpace(proposal.Reason),
		Kind:          "proposal",
		AddedNodes:    append([]string(nil), addedNodes...),
		RewiredNodes:  append([]string(nil), rewiredNodes...),
		Fingerprints:  fingerprints,
	})
	if err := planner.Save(p, plan); err != nil {
		return model.ReplanResult{}, err
	}
	_ = archiveProposal(p, parent, "applied")
	_ = telemetry.Record(p, telemetry.Event{Type: "replan.applied", Workspace: []string{plan.Workspace}, Data: map[string]any{"planId": plan.ID, "revision": revision, "taskId": parent.ID, "addedNodes": len(addedNodes), "createdTasks": len(created), "rewiredTasks": len(rewiredTasks)}})
	return model.ReplanResult{Applied: true, PlanID: plan.ID, Revision: revision, Reason: proposal.Reason, AddedNodes: addedNodes, CreatedTasks: taskIDs(created), RewiredTasks: rewiredTasks, Warnings: warnings}, nil
}

func recoverFailure(p paths.Paths, cfg model.ReplanConfig, failed model.TaskRecord) (model.ReplanResult, error) {
	plan, err := planner.Load(p, failed.PlanID)
	if err != nil {
		return model.ReplanResult{}, err
	}
	sig := failureSignature(failed)
	failed.FailureSignature = sig
	_ = tasks.SaveRecord(p, failed)

	same := 0
	for _, r := range plan.RevisionHistory {
		if r.FailureSignature == sig && sig != "" {
			same++
		}
	}
	if failed.DynamicDepth >= cfg.MaxRepairDepth || same >= cfg.MaxSameFailure {
		reason := fmt.Sprintf("no-progress detector stopped automatic repair: depth=%d/%d sameFailure=%d/%d signature=%s", failed.DynamicDepth, cfg.MaxRepairDepth, same, cfg.MaxSameFailure, sig)
		return blockPlan(p, plan, failed, "no-progress", reason, sig)
	}
	if err := budgetCheck(plan, cfg, 3); err != nil {
		return blockPlan(p, plan, failed, "failure-budget", err.Error(), sig)
	}

	revision := plan.Revision + 1
	strategy := same + 1
	logEvidence := tailLog(failed.OutputLog, 14)
	baseDeps := append([]string(nil), failed.Dependencies...)
	baseNodeDeps := nodeDependencies(plan, failed.NodeID)
	depth := failed.DynamicDepth + 1

	diagnoseID := fmt.Sprintf("r%d-diagnose-%s", revision, safeID(failed.NodeID))
	repairID := fmt.Sprintf("r%d-repair-%s", revision, safeID(failed.NodeID))
	verifyID := fmt.Sprintf("r%d-reverify-%s", revision, safeID(failed.NodeID))
	strategyNote := fmt.Sprintf("Recovery strategy %d. Do not repeat a previously failed approach. Failure signature: %s.", strategy, sig)

	diagnoseNode := model.PlanNode{ID: diagnoseID, Title: "Diagnose failed DAG node", Objective: "Diagnose the root cause of failed node " + failed.NodeID + ". " + strategyNote + "\nFailure: " + failed.Error + "\nRecent execution evidence:\n" + logEvidence, Agent: "architect", DependsOn: baseNodeDeps, Tags: []string{"dynamic", "diagnosis", "read-only"}, Resources: model.ResourceRequest{CPUWeight: 15, ReadOnly: true}, Risk: "read-low", Dynamic: true, ParentNodeID: failed.NodeID, Depth: depth}
	repairNode := model.PlanNode{ID: repairID, Title: "Repair failed DAG node", Objective: "Implement a root-cause fix for failed node " + failed.NodeID + " using the diagnosis. " + strategyNote + " Preserve unrelated behavior and do not merely suppress the failing check.", Agent: "implementer", DependsOn: []string{diagnoseID}, Tags: []string{"dynamic", "repair", "write"}, Resources: model.ResourceRequest{CPUWeight: 45, BuildSlots: maxInt(1, failed.Resources.BuildSlots), ExclusiveWorkspace: true}, Risk: "write-medium", Dynamic: true, ParentNodeID: failed.NodeID, Depth: depth}
	verifyChecks := []string{"rerun the failed check", "targeted regression verification"}
	verifyNode := model.PlanNode{ID: verifyID, Title: "Verify recovery", Objective: "Independently verify the root-cause repair for failed node " + failed.NodeID + ". Confirm the original failure is gone and no material regression was introduced.", Agent: "test-engineer", DependsOn: []string{repairID}, Verification: verifyChecks, Tags: []string{"dynamic", "verification", "read-only"}, Resources: model.ResourceRequest{CPUWeight: 35, BuildSlots: maxInt(1, failed.Resources.BuildSlots), BrowserSlots: failed.Resources.BrowserSlots, ReadOnly: true}, Risk: "read-low", Dynamic: true, ParentNodeID: failed.NodeID, Depth: depth}

	originalNodeCount := len(plan.Nodes)
	plan.Nodes = append(plan.Nodes, diagnoseNode, repairNode, verifyNode)

	diagnoseTask, err := tasks.AddAdvanced(p, tasks.Spec{Prompt: diagnoseNode.Objective, Workspace: failed.Workspace, Priority: failed.Priority + 2, NativeGoal: failed.UseNativeGoal, Agent: diagnoseNode.Agent, Tags: diagnoseNode.Tags, PlanID: plan.ID, NodeID: diagnoseID, Dependencies: baseDeps, Resources: diagnoseNode.Resources, Revision: revision, DynamicDepth: depth, ParentTaskID: failed.ID, BaseWorkspace: baseWorkspace(failed)})
	if err != nil {
		return model.ReplanResult{}, err
	}
	repairTask, err := tasks.AddAdvanced(p, tasks.Spec{Prompt: repairNode.Objective, Workspace: failed.Workspace, Priority: failed.Priority + 2, NativeGoal: failed.UseNativeGoal, Agent: repairNode.Agent, Tags: repairNode.Tags, PlanID: plan.ID, NodeID: repairID, Dependencies: []string{diagnoseTask.ID}, Resources: repairNode.Resources, Revision: revision, DynamicDepth: depth, ParentTaskID: failed.ID, BaseWorkspace: baseWorkspace(failed)})
	if err != nil {
		return model.ReplanResult{}, err
	}
	verifyTask, err := tasks.AddAdvanced(p, tasks.Spec{Prompt: verifyNode.Objective, Workspace: failed.Workspace, Priority: failed.Priority + 2, NativeGoal: failed.UseNativeGoal, Agent: verifyNode.Agent, Tags: verifyNode.Tags, PlanID: plan.ID, NodeID: verifyID, Dependencies: []string{repairTask.ID}, Resources: verifyNode.Resources, Revision: revision, DynamicDepth: depth, ParentTaskID: failed.ID, BaseWorkspace: baseWorkspace(failed)})
	if err != nil {
		return model.ReplanResult{}, err
	}

	rewiredTasks, err := tasks.ReplaceDependency(p, plan.ID, failed.ID, []string{verifyTask.ID})
	if err != nil {
		return model.ReplanResult{}, err
	}
	rewiredNodes := rewirePlanNodes(&plan, originalNodeCount, failed.NodeID, []string{verifyID})
	plan.Revision = revision
	plan.Status = "recovering"
	plan.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	plan.DynamicNodeCount += 3
	plan.RevisionHistory = append(plan.RevisionHistory, model.PlanRevision{Number: revision, CreatedAt: plan.UpdatedAt, TriggerTaskID: failed.ID, TriggerNodeID: failed.NodeID, Reason: "automatic failure recovery: " + failed.Error, Kind: "failure-recovery", AddedNodes: []string{diagnoseID, repairID, verifyID}, RewiredNodes: rewiredNodes, FailureSignature: sig, Fingerprints: []string{sig}})
	if err := planner.Save(p, plan); err != nil {
		return model.ReplanResult{}, err
	}
	failed.Status = tasks.StatusSuperseded
	failed.Error = "superseded by adaptive recovery revision " + fmt.Sprint(revision) + ": " + failed.Error
	_ = tasks.SaveRecord(p, failed)
	_ = telemetry.Record(p, telemetry.Event{Type: "replan.failure_recovery", Reason: failed.Error, Workspace: []string{plan.Workspace}, Data: map[string]any{"planId": plan.ID, "revision": revision, "failedTask": failed.ID, "failureSignature": sig, "strategy": strategy, "depth": depth}})
	return model.ReplanResult{Applied: true, PlanID: plan.ID, Revision: revision, Reason: "automatic failure recovery", AddedNodes: []string{diagnoseID, repairID, verifyID}, CreatedTasks: []string{diagnoseTask.ID, repairTask.ID, verifyTask.ID}, RewiredTasks: rewiredTasks}, nil
}

func materializeActions(p paths.Paths, cfg model.ReplanConfig, plan *model.ExecutionPlan, parent model.TaskRecord, revision int, actions []model.ReplanAction) ([]model.TaskRecord, []string, []string, []string, []string, error) {
	idMap := map[string]string{}
	nodeMap := map[string]string{}
	for _, a := range actions {
		nodeMap[a.ID] = a.ID
	}

	useParallelWorktrees := cfg.PreferWorktrees && parent.Resources.ReadOnly && countParallelWrites(actions) >= 2
	var created []model.TaskRecord
	var addedNodes []string
	var warnings []string
	var worktreeBranches []string
	var worktreePaths []string

	for _, a := range topoActions(actions) {
		workspace := plan.Workspace
		base := plan.Workspace
		branch := ""
		if useParallelWorktrees && a.Parallelizable && !isReadRisk(a.Risk) && len(a.DependsOn) == 0 {
			name := fmt.Sprintf("%s-r%d-%s", shortPlan(plan.ID), revision, safeID(a.ID))
			wt, err := worktree.Create(plan.Workspace, name, "HEAD")
			if err != nil {
				warnings = append(warnings, "worktree fallback for "+a.ID+": "+err.Error())
			} else {
				workspace = wt.Path
				branch = wt.Branch
				worktreeBranches = append(worktreeBranches, wt.Branch)
				worktreePaths = append(worktreePaths, wt.Path)
			}
		}
		deps := []string{}
		nodeDeps := []string{}
		if len(a.DependsOn) == 0 {
			deps = append(deps, parent.ID)
			nodeDeps = append(nodeDeps, parent.NodeID)
		} else {
			for _, d := range a.DependsOn {
				tid, ok := idMap[d]
				if !ok {
					return created, addedNodes, nil, nil, warnings, fmt.Errorf("action %s depends on unresolved action %s", a.ID, d)
				}
				deps = append(deps, tid)
				nodeDeps = append(nodeDeps, nodeMap[d])
			}
		}
		depth := parent.DynamicDepth + 1
		node := model.PlanNode{ID: a.ID, Title: a.Title, Objective: a.Objective, Agent: a.Agent, DependsOn: nodeDeps, Verification: a.Verification, Tags: append([]string{"dynamic"}, a.Tags...), Resources: a.Resources, Risk: normalizeRisk(a.Risk), Dynamic: true, ParentNodeID: parent.NodeID, Depth: depth, Workspace: workspace, WorktreeBranch: branch}
		if node.Resources.CPUWeight == 0 {
			node.Resources.CPUWeight = 25
		}
		plan.Nodes = append(plan.Nodes, node)
		taskPrompt := node.Objective + verificationSuffix(node.Verification)
		if branch != "" {
			taskPrompt = "ISOLATED WORKTREE EXECUTION:\nYou are working in an agctl-created Git worktree on branch " + branch + ". Keep changes scoped to this action. Before successful completion, create a local commit containing the intended changes; do not push. This commit is required so the later integration node can merge the branch safely.\n\n" + taskPrompt
		}
		rec, err := tasks.AddAdvanced(p, tasks.Spec{Prompt: taskPrompt, Workspace: workspace, Priority: parent.Priority + 1, NativeGoal: parent.UseNativeGoal, Agent: node.Agent, Tags: node.Tags, PlanID: plan.ID, NodeID: node.ID, Dependencies: deps, Resources: node.Resources, Revision: revision, DynamicDepth: depth, ParentTaskID: parent.ID, BaseWorkspace: base, WorktreeBranch: branch})
		if err != nil {
			return created, addedNodes, nil, nil, warnings, err
		}
		idMap[a.ID] = rec.ID
		created = append(created, rec)
		addedNodes = append(addedNodes, node.ID)
	}

	gateTasks := leafTaskIDs(actions, idMap)
	gateNodes := leafActionIDs(actions)
	if len(worktreeBranches) >= 2 {
		integrationNodeID := fmt.Sprintf("r%d-integrate-%s", revision, safeID(parent.NodeID))
		integrationObjective := "Integrate the independent worktree branches created by adaptive replanning into the base workspace. Review each branch, merge/cherry-pick only the intended changes, resolve conflicts conservatively, then run targeted regression verification. Branches:\n- " + strings.Join(worktreeBranches, "\n- ") + "\nWorktrees:\n- " + strings.Join(worktreePaths, "\n- ")
		integrationNode := model.PlanNode{ID: integrationNodeID, Title: "Integrate adaptive worktree branches", Objective: integrationObjective, Agent: "implementer", DependsOn: append([]string(nil), gateNodes...), Verification: []string{"integrated branches reviewed", "targeted regression checks pass"}, Tags: []string{"dynamic", "integration", "write"}, Resources: model.ResourceRequest{CPUWeight: 45, BuildSlots: 1, ExclusiveWorkspace: true}, Risk: "write-medium", Dynamic: true, ParentNodeID: parent.NodeID, Depth: parent.DynamicDepth + 1, Workspace: plan.Workspace}
		plan.Nodes = append(plan.Nodes, integrationNode)
		integrationTask, err := tasks.AddAdvanced(p, tasks.Spec{Prompt: integrationObjective + verificationSuffix(integrationNode.Verification), Workspace: plan.Workspace, Priority: parent.Priority + 1, NativeGoal: parent.UseNativeGoal, Agent: integrationNode.Agent, Tags: integrationNode.Tags, PlanID: plan.ID, NodeID: integrationNodeID, Dependencies: gateTasks, Resources: integrationNode.Resources, Revision: revision, DynamicDepth: parent.DynamicDepth + 1, ParentTaskID: parent.ID, BaseWorkspace: plan.Workspace})
		if err != nil {
			return created, addedNodes, nil, nil, warnings, err
		}
		created = append(created, integrationTask)
		addedNodes = append(addedNodes, integrationNodeID)
		gateTasks = []string{integrationTask.ID}
		gateNodes = []string{integrationNodeID}
	}
	return created, addedNodes, gateTasks, gateNodes, warnings, nil
}

func normalizeActions(in []model.ReplanAction, prefix string) ([]model.ReplanAction, error) {
	seen := map[string]bool{}
	mapping := map[string]string{}
	for i := range in {
		orig := safeID(in[i].ID)
		if orig == "" {
			orig = fmt.Sprintf("action-%d", i+1)
		}
		if seen[orig] {
			return nil, fmt.Errorf("duplicate replan action id %s", orig)
		}
		seen[orig] = true
		mapping[in[i].ID] = prefix + orig
		mapping[orig] = prefix + orig
	}
	out := make([]model.ReplanAction, len(in))
	for i, a := range in {
		orig := safeID(a.ID)
		if orig == "" {
			orig = fmt.Sprintf("action-%d", i+1)
		}
		a.ID = prefix + orig
		a.Title = strings.TrimSpace(a.Title)
		if a.Title == "" {
			a.Title = "Adaptive action " + orig
		}
		a.Objective = strings.TrimSpace(a.Objective)
		if a.Objective == "" {
			return nil, fmt.Errorf("replan action %s has empty objective", orig)
		}
		if strings.TrimSpace(a.Agent) == "" {
			if isReadRisk(a.Risk) {
				a.Agent = "architect"
			} else {
				a.Agent = "implementer"
			}
		}
		for j, d := range a.DependsOn {
			mapped, ok := mapping[d]
			if !ok {
				mapped, ok = mapping[safeID(d)]
			}
			if !ok {
				return nil, fmt.Errorf("replan action %s depends on unknown action %s", orig, d)
			}
			a.DependsOn[j] = mapped
		}
		a.Risk = normalizeRisk(a.Risk)
		out[i] = a
	}
	return out, nil
}

func topoActions(actions []model.ReplanAction) []model.ReplanAction {
	byID := map[string]model.ReplanAction{}
	indegree := map[string]int{}
	children := map[string][]string{}
	for _, a := range actions {
		byID[a.ID] = a
		indegree[a.ID] = len(a.DependsOn)
		for _, d := range a.DependsOn {
			children[d] = append(children[d], a.ID)
		}
	}
	var q []string
	for id, n := range indegree {
		if n == 0 {
			q = append(q, id)
		}
	}
	sort.Strings(q)
	var out []model.ReplanAction
	for len(q) > 0 {
		id := q[0]
		q = q[1:]
		out = append(out, byID[id])
		for _, c := range children[id] {
			indegree[c]--
			if indegree[c] == 0 {
				q = append(q, c)
				sort.Strings(q)
			}
		}
	}
	return out
}

func leafActionIDs(actions []model.ReplanAction) []string {
	dep := map[string]bool{}
	for _, a := range actions {
		for _, d := range a.DependsOn {
			dep[d] = true
		}
	}
	var out []string
	for _, a := range actions {
		if !dep[a.ID] {
			out = append(out, a.ID)
		}
	}
	sort.Strings(out)
	return out
}

func leafTaskIDs(actions []model.ReplanAction, idMap map[string]string) []string {
	var out []string
	for _, id := range leafActionIDs(actions) {
		if tid := idMap[id]; tid != "" {
			out = append(out, tid)
		}
	}
	return out
}

func rewirePlanNodes(plan *model.ExecutionPlan, originalNodeCount int, oldNode string, replacements []string) []string {
	var changed []string
	if originalNodeCount > len(plan.Nodes) {
		originalNodeCount = len(plan.Nodes)
	}
	for i := 0; i < originalNodeCount; i++ {
		n := &plan.Nodes[i]
		found := false
		var deps []string
		for _, d := range n.DependsOn {
			if d == oldNode {
				deps = append(deps, replacements...)
				found = true
			} else {
				deps = append(deps, d)
			}
		}
		if found {
			n.DependsOn = unique(deps)
			changed = append(changed, n.ID)
		}
	}
	return changed
}

func budgetCheck(plan model.ExecutionPlan, cfg model.ReplanConfig, add int) error {
	if plan.Revision >= cfg.MaxRevisions {
		return fmt.Errorf("plan reached max revisions %d", cfg.MaxRevisions)
	}
	if plan.DynamicNodeCount+add > cfg.MaxDynamicNodes {
		return fmt.Errorf("dynamic node budget exceeded: %d + %d > %d", plan.DynamicNodeCount, add, cfg.MaxDynamicNodes)
	}
	return nil
}

func blockPlan(p paths.Paths, plan model.ExecutionPlan, trigger model.TaskRecord, kind, reason, sig string) (model.ReplanResult, error) {
	plan.Status = "blocked"
	plan.BlockReason = reason
	plan.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	plan.RevisionHistory = append(plan.RevisionHistory, model.PlanRevision{Number: plan.Revision, CreatedAt: plan.UpdatedAt, TriggerTaskID: trigger.ID, TriggerNodeID: trigger.NodeID, Reason: reason, Kind: kind, FailureSignature: sig})
	if err := planner.Save(p, plan); err != nil {
		return model.ReplanResult{}, err
	}
	_ = telemetry.Record(p, telemetry.Event{Type: "replan.blocked", Reason: reason, Workspace: []string{plan.Workspace}, Data: map[string]any{"planId": plan.ID, "taskId": trigger.ID, "kind": kind, "signature": sig}})
	return model.ReplanResult{Applied: false, PlanID: plan.ID, Revision: plan.Revision, Reason: reason, NoProgress: kind == "no-progress"}, nil
}

func rejectProposal(p paths.Paths, parent model.TaskRecord, planID, reason string) (model.ReplanResult, error) {
	_ = archiveProposal(p, parent, "rejected")
	_ = telemetry.Record(p, telemetry.Event{Type: "replan.proposal.rejected", Reason: reason, Data: map[string]any{"planId": planID, "taskId": parent.ID}})
	return model.ReplanResult{Applied: false, PlanID: planID, Reason: reason}, nil
}

func archiveProposal(p paths.Paths, rec model.TaskRecord, suffix string) error {
	if rec.ReplanProposalPath == "" {
		return nil
	}
	if _, err := os.Stat(rec.ReplanProposalPath); err != nil {
		return nil
	}
	if err := os.MkdirAll(p.ReplanArchive, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(p.ReplanArchive, rec.ID+"-"+suffix+"-"+time.Now().UTC().Format("20060102T150405.000000000Z")+".json")
	return os.Rename(rec.ReplanProposalPath, dst)
}

func failureSignature(rec model.TaskRecord) string {
	material := strings.ToLower(strings.TrimSpace(rec.Error + "\n" + tailLog(rec.OutputLog, 8)))
	material = strings.Join(strings.Fields(material), " ")
	if len(material) > 4096 {
		material = material[len(material)-4096:]
	}
	h := sha256.Sum256([]byte(material))
	return hex.EncodeToString(h[:8])
}

func tailLog(path string, lines int) string {
	if strings.TrimSpace(path) == "" || lines <= 0 {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	parts := strings.Split(string(b), "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	var out []string
	for _, raw := range parts {
		var v map[string]any
		if json.Unmarshal([]byte(raw), &v) == nil {
			if line, ok := v["line"].(string); ok && strings.TrimSpace(line) != "" {
				out = append(out, line)
				continue
			}
		}
		if strings.TrimSpace(raw) != "" {
			out = append(out, raw)
		}
	}
	return strings.Join(out, "\n")
}

func finalizePlans(p paths.Paths) error {
	plans, err := planner.List(p)
	if err != nil {
		return err
	}
	allTasks, err := tasks.List(p)
	if err != nil {
		return err
	}
	for _, pl := range plans {
		var relevant []model.TaskRecord
		for _, t := range allTasks {
			if t.PlanID == pl.ID {
				relevant = append(relevant, t)
			}
		}
		if len(relevant) == 0 {
			continue
		}
		allSucceeded := true
		terminalFailure := false
		for _, t := range relevant {
			if t.Status != tasks.StatusSucceeded && t.Status != tasks.StatusSuperseded {
				allSucceeded = false
			}
			if t.Status == tasks.StatusFailed || t.Status == tasks.StatusBlocked || t.Status == tasks.StatusCancelled {
				terminalFailure = true
			}
		}
		newStatus := pl.Status
		if allSucceeded {
			newStatus = "completed"
			pl.BlockReason = ""
		} else if terminalFailure && pl.Status != "recovering" {
			newStatus = "blocked"
		}
		if newStatus != pl.Status {
			pl.Status = newStatus
			pl.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			_ = planner.Save(p, pl)
			_ = telemetry.Record(p, telemetry.Event{Type: "plan.status", Workspace: []string{pl.Workspace}, Data: map[string]any{"planId": pl.ID, "status": pl.Status}})
		}
	}
	return nil
}

func Status(p paths.Paths, planID string) (map[string]any, error) {
	cfg, err := LoadConfig(p)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"config": cfg}
	if strings.TrimSpace(planID) != "" {
		pl, err := planner.Load(p, planID)
		if err != nil {
			return nil, err
		}
		out["plan"] = pl
		var ts []model.TaskRecord
		all, _ := tasks.List(p)
		for _, t := range all {
			if t.PlanID == planID {
				ts = append(ts, t)
			}
		}
		out["tasks"] = ts
	}
	inbox, _ := Inbox(p)
	out["inbox"] = inbox
	return out, nil
}

func Inbox(p paths.Paths) ([]string, error) {
	entries, err := os.ReadDir(p.ReplanInbox)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			out = append(out, filepath.Join(p.ReplanInbox, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

func proposalFingerprint(p model.ReplanProposal) string {
	b, _ := json.Marshal(struct {
		Reason  string               `json:"reason"`
		Actions []model.ReplanAction `json:"actions"`
	}{p.Reason, p.Actions})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:8])
}

func nodeDependencies(plan model.ExecutionPlan, nodeID string) []string {
	for _, n := range plan.Nodes {
		if n.ID == nodeID {
			return append([]string(nil), n.DependsOn...)
		}
	}
	return nil
}

func riskAllowed(risk, max string) bool {
	r, ok := riskRank(normalizeRisk(risk))
	if !ok {
		return false
	}
	m, ok := riskRank(max)
	return ok && r <= m
}
func riskRank(r string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(r)) {
	case "read-low":
		return 1, true
	case "write-medium":
		return 2, true
	case "execution-high":
		return 3, true
	case "external-write-high":
		return 4, true
	case "destructive-critical":
		return 5, true
	default:
		return 0, false
	}
}
func normalizeRisk(r string) string {
	r = strings.ToLower(strings.TrimSpace(r))
	if _, ok := riskRank(r); ok {
		return r
	}
	return "write-medium"
}
func isReadRisk(r string) bool { return normalizeRisk(r) == "read-low" }
func countParallelWrites(actions []model.ReplanAction) int {
	n := 0
	for _, a := range actions {
		if a.Parallelizable && !isReadRisk(a.Risk) && len(a.DependsOn) == 0 {
			n++
		}
	}
	return n
}
func taskIDs(xs []model.TaskRecord) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, x.ID)
	}
	return out
}
func nonEmpty(xs []string) []string {
	var out []string
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			out = append(out, strings.TrimSpace(x))
		}
	}
	return out
}
func verificationSuffix(xs []string) string {
	xs = nonEmpty(xs)
	if len(xs) == 0 {
		return ""
	}
	return "\n\nRequired verification:\n- " + strings.Join(xs, "\n- ")
}
func safeID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
func shortPlan(id string) string {
	x := safeID(id)
	if len(x) > 18 {
		x = x[len(x)-18:]
	}
	if x == "" {
		x = "plan"
	}
	return x
}
func unique(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if strings.TrimSpace(x) != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func baseWorkspace(rec model.TaskRecord) string {
	if strings.TrimSpace(rec.BaseWorkspace) != "" {
		return rec.BaseWorkspace
	}
	return rec.Workspace
}
