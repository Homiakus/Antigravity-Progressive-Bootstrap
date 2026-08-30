package engineering

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditMutationsKillDefects(t *testing.T) {
	t.Run("mutant: duplicate task ID permitted", func(t *testing.T) {
		dupPlan := minimalValidPlan + "\n### T-002 — Duplicate Heading\n**Status:** TODO.\n"
		report, err := AuditPlan(dupPlan)
		if err == nil || !report.HasErrors {
			t.Fatal("mutant survival: duplicate task ID was admitted without error")
		}
	})

	t.Run("mutant: lowercase or non-canonical status permitted", func(t *testing.T) {
		badStatusPlan := strings.Replace(minimalValidPlan, "**Status:** READY.", "**Status:** ready_for_action.", 1)
		report, err := AuditPlan(badStatusPlan)
		if err == nil || !report.HasErrors {
			t.Fatal("mutant survival: non-canonical status was admitted without error")
		}
	})

	t.Run("mutant: missing checkpoint field permitted", func(t *testing.T) {
		badCheckpointPlan := strings.Replace(minimalValidPlan, "`NEW PROCESS LEARNING:` Learning 1.", "", 1)
		report, err := AuditPlan(badCheckpointPlan)
		if err == nil || !report.HasErrors {
			t.Fatal("mutant survival: missing checkpoint field was admitted without error")
		}
	})

	t.Run("mutant: dangling dependency permitted", func(t *testing.T) {
		danglingPlan := strings.Replace(minimalValidPlan, "Dependencies: T-001.", "Dependencies: T-9999.", 1)
		report, err := AuditPlan(danglingPlan)
		if err == nil || !report.HasErrors {
			t.Fatal("mutant survival: dangling dependency was admitted without error")
		}
	})

	t.Run("mutant: circular dependency permitted", func(t *testing.T) {
		cyclePlan := strings.Replace(minimalValidPlan, "### T-001 — First task\n**Status:** DONE. **Priority:** P0.", "### T-001 — First task\n**Status:** DONE. **Priority:** P0. Dependencies: T-002.", 1)
		report, err := AuditPlan(cyclePlan)
		if err == nil || !report.HasErrors {
			t.Fatal("mutant survival: circular dependency was admitted without error")
		}
	})

	t.Run("mutant: missing preflight file for IN_PROGRESS task permitted", func(t *testing.T) {
		tmpDir := t.TempDir()
		planPath := filepath.Join(tmpDir, PlanFileName)
		inProgPlan := strings.Replace(minimalValidPlan, "**Status:** READY.", "**Status:** IN_PROGRESS.", 1)
		if err := os.WriteFile(planPath, []byte(inProgPlan), 0644); err != nil {
			t.Fatal(err)
		}

		report, err := AuditPlanFile(planPath)
		if err == nil || !report.HasErrors {
			t.Fatal("mutant survival: IN_PROGRESS task without pre-flight was admitted without error")
		}
	})
}
