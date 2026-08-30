package engineering

import (
	"testing"
)

func TestComputeProcessMetrics(t *testing.T) {
	report, err := AuditPlan(minimalValidPlan)
	if err != nil {
		t.Fatalf("AuditPlan failed: %v", err)
	}

	metrics := ComputeProcessMetrics(report)
	if metrics.TotalTasks != 2 {
		t.Errorf("expected 2 total tasks, got %d", metrics.TotalTasks)
	}
	if metrics.DoneTasks != 1 {
		t.Errorf("expected 1 done task, got %d", metrics.DoneTasks)
	}
	if metrics.ReadyTasks != 1 {
		t.Errorf("expected 1 ready task, got %d", metrics.ReadyTasks)
	}
	if metrics.CompletionRatePct != 50.0 {
		t.Errorf("expected 50.0%% completion rate, got %f", metrics.CompletionRatePct)
	}
	if metrics.TotalFindings != 1 {
		t.Errorf("expected 1 total finding, got %d", metrics.TotalFindings)
	}
	if metrics.ResolvedFindings != 1 {
		t.Errorf("expected 1 resolved finding, got %d", metrics.ResolvedFindings)
	}
	if metrics.OpenFindings != 0 {
		t.Errorf("expected 0 open findings, got %d", metrics.OpenFindings)
	}
	if !metrics.AuditValid {
		t.Errorf("expected AuditValid=true")
	}
	if metrics.PlanDigest == "" {
		t.Errorf("expected non-empty PlanDigest")
	}
}

func TestComputeProcessMetricsLivePlan(t *testing.T) {
	path, _, managed, err := LocatePlan("")
	if err != nil || !managed {
		t.Fatalf("LocatePlan failed: %v", err)
	}
	report, err := AuditPlanFile(path)
	if err != nil {
		t.Fatalf("AuditPlanFile failed: %v", err)
	}

	metrics := ComputeProcessMetrics(report)
	if metrics.TotalTasks < 25 {
		t.Errorf("expected at least 25 tasks in live plan, got %d", metrics.TotalTasks)
	}
	if metrics.DoneTasks < 15 {
		t.Errorf("expected at least 15 done tasks in live plan, got %d", metrics.DoneTasks)
	}
	if metrics.CompletionRatePct <= 0.0 || metrics.CompletionRatePct > 100.0 {
		t.Errorf("invalid completion rate: %f", metrics.CompletionRatePct)
	}
	if !metrics.AuditValid {
		t.Errorf("expected AuditValid=true on live plan")
	}
}
