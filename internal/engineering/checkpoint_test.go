package engineering

import (
	"strings"
	"testing"
)

func checkpointFields(task string) string {
	return "`CURRENT HEAD:` published-main-head.  \n" +
		"`CURRENT QUALIFIED MILESTONE:` publication proof verified.  \n" +
		"`ARCHITECTURE:` completion validator is read-only.  \n" +
		"`CRITICAL INVARIANTS:` I-008,I-009.  \n" +
		"`COMPLETED THIS ITERATION:` " + task + ".  \n" +
		"`RESOLVED FINDINGS:` F-031.  \n" +
		"`OPEN CRITICAL/HIGH FINDINGS:` F-004,F-007.  \n" +
		"`BLOCKERS:` none.  \n" +
		"`NEXT TASK:` T-010.  \n" +
		"`WHY NEXT:` unlock critical reservation correctness.  \n" +
		"`CRITICAL FILES:` internal/engineering/process.go.  \n" +
		"`VERIFICATION COMMANDS:` go test ./....  \n" +
		"`IMPORTANT DECISIONS:` validator does not push.  \n" +
		"`REJECTED OPTIONS:` free-form publication claims.  \n" +
		"`NEW PROCESS LEARNING:` verify evidence.\n"
}

func checkpointPlan(task string) string {
	return `# MASTER PLAN

### T-029 — publication proof
**Status:** DONE.

### Context Compression Checkpoint — after T-029

` + checkpointFields(task)
}

func TestValidatePlanCheckpoint(t *testing.T) {
	if err := ValidatePlanCheckpoint(checkpointPlan("T-029"), "T-029"); err != nil {
		t.Fatal(err)
	}
}

func TestLatestPlanCheckpointMustMatchCurrentTask(t *testing.T) {
	plan := checkpointPlan("T-029") + "\n\n### Context Compression Checkpoint — newer\n\n" + checkpointFields("T-028")
	if err := ValidatePlanCheckpoint(plan, "T-029"); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("expected stale checkpoint rejection, got %v", err)
	}
}

func TestPlanCheckpointMutationSentinel(t *testing.T) {
	base := checkpointPlan("T-029")
	for _, field := range checkpointRequiredFields {
		t.Run(field, func(t *testing.T) {
			mutated := strings.Replace(base, "`"+field+":", "`REMOVED "+field+":", 1)
			if err := ValidatePlanCheckpoint(mutated, "T-029"); err == nil {
				t.Fatalf("checkpoint field mutant %q survived", field)
			}
		})
	}
}

func TestCheckpointMissingFailsClosed(t *testing.T) {
	if _, err := ParseLatestPlanCheckpoint("# MASTER PLAN\n"); err == nil {
		t.Fatal("missing checkpoint accepted")
	}
}
