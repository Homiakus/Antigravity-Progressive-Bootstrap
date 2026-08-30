package engineering

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

const minimalValidPlan = `# MASTER PLAN

## 1. Executive Summary
Overview.

## 6. DAG and Priority
` + "```text" + `
T-001 -> T-002
` + "```" + `

## 7. Atomic Tasks

### T-001 — First task
**Status:** DONE. **Priority:** P0.
Completed successfully.

### T-002 — Second task
**Status:** READY. **Priority:** P1. Dependencies: T-001.
Ready to run.

## 8. Findings

### F-001 — Sample Finding
**Status:** RESOLVED. **Severity:** HIGH.
Resolved with T-001.

## 15. Context Compression Checkpoint

### Context Compression Checkpoint — after T-001

` + "`CURRENT HEAD:`" + ` abc1234
` + "`CURRENT QUALIFIED MILESTONE:`" + ` Initial milestone qualified.
` + "`ARCHITECTURE:`" + ` Architecture overview.
` + "`CRITICAL INVARIANTS:`" + ` I-001.
` + "`COMPLETED THIS ITERATION:`" + ` T-001.
` + "`RESOLVED FINDINGS:`" + ` F-001.
` + "`OPEN CRITICAL/HIGH FINDINGS:`" + ` None.
` + "`BLOCKERS:`" + ` None.
` + "`NEXT TASK:`" + ` T-002.
` + "`WHY NEXT:`" + ` Ready.
` + "`CRITICAL FILES:`" + ` file.go.
` + "`VERIFICATION COMMANDS:`" + ` go test.
` + "`IMPORTANT DECISIONS:`" + ` Decision 1.
` + "`REJECTED OPTIONS:`" + ` None.
` + "`NEW PROCESS LEARNING:`" + ` Learning 1.
`

func TestAuditPlanValidMinimal(t *testing.T) {
	report, err := AuditPlan(minimalValidPlan)
	if err != nil {
		t.Fatalf("AuditPlan failed on valid minimal plan: %v", err)
	}
	if len(report.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(report.Tasks))
	}
	if len(report.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(report.Findings))
	}
	if report.HasErrors {
		t.Errorf("expected HasErrors=false, got true; issues=%v", report.Issues)
	}
	if len(report.DAGOrder) != 2 {
		t.Errorf("expected DAGOrder len 2, got %d", len(report.DAGOrder))
	}
}

func TestAuditPlanLiveMasterPlan(t *testing.T) {
	// Locate repository's actual MASTER_PLAN.md
	path, _, managed, err := LocatePlan("")
	if err != nil {
		t.Fatalf("LocatePlan failed: %v", err)
	}
	if !managed {
		t.Fatal("expected managed MASTER_PLAN.md")
	}

	report, err := AuditPlanFile(path)
	if err != nil {
		t.Fatalf("AuditPlanFile failed on live MASTER_PLAN.md: %v; issues:\n%v", err, report.Issues)
	}
	if report.HasErrors {
		t.Fatalf("live MASTER_PLAN.md has errors: %v", report.Issues)
	}
	if len(report.Tasks) == 0 {
		t.Error("expected non-zero tasks in live MASTER_PLAN.md")
	}
}

func TestAuditPlanDuplicateTaskID(t *testing.T) {
	planWithDup := minimalValidPlan + `
### T-001 — Duplicate Task
**Status:** TODO. **Priority:** P2.
`
	report, err := AuditPlan(planWithDup)
	if err == nil {
		t.Fatal("expected error on duplicate task ID, got nil")
	}
	if !report.HasErrors {
		t.Fatal("expected HasErrors=true on duplicate task ID")
	}
	var found bool
	for _, issue := range report.Issues {
		if issue.Category == IssueDuplicateID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected IssueDuplicateID, got issues: %v", report.Issues)
	}
}

func TestAuditPlanDuplicateFindingID(t *testing.T) {
	planWithDupFinding := minimalValidPlan + `
### F-001 — Duplicate Finding
**Status:** OPEN. **Severity:** MEDIUM.
`
	report, err := AuditPlan(planWithDupFinding)
	if err == nil {
		t.Fatal("expected error on duplicate finding ID, got nil")
	}
	if !report.HasErrors {
		t.Fatal("expected HasErrors=true on duplicate finding ID")
	}
}

func TestAuditPlanInvalidTaskStatus(t *testing.T) {
	invalidStatusPlan := `# MASTER PLAN

## 7. Atomic Tasks

### T-001 — Task With Bad Status
**Status:** INVALID_STATUS. **Priority:** P0.

## 15. Context Compression Checkpoint
### Context Compression Checkpoint — after T-001
` + "`CURRENT HEAD:`" + ` abc
` + "`CURRENT QUALIFIED MILESTONE:`" + ` M
` + "`ARCHITECTURE:`" + ` A
` + "`CRITICAL INVARIANTS:`" + ` I
` + "`COMPLETED THIS ITERATION:`" + ` T-001
` + "`RESOLVED FINDINGS:`" + ` F
` + "`OPEN CRITICAL/HIGH FINDINGS:`" + ` O
` + "`BLOCKERS:`" + ` B
` + "`NEXT TASK:`" + ` N
` + "`WHY NEXT:`" + ` W
` + "`CRITICAL FILES:`" + ` C
` + "`VERIFICATION COMMANDS:`" + ` V
` + "`IMPORTANT DECISIONS:`" + ` D
` + "`REJECTED OPTIONS:`" + ` R
` + "`NEW PROCESS LEARNING:`" + ` L
`
	report, err := AuditPlan(invalidStatusPlan)
	if err == nil {
		t.Fatal("expected error for invalid task status, got nil")
	}
	var found bool
	for _, issue := range report.Issues {
		if issue.Category == IssueInvalidStatus {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected IssueInvalidStatus, got issues: %v", report.Issues)
	}
}

func TestAuditPlanDanglingDependency(t *testing.T) {
	danglingDepPlan := `# MASTER PLAN

## 7. Atomic Tasks

### T-001 — Task
**Status:** READY. **Priority:** P0. Dependencies: T-999.

## 15. Context Compression Checkpoint
### Context Compression Checkpoint — after T-001
` + "`CURRENT HEAD:`" + ` abc
` + "`CURRENT QUALIFIED MILESTONE:`" + ` M
` + "`ARCHITECTURE:`" + ` A
` + "`CRITICAL INVARIANTS:`" + ` I
` + "`COMPLETED THIS ITERATION:`" + ` T-001
` + "`RESOLVED FINDINGS:`" + ` F
` + "`OPEN CRITICAL/HIGH FINDINGS:`" + ` O
` + "`BLOCKERS:`" + ` B
` + "`NEXT TASK:`" + ` N
` + "`WHY NEXT:`" + ` W
` + "`CRITICAL FILES:`" + ` C
` + "`VERIFICATION COMMANDS:`" + ` V
` + "`IMPORTANT DECISIONS:`" + ` D
` + "`REJECTED OPTIONS:`" + ` R
` + "`NEW PROCESS LEARNING:`" + ` L
`
	report, err := AuditPlan(danglingDepPlan)
	if err == nil {
		t.Fatal("expected error for dangling dependency, got nil")
	}
	var found bool
	for _, issue := range report.Issues {
		if issue.Category == IssueDanglingDependency {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected IssueDanglingDependency, got issues: %v", report.Issues)
	}
}

func TestAuditPlanCyclicDependency(t *testing.T) {
	cyclePlan := `# MASTER PLAN

## 7. Atomic Tasks

### T-001 — Task 1
**Status:** READY. **Priority:** P0. Dependencies: T-002.

### T-002 — Task 2
**Status:** READY. **Priority:** P0. Dependencies: T-001.

## 15. Context Compression Checkpoint
### Context Compression Checkpoint — after T-001
` + "`CURRENT HEAD:`" + ` abc
` + "`CURRENT QUALIFIED MILESTONE:`" + ` M
` + "`ARCHITECTURE:`" + ` A
` + "`CRITICAL INVARIANTS:`" + ` I
` + "`COMPLETED THIS ITERATION:`" + ` T-001
` + "`RESOLVED FINDINGS:`" + ` F
` + "`OPEN CRITICAL/HIGH FINDINGS:`" + ` O
` + "`BLOCKERS:`" + ` B
` + "`NEXT TASK:`" + ` N
` + "`WHY NEXT:`" + ` W
` + "`CRITICAL FILES:`" + ` C
` + "`VERIFICATION COMMANDS:`" + ` V
` + "`IMPORTANT DECISIONS:`" + ` D
` + "`REJECTED OPTIONS:`" + ` R
` + "`NEW PROCESS LEARNING:`" + ` L
`
	report, err := AuditPlan(cyclePlan)
	if err == nil {
		t.Fatal("expected error for cyclic dependency, got nil")
	}
	var found bool
	for _, issue := range report.Issues {
		if issue.Category == IssueDependencyCycle {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected IssueDependencyCycle, got issues: %v", report.Issues)
	}
}

func TestAuditPlanMissingPreflight(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, PlanFileName)

	inProgressPlan := `# MASTER PLAN

## 7. Atomic Tasks

### T-001 — In Progress Task
**Status:** IN_PROGRESS. **Priority:** P0.

## 15. Context Compression Checkpoint
### Context Compression Checkpoint — after T-001
` + "`CURRENT HEAD:`" + ` abc
` + "`CURRENT QUALIFIED MILESTONE:`" + ` M
` + "`ARCHITECTURE:`" + ` A
` + "`CRITICAL INVARIANTS:`" + ` I
` + "`COMPLETED THIS ITERATION:`" + ` T-001
` + "`RESOLVED FINDINGS:`" + ` F
` + "`OPEN CRITICAL/HIGH FINDINGS:`" + ` O
` + "`BLOCKERS:`" + ` B
` + "`NEXT TASK:`" + ` N
` + "`WHY NEXT:`" + ` W
` + "`CRITICAL FILES:`" + ` C
` + "`VERIFICATION COMMANDS:`" + ` V
` + "`IMPORTANT DECISIONS:`" + ` D
` + "`REJECTED OPTIONS:`" + ` R
` + "`NEW PROCESS LEARNING:`" + ` L
`
	if err := os.WriteFile(planPath, []byte(inProgressPlan), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := AuditPlanFile(planPath)
	if err == nil {
		t.Fatal("expected error for missing preflight file of IN_PROGRESS task, got nil")
	}
	var found bool
	for _, issue := range report.Issues {
		if issue.Category == IssueMissingPreflightFile {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected IssueMissingPreflightFile, got issues: %v", report.Issues)
	}
}

func TestAuditPlanConcurrentExecution(t *testing.T) {
	const goroutines = 64
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			report, err := AuditPlan(minimalValidPlan)
			if err != nil || report.HasErrors {
				t.Errorf("concurrent AuditPlan failed: %v", err)
			}
		}()
	}
	wg.Wait()
}
