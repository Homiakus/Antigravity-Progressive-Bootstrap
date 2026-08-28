package engineering

import (
	"strings"
	"testing"
)

func checkpointPlan(task string) string {
	return `# MASTER PLAN

### T-029 — publication proof
**Status:** DONE.

### Context Compression Checkpoint — after T-029

` + "`CURRENT QUALIFIED MILESTONE:` publication proof verified.  \n" +
		"`CRITICAL INVARIANTS:` I-008,I-009.  \n" +
		"`COMPLETED THIS ITERATION:` " + task + ".  \n" +
		"`NEXT TASK:` T-030.  \n" +
		"`WHY NEXT:` structural audit.  \n" +
		"`VERIFICATION COMMANDS:` go test ./....  \n" +
		"`IMPORTANT DECISIONS:` validator does not push.  \n" +
		"`NEW PROCESS LEARNING:` verify evidence.\n"
}

func TestValidatePlanCheckpoint(t *testing.T) {
	if err := ValidatePlanCheckpoint(checkpointPlan("T-029"), "T-029"); err != nil {
		t.Fatal(err)
	}
}

func TestLatestPlanCheckpointMustMatchCurrentTask(t *testing.T) {
	plan := checkpointPlan("T-028") + "\n\n### Context Compression Checkpoint — newer\n\n" +
		"`CURRENT QUALIFIED MILESTONE:` newer.  \n" +
		"`CRITICAL INVARIANTS:` I-008.  \n" +
		"`COMPLETED THIS ITERATION:` T-028.  \n" +
		"`NEXT TASK:` T-029.  \n" +
		"`WHY NEXT:` proof.  \n" +
		"`VERIFICATION COMMANDS:` go test.  \n" +
		"`IMPORTANT DECISIONS:` none.  \n" +
		"`NEW PROCESS LEARNING:` stale cannot pass.\n"
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
