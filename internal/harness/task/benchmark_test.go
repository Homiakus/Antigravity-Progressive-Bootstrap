package task

import (
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func BenchmarkTaskEnvelopeDigest1000Operations(b *testing.B) {
	env := harnessmodel.TaskEnvelope{
		ID:           "tenv_bench_001",
		TaskID:       "T-013",
		PlanDigest:   samplePlanDigest,
		TaskClass:    harnessmodel.TaskClassCodegen,
		Title:        "Benchmark task envelope",
		Objective:    "Measure throughput of envelope validation and digest calculation",
		Instructions: "Run canonical serialization and compute SHA-256 digest across 1000 iterations",
		Workspace: harnessmodel.WorkspaceSpec{
			RootPath: "c:/repo",
			RepoID:   "repo1",
		},
		Role:                 "worker",
		RequiredCapabilities: []string{"tools", "file_edit", "bash"},
		ContextRefs: []harnessmodel.ContextRef{
			{ID: "ref1", URI: "file:///c:/repo/file1.go", Role: harnessmodel.ContextRoleInputCode},
			{ID: "ref2", URI: "file:///c:/repo/file2.go", Role: harnessmodel.ContextRoleSpecification},
		},
		Metadata: map[string]string{
			"env":     "production",
			"lane":    "fast",
			"version": "1.0",
		},
		CreatedAt: time.Now().UTC(),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			d, err := env.Digest()
			if err != nil || len(d) != 64 {
				b.Fatalf("Digest failed: %v", err)
			}
		}
	}
}
