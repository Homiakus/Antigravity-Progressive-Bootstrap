package engineering

import (
	"testing"
)

func BenchmarkAuditPlanMASTERPLAN(b *testing.B) {
	path, planBytes, managed, err := LocatePlan("")
	if err != nil || !managed {
		b.Fatalf("LocatePlan failed: %v", err)
	}
	_ = path
	planStr := string(planBytes)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		report, err := AuditPlan(planStr)
		if err != nil || report.HasErrors {
			b.Fatalf("AuditPlan failed: %v", err)
		}
	}
}

func BenchmarkComputeProcessMetrics(b *testing.B) {
	path, planBytes, managed, err := LocatePlan("")
	if err != nil || !managed {
		b.Fatalf("LocatePlan failed: %v", err)
	}
	_ = path
	report, err := AuditPlan(string(planBytes))
	if err != nil || report.HasErrors {
		b.Fatalf("initial audit failed: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		metrics := ComputeProcessMetrics(report)
		if !metrics.AuditValid {
			b.Fatal("invalid metrics")
		}
	}
}
