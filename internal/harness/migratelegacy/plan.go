package migratelegacy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/harness/compiler"
	"github.com/homiakus/agctl/internal/harness/events"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	legacymodel "github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/planner"
	"github.com/homiakus/agctl/internal/tasks"
)

type Bundle struct {
	SourceID          string
	SourceFingerprint string
	Definition        harnessmodel.WorkflowDefinition
	Run               harnessmodel.WorkflowRun
	Revisions         []harnessmodel.GraphRevision
	NodeRuns          []harnessmodel.NodeRun
	Events            []events.Event
	Warnings          []string
}

type Report struct {
	Planned         int      `json:"planned"`
	Imported        int      `json:"imported"`
	AlreadyImported int      `json:"alreadyImported"`
	Standalone      int      `json:"standalone"`
	OrphanTasks     []string `json:"orphanTasks,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
}

type deterministicGenerator struct {
	seed string
	at   time.Time
}

func (g deterministicGenerator) New(kind harnessmodel.IDKind) (string, error) {
	return deterministicID(kind, g.seed, g.at), nil
}

func deterministicID(kind harnessmodel.IDKind, seed string, at time.Time) string {
	if at.IsZero() {
		at = time.UnixMilli(0).UTC()
	}
	sum := sha256.Sum256([]byte(string(kind) + "\x00" + seed))
	return fmt.Sprintf("%s_%013d_%s", kind, at.UTC().UnixMilli(), hex.EncodeToString(sum[:10]))
}

func BuildBundles(plans []legacymodel.ExecutionPlan, records []legacymodel.TaskRecord) ([]Bundle, Report, error) {
	byPlan := make(map[string][]legacymodel.TaskRecord)
	var standalone []legacymodel.TaskRecord
	knownPlans := make(map[string]struct{}, len(plans))
	for _, p := range plans {
		knownPlans[p.ID] = struct{}{}
	}
	var report Report
	for _, r := range records {
		if strings.TrimSpace(r.PlanID) == "" {
			standalone = append(standalone, r)
			continue
		}
		if _, ok := knownPlans[r.PlanID]; !ok {
			report.OrphanTasks = append(report.OrphanTasks, r.ID)
			continue
		}
		byPlan[r.PlanID] = append(byPlan[r.PlanID], r)
	}

	bundles := make([]Bundle, 0, len(plans)+len(standalone))
	for _, p := range plans {
		bundle, err := BuildPlanBundle(p, byPlan[p.ID])
		if err != nil {
			return nil, report, fmt.Errorf("legacy plan %s: %w", p.ID, err)
		}
		bundles = append(bundles, bundle)
	}
	for _, r := range standalone {
		bundle, err := BuildStandaloneBundle(r)
		if err != nil {
			return nil, report, fmt.Errorf("legacy standalone task %s: %w", r.ID, err)
		}
		bundles = append(bundles, bundle)
		report.Standalone++
	}
	sort.Strings(report.OrphanTasks)
	report.Planned = len(bundles)
	return bundles, report, nil
}

func BuildPlanBundle(plan legacymodel.ExecutionPlan, records []legacymodel.TaskRecord) (Bundle, error) {
	createdAt, err := parseRequiredTime(plan.CreatedAt, "plan.createdAt")
	if err != nil {
		return Bundle{}, err
	}
	sorted := append([]legacymodel.TaskRecord(nil), records...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].CreatedAt != sorted[j].CreatedAt {
			return sorted[i].CreatedAt < sorted[j].CreatedAt
		}
		return sorted[i].ID < sorted[j].ID
	})
	fingerprint, err := sourceFingerprint(plan, sorted)
	if err != nil {
		return Bundle{}, err
	}
	draft, err := planner.HarnessDraft(plan)
	if err != nil {
		return Bundle{}, err
	}
	if draft.Metadata == nil {
		draft.Metadata = map[string]string{}
	}
	draft.Metadata["legacySourceFingerprint"] = fingerprint
	draft.Metadata["legacyImportVersion"] = "1"
	def, err := compiler.Compile(draft, compiler.Options{IDs: deterministicGenerator{seed: "definition:" + plan.ID, at: createdAt}, Now: func() time.Time { return createdAt }})
	if err != nil {
		return Bundle{}, fmt.Errorf("compile legacy workflow: %w", err)
	}
	runID := harnessmodel.WorkflowRunID(deterministicID(harnessmodel.IDWorkflowRun, "run:"+plan.ID, createdAt))

	nodeSet := make(map[string]struct{}, len(def.Nodes))
	for _, n := range def.Nodes {
		nodeSet[string(n.ID)] = struct{}{}
	}
	for _, r := range sorted {
		if strings.TrimSpace(r.NodeID) == "" {
			return Bundle{}, fmt.Errorf("task %s has no nodeId", r.ID)
		}
		if _, ok := nodeSet[r.NodeID]; !ok {
			return Bundle{}, fmt.Errorf("task %s references node %s missing from plan", r.ID, r.NodeID)
		}
	}

	maxRevision := plan.Revision
	for _, r := range sorted {
		if r.Revision > maxRevision {
			maxRevision = r.Revision
		}
	}
	if maxRevision < 1 {
		maxRevision = 1
	}
	revisions := buildRevisions(plan, runID, maxRevision, createdAt)

	statusByTask := make(map[string]string, len(sorted))
	for _, r := range sorted {
		statusByTask[r.ID] = r.Status
	}
	generation := map[string]int{}
	nodeRuns := make([]harnessmodel.NodeRun, 0, len(sorted))
	eventsOut := make([]events.Event, 0, len(sorted)+1)
	warnings := []string{}
	for _, r := range sorted {
		generation[r.NodeID]++
		gen := generation[r.NodeID]
		created, warning := parseLegacyTime(r.CreatedAt, createdAt)
		if warning != "" {
			warnings = append(warnings, fmt.Sprintf("task %s: %s", r.ID, warning))
		}
		updated, warning := parseLegacyTime(r.UpdatedAt, created)
		if warning != "" {
			warnings = append(warnings, fmt.Sprintf("task %s: %s", r.ID, warning))
		}
		remaining := 0
		for _, dep := range r.Dependencies {
			if !legacyDependencySatisfied(statusByTask[dep]) {
				remaining++
			}
		}
		state := mapTaskState(r.Status, remaining)
		nrID := harnessmodel.NodeRunID(deterministicID(harnessmodel.IDNodeRun, "node-run:"+plan.ID+":"+r.NodeID+fmt.Sprintf(":%d", gen), created))
		nodeRuns = append(nodeRuns, harnessmodel.NodeRun{ID: nrID, WorkflowRunID: runID, NodeID: harnessmodel.NodeID(r.NodeID), GraphRevision: clampRevision(r.Revision, maxRevision), Generation: gen, CreatedAt: created, UpdatedAt: updated, State: state, RemainingDependencies: remaining})
		payload, _ := json.Marshal(map[string]any{
			"legacyTaskId":       r.ID,
			"legacyStatus":       r.Status,
			"legacyAttemptCount": r.Attempts,
			"historyIncomplete":  true,
			"startedAt":          r.StartedAt,
			"finishedAt":         r.FinishedAt,
			"exitCode":           r.ExitCode,
			"error":              r.Error,
			"outputLogOmitted":   strings.TrimSpace(r.OutputLog) != "",
		})
		eventsOut = append(eventsOut, events.Event{ID: harnessmodel.EventID(deterministicID(harnessmodel.IDEvent, "legacy-task:"+r.ID, updated)), WorkflowRunID: runID, Type: "LegacyTaskImported", Timestamp: updated, EntityType: "node_run", EntityID: string(nrID), PayloadVersion: 1, Payload: payload})
	}
	runState := importedWorkflowState(nodeRuns)
	runUpdated := createdAt
	for _, nr := range nodeRuns {
		if nr.UpdatedAt.After(runUpdated) {
			runUpdated = nr.UpdatedAt
		}
	}
	run := harnessmodel.WorkflowRun{ID: runID, DefinitionID: def.ID, DefinitionVersion: def.Version, State: runState, CurrentGraphRevision: maxRevision, CreatedAt: createdAt, UpdatedAt: runUpdated}
	markerPayload, _ := json.Marshal(map[string]any{"legacySourceId": plan.ID, "sourceFingerprint": fingerprint, "historyIncomplete": true, "nodeRuns": len(nodeRuns)})
	eventsOut = append(eventsOut, events.Event{ID: harnessmodel.EventID(deterministicID(harnessmodel.IDEvent, "legacy-import-complete:"+plan.ID, runUpdated)), WorkflowRunID: runID, Type: "LegacyImportCompleted", Timestamp: runUpdated, EntityType: "workflow_run", EntityID: string(runID), PayloadVersion: 1, Payload: markerPayload})
	return Bundle{SourceID: plan.ID, SourceFingerprint: fingerprint, Definition: def, Run: run, Revisions: revisions, NodeRuns: nodeRuns, Events: eventsOut, Warnings: warnings}, nil
}

func BuildStandaloneBundle(r legacymodel.TaskRecord) (Bundle, error) {
	created, warning := parseLegacyTime(r.CreatedAt, time.UnixMilli(0).UTC())
	planID := "legacy-standalone-" + r.ID
	plan := legacymodel.ExecutionPlan{ID: planID, Revision: 0, Status: "legacy-standalone", Prompt: r.Prompt, Workspace: r.Workspace, CreatedAt: created.Format(time.RFC3339Nano), GeneratedBy: "legacy-import", Nodes: []legacymodel.PlanNode{{ID: "task", Title: "Imported standalone task", Objective: r.Prompt, Agent: r.Agent, Resources: r.Resources, Tags: r.Tags}}}
	copyRecord := r
	copyRecord.PlanID = planID
	copyRecord.NodeID = "task"
	copyRecord.Dependencies = nil
	bundle, err := BuildPlanBundle(plan, []legacymodel.TaskRecord{copyRecord})
	if err != nil {
		return Bundle{}, err
	}
	if warning != "" {
		bundle.Warnings = append(bundle.Warnings, "standalone task "+r.ID+": "+warning)
	}
	return bundle, nil
}

func sourceFingerprint(plan legacymodel.ExecutionPlan, records []legacymodel.TaskRecord) (string, error) {
	payload, err := json.Marshal(struct {
		Plan  legacymodel.ExecutionPlan `json:"plan"`
		Tasks []legacymodel.TaskRecord  `json:"tasks"`
	}{plan, records})
	if err != nil {
		return "", fmt.Errorf("marshal legacy source fingerprint: %w", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func buildRevisions(plan legacymodel.ExecutionPlan, runID harnessmodel.WorkflowRunID, max int, created time.Time) []harnessmodel.GraphRevision {
	byNumber := map[int]legacymodel.PlanRevision{}
	for _, r := range plan.RevisionHistory {
		byNumber[r.Number] = r
	}
	out := make([]harnessmodel.GraphRevision, 0, max)
	for n := 1; n <= max; n++ {
		r := byNumber[n]
		at, _ := parseLegacyTime(r.CreatedAt, created)
		reason := strings.TrimSpace(r.Reason)
		if reason == "" {
			reason = "legacy import"
		}
		parent := 0
		if n > 1 {
			parent = n - 1
		}
		out = append(out, harnessmodel.GraphRevision{WorkflowRunID: runID, Number: n, ParentNumber: parent, Reason: reason, CreatedAt: at})
	}
	return out
}

func clampRevision(v, max int) int {
	if v < 1 {
		return 1
	}
	if v > max {
		return max
	}
	return v
}

func legacyDependencySatisfied(status string) bool {
	return status == tasks.StatusSucceeded || status == tasks.StatusSuperseded
}

func mapTaskState(status string, remaining int) harnessmodel.NodeState {
	switch status {
	case tasks.StatusSucceeded:
		return harnessmodel.NodeSucceeded
	case tasks.StatusFailed:
		return harnessmodel.NodeFailed
	case tasks.StatusCancelled:
		return harnessmodel.NodeCancelled
	case tasks.StatusSuperseded:
		return harnessmodel.NodeSkipped
	case tasks.StatusRunning:
		return harnessmodel.NodeInDoubt
	case tasks.StatusBlocked:
		return harnessmodel.NodeUnschedulable
	case tasks.StatusQueued:
		if remaining == 0 {
			return harnessmodel.NodeReady
		}
		return harnessmodel.NodePendingDependencies
	default:
		return harnessmodel.NodeInDoubt
	}
}

func importedWorkflowState(nodes []harnessmodel.NodeRun) harnessmodel.WorkflowState {
	if len(nodes) == 0 {
		return harnessmodel.WorkflowPaused
	}
	allTerminal := true
	allSuccessLike := true
	allCancelled := true
	for _, n := range nodes {
		if n.State == harnessmodel.NodeInDoubt {
			return harnessmodel.WorkflowBlocked
		}
		if !n.State.Terminal() {
			allTerminal = false
		}
		if n.State != harnessmodel.NodeSucceeded && n.State != harnessmodel.NodeSkipped {
			allSuccessLike = false
		}
		if n.State != harnessmodel.NodeCancelled && n.State != harnessmodel.NodeSkipped {
			allCancelled = false
		}
	}
	if allTerminal && allSuccessLike {
		return harnessmodel.WorkflowSucceeded
	}
	if allTerminal && allCancelled {
		return harnessmodel.WorkflowCancelled
	}
	if allTerminal {
		return harnessmodel.WorkflowFailed
	}
	return harnessmodel.WorkflowPaused
}

func parseRequiredTime(v, field string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(v))
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s: %w", field, err)
	}
	return t.UTC(), nil
}

func parseLegacyTime(v string, fallback time.Time) (time.Time, string) {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback.UTC(), "timestamp missing; fallback used"
	}
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return fallback.UTC(), "invalid timestamp; fallback used"
	}
	return t.UTC(), ""
}
