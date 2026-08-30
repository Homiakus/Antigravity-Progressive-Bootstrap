package task

import (
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestMutationsKillDefects(t *testing.T) {
	base := baseEnvelope()
	originalDigest, err := base.Digest()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("mutant: uppercase plan digest accepted", func(t *testing.T) {
		env := base
		env.PlanDigest = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF"
		if err := env.Validate(); err == nil {
			t.Fatal("mutant survival: uppercase plan digest was accepted")
		}
	})

	t.Run("mutant: role alteration ignored in digest", func(t *testing.T) {
		env := base
		env.Role = "coordinator" // changed from worker
		d, err := env.Digest()
		if err != nil {
			t.Fatal(err)
		}
		if d == originalDigest {
			t.Fatal("mutant survival: role alteration did not change envelope digest")
		}
	})

	t.Run("mutant: read-only workspace alteration ignored in digest", func(t *testing.T) {
		env := base
		env.Workspace.ReadOnly = false // changed from true
		env.Workspace.Isolated = true
		env.Workspace.IsolationRoot = "c:/repo/.scratch"
		d, err := env.Digest()
		if err != nil {
			t.Fatal(err)
		}
		if d == originalDigest {
			t.Fatal("mutant survival: workspace ReadOnly change did not change envelope digest")
		}
	})

	t.Run("mutant: forbidden capabilities ignored in digest", func(t *testing.T) {
		env := base
		env.ForbiddenCapabilities = []string{"git_push"}
		d, err := env.Digest()
		if err != nil {
			t.Fatal(err)
		}
		if d == originalDigest {
			t.Fatal("mutant survival: forbidden capabilities change did not change envelope digest")
		}
	})

	t.Run("mutant: context ref digest alteration ignored in digest", func(t *testing.T) {
		env := base
		env.ContextRefs = []harnessmodel.ContextRef{
			{ID: "ref1", URI: "uri://1", Digest: samplePlanDigest},
		}
		d1, err := env.Digest()
		if err != nil {
			t.Fatal(err)
		}

		envMutated := env
		envMutated.ContextRefs = []harnessmodel.ContextRef{
			{ID: "ref1", URI: "uri://1", Digest: "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"},
		}
		d2, err := envMutated.Digest()
		if err != nil {
			t.Fatal(err)
		}
		if d1 == d2 {
			t.Fatal("mutant survival: context ref digest change did not change envelope digest")
		}
	})

	t.Run("mutant: plan drift single character bypass", func(t *testing.T) {
		plan := []byte("plan text content")
		env := base
		env.PlanDigest = harnessmodel.ComputePlanDigest(plan)

		mutatedPlan := []byte("plan text content.") // single dot appended
		if err := CheckPlanDrift(env, mutatedPlan); err == nil {
			t.Fatal("mutant survival: single character change in plan was not detected as drift")
		}
	})

	t.Run("mutant: negative timeout accepted", func(t *testing.T) {
		env := base
		env.Timeout = -5 * time.Minute
		if err := env.Validate(); err == nil {
			t.Fatal("mutant survival: negative timeout was accepted")
		}
	})
}
