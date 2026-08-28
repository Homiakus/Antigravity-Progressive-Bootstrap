package engineering

import (
	"strings"
	"testing"
)

func TestWorkerRoleHasNoCoordinatorAuthority(t *testing.T) {
	contract, err := ContractForRole(RoleWorker)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateRoleContract(contract); err != nil {
		t.Fatal(err)
	}
	a := contract.Authority
	if a.SelectPlanTask || a.MutateLivingPlan || a.CommitRepository || a.PublishMain || a.ContinuationCheckpoint {
		t.Fatalf("worker leaked coordinator authority: %+v", a)
	}
	if !a.ModifyWorkspace || !a.ProposeReplan {
		t.Fatalf("worker lost required delegated authority: %+v", a)
	}
}

func TestCoordinatorRetainsPublicationAuthority(t *testing.T) {
	contract, err := ContractForRole(RoleCoordinator)
	if err != nil {
		t.Fatal(err)
	}
	a := contract.Authority
	if !a.SelectPlanTask || !a.MutateLivingPlan || !a.CommitRepository || !a.PublishMain || !a.ContinuationCheckpoint {
		t.Fatalf("coordinator contract incomplete: %+v", a)
	}
	if !strings.Contains(contract.Instructions, "PUSH MAIN without force") {
		t.Fatal("coordinator instructions lost explicit publication discipline")
	}
}

func TestWorkerAuthorityMutationSentinel(t *testing.T) {
	base, err := ContractForRole(RoleWorker)
	if err != nil {
		t.Fatal(err)
	}
	mutants := map[string]func(*Authority){
		"select-task": func(a *Authority) { a.SelectPlanTask = true },
		"mutate-plan": func(a *Authority) { a.MutateLivingPlan = true },
		"commit": func(a *Authority) { a.CommitRepository = true },
		"publish-main": func(a *Authority) { a.PublishMain = true },
		"checkpoint": func(a *Authority) { a.ContinuationCheckpoint = true },
	}
	for name, mutate := range mutants {
		t.Run(name, func(t *testing.T) {
			candidate := base
			mutate(&candidate.Authority)
			if err := ValidateRoleContract(candidate); err == nil {
				t.Fatalf("authority mutant %s survived", name)
			}
		})
	}
}

func TestWorkerContractIsExplicitlyBounded(t *testing.T) {
	text := WorkerProcessContract()
	for _, want := range []string{
		"ENGINEERING ROLE: WORKER",
		"Task-selection authority: DENIED",
		"Living-plan mutation authority: DENIED",
		"Repository commit authority: DENIED",
		"Main publication authority: DENIED",
		"Evidence-based replan proposal authority: ALLOWED",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("worker contract missing %q", want)
		}
	}
}

func TestUnknownRoleFailsClosed(t *testing.T) {
	if _, err := ContractForRole(Role("mystery")); err == nil {
		t.Fatal("unknown engineering role must fail closed")
	}
}

func TestWrapWorkerTaskPreservesDelegationIdentity(t *testing.T) {
	got := WrapWorkerTask("verify invariant", "plan-1", "node-2")
	for _, want := range []string{"ENGINEERING ROLE: WORKER", "Plan ID: plan-1", "Node ID: node-2", "verify invariant"} {
		if !strings.Contains(got, want) {
			t.Fatalf("wrapped task missing %q", want)
		}
	}
}
