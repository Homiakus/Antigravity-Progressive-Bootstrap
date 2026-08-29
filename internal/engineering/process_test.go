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

### F-028 — Completion inferred repository from ambient cwd
**Status:** Resolved. **Severity:** High.

### T-027 — Enforce living engineering process
**Status:** ` + taskStatus + `. **Priority:** P0.

### Context Compression Checkpoint — after T-027

` + checkpointFields("T-027")
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
		"findings:F-027 F-028",
		"self-review:root cause fixed without duplicate source of truth",
		"plan-reconcile:MASTER_PLAN updated with task, findings and iteration result",
		"process-review:missing completion categories and ambient cwd coupling are now executable guards",
		"push-main:" + validPublicationProofText(),
		"checkpoint:plan",
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

func TestExplicitUnmanagedWorkspaceDoesNotInheritFallbackPlan(t *testing.T) {
	managedFallback := t.TempDir()
	writePlan(t, managedFallback, "DONE")
	unmanagedWorkspace := t.TempDir()
	got, err := ValidateCompletionForWorkspaces([]string{unmanagedWorkspace}, managedFallback, []string{"synthetic verification"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Managed {
		t.Fatalf("explicit unmanaged workspace inherited ambient plan %q", got.PlanPath)
	}
}

func TestMultipleIndependentLivingPlansFailClosed(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()
	writePlan(t, left, "DONE")
	writePlan(t, right, "DONE")
	_, err := ValidateCompletionForWorkspaces([]string{left, right}, "", completeEvidence())
	if err == nil || !strings.Contains(err.Error(), "multiple MASTER_PLAN.md") {
		t.Fatalf("expected multi-plan fail-closed error, got %v", err)
	}
}

func TestMultipleWorkspacesUnderSamePlanAreAllowed(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, "DONE")
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ValidateCompletionForWorkspaces([]string{left, right}, "", completeEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Managed || got.PlanPath != filepath.Join(root, PlanFileName) {
		t.Fatalf("unexpected shared-plan resolution: %+v", got)
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
	if len(got.FindingIDs) != 2 || got.FindingIDs[0] != "F-027" || got.FindingIDs[1] != "F-028" {
		t.Fatalf("unexpected finding linkage: %v", got.FindingIDs)
	}
}

func TestValidateCompletionRejectsFreeFormPublicationClaim(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, "DONE")
	evidence := completeEvidence()
	for i := range evidence {
		if strings.HasPrefix(evidence[i], "push-main:") {
			evidence[i] = "push-main:verified"
		}
	}
	_, err := ValidateCompletion(root, evidence)
	if err == nil || !strings.Contains(err.Error(), "publication proof") {
		t.Fatalf("expected structured publication proof error, got %v", err)
	}
}

func TestValidateCompletionRejectsFreeFormCheckpointClaim(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, "DONE")
	evidence := completeEvidence()
	for i := range evidence {
		if strings.HasPrefix(evidence[i], "checkpoint:") {
			evidence[i] = "checkpoint:recorded elsewhere"
		}
	}
	_, err := ValidateCompletion(root, evidence)
	if err == nil || !strings.Contains(err.Error(), "checkpoint:plan") {
		t.Fatalf("expected repository checkpoint proof error, got %v", err)
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

func TestValidateCompletionAllowsReasonedNoFindings(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, "DONE")
	evidence := completeEvidence()
	for i := range evidence {
		if strings.HasPrefix(evidence[i], "findings:") {
			evidence[i] = "findings:none: no unexpected substantial finding"
		}
	}
	got, err := ValidateCompletion(root, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.FindingIDs) != 0 {
		t.Fatalf("reasoned no-findings evidence produced IDs: %v", got.FindingIDs)
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
