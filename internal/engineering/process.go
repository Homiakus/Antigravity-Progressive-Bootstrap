package engineering

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const PlanFileName = "MASTER_PLAN.md"

var (
	taskHeadingRE    = regexp.MustCompile(`^###\s+(T-\d{3,})\b`)
	findingHeadingRE = regexp.MustCompile(`^###\s+(F-\d{3,})\b`)
	statusRE         = regexp.MustCompile(`(?i)\*\*Status:\*\*\s*([A-Za-z_-]+)`)
	taskIDRE         = regexp.MustCompile(`^T-\d{3,}$`)
	findingIDRE      = regexp.MustCompile(`^F-\d{3,}$`)
)

var requiredCompletionEvidence = []string{
	"task",
	"preflight",
	"characterization",
	"edge-space",
	"tests",
	"mutation",
	"race",
	"static",
	"security",
	"compatibility",
	"performance",
	"findings",
	"self-review",
	"plan-reconcile",
	"process-review",
	"push-main",
	"checkpoint",
}

// CompletionEvidence is a validated, digest-bound view of the engineering
// evidence supplied to the autonomous completion gate.
type CompletionEvidence struct {
	Managed     bool
	PlanPath    string
	PlanDigest  string
	TaskID      string
	FindingIDs  []string
	Categories  map[string][]string
	Verification []string
}

// LocatePlan searches start and its parents for MASTER_PLAN.md. A repository
// without a living plan remains supported; the stricter engineering contract is
// activated only when the plan is present.
func LocatePlan(start string) (string, []byte, bool, error) {
	if strings.TrimSpace(start) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", nil, false, err
		}
		start = cwd
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", nil, false, err
	}
	if info, statErr := os.Stat(abs); statErr == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
	}

	for {
		candidate := filepath.Join(abs, PlanFileName)
		b, readErr := os.ReadFile(candidate)
		if readErr == nil {
			return candidate, b, true, nil
		}
		if !os.IsNotExist(readErr) {
			return "", nil, false, readErr
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", nil, false, nil
		}
		abs = parent
	}
}

// RequiredCompletionEvidence exposes the semantic evidence classes so tests and
// adapters can prove that every mandatory gate is represented.
func RequiredCompletionEvidence() []string {
	return append([]string(nil), requiredCompletionEvidence...)
}

// ValidateCompletion applies the executable portion of the living-plan
// engineering process. It intentionally validates observable evidence and plan
// linkage rather than pretending to prove that a command was truthful.
func ValidateCompletion(start string, verification []string) (CompletionEvidence, error) {
	path, planBytes, managed, err := LocatePlan(start)
	if err != nil {
		return CompletionEvidence{}, err
	}
	result := CompletionEvidence{
		Managed:      managed,
		PlanPath:     path,
		Categories:   map[string][]string{},
		Verification: append([]string(nil), verification...),
	}
	if !managed {
		return result, nil
	}

	for _, raw := range verification {
		item := strings.TrimSpace(raw)
		idx := strings.Index(item, ":")
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(item[:idx]))
		value := strings.TrimSpace(item[idx+1:])
		if key == "" || value == "" {
			continue
		}
		result.Categories[key] = append(result.Categories[key], value)
	}

	var missing []string
	for _, key := range requiredCompletionEvidence {
		if !hasMeaningfulValue(result.Categories[key]) {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return result, fmt.Errorf("living-plan completion evidence missing or empty: %s", strings.Join(missing, ", "))
	}

	tasks, findings := parsePlanIDs(string(planBytes))
	taskValues := result.Categories["task"]
	if len(taskValues) != 1 {
		return result, fmt.Errorf("exactly one task:T-XXX evidence item is required")
	}
	result.TaskID = strings.TrimSpace(taskValues[0])
	if !taskIDRE.MatchString(result.TaskID) {
		return result, fmt.Errorf("invalid living-plan task id %q", result.TaskID)
	}
	status, ok := tasks[result.TaskID]
	if !ok {
		return result, fmt.Errorf("task %s is not declared in %s", result.TaskID, PlanFileName)
	}
	if !strings.EqualFold(status, "DONE") {
		return result, fmt.Errorf("task %s must be DONE in %s before completion; status=%q", result.TaskID, PlanFileName, status)
	}

	findingValues := result.Categories["findings"]
	for _, value := range findingValues {
		for _, token := range splitIDs(value) {
			if strings.EqualFold(token, "none") {
				continue
			}
			if !findingIDRE.MatchString(token) {
				return result, fmt.Errorf("invalid finding id %q", token)
			}
			if _, ok := findings[token]; !ok {
				return result, fmt.Errorf("finding %s is not declared in %s", token, PlanFileName)
			}
			result.FindingIDs = append(result.FindingIDs, token)
		}
	}
	result.FindingIDs = uniqueSorted(result.FindingIDs)

	sum := sha256.Sum256(planBytes)
	result.PlanDigest = hex.EncodeToString(sum[:])
	result.Verification = append(result.Verification, "plan-digest:"+result.PlanDigest)
	return result, nil
}

func hasMeaningfulValue(values []string) bool {
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}
		normalized := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(v, ".")))
		if normalized == "n/a" || normalized == "na" || normalized == "none" || normalized == "skipped" {
			continue
		}
		return true
	}
	return false
}

func parsePlanIDs(plan string) (map[string]string, map[string]struct{}) {
	tasks := map[string]string{}
	findings := map[string]struct{}{}
	currentTask := ""
	for _, line := range strings.Split(plan, "\n") {
		trimmed := strings.TrimSpace(line)
		if match := taskHeadingRE.FindStringSubmatch(trimmed); len(match) == 2 {
			currentTask = match[1]
			if _, ok := tasks[currentTask]; !ok {
				tasks[currentTask] = ""
			}
			continue
		}
		if match := findingHeadingRE.FindStringSubmatch(trimmed); len(match) == 2 {
			currentTask = ""
			findings[match[1]] = struct{}{}
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			currentTask = ""
			continue
		}
		if currentTask != "" && tasks[currentTask] == "" {
			if match := statusRE.FindStringSubmatch(trimmed); len(match) == 2 {
				tasks[currentTask] = strings.ToUpper(match[1])
			}
		}
	}
	return tasks, findings
}

func splitIDs(value string) []string {
	value = strings.NewReplacer(",", " ", ";", " ", "|", " ").Replace(value)
	return strings.Fields(value)
}

func uniqueSorted(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		set[value] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// ProcessContract is intentionally concise enough for repeated injection while
// preserving the non-negotiable semantics of the Living Plan Autonomous
// Execution & Self-Improving Engineering Loop.
func ProcessContract() string {
	return `LIVING PLAN ENGINEERING PROCESS — MANDATORY WHEN MASTER_PLAN.md EXISTS

MASTER_PLAN.md is the only execution roadmap. Repository behavior, reproducible tests/experiments and security/correctness invariants outrank the plan; reconcile the plan when evidence changes assumptions. Never create a parallel roadmap.

Per iteration:
SYNCHRONIZE -> OBSERVE -> UNDERSTAND -> SELECT exactly one T-XXX -> mark IN_PROGRESS -> PRE-FLIGHT -> CHARACTERIZE -> IMPLEMENT minimal root-cause fix -> VERIFY cheap-to-expensive -> ATTACK SOLUTION -> LEARN -> IMPROVE CODE/TESTS/PLAN/PROCESS -> mark DONE -> COMMIT -> re-check remote main -> PUSH MAIN without force -> verify remote head -> CHECKPOINT -> recalculate priority -> REPEAT.

Unexpected substantial problems are never patched silently: evidence -> F-XXX -> impact/blast-radius/invariants -> MASTER_PLAN reconciliation -> dependency/priority decision -> implementation. Repeated symptoms require root-cause escalation before patchwork fixes.

Pre-flight must explicitly cover root cause, invariants, change surface, protected surface, observable contract, characterization, compatibility, failure modes, rollback and verification. Critical edge analysis projects INPUT x STATE x CONCURRENCY x TIMING x FAILURE x PERMISSIONS x CONFIGURATION x EXTERNAL STATE x VERSION x PLATFORM x RESOURCE PRESSURE using pairwise baseline plus high-risk N-wise/property/fuzz/fault-injection cases.

Verification ladder: formatter -> targeted tests -> targeted race/property/fuzz -> package tests -> static analysis -> integration -> compatibility -> security -> full suite -> mutation -> benchmark -> milestone/system qualification. A flaky retry is not a pass. Security defaults fail closed. Persistence changes must prove crash/replay/migration behavior. Performance changes require measurement.

Before completion perform adversarial architecture review, simplification/debt-deletion review, and process-improvement review (detection, prevention, feedback speed, automation, signal quality, test quality, planning, context). Reconcile MASTER_PLAN.md before completion.

For a managed repository, agctl state complete requires categorized --verify evidence for: task, preflight, characterization, edge-space, tests, mutation, race, static, security, compatibility, performance, findings, self-review, plan-reconcile, process-review, push-main, checkpoint. Use explicit reasoned "n/a: ..." only when a gate is genuinely not applicable; bare n/a/none/skipped is rejected. findings must be "none: no unexpected substantial finding" or declared F-XXX IDs. task must be exactly one declared T-XXX whose plan status is DONE. The completion state is automatically bound to the SHA-256 digest of MASTER_PLAN.md.

Never force-push. Never publish a knowingly broken/insecure/flaky/incomplete main. If main moved, inspect semantic overlap, integrate safely, re-run relevant verification, then normal push. External blockers block only the affected task; continue independent READY work. Stop only at convergence, universal external blockage, an unsafe non-inferable business decision, or explicit user stop.`
}
