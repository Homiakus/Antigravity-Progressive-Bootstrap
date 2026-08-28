package planner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/capability"
	"github.com/homiakus/agctl/internal/engineering"
	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/project"
	"github.com/homiakus/agctl/internal/tasks"
	"github.com/homiakus/agctl/internal/telemetry"
)

const GeneratedBy = "agctl-planner/3.2.1"

// Create builds a deterministic multi-agent DAG from task text, project signals,
// and the current capability registry. It intentionally remains explainable and
// conservative; Antigravity can refine the plan during execution.
func Create(p paths.Paths, prompt, workspace string) (model.ExecutionPlan, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return model.ExecutionPlan{}, fmt.Errorf("prompt is required")
	}
	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return model.ExecutionPlan{}, err
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return model.ExecutionPlan{}, fmt.Errorf("workspace does not exist: %s", abs)
	}

	det, _ := project.Detect(abs)
	reg, _ := capability.Build(p, []string{abs})
	ranked := capability.Rank(reg, prompt, 16)

	id := fmt.Sprintf("plan-%s", time.Now().UTC().Format("20060102T150405.000000000Z"))
	now := time.Now().UTC().Format(time.RFC3339Nano)
	plan := model.ExecutionPlan{
		ID:          id,
		Revision:    0,
		Status:      "planned",
		UpdatedAt:   now,
		Prompt:      prompt,
		Workspace:   abs,
		Profiles:    append([]string(nil), det.Profiles...),
		CreatedAt:   now,
		GeneratedBy: GeneratedBy,
	}

	ctx := classify(prompt, det.Profiles, ranked)
	plan.CapabilityHints = append([]string(nil), ctx.RelevantCaps...)
	plan.Nodes = buildNodes(prompt, ctx)
	if err := validate(plan); err != nil {
		return plan, err
	}
	if err := Save(p, plan); err != nil {
		return plan, err
	}
	_ = telemetry.Record(p, telemetry.Event{Type: "plan.created", Workspace: []string{plan.Workspace}, Data: map[string]any{"planId": plan.ID, "nodes": len(plan.Nodes), "profiles": plan.Profiles}})
	return plan, nil
}

type context struct {
	Complex         bool
	Research        bool
	Security        bool
	Browser         bool
	Infrastructure  bool
	Editorial       bool
	Migration       bool
	Audit           bool
	Profiles        map[string]bool
	Checks          []string
	RelevantCaps    []string
	LikelyWriteTask bool
}

func classify(prompt string, profiles []string, ranked []capability.RankedCapability) context {
	q := strings.ToLower(prompt)
	c := context{Profiles: map[string]bool{}, LikelyWriteTask: true}
	for _, x := range profiles {
		c.Profiles[x] = true
		if pr, ok := project.FindProfile(x); ok {
			c.Checks = append(c.Checks, pr.Checks...)
		}
	}
	c.Research = hasAny(q, "исслед", "research", "latest", "актуаль", "документац", "documentation", "compare", "сравни")
	c.Security = hasAny(q, "security", "безопас", "threat", "уязв", "vulnerab", "auth", "permission") || c.Profiles["infra"]
	c.Browser = c.Profiles["web"] || hasAny(q, "ui", "ux", "browser", "брауз", "playwright", "frontend", "форма", "responsive", "mobile")
	c.Infrastructure = c.Profiles["infra"] || hasAny(q, "terraform", "kubernetes", "docker", "deploy", "infra", "инфраструкт")
	c.Editorial = hasAny(q, "редактор", "proofread", "copyedit", "rewrite", "перепиши текст", "нейрослоп", "no-ai-slop")
	c.Migration = hasAny(q, "migrat", "мигр", "перенеси", "перепиши на")
	c.Audit = hasAny(q, "audit", "аудит", "review", "ревью", "анализ код")
	c.Complex = c.Research || c.Security || c.Browser || c.Infrastructure || c.Migration || c.Audit || wordCount(q) > 35
	if hasAny(q, "только проверь", "только анализ", "ничего не меняй", "do not edit", "read-only", "без изменений") {
		c.LikelyWriteTask = false
	}
	for _, r := range ranked {
		if r.Score < 4 {
			continue
		}
		c.RelevantCaps = append(c.RelevantCaps, r.Capability.Kind+":"+r.Capability.ID)
		if len(c.RelevantCaps) >= 8 {
			break
		}
	}
	c.Checks = unique(c.Checks)
	return c
}

func buildNodes(prompt string, c context) []model.PlanNode {
	var nodes []model.PlanNode
	add := func(n model.PlanNode) { nodes = append(nodes, n) }

	if c.Complex {
		add(model.PlanNode{
			ID:        "inspect",
			Title:     "Inspect architecture and constraints",
			Objective: "Inspect the repository and convert the delegated request into a concrete Definition of Done. Identify affected boundaries, contracts, risks, and existing verification paths. Capability hints: " + strings.Join(c.RelevantCaps, ", ") + ". Task: " + prompt,
			Agent:     "architect",
			Tags:      []string{"analysis", "read-only"},
			Resources: model.ResourceRequest{CPUWeight: 15, ReadOnly: true},
			Risk:      "read-low",
		})
	}
	if c.Research {
		add(model.PlanNode{
			ID:        "research",
			Title:     "Research authoritative context",
			Objective: "Gather only the external/current documentation or evidence materially needed for the task. Prefer primary sources and return concrete constraints to implementation. Task: " + prompt,
			Agent:     "researcher",
			Tags:      []string{"research", "read-only"},
			Resources: model.ResourceRequest{CPUWeight: 10, ReadOnly: true},
			Risk:      "read-low",
		})
	}

	deps := []string{}
	if nodeExists(nodes, "inspect") {
		deps = append(deps, "inspect")
	}
	if nodeExists(nodes, "research") {
		deps = append(deps, "research")
	}

	if c.LikelyWriteTask {
		add(model.PlanNode{
			ID:        "implement",
			Title:     "Implement delegated outcome",
			Objective: "Implement the requested outcome completely, preserving existing architecture unless a justified change is required. Consume findings from prerequisite nodes. Task: " + prompt,
			Agent:     "implementer",
			DependsOn: deps,
			Tags:      []string{"implementation", "write"},
			Resources: model.ResourceRequest{CPUWeight: 45, BuildSlots: boolInt(c.Infrastructure || c.Profiles["go"] || c.Profiles["web"]), ExclusiveWorkspace: true},
			Risk:      "write-medium",
		})
	}

	verifyDeps := []string{}
	if c.LikelyWriteTask {
		verifyDeps = []string{"implement"}
	} else {
		verifyDeps = append(verifyDeps, deps...)
	}
	checks := c.Checks
	if len(checks) == 0 {
		checks = []string{"targeted tests", "regression review"}
	}
	vr := model.ResourceRequest{CPUWeight: 35, BuildSlots: 1, ReadOnly: true}
	if c.Browser {
		vr.BrowserSlots = 1
	}
	add(model.PlanNode{
		ID:           "verify",
		Title:        "Independent verification",
		Objective:    "Independently verify the requested outcome and capture concrete evidence. Do not accept implementation claims without checks.",
		Agent:        "test-engineer",
		DependsOn:    verifyDeps,
		Verification: checks,
		Tags:         []string{"verification", "qa"},
		Resources:    vr,
		Risk:         "read-low",
	})

	if c.Security {
		securityDeps := verifyDeps
		if c.LikelyWriteTask {
			securityDeps = []string{"implement"}
		}
		add(model.PlanNode{
			ID:        "security",
			Title:     "Security and trust-boundary review",
			Objective: "Review the implemented/current solution for trust-boundary, secrets, permission, injection, supply-chain and destructive-action risks. Report only actionable evidence.",
			Agent:     "security-reviewer",
			DependsOn: securityDeps,
			Tags:      []string{"security", "review", "read-only"},
			Resources: model.ResourceRequest{CPUWeight: 20, ReadOnly: true},
			Risk:      "read-low",
		})
	}

	finalDeps := []string{"verify"}
	if nodeExists(nodes, "security") {
		finalDeps = append(finalDeps, "security")
	}
	add(model.PlanNode{
		ID:        "review",
		Title:     "Final requirement and regression gate",
		Objective: "Review the final state against the original request, affected contracts and verification evidence. Identify omissions/regressions; if fixes are required, return precise actionable findings rather than approving prematurely.",
		Agent:     "code-reviewer",
		DependsOn: finalDeps,
		Tags:      []string{"review", "completion-gate", "read-only"},
		Resources: model.ResourceRequest{CPUWeight: 20, ReadOnly: true},
		Risk:      "read-low",
	})
	return nodes
}

func Save(p paths.Paths, plan model.ExecutionPlan) error {
	if p.PlansRoot == "" {
		return fmt.Errorf("plans root is not configured")
	}
	return jsonx.WriteAtomic(filepath.Join(p.PlansRoot, plan.ID+".json"), plan, p.BackupsRoot)
}

func Load(p paths.Paths, id string) (model.ExecutionPlan, error) {
	plan, err := jsonx.Read(filepath.Join(p.PlansRoot, id+".json"), model.ExecutionPlan{})
	if err != nil {
		return plan, err
	}
	if plan.ID == "" {
		return plan, fmt.Errorf("plan %s not found", id)
	}
	return plan, nil
}

func List(p paths.Paths) ([]model.ExecutionPlan, error) {
	entries, err := os.ReadDir(p.PlansRoot)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []model.ExecutionPlan
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		pl, err := jsonx.Read(filepath.Join(p.PlansRoot, e.Name()), model.ExecutionPlan{})
		if err == nil && pl.ID != "" {
			out = append(out, pl)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

// Enqueue materializes a plan into dependency-aware headless tasks.
func Enqueue(p paths.Paths, plan model.ExecutionPlan, priority int, nativeGoal bool) ([]model.TaskRecord, error) {
	if err := validate(plan); err != nil {
		return nil, err
	}
	byNode := map[string]string{}
	var created []model.TaskRecord
	for _, n := range topo(plan.Nodes) {
		deps := make([]string, 0, len(n.DependsOn))
		for _, dep := range n.DependsOn {
			id, ok := byNode[dep]
			if !ok {
				return created, fmt.Errorf("node %s dependency %s was not materialized", n.ID, dep)
			}
			deps = append(deps, id)
		}
		prompt := n.Objective
		if len(n.Verification) > 0 {
			prompt += "\n\nRequired verification:\n- " + strings.Join(n.Verification, "\n- ")
		}
		prompt = engineering.WrapWorkerTask(prompt, plan.ID, n.ID)
		nodeWorkspace := plan.Workspace
		if strings.TrimSpace(n.Workspace) != "" {
			nodeWorkspace = n.Workspace
		}
		rec, err := tasks.AddAdvanced(p, tasks.Spec{
			Prompt:         prompt,
			Workspace:      nodeWorkspace,
			Priority:       priority,
			NativeGoal:     nativeGoal,
			Agent:          n.Agent,
			Tags:           append([]string{"plan:" + plan.ID, "node:" + n.ID, "engineering-role:worker"}, n.Tags...),
			PlanID:         plan.ID,
			NodeID:         n.ID,
			Dependencies:   deps,
			Resources:      n.Resources,
			Revision:       plan.Revision,
			DynamicDepth:   n.Depth,
			BaseWorkspace:  plan.Workspace,
			WorktreeBranch: n.WorktreeBranch,
		})
		if err != nil {
			return created, err
		}
		byNode[n.ID] = rec.ID
		created = append(created, rec)
	}
	_ = telemetry.Record(p, telemetry.Event{Type: "plan.enqueued", Workspace: []string{plan.Workspace}, Data: map[string]any{"planId": plan.ID, "tasks": len(created)}})
	return created, nil
}

func validate(plan model.ExecutionPlan) error {
	if plan.ID == "" || plan.Workspace == "" || strings.TrimSpace(plan.Prompt) == "" {
		return fmt.Errorf("plan requires id, workspace and prompt")
	}
	seen := map[string]bool{}
	for _, n := range plan.Nodes {
		if n.ID == "" {
			return fmt.Errorf("plan contains node with empty id")
		}
		if seen[n.ID] {
			return fmt.Errorf("duplicate node %s", n.ID)
		}
		seen[n.ID] = true
	}
	for _, n := range plan.Nodes {
		for _, d := range n.DependsOn {
			if !seen[d] {
				return fmt.Errorf("node %s depends on unknown node %s", n.ID, d)
			}
		}
	}
	if len(topo(plan.Nodes)) != len(plan.Nodes) {
		return fmt.Errorf("plan DAG contains a dependency cycle")
	}
	return nil
}

func topo(nodes []model.PlanNode) []model.PlanNode {
	byID := map[string]model.PlanNode{}
	indegree := map[string]int{}
	children := map[string][]string{}
	for _, n := range nodes {
		byID[n.ID] = n
		indegree[n.ID] = len(n.DependsOn)
		for _, d := range n.DependsOn {
			children[d] = append(children[d], n.ID)
		}
	}
	var q []string
	for id, d := range indegree {
		if d == 0 {
			q = append(q, id)
		}
	}
	sort.Strings(q)
	var out []model.PlanNode
	for len(q) > 0 {
		id := q[0]
		q = q[1:]
		out = append(out, byID[id])
		for _, child := range children[id] {
			indegree[child]--
			if indegree[child] == 0 {
				q = append(q, child)
				sort.Strings(q)
			}
		}
	}
	return out
}

func nodeExists(nodes []model.PlanNode, id string) bool {
	for _, n := range nodes {
		if n.ID == id {
			return true
		}
	}
	return false
}
func hasAny(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}
func wordCount(s string) int { return len(strings.Fields(s)) }
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func unique(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
