package engineering

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePlan(t *testing.T, root, taskStatus string) {
	t.Helper()
	plan := `# MASTER PLAN

### F-027 — Process completion was not machine enforced
**Status:** Resolved. **Severity:** High.

### T-027 — Enforce living engineering process
**Status:** ` + taskStatus + `. **Priority:** P0.
`
	if err := os.WriteFile(filepath.Join(root, PlanFileName), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
}

func completeEvidence() []string {
	return []string{
		"task:T-027",
		"preflight:root cause and invariants recorded",
		"characterization:regression evidence captured before modification",
		"edge-space:pairwise plus high-risk crash/concurrency cases reviewed",
		"tests:go test ./... passed",
		"mutation:semantic omission matrix killed required-gate mutants",
		"race:go test -race ./... passed",
		"static:go vet ./... passed",
		"security:trust boundaries and fail-closed defaults reviewed",
		"compatibility:no public API break",
		"performance:n/a: no hot-path behavior changed",
		"findings:F-027",
		"self-review:root cause fixed without duplicate source of truth",
		"plan-reconcile:MASTER_PLAN updated with task, finding and iteration result",
		"process-review:missing completion categories are now executable guards",
		"push-main:remote main head verified after normal push",
		"checkpoint:continuation state recorded in MASTER_PLAN",
	}
}

func TestValidateCompletionUnmanagedRepositoryPreservesLegacyBehavior(t *testing.T) {
	root := t.TempDir()
	got, err := ValidateCompletion(root, []string{"go test ./... passed"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Managed {
		t.Fatal("repository without MASTER_PLAN.md must not be treated as managed")
	}
}

func TestValidateCompletionRequiresDeclaredDoneTaskAndBindsPlanDigest(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, "DONE")
	got, err := ValidateCompletion(root, completeEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Managed || got.TaskID != "T-027" {
		t.Fatalf("unexpected completion evidence: %+v", got)
	}
	if len(got.PlanDigest) != 64 {
		t.Fatalf("expected sha256 plan digest, got %q", got.PlanDigest)
	}
	if last := got.Verification[len(got.Verification)-1]; !strings.HasPrefix(last, "plan-digest:") {
		t.Fatalf("completion evidence is not digest-bound: %q", last)
	}
	if len(got.FindingIDs) != 1 || got.FindingIDs[0] != "F-027" {
		t.Fatalf("unexpected finding linkage: %v", got.FindingIDs)
	}
}

func TestValidateCompletionRejectsTaskNotDone(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, "VERIFYING")
	_, err := ValidateCompletion(root, completeEvidence())
	if err == nil || !strings.Contains(err.Error(), "must be DONE") {
		t.Fatalf("expected DONE gate, got %v", err)
	}
}

func TestValidateCompletionRejectsUndeclaredFinding(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, "DONE")
	evidence := completeEvidence()
	for i := range evidence {
		if strings.HasPrefix(evidence[i], "findings:") {
			evidence[i] = "findings:F-999"
		}
	}
	_, err := ValidateCompletion(root, evidence)
	if err == nil || !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("expected undeclared finding error, got %v", err)
	}
}

func TestValidateCompletionRejectsBareNA(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, "DONE")
	evidence := completeEvidence()
	for i := range evidence {
		if strings.HasPrefix(evidence[i], "performance:") {
			evidence[i] = "performance:n/a"
		}
	}
	_, err := ValidateCompletion(root, evidence)
	if err == nil || !strings.Contains(err.Error(), "performance") {
		t.Fatalf("expected reasoned n/a gate, got %v", err)
	}
}

// This is a deterministic test-of-tests sentinel: omitting any one mandatory
// evidence class must make the completion contract observably fail. A mutant
// that deletes a required category from the guard is therefore killed here.
func TestEveryRequiredEvidenceCategoryIsObservable(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, "DONE")
	baseline := completeEvidence()
	for _, required := range RequiredCompletionEvidence() {
		required := required
		t.Run(required, func(t *testing.T) {
			var mutated []string
			for _, item := range baseline {
				if strings.HasPrefix(item, required+":") {
					continue
				}
				mutated = append(mutated, item)
			}
			if _, err := ValidateCompletion(root, mutated); err == nil {
				t.Fatalf("completion survived omission of required evidence %q", required)
			}
		})
	}
}

func TestLocatePlanWalksParents(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, "DONE")
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	path, _, ok, err := LocatePlan(nested)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || path != filepath.Join(root, PlanFileName) {
		t.Fatalf("unexpected plan location: ok=%v path=%q", ok, path)
	}
}
