package engineering

import (
	"fmt"
	"strings"
)

// Role identifies the authority boundary under which an autonomous engineering
// agent is operating. Coordinator and worker are intentionally different
// security roles, not just prompt styles.
type Role string

const (
	RoleCoordinator Role = "coordinator"
	RoleWorker      Role = "worker"
)

// Authority is the executable description of which process-level actions a
// role is allowed to own. It deliberately separates workspace mutation from
// plan/main publication authority.
type Authority struct {
	SelectPlanTask        bool
	MutateLivingPlan      bool
	ModifyWorkspace       bool
	CommitRepository      bool
	PublishMain           bool
	ContinuationCheckpoint bool
	ProposeReplan         bool
}

// RoleContract binds a role to its authority and the instruction block that is
// safe to inject into an autonomous agent.
type RoleContract struct {
	Role         Role
	Authority    Authority
	Instructions string
}

// ContractForRole returns a fail-closed process contract for the requested
// authority role.
func ContractForRole(role Role) (RoleContract, error) {
	switch role {
	case RoleCoordinator:
		return RoleContract{
			Role: role,
			Authority: Authority{
				SelectPlanTask:         true,
				MutateLivingPlan:       true,
				ModifyWorkspace:        true,
				CommitRepository:       true,
				PublishMain:            true,
				ContinuationCheckpoint: true,
				ProposeReplan:          true,
			},
			Instructions: ProcessContract(),
		}, nil
	case RoleWorker:
		contract := RoleContract{
			Role: role,
			Authority: Authority{
				SelectPlanTask:         false,
				MutateLivingPlan:       false,
				ModifyWorkspace:        true,
				CommitRepository:       false,
				PublishMain:            false,
				ContinuationCheckpoint: false,
				ProposeReplan:          true,
			},
			Instructions: workerProcessContract,
		}
		if err := ValidateRoleContract(contract); err != nil {
			return RoleContract{}, err
		}
		return contract, nil
	default:
		return RoleContract{}, fmt.Errorf("unknown engineering role %q", role)
	}
}

// ValidateRoleContract is an architecture guard. A worker contract fails closed
// if later code accidentally grants any coordinator publication authority.
func ValidateRoleContract(contract RoleContract) error {
	if contract.Role != RoleWorker {
		return nil
	}
	a := contract.Authority
	if a.SelectPlanTask || a.MutateLivingPlan || a.CommitRepository || a.PublishMain || a.ContinuationCheckpoint {
		return fmt.Errorf("worker engineering contract contains coordinator authority")
	}
	if !a.ModifyWorkspace {
		return fmt.Errorf("worker engineering contract must allow delegated workspace work")
	}
	if !a.ProposeReplan {
		return fmt.Errorf("worker engineering contract must preserve evidence-based replanning proposals")
	}
	return nil
}

// WorkerProcessContract exposes the bounded contract for headless DAG workers.
func WorkerProcessContract() string {
	contract, err := ContractForRole(RoleWorker)
	if err != nil {
		panic(err)
	}
	return contract.Instructions
}

// WrapWorkerTask binds one delegated plan node to the worker authority contract.
func WrapWorkerTask(prompt, planID, nodeID string) string {
	prompt = strings.TrimSpace(prompt)
	return fmt.Sprintf(`%s

DELEGATED NODE
Plan ID: %s
Node ID: %s

NODE OBJECTIVE:
%s`, WorkerProcessContract(), strings.TrimSpace(planID), strings.TrimSpace(nodeID), prompt)
}

const workerProcessContract = `LIVING PLAN ENGINEERING PROCESS — HEADLESS WORKER ROLE

ENGINEERING ROLE: WORKER
Task-selection authority: DENIED
Living-plan mutation authority: DENIED
Repository commit authority: DENIED
Main publication authority: DENIED
Continuation-checkpoint authority: DENIED
Workspace implementation authority: ALLOWED
Evidence-based replan proposal authority: ALLOWED

You own exactly one delegated DAG node. Do not choose the next T-XXX, mark MASTER_PLAN.md task states, reconcile the global roadmap, create coordinator continuation checkpoints, commit repository history, integrate branches, or push any branch/main. Those are coordinator responsibilities.

Within the delegated node, follow the product loop rigorously: OBSERVE -> UNDERSTAND -> PRE-FLIGHT -> CHARACTERIZE -> IMPLEMENT the minimum root-cause change when writes are authorized -> VERIFY cheap-to-expensive -> ATTACK the result with relevant edge/security/concurrency/persistence/mutation thinking -> SELF-REVIEW -> REPORT EVIDENCE.

Before modifying production code, identify root cause, affected invariants, change surface, protected surface, observable contract, compatibility risk, failure modes, rollback and minimum verification. Prefer characterization/regression evidence first when practical. Never convert a failing test to green by weakening a valid contract.

For critical behavior, project INPUT x STATE x CONCURRENCY x TIMING x FAILURE x PERMISSIONS x CONFIGURATION x EXTERNAL STATE x VERSION x PLATFORM x RESOURCE PRESSURE. Use pairwise baseline plus high-risk N-wise/property/fuzz/fault-injection cases where applicable. Classify failures; do not mask flakiness with retries. Security-sensitive unknown/nil/parse/authorization failure defaults fail closed unless the delegated business contract explicitly proves otherwise.

If you discover material work outside this node, do not silently expand scope and do not edit MASTER_PLAN.md. Record concrete evidence through the supplied adaptive replan proposal mechanism. Multiple related symptoms require root-cause escalation in that proposal rather than patchwork fixes.

Return concrete implementation/test/security/performance/findings evidence to the coordinator. A successful worker result means the delegated node is qualified for coordinator review; it never means the repository is globally DONE or that main may be published.`
