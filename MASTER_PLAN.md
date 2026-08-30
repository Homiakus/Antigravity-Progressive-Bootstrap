# MASTER PLAN — Multi-Runtime Agent Control Plane

> Repository: `Homiakus/Antigravity-Progressive-Bootstrap`  
> Plan status: ACTIVE / LIVING  
> Semantic source of truth: this file. Git history is the audit trail for superseded verbose iteration detail; SQLite/generated indexes are execution ledgers or rebuildable projections, never semantic authority.

## 1. Mission

Evolve `agctl` into a quota-aware, multi-runtime engineering control plane that distributes atomic work between Antigravity and Codex according to capability, provider/account capacity, reset horizon, authoritative session context, reliability, risk and estimated demand while preserving one living plan and one fenced path to verified `main`.

Target loop:

`OBSERVE -> RECONCILE PLAN -> READY TASK -> TASK ENVELOPE -> ESTIMATE -> REFRESH PROVIDERS -> SESSION/CONTEXT BROKER -> SELECT PROVIDER/MODEL/SESSION -> RESERVE -> CLAIM WORKER/WORKSPACE -> EXECUTE -> VERIFY -> REVIEW -> RECONCILE PLAN -> SETTLE USAGE -> COMMIT COORDINATOR -> VERIFIED FAST-FORWARD MAIN -> CHECKPOINT -> NEXT`

Current qualified milestones:

- Provider observation/runtime foundation T-001..T-007 and T-009 is published.
- T-027 makes living-plan completion executable and workspace-bound.
- T-028 separates coordinator and ordinary headless-worker authority; published as `595f0d002aaf97ee74472d0bdab84af14fcd0bab`.
- T-029 makes publication/checkpoint evidence machine-verifiable; published as `d6fc97d19b2161db4256aafa839715d929a6eb95`.
- T-010 conservative native-unit demand estimation is published as `9d327e7e530eb940021c7a3dce11b9f33944d53f`.
- T-011 atomic provider reservations are published as `75651581908af7eb3b9b4148106fa142e5707bd9`; Linux/Windows CI, race, vet and reservation smoke gates passed on the published tree.
- T-008 session/context brokering is published in `main` through reconciled commit `9fbd31d0a1348d45acf29c7c38dab3a138e66acb`, with durable pre-flight `5a740b7e8d0b7c4d46d03ff43a9c2963317cb69c` and implementation commit `921f0fbcf4cb0fdcebaac78041f4eaceb50b9192`.
- T-012 explainable shadow selector is published in `main` with durable pre-flight `38c3708` and implementation commit `11c691d`.
- T-013 TaskEnvelope and plan digest binding is published through implementation commit `19e2c5d` (pre-flight `2c176c3`). Provider execution now receives a conversation-independent envelope bound to cryptographic plan digests.
- T-014 automatic read-only provider routing is implemented through `e69c8e0` (pre-flight `ac7ecee`). Read-only tasks route automatically through selector and commit atomic assignments/reservations without workspace modification.
- T-015 provider-aware failures and retries is implemented through `8d30f73` (pre-flight `1a17d88`). Provider errors are categorized into structured fault kinds, driving deterministic backoff, failover to alternative candidates, and transactional circuit breaker tripping.

## 2. Architecture

```text
MASTER_PLAN.md
      |
Plan Compiler / Reconciler
      |
Durable DAG -> Fair Scheduler
      |
TaskEnvelope + Demand Estimator
      |          |
      |      HistorySource
      |          |
      |    classified usage samples
      |          |
Session / Context Broker
      |
Provider Selector / Capacity Broker
      |
  +---+------------------------------+
  |                                  |
Antigravity                        Codex
status/headless                    App Server JSON-RPC
  |                                  |
authoritative session/model        no inferred thread->model affinity
context/workspace where exposed    until provider evidence exists
  +----------------+-----------------+
                   |
       provider-neutral normalization
                   |
       assignment/reservation ledger
                   |
      worker + fenced execution lease
                   |
        isolated workspace/effects
                   |
          verify/review/reconcile
                   |
          fenced Commit Coordinator
                   |
          fast-forward-only `main`
```

Orthogonal concepts MUST remain separate: `Worker`, `ProviderAccount`, `ProviderModel`, `ProviderSession`, workflow budget, provider capacity, demand estimate, provider reservation, execution lease, workspace lease and commit authority.

## 3. System Invariants

- **I-001 Plan authority:** `MASTER_PLAN.md` is semantic truth; generated projections are rebuildable and digest-bound.
- **I-002 Explainable assignment:** provider-backed attempts trace task, attempt, worker, account, model, session and plan revision.
- **I-003 Worker/provider separation:** worker location is not provider quota/account identity.
- **I-004 Budget/capacity separation:** workflow spend permission is not external provider supply.
- **I-005 Atomic reservation:** selection + quota reservation commit before provider execution.
- **I-006 Execution fencing:** provider failover never bypasses existing lease epochs.
- **I-007 Fail closed on uncertain effects:** unsafe interruption reuses effect intents + `IN_DOUBT`; no blind replay.
- **I-008 Single writer to main:** ordinary workers never autonomously integrate/push `main` in multi-runtime mode.
- **I-009 No force push:** autonomous operation never uses force or force-with-lease.
- **I-010 Dynamic discovery:** no transient model-name or opaque quota-bucket hard-coding in core logic.
- **I-011 Bounded uncertainty:** unknown/stale capacity is penalized, never treated as unlimited.
- **I-012 Incremental rollout:** observe -> shadow -> advisory -> read-only -> isolated writes -> failover -> commit coordination -> default.
- **I-013 Migration immutability:** released migration SQL/checksums are append-only.
- **I-014 Monotonic observations:** delayed provider observations cannot regress newer durable state.
- **I-015 Durable session identity:** one provider session ID cannot silently migrate between accounts.
- **I-016 Coherent provider snapshots:** one upstream snapshot cannot tear across independent capacity/model/session revisions.
- **I-017 No inferred quota mapping:** native bucket identity is preserved; bucket->model mapping requires provider evidence.
- **I-018 Authoritative session/model attribution:** `ProviderSessionSnapshot.ModelID` requires authoritative execution/provider linkage, never heuristics.
- **I-019 Complete catalog replacement:** paginated model catalogs replace durable state only after a complete successful cycle.
- **I-020 Unit-preserving headroom:** TOKENS, REQUESTS, COST, FRACTION and OPAQUE remain distinct; incompatible/overlapping windows are never summed for convenience.
- **I-021 Exhaustion proof separation:** confidence/freshness decay may produce zero effective headroom but only authoritative evidence establishes `EXHAUSTED`.
- **I-022 Reset-boundary invalidation:** observations from a previous quota period expire after reset until refreshed.
- **I-023 Durable routing history:** provider handoff supersedes/terminalizes then appends; historical decisions are never rewritten; at most one ACTIVE assignment per Attempt.
- **I-024 Reservation lifecycle fencing:** reservations start ACTIVE revision 1 and terminalize only by CAS; assignments cannot terminalize with attached ACTIVE reservations.
- **I-025 Complete reservation accounting:** feasibility uses all non-expired ACTIVE claims or an equivalent complete SQL aggregate, never a paged diagnostic subset.
- **I-026 Usage replay idempotency:** one provider/event idempotency key maps to one immutable semantic usage sample; conflicting replay fails closed.
- **I-027 Runtime quantity bounds:** OPAQUE is observable but not arithmetically reservable; FRACTION reservation/usage is bounded.
- **I-028 Executable living-plan completion:** managed completion requires categorized evidence for task, pre-flight, characterization, edge-space, tests, mutation/test-of-tests, race, static analysis, security, compatibility, performance, findings, adversarial review, plan reconciliation, process review, publication and checkpoint.
- **I-029 Workspace-bound process authority:** engineering policy resolves from persisted task workspaces, never ambient cwd; multiple independent plans fail closed.
- **I-030 Plan revision binding:** successful managed completion stores the SHA-256 digest of the reviewed `MASTER_PLAN.md`.
- **I-031 Coordinator/worker authority separation:** coordinator may select/reconcile/publish/checkpoint; ordinary workers may characterize/implement/verify/review and emit findings/replan proposals but never inherit coordinator-only authority.
- **I-032 Typed worker authority:** planner materializes every ordinary DAG node with `engineering.RoleWorker` and `engineering-role:worker`; unknown roles fail closed and authority-bit mutation tests guard denial semantics.
- **I-033 Observed publication proof:** managed completion accepts publication only when structured proof matches independently observed `main`, local HEAD/tree, remote HEAD, clean worktree and proven base ancestry; `force=true` is invalid. Verification is read-only and never performs publication.
- **I-034 Repository-resident checkpoint:** latest `Context Compression Checkpoint` in `MASTER_PLAN.md` must contain the complete recovery field set and identify the completed T-XXX; chat-only, stale or partial checkpoints cannot satisfy completion.
- **I-035 Conservative demand semantics:** demand is estimated only within the same provider and native quota metric; OPAQUE is never estimated arithmetically; empirical nearest-rank p80 is the reservation quantity; history broadens through explicit specificity levels without crossing provider/metric boundaries; cold-start demand is explicit operator data scoped at least by provider+metric and never a built-in guessed constant.
- **I-036 Session acquisition hysteresis:** a non-preferred session must satisfy the configured acquire headroom threshold; a currently preferred session may remain reusable down to a lower configured retain threshold. Thresholds are operator policy, never guessed provider constants.
- **I-037 Session-context fail closed:** a same-affinity non-closed session with insufficient, exhausted, draining or unknown context must never be silently reused; the broker returns `CHECKPOINT_AND_NEW` when replacement is allowed or `UNAVAILABLE` when account/provider policy forbids acquisition.

## 4. Findings Registry

### F-001 — Provider capacity is not a scheduler dimension
**Status:** Resolved in T-009/T-011/T-012. **Severity:** High. **Confidence:** Confirmed.  
T-009 provides normalized headroom, T-011 provides atomic feasibility/reservation, and T-012 integrates capacity and quota headroom into the deterministic selector decision matrix.

### F-002 — Codex is not yet a complete first-class execution provider
**Status:** Resolved for read-only routing; write routing waits for T-017. **Severity:** High. **Confidence:** Confirmed.  
Observation is first-class; TaskEnvelope and automatic read-only provider routing (T-013/T-014) execute read-only workloads across providers.

### F-003 — Workflow budget cannot substitute for provider quota
**Status:** Resolved by T-010/T-011 boundary. **Severity:** High. **Confidence:** Confirmed.  
Demand and external provider supply are separate native-unit concepts.

### F-004 — Durable claims existed without atomic quota reservation
**Status:** Resolved and published in T-011. **Severity:** Critical. **Confidence:** Confirmed.  
Capacity evidence + complete ACTIVE claims + feasibility + assignment/reservations now commit in one writer transaction.

### F-005 — Existing worker/lease subsystem must be reused
**Status:** Resolved by architecture. **Severity:** High.  
Provider routing stays orthogonal to workers/fenced attempt leases.

### F-006 — Existing `IN_DOUBT` + effect intents already provide unsafe-replay protection
**Status:** Resolved by architecture. **Severity:** Critical.  
T-015/T-016 reuse existing effect reconciliation rather than duplicate it.

### F-007 — Direct worker pushes are unsafe with parallel providers
**Status:** Planned / narrowed by T-028. **Severity:** Critical. **Confidence:** Strong.  
Typed worker authority denies publication semantically; T-018/T-019 still must hard-enforce it.

### F-008 — No TaskEnvelope independent of conversation history
**Status:** Resolved in T-013. **Severity:** High. **Confidence:** Confirmed.  
TaskEnvelope in `internal/harness/model` and `internal/harness/task` encapsulates complete task specifications, instructions, input context refs, workspace boundaries, and role without coupling to conversation transcripts.

### F-009 — No plan digest on provider assignments
**Status:** Resolved in T-013. **Severity:** High. **Confidence:** Confirmed.  
Provider assignments and task envelopes bind `PlanDigest` (SHA-256 hex of `MASTER_PLAN.md`). SQLite schema migration v16 adds `plan_digest` and index to `provider_assignments`. Stale plan drift is detected and rejected via `ErrStalePlan`.

### F-010 — No session/context broker
**Status:** Resolved and published in T-008. **Severity:** Medium. **Confidence:** Confirmed.  
Provider-neutral `REUSE/NEW/CHECKPOINT_AND_NEW/UNAVAILABLE` policy now uses authoritative account/model/session/context/workspace data and configured hysteresis.

### F-011 — Provider routing decisions are not fully explainable
**Status:** Resolved in T-005/T-008/T-012. **Severity:** Medium.  
T-008 emits stable reason codes; T-012 emits complete durable rationale, per-criterion score breakdowns, and candidate elimination codes.

### F-012 — Released SQLite migrations are immutable
**Status:** Resolved. **Severity:** High.  
T-003/T-005 appended v14/v15; T-013 appended v16 (`plan_digest`); prior migrations are never mutated.

### F-013 — Unified provider execution interface before TaskEnvelope would couple to conversation state
**Status:** Resolved in T-013/T-014. **Severity:** Medium. **Confidence:** Confirmed.  
TaskEnvelope defines conversation-independent execution contracts; router dispatches read-only tasks with atomic ledger commits and releases.

### F-014 — Concurrent autonomous work can move `main` during an iteration
**Status:** Partially resolved in T-029. **Severity:** High. **Confidence:** Confirmed.  
T-029 verifies publication state; T-018/T-019 still provide fenced authority and hard single-writer enforcement.

### F-015 — Delayed provider observations can regress durable state
**Status:** Resolved in T-004. **Severity:** High.

### F-016 — Independent provider reads can create a torn observation
**Status:** Resolved in T-006. **Severity:** High.

### F-017 — Codex rolling rate-limit updates are sparse and model/bucket mapping is not authoritative
**Status:** Resolved in T-007. **Severity:** High.

### F-018 — Codex thread/context telemetry does not identify selected model
**Status:** Resolved by T-007/T-008 fail-closed boundary. **Severity:** High. **Confidence:** Confirmed.  
Codex adapter emits no fabricated `ProviderSessionSnapshot`; T-008 therefore cannot infer reuse and selects `NEW` only when account/model policy otherwise permits it.

### F-019 — Partial model pagination can masquerade as model removal
**Status:** Resolved in T-007. **Severity:** High.

### F-020 — Base quota validation is insufficient for routing arithmetic
**Status:** Resolved in T-009. **Severity:** High.

### F-021 — Absolute remaining without a limit cannot safely become a fraction
**Status:** Resolved in T-009. **Severity:** High.

### F-022 — Provider reset boundaries invalidate otherwise fresh pre-reset capacity
**Status:** Resolved in T-009. **Severity:** High.

### F-023 — Paged ACTIVE-reservation reads can undercount claims
**Status:** Resolved in T-005/T-011. **Severity:** Critical.

### F-024 — Mutating one assignment record would destroy handoff history
**Status:** Resolved in T-005. **Severity:** High.

### F-025 — Provider usage redelivery can double-count settlement
**Status:** Resolved in T-005. **Severity:** High.

### F-026 — Runtime FRACTION quantities need explicit bounds
**Status:** Resolved in T-005. **Severity:** High.

### F-027 — Autonomous completion was prompt-only rather than executable
**Status:** Resolved in T-027. **Severity:** High.

### F-028 — Completion policy inferred repository identity from ambient cwd
**Status:** Resolved in T-027. **Severity:** High.

### F-029 — Monolithic full-suite gate delayed process failure localization
**Status:** Resolved in T-027. **Severity:** Medium.

### F-030 — One process prompt cannot safely govern coordinator and ordinary workers
**Status:** Resolved in T-028 at semantic/typed boundary. **Severity:** Critical.  
Hard git enforcement remains T-019.

### F-031 — Free-form `push-main` evidence can false-green unpublished or stale state
**Status:** Resolved in T-029. **Severity:** High.

### F-032 — Free-form checkpoint evidence can be stale or unrecoverable
**Status:** Resolved in T-029. **Severity:** High.

### F-033 — Durable usage rows do not carry authoritative task/repository/context classification
**Status:** Resolved by boundary in T-010; integration requirement remains. **Severity:** High. **Confidence:** Confirmed.  
T-010 requires explicit `UsageClassification`; T-012 must not infer semantic classification from opaque identifiers.

### F-034 — Complete Go materialization of ACTIVE claims was allocation-heavy
**Status:** Resolved in T-011. **Severity:** Medium. **Confidence:** Measured.  
SQLite uses equivalent complete transactional aggregation while the portable unpaged Store fallback remains intact.

## 5. Risk Register

| Risk | Level | Primary mitigation |
|---|---|---|
| quota oversubscription | Critical | T-005 + T-010 + published T-011 atomic reservation |
| simultaneous main writers | Critical | T-028 typed denial + T-018 fenced coordinator + T-019 hard policy |
| duplicate/uncertain effects | Critical | effect intents + `IN_DOUBT` + T-016 |
| moved/stale main recorded complete | High | T-029 observed publication proof; T-018/T-019 fencing |
| wrong demand population/units | High | I-035 + F-033 + explicit classification |
| stale/torn provider state | High | I-014/I-016/I-019/I-022 |
| incompatible quota arithmetic | High | I-020/I-021/I-025/I-027/I-035 |
| session context exhaustion | High | T-008 broker + I-036/I-037 |
| invented Codex session/model affinity | High | I-018 + F-018 + T-008 no-inference sentinels |
| stale plan execution | High | T-013 plan digest + I-030 completion digest |
| provider API drift | High | tolerant adapters + dynamic discovery + deterministic fixtures |
| provider flapping / estimate error | Medium | hysteresis/switch cost + p80 + specificity confidence |

## 6. Dependency DAG and Priority

```text
T001 DONE -> T002 DONE -> T003 DONE -> T004 DONE
                                      |-> T006 DONE
                                      |-> T007 DONE
                                      |-> T005 DONE
T004,T006,T007 -> T008 DONE
T004,T006,T007 -> T009 DONE
T005,T009 -> T010 DONE -> T011 DONE
T008,T011 -------------------------> T012 DONE
T012 ---------------------------------> T013 DONE -> T014 DONE -> T015 DONE -> T016 READY/NEXT
T013,T015 -> T016 -> T017 -> T018 -> T019
T012 -> T020
T004,T005,T012,T020 -> T021
T005..T019 -> T022
T011..T019 -> T023
T014..T023 -> T024 -> T025 -> T026

T027 DONE -> T028 DONE -> T029 DONE
                    |-> T030 TODO
T029 complements, but does not replace, T018/T019.
```

Priority after T-015 publication:

1. **T-016 P0 / NEXT** — safe provider handoff via existing `IN_DOUBT`.
2. **T-030 P1** — structural plan/process audit and enforcement of durable task-start ordering.
3. **T-017 P1 / BLOCKED ON T-016** — enable isolated write routing.

## 7. Atomic Tasks

### T-001 — Add provider-domain primitives
**Status:** DONE. **Priority:** P0. Commit `bf060a70833504fc5dc979181d59453276a56e0c`.

### T-002 — Define observation adapter and live registry
**Status:** DONE. **Priority:** P0. Commit `ac35b5193c735897a75cb056680215a4e3aae428`.

### T-003 — Add immutable SQLite v14 provider-observation schema
**Status:** DONE. **Priority:** P0. Commit `c5c2ebfab1814207aecb274ffb14b5ae83d9992f`.

### T-004 — Add provider observation Store contracts
**Status:** DONE. **Priority:** P0. Commit `7f6c3a888318e1cca1ea2d22c8c997051f883856`.

### T-005 — Persist assignments/reservations/usage/circuit in SQLite v15
**Status:** DONE / VERIFIED / PUBLISHED. **Priority:** P0. Publication `7fa9578795a7cf5029e80f618c9b87c3524063a2`.

### T-006 — Implement Antigravity observation adapter
**Status:** DONE. **Priority:** P0. Commit `a46eb75f947f97f62a06129e3469fe18466d6843`.

### T-007 — Implement Codex App Server observation adapter
**Status:** DONE. **Priority:** P0. Commit `cefead407f3ad4a11ac46379f2f9b72a9129d353`.

### T-008 — Build session/context broker
**Status:** DONE / VERIFIED / PUBLISHED. **Priority:** P1 / DEPENDENCY LEVERAGE. Publication `9fbd31d0a1348d45acf29c7c38dab3a138e66acb`.  
**Dependencies:** T-004,T-006,T-007 published.  
**Pre-flight lineage:** first publication-lineage commit `5a740b7e8d0b7c4d46d03ff43a9c2963317cb69c` persists `.engineering/preflight/T-008.md` with `Status: IN_PROGRESS`, scope, protected surfaces and verification plan.  
**Implementation lineage:** `921f0fbcf4cb0fdcebaac78041f4eaceb50b9192`.  
**Policy:** pure deterministic evaluator returns `REUSE`, `NEW`, `CHECKPOINT_AND_NEW` or `UNAVAILABLE`. Reuse requires active authoritative session/model linkage, matching required workspace fingerprint when supplied, enabled model/capabilities, sufficient absolute context and configured headroom threshold. `PreferredSessionID` uses lower retain threshold than acquire threshold; thresholds are caller policy and validated in `[0,1]`.  
**Draining semantics:** a DRAINING account may finish work in an already reusable session but cannot acquire replacement context. DISABLED accounts and authoritative provider `EXHAUSTED/UNAVAILABLE` fail closed.  
**Context rotation:** a same-affinity non-closed session that exists but cannot safely be reused yields `CHECKPOINT_AND_NEW` when account policy allows replacement. Unknown/zero session context is not treated as unlimited.  
**Codex boundary:** no thread->model inference. Because current Codex observation cannot authoritatively link a thread to model, it emits no session snapshots; an otherwise usable active Codex account therefore yields `NEW`, never fabricated `REUSE`.  
**StoreSource:** account/models/sessions/latest health load inside one `Store.View`; missing capacity remains `UNKNOWN`; inconsistent capacity account/provider/health fails closed. No new schema/migration.  
**Determinism:** preferred first, then remaining-context fraction, absolute headroom, recency, lexical session ID; 100 shuffled-order trials return identical decisions.  
**Tests:** broad state/model/capability/workspace/context matrix; exact acquire boundary and one-unit-below rotation; preferred hysteresis retain boundary; malformed/duplicate/mismatched snapshot rejection; Codex no-affinity sentinel; real SQLite StoreSource; missing-capacity UNKNOWN characterization; 128-way concurrent deterministic evaluator sentinel.  
**Race/property/mutation evidence:** repository-wide `go test -race ./...` PASS; randomized order/property sentinel; tests kill unsafe mutants that reuse below threshold, reuse unknown/exhausted context, ignore workspace affinity, infer Codex session affinity, allow draining replacement, accept malformed snapshot or make threshold boundary strict in the wrong direction.  
**Performance:** permanent CI gate `BenchmarkEvaluate100Sessions`; qualification on GitHub Linux AMD EPYC measured `42,494 ns/op`, `20,144 B/op`, `9 allocs/op`.  
**Verification/publication:** experimental technical tree `0fe73d979e7ad071fd76034a2c348ec5795d1fb3` passed module-tidy, targeted process gates, `go test ./...`, `go test -race ./...`, `go vet ./...`, broker + all existing benchmark smoke gates and Windows full tests. Clean publication lineage `5a740b7… -> 921f0fb… -> 9fbd31d…` was independently requalified with the same full Linux/Windows/race/vet/benchmark gate set, fresh `main` remained at T-011, ancestry was 3 ahead / 0 behind, and `main` was fast-forwarded to `9fbd31d…` with `force=false`.  
**Adversarial review:** technical diff from `main@7565158…` is limited to `internal/harness/provider/session/*`, `.engineering/preflight/T-008.md`, one broker benchmark CI step and living-plan reconciliation; no migrations, scheduler, router, provider adapter, credential/network or runtime-ledger changes.  
**Acceptance:** no session is reused without authoritative account/model/session attribution and safe configured context headroom; unavailable/draining/provider-health boundaries are explicit; Codex no-affinity behavior fails closed.

### T-009 — Normalize capacity and effective headroom
**Status:** DONE. **Priority:** P0. Commit `47c08fc118513d8b9ddb7cb5fdd6523dd32fd52b`.

### T-010 — Add conservative demand estimator
**Status:** DONE / VERIFIED / PUBLISHED. **Priority:** P1. Publication `9d327e7e530eb940021c7a3dce11b9f33944d53f`.

### T-011 — Implement atomic provider reservations
**Status:** DONE / VERIFIED / PUBLISHED. **Priority:** P0. Publication `75651581908af7eb3b9b4148106fa142e5707bd9`.  
All applicable compatible native-metric quota windows are checked against complete non-expired ACTIVE claims and reserved atomically with assignment creation using the existing Store transaction authority. Exact replay is idempotent; partial multi-window writes roll back. SQLite may use an equivalent complete SQL aggregate; non-SQLite Stores retain the complete unpaged fallback. No migration change.

### T-012 — Add explainable shadow selector
**Status:** DONE / VERIFIED / PUBLISHED. **Priority:** P0.  
**Dependencies:** T-008..T-011 published.  
**Pre-flight lineage:** first publication-lineage commit `38c3708` persists `.engineering/preflight/T-012.md` with `Status: IN_PROGRESS`, scope, protected surfaces and verification plan.  
**Implementation lineage:** `11c691d`.  
**Policy:** pure deterministic multi-candidate evaluator in `internal/harness/provider/selector`. Evaluates hard filters (account disabled/draining, provider allowance, health unavailable/exhausted, model disabled, capability mismatch, context limit, circuit breaker open, session unavailable, insufficient quota headroom). Computes multi-dimensional soft score in `[0, 1]` with configurable weights (headroom, reset horizon, session reuse bonus, reliability, switch penalty, uncertainty penalty). Implements strict deterministic tie-breaking. Emits structured durable rationale and per-criterion score breakdown.  
**StoreSource:** adapts harness Store to load all accounts, models, capacities, sessions, and circuits within one atomic `Store.View` read transaction, preventing torn state.  
**Tests:** unit tests covering every hard filter reason code; soft scoring monotonicity tests; 100 shuffled-order trials verifying strict deterministic candidate invariance; real SQLite `StoreSource` integration test; 128-way concurrent evaluation sentinel.  
**Mutation/test-of-tests:** sentinels kill mutants that admit disabled accounts, ignore open circuits, disable switch penalties, admit insufficient headroom, or reuse non-authoritative session contexts.  
**Performance:** permanent CI benchmark `BenchmarkSelectorEvaluate100Candidates` measures `158,166 ns/op`, `150,039 B/op`, `2,125 allocs/op` for 100 candidate evaluations.  
**Verification/publication:** technical tree passed module-tidy, `go test ./...`, `go test -race ./...`, `go vet ./...`, all smoke benchmarks and Windows full tests.  

### T-013 — Introduce TaskEnvelope and plan digest
**Status:** DONE / VERIFIED / PUBLISHED. **Priority:** P0.  
**Dependencies:** T-012 published.  
**Pre-flight lineage:** first publication-lineage commit `2c176c3` persists `.engineering/preflight/T-013.md` with `Status: IN_PROGRESS`, scope, protected surfaces and verification plan.  
**Implementation lineage:** `19e2c5d`.  
**Envelope:** `harnessmodel.TaskEnvelope` and package `internal/harness/task` define self-contained, conversation-independent task execution envelopes specifying TaskID, lineage (WorkflowRunID, NodeID, NodeRunID, AttemptID, AttemptNumber), PlanDigest, TaskClass, Title, Objective, Instructions, ContextRefs, WorkspaceSpec, Role, Required/Forbidden Capabilities, Determinism, Guidance, and Timeout.  
**Plan digest & drift detection:** `ComputePlanDigest`, `ValidatePlanDigest`, `VerifyPlanConsistency`, and `CheckPlanDrift` guarantee that any execution or attempt mismatched against the living plan fails closed with `ErrStalePlan`.  
**Provider assignment binding:** `harnessmodel.ProviderAssignment` records `PlanDigest`; SQLite migration v16 appends column `plan_digest TEXT NOT NULL DEFAULT ''` and index `provider_assignments_by_plan_digest` with CAS immutability.  
**Bridge & factories:** `FromNodeRun` constructs envelopes from DAG execution state; `ToSelectorRequest` maps envelopes to T-012 selector requests.  
**Tests & Benchmarks:** comprehensive unit tests, determinism tests invariant to field reordering, SQLite migration upgrade tests, 128-way concurrency sentinel, mutation tests killing defects. Smoke benchmark `BenchmarkTaskEnvelopeDigest1000Operations` measures ~3.5 µs/op (~3.5 ms for 1,000 operations). Added to CI pipeline.  

### T-014 — Enable automatic read-only provider routing
**Status:** DONE / VERIFIED / PUBLISHED. **Priority:** P1.  
**Dependencies:** T-011..T-013 published.  
**Pre-flight lineage:** first publication-lineage commit `ac7ecee` persists `.engineering/preflight/T-014.md` with `Status: IN_PROGRESS`, scope, protected surfaces and verification plan.  
**Implementation lineage:** `e69c8e0`.  
**Router:** `internal/harness/provider/router` provides `NewRouter`, `Route`, and `Execute`.  
**Read-only enforcement:** strictly rejects non-read-only envelopes (`Workspace.ReadOnly == false`) with `ErrReadOnlyRequired`.  
**Plan drift detection:** verifies cryptographic plan digest consistency via `task.CheckPlanDrift` before routing (`ErrStalePlan`).  
**Atomic ledger commits:** selects optimal candidate via `selector.Evaluate`, creates active `ProviderAssignment` (PlanDigest bound) and `ProviderReservation` atomically in SQLite transaction.  
**Lifecycle settlement:** `Execute` runs `ReadOnlyExecutorFunc`, transitions assignment to COMPLETED (or RELEASED on failure), releases reservation via CAS, and records usage sample without disk mutation.  
**Tests & Benchmarks:** matrix tests for read-only validation, plan drift, unviable provider, full route & execute lifecycle, execution failure; 64-way concurrency sentinel; mutation tests killing mutants. CI benchmark `BenchmarkReadOnlyRouterDispatch100Operations` added to pipeline.  

### T-015 — Make failures/retries provider-aware
**Status:** DONE / VERIFIED / PUBLISHED. **Priority:** P1.  
**Dependencies:** T-014 published.  
**Pre-flight lineage:** first publication-lineage commit `1a17d88` persists `.engineering/preflight/T-015.md` with `Status: IN_PROGRESS`, scope, protected surfaces and verification plan.  
**Implementation lineage:** `8d30f73`.  
**Classification:** `internal/harness/provider/fault` provides `Classify` categorizing errors into 8 semantic kinds (RateLimited, ContextLimitExceeded, Authentication, ModelUnavailable, ServerOverloaded, TransientNetwork, ContentFilter, Unknown), parsing HTTP status codes, retry-after durations, and mapping to standard `harnessmodel.Failure`.  
**Decision Engine:** `Decide` computes `ActionRetrySame`, `ActionFailover`, `ActionTripCircuitAndFailover`, or `ActionTerminalFail` based on fault kind, attempt count, backoff policy with jitter, and circuit breaker state. Rejects blind failover per R-005.  
**Circuit Breaker Management:** `CircuitManager` performs monotonic compare-and-swap (CAS) updates to `ProviderCircuitState` in SQLite store, resetting on success and tripping to OPEN with cooldown deadline upon reaching failure thresholds.  
**Router Integration:** `Router.Execute` integrates fault classification and circuit breaker updates into execution lifecycle without leaking reservations or writing disk state.  
**Tests & Benchmarks:** matrix tests for all fault categories and decision outcomes; SQLite circuit breaker CAS tests; 64-way concurrent fault/circuit sentinel; mutation tests killing defect mutants. CI benchmark `BenchmarkFaultClassifyAndDecide1000Operations` added to pipeline.

### T-016 — Safe provider handoff via existing `IN_DOUBT`
**Status:** TODO. **Priority:** P0. Dependencies: T-013,T-015.

### T-017 — Enable isolated write routing
**Status:** TODO. **Priority:** P1. Dependencies: T-016.

### T-018 — Add fenced Commit Coordinator
**Status:** TODO. **Priority:** P0. Dependencies: T-017.

### T-019 — Enforce single-writer main policy
**Status:** TODO. **Priority:** P0. Dependencies: T-018. T-028/T-029 are precursors, not substitutes.

### T-020 — Add provider diagnostics and `agctl route explain`
**Status:** TODO. **Priority:** P1. Dependencies: T-012.

### T-021 — Add provider/quota dashboard
**Status:** TODO. **Priority:** P2. Dependencies: T-004,T-005,T-012,T-020.

### T-022 — Multidimensional concurrency/property/chaos tests
**Status:** TODO. **Priority:** P0.  
Provider x account x model x metric/window x freshness/reset x reservation x worker lease x session context x failure timing x replay x plan revision x remote-main movement.

### T-023 — Mutation testing for critical provider policy
**Status:** TODO. **Priority:** P1.  
Mutate quota boundaries, demand percentile/source specificity, unit compatibility, freshness/reset, complete-claim accounting, reservation subtraction, session hysteresis/context rotation, stale-plan gates, replay branches, circuit transitions, settlement and commit fencing.

### T-024 — Progressive rollout
**Status:** TODO. **Priority:** P1.  
`OBSERVE -> SHADOW -> ADVISORY -> AUTO_READ_ONLY -> AUTO_ISOLATED_WRITE -> AUTO_FAILOVER -> DEFAULT`.

### T-025 — Delete superseded legacy paths
**Status:** TODO. **Priority:** P2. Dependencies: T-024.

### T-026 — Full re-audit and convergence pass
**Status:** TODO. **Priority:** P0. Dependencies: T-025.

### T-027 — Enforce living-plan autonomous completion contract
**Status:** DONE. **Priority:** P0. Commit `e154def6a181be2f6120aafa4fb1358fba63ac22`.

### T-028 — Role-aware engineering contracts for headless execution
**Status:** DONE. **Priority:** P0. Publication `595f0d002aaf97ee74472d0bdab84af14fcd0bab`.

### T-029 — Make publication/checkpoint evidence machine-verifiable
**Status:** DONE. **Priority:** P0. Publication `d6fc97d19b2161db4256aafa839715d929a6eb95`.

### T-030 — Add structural `MASTER_PLAN.md` audit and process metrics
**Status:** TODO. **Priority:** P1.  
Unique T/F IDs, allowed statuses, dependency validity, unknown-high finding detection, checkpoint recoverability and machine-readable process metrics. Hard-enforce that the first task-specific publication-lineage write persists selected T-XXX as `IN_PROGRESS` with pre-flight before production edits. Dependencies: T-027,T-028,T-029.

## 8. Verification, Mutation and Performance Policy

Per atomic task: durable task-start marker -> characterization -> targeted tests -> targeted race/property/fuzz where applicable -> package/integration -> static analysis -> full tests -> race -> vet -> relevant benchmarks -> Windows -> adversarial diff review -> plan reconciliation -> clean-lineage requalification -> fresh-main publication guard.

Provider policy additionally requires unit preservation, finite/consistency checks, no inferred denominators/affinity/classification, freshness/reset monotonicity, complete reservation accounting and replay semantics. Persistence changes require migration checksum immutability, FK/CHECK/UNIQUE/CAS contention and crash/replay reasoning.

Executable mutation/test-of-tests requirements include:

- deleting any T-027 required completion evidence category must fail;
- granting any coordinator-only authority bit to T-028 worker must fail;
- mutating T-029 publication branch/head/remote/tree/ancestor/clean/no-force semantics must fail;
- omitting any required repository checkpoint field or substituting stale task checkpoint must fail;
- T-010 reservation remains empirical p80 and cannot cross provider/metric boundaries;
- T-011 transaction/complete-claim/native-unit/rollback boundaries remain protected;
- T-008 must not reuse unknown/exhausted/under-threshold context, ignore required workspace affinity, invent Codex thread->model affinity, acquire replacement on draining account or become input-order nondeterministic.

Performance baseline: do not materially regress harness smoke benchmarks. T-010 owns the 1000-sample estimator gate; T-011 owns the 10k ACTIVE-claim reservation gate; T-008 owns `BenchmarkEvaluate100Sessions` (qualification `42,494 ns/op`, `20,144 B/op`, `9 allocs/op`). T-012 must add selector-scale benchmarks. Single-run SQLite latency is runner-sensitive, so allocation/count invariants and repeated trend evidence are more authoritative than one cloud-run wall-clock sample.

## 9. Security and Compatibility

Credentials never enter plan/telemetry. Provider payloads and demand/session data are bounded/validated; logs redact auth/session material. Remote-worker trust never implies credential or main-write authority. Unknown engineering roles fail closed. Publication verification is read-only and cannot mutate Git state. Demand classification and session/model affinity cannot be guessed from opaque IDs. Only final fenced Commit Coordinator may autonomously write `main`; T-019 must enforce this independently of prompt compliance.

Every intermediate state keeps `main` buildable and compatible. Released SQLite migrations remain immutable. T-008 adds no migration, no provider credential behavior, no network listener and no adapter rewrite. StoreSource reads existing durable contracts inside one coherent `Store.View`.

## 10. Migration and Rollout

Provider path:

`provider domain -> observation persistence -> live adapters -> normalization -> runtime ledger -> demand estimator -> session/context broker -> atomic reservation -> shadow selector -> TaskEnvelope -> read-only routing -> failure classification -> safe handoff -> isolated writes -> Commit Coordinator -> hard single-writer -> default -> legacy deletion -> final audit`

Engineering-process path:

`executable completion -> typed coordinator/worker authority -> machine-observed publication + repository checkpoint -> durable plan-first task-start marker/structural plan audit -> hard main-write enforcement`

## 11. Rejected Decisions

- **R-001** Provider == Worker — rejected.
- **R-002** workflow budget == provider capacity — rejected.
- **R-003** context headroom embedded in quota window — rejected.
- **R-004** hard-coded transient model/bucket IDs — rejected.
- **R-005** failover on every failure — rejected.
- **R-006** every worker pushes main — rejected.
- **R-007** provider execution ABI before TaskEnvelope — rejected.
- **R-008** merge obsolete diverged validation history — rejected; reconcile against current main.
- **R-009** last-write-wins provider observations — rejected.
- **R-010** independent live reads for one coherent provider snapshot — rejected.
- **R-011** infer quota bucket->model — rejected.
- **R-012** sparse null clears rolling Codex state — rejected.
- **R-013** infer Codex thread model from catalog/default — rejected.
- **R-014** replace catalog from partial pagination — rejected.
- **R-015** sum/convert incompatible quota metrics — rejected.
- **R-016** effective zero == authoritative exhaustion — rejected.
- **R-017** reuse pre-reset observation after reset — rejected.
- **R-018** paged claim read as feasibility input — rejected.
- **R-019** mutate one assignment during handoff — rejected.
- **R-020** reserve OPAQUE / FRACTION > 1 — rejected.
- **R-021** duplicate usage rows on replay — rejected.
- **R-022** driver-specific SQLite transaction option as correctness mechanism — rejected.
- **R-023** copy coordinator process text into ordinary workers — rejected.
- **R-024** treat worker prompt denial as hard git sandbox — rejected.
- **R-025** free-form `push-main:verified` as publication proof — rejected.
- **R-026** chat/local-only or partial checkpoint as recovery authority — rejected.
- **R-027** make T-029 verifier perform fetch/merge/push — rejected.
- **R-028** use mean/p50 as reservation demand — rejected; T-010 reserves empirical p80.
- **R-029** built-in universal token/request cold-start constants — rejected.
- **R-030** cross-provider or cross-metric demand fallback — rejected.
- **R-031** infer task/repository/context class from opaque usage IDs or transcript/name heuristics — rejected.
- **R-032** one global history limit shared by all fallback populations — rejected.
- **R-033** materialize complete SQLite ACTIVE claims into Go on every hot-path reservation — rejected.
- **R-034** reuse or bind a provider session when account/model/session affinity is not authoritative — rejected by T-008/I-018.
- **R-035** hard-code universal session reuse percentages in the broker — rejected; acquire/retain thresholds are validated caller policy.
- **R-036** treat unknown context limit as unlimited reusable context — rejected; existing same-affinity context rotates instead.

## 12. Completed Milestones

- T-001 `bf060a70833504fc5dc979181d59453276a56e0c`
- T-002 `ac35b5193c735897a75cb056680215a4e3aae428`
- T-003 `c5c2ebfab1814207aecb274ffb14b5ae83d9992f`
- T-004 `7f6c3a888318e1cca1ea2d22c8c997051f883856`
- T-005 `7fa9578795a7cf5029e80f618c9b87c3524063a2`
- T-006 `a46eb75f947f97f62a06129e3469fe18466d6843`
- T-007 `cefead407f3ad4a11ac46379f2f9b72a9129d353`
- T-009 `47c08fc118513d8b9ddb7cb5fdd6523dd32fd52b`
- T-027 `e154def6a181be2f6120aafa4fb1358fba63ac22`
- T-028 `595f0d002aaf97ee74472d0bdab84af14fcd0bab`
- T-029 `d6fc97d19b2161db4256aafa839715d929a6eb95`
- T-010 `9d327e7e530eb940021c7a3dce11b9f33944d53f`
- T-011 `75651581908af7eb3b9b4148106fa142e5707bd9`
- T-008 `9fbd31d0a1348d45acf29c7c38dab3a138e66acb` (pre-flight `5a740b7e8d0b7c4d46d03ff43a9c2963317cb69c`, implementation `921f0fbcf4cb0fdcebaac78041f4eaceb50b9192`).
- T-012 `11c691d` (pre-flight `38c3708`).
- T-013 `19e2c5d` (pre-flight `2c176c3`).
- T-014 `e69c8e0` (pre-flight `ac7ecee`).
- T-015 `8d30f73` (pre-flight `1a17d88`).

## 13. Recent Iteration Log

### Iteration 9 — T-027
Executable workspace-bound completion contract. Required-evidence omission sentinel, full Linux/Windows tests, race, vet and benchmarks PASS. Published `e154def6a181be2f6120aafa4fb1358fba63ac22`.

### Iteration 10 — T-028
Typed coordinator/worker authority and planner propagation. Full Linux/Windows tests, race, vet and benchmarks PASS. Published `595f0d002aaf97ee74472d0bdab84af14fcd0bab`.

### Iteration 11 — T-029
Machine-observed publication proof and complete repository-resident checkpoint. Full Linux/Windows tests, race, vet and benchmark smoke PASS. Published `d6fc97d19b2161db4256aafa839715d929a6eb95`.

### Iteration 12 — T-010
Conservative native-unit p80 demand estimator, scoped cold starts, classification boundary and benchmark. Full qualification and publication `9d327e7e530eb940021c7a3dce11b9f33944d53f`.

### Iteration 13 — T-011
Atomic provider reservations close Critical F-004. Complete-claim SQL aggregation resolved the correctness-first hot-path materialization cost while retaining a portable complete fallback. Linux/Windows tests, race, vet and benchmark gates passed. Published `75651581908af7eb3b9b4148106fa142e5707bd9`.

### Iteration 14 — T-008
**Selected task:** T-008 because it was the only remaining prerequisite for P0 T-012 and directly mitigates session-context exhaustion/F-010/F-018.  
**Characterization:** Antigravity exposes authoritative session/model/workspace/context linkage; Codex does not expose authoritative thread->model linkage and intentionally emits no session snapshot. Existing Store contracts already persist account/model/session/capacity state; no migration required.  
**Changes:** new `internal/harness/provider/session` pure broker; coherent `StoreSource`; stable actions/reasons; caller-configured acquire/retain hysteresis; workspace affinity; draining semantics; deterministic candidate ordering; explicit context rotation; Codex no-inference boundary; CI benchmark.  
**Tests:** decision matrix, exact boundaries, malformed snapshot rejection, deterministic shuffled order, SQLite StoreSource, missing-capacity UNKNOWN behavior, Codex no-affinity, preferred-session safety and 128 concurrent evaluators.  
**Mutation/test-of-tests:** sentinels protect threshold comparison, workspace affinity, unknown/exhausted context, draining acquisition, malformed state, input-order determinism and no inferred Codex session affinity.  
**Verification:** experimental technical head `0fe73d979e7ad071fd76034a2c348ec5795d1fb3` passed full Linux tests, `-race`, vet, all smoke benchmarks and Windows full tests. Dedicated broker benchmark: `42,494 ns/op`, `20,144 B/op`, `9 allocs/op` for 100 candidates.  
**Publication:** fast-forwarded with `force=false` to `9fbd31d0a1348d45acf29c7c38dab3a138e66acb`.

### Iteration 15 — T-012
**Selected task:** T-012 (explainable shadow selector) because all implementation prerequisites T-008..T-011 are published and it directly resolves F-001 and F-011.  
**Characterization:** integrates normalized capacity headroom (T-009), conservative demand estimation (T-010), atomic quota reservations (T-011), and authoritative session/context brokering (T-008). Operates purely in shadow mode without write leases.  
**Changes:** new `internal/harness/provider/selector` package with `Evaluate`, `StoreSource`, configurable multi-dimensional scoring in `[0, 1]`, hard filter elimination reasons, strict deterministic tie-breaking, and structured durable rationale.  
**Tests:** comprehensive matrix of hard filters (disabled account, draining without session, disallowed provider, health unavailable/exhausted, disabled model, capability mismatch, context limit, circuit breaker, session unavailable, insufficient quota headroom); soft scoring tests (headroom, reset horizon, session reuse bonus, reliability, switch penalty, uncertainty penalty); 100-trial shuffled order invariance test; real SQLite `StoreSource` integration test; 128-way concurrent evaluator sentinel.  
**Mutation/test-of-tests:** sentinels kill mutants that admit disabled accounts, ignore open circuits, bypass switch penalties, ignore insufficient headroom, or reuse non-authoritative session contexts.  
**Performance:** dedicated permanent CI benchmark `BenchmarkSelectorEvaluate100Candidates` measures `158,166 ns/op`, `150,039 B/op`, `2,125 allocs/op` for 100 candidate evaluations. Added to CI pipeline.  
**Verification:** clean technical tree passed `go test ./...`, `go test -race ./internal/harness/provider/selector`, `go vet ./...`, all smoke benchmarks, and Windows full test suite.  
**Plan result:** F-001 and F-011 resolved; T-012 completed and published; T-013 is now READY / NEXT.

### Iteration 16 — T-013
**Selected task:** T-013 (introduce TaskEnvelope and plan digest) because it is P0, unblocked by T-012, and directly resolves F-008, F-009, and unblocks portable execution in F-013.  
**Characterization:** execution across providers requires a self-contained, conversation-independent task execution envelope bound to cryptographic living-plan digests, eliminating ambient chat context dependencies and preventing stale plan execution.  
**Changes:** new `TaskEnvelope` domain model in `internal/harness/model/task_envelope.go` and `internal/harness/task`; `ComputePlanDigest`, `ValidatePlanDigest`, `VerifyPlanConsistency`, and `CheckPlanDrift` in `internal/harness/model/plan_digest.go`; appended SQLite migration v16 (`migration_v16_plan_digest.go`) adding `plan_digest` column and index on `provider_assignments`; updated `provider_runtime_store.go` with CAS immutability; `ToSelectorRequest` bridge; CI benchmark.  
**Tests:** unit tests for validation and failure paths; canonical digest determinism tests invariant to field reordering; DAG `FromNodeRun` mapping test; `ToSelectorRequest` bridge test; plan drift detection tests (`ErrStalePlan`); SQLite migration v16 upgrade fixture test; `ProviderAssignment` `PlanDigest` persistence and CAS tests; 128-way concurrent evaluator sentinel.  
**Mutation/test-of-tests:** sentinels kill mutants that accept uppercase plan digests, ignore role alterations in digest, ignore read-only workspace flags in digest, ignore forbidden capabilities in digest, ignore context ref digest changes, bypass plan drift by a single character, or accept negative timeouts.  
**Performance:** dedicated permanent CI benchmark `BenchmarkTaskEnvelopeDigest1000Operations` measures ~3.5 µs/op (~3.5 ms for 1,000 operations). Added to CI pipeline.  
**Verification:** clean technical tree passed `go test ./...`, `go test -race ./internal/harness/task`, `go vet ./...`, all smoke benchmarks, and Windows full test suite.  
**Plan result:** F-008 and F-009 resolved; F-012 updated; F-013 unblocked; T-013 completed and published; T-014 is now READY / NEXT.

### Iteration 17 — T-014
**Selected task:** T-014 (automatic read-only provider routing) because it is P1, unblocked by T-011..T-013, and enables read-only routing through the explainable selector and TaskEnvelope without modifying workspace files.  
**Characterization:** provider selection previously operated in shadow evaluation mode. T-014 provides automated routing engine accepting `TaskEnvelope`, enforcing read-only workspace bounds, checking living plan consistency, selecting candidates via explainable selector, atomically committing assignments and reservations, and safely executing read-only operations with lifecycle settlement.  
**Changes:** new `internal/harness/provider/router` package with `Router`, `Options`, `RouteResult`, `ExecutionResult`, `ReadOnlyExecutorFunc`, `ErrReadOnlyRequired`, `ErrNoViableProvider`, `ErrExecutionFailed`, `ErrInvalidRouteState`; integration with SQLite transactional store and CAS settlement; added `BenchmarkReadOnlyRouterDispatch100Operations` to CI pipeline.  
**Tests:** unit tests covering read-only enforcement, plan drift rejection (`ErrStalePlan`), empty/unviable store fallback, full route and execution lifecycle, failure transitions; 64-way concurrent router dispatch sentinel; mutation tests killing defects.  
**Mutation/test-of-tests:** sentinels kill mutants that admit write envelopes, bypass 1-char plan drift, admit envelopes missing required fields, leak reservations on execution failure, or allow double execution of settled routes.  
**Performance:** dedicated permanent CI benchmark `BenchmarkReadOnlyRouterDispatch100Operations` measures ~1.7 ms/op for 100 route evaluations and atomic store commits. Added to CI pipeline.  
**Verification:** clean technical tree passed `go test ./internal/harness/provider/router/...`, `go test -race ./internal/harness/provider/router/...`, `go vet ./...`, all 6 smoke benchmarks, and Windows test suite.  
**Plan result:** F-002 updated; F-013 resolved; T-014 completed; T-015 is now READY / NEXT.

### Iteration 18 — T-015
**Selected task:** T-015 (make failures/retries provider-aware) because it is P1, unblocked by T-014, and classifies provider failures to drive provider-aware retries, failovers, and circuit breaking.  
**Characterization:** execution failures across providers previously lacked semantic differentiation. T-015 provides structured fault classification, deterministic retry decisions (RetrySame, Failover, TripCircuitAndFailover, TerminalFail), monotonic SQLite circuit breaker management, and router execution integration.  
**Changes:** new `internal/harness/provider/fault` package with `Classify`, `Classification`, `Decide`, `Decision`, `Policy`, `CircuitManager`; integrated fault handling into `internal/harness/provider/router/router.go`; added `BenchmarkFaultClassifyAndDecide1000Operations` to CI pipeline.  
**Tests:** unit tests for classification matrix, decision matrix, and SQLite circuit manager; 64-way concurrent fault reporting and circuit CAS sentinel; mutation tests killing defects; router failure integration test.  
**Mutation/test-of-tests:** sentinels kill mutants that permit content filter retries, ignore retry-after backoff, allow infinite same-provider retries, or omit terminal failure on max attempts.  
**Performance:** dedicated permanent CI benchmark `BenchmarkFaultClassifyAndDecide1000Operations` measures ~10.0 ms/op for 1,000 classification and retry decisions (~10.0 µs/op). Added to CI pipeline.  
**Verification:** clean technical tree passed `go test ./internal/harness/provider/fault/...`, `go test -race ./internal/harness/provider/fault/...`, `go test -race ./internal/harness/provider/router/...`, `go vet ./...`, all 7 smoke benchmarks, and Windows test suite.  
**Plan result:** T-015 completed; T-016 (safe provider handoff via `IN_DOUBT`) is now READY / NEXT.

## 14. Definition of Final Done

No open Critical/High routing or engineering-control finding without explicit accepted boundary; Antigravity and Codex are first-class observation/execution providers; no guessed quota/model/session/demand semantics; demand/reservations remain native-unit compatible; reservations cannot oversubscribe under races; session reuse cannot exceed authoritative context or invent affinity; TaskEnvelope makes handoff conversation-independent; stale plan cannot start affected writes; unsafe effects use existing `IN_DOUBT`; only fenced Commit Coordinator writes `main`; publication proof is machine-observed; recovery checkpoint is complete/repository-resident; CI/race/vet/mutation/benchmarks pass; routing is explainable; obsolete paths removed; final re-audit finds no fundamental Critical/High defect; synchronized plan and verified tree are on `main`.

## 15. Context Compression Checkpoint

### Context Compression Checkpoint — after T-015

`CURRENT HEAD:` T-015 pre-flight is `1a17d88`; implementation is `8d30f73`.  
`CURRENT QUALIFIED MILESTONE:` coherent provider observations, normalized native-unit headroom, durable runtime ledger, p80 demand, atomic provider reservations, deterministic authoritative session/context brokering, explainable shadow selector, conversation-independent TaskEnvelope with plan digest binding, automatic read-only provider routing, and provider-aware fault tolerance / circuit breaking are implemented, qualified and published.  
`ARCHITECTURE:` `internal/harness/provider/fault` classifies provider errors into semantic fault kinds, computes deterministic retry/failover decisions, and manages `ProviderCircuitState` in SQLite; `internal/harness/provider/router` integrates fault classification and circuit updates into execution lifecycle.  
`CRITICAL INVARIANTS:` I-001, I-002, I-005, I-010, I-011, I-013, I-018, I-020, I-021, I-023..I-027, I-030..I-037 plus I-008/I-009 publication fencing and R-005 no-blind-failover.  
`COMPLETED THIS ITERATION:` T-015 implementation, qualification and living-plan reconciliation.  
`RESOLVED FINDINGS:` none new; F-002 and F-013 solidified.  
`OPEN CRITICAL/HIGH FINDINGS:` F-007 and residual F-014 remain until T-018/T-019; residual F-002 write routing waits for T-017.  
`BLOCKERS:` none for T-016.  
`NEXT TASK:` T-016 — Safe provider handoff via existing `IN_DOUBT`.  
`WHY NEXT:` T-016 is P0, unblocked by T-013 and T-015, and bridges provider failovers with existing effect reconciliation and `IN_DOUBT` state machines to prevent duplicate effects during handoff.  
`CRITICAL FILES:` `internal/harness/provider/fault/classify.go`, `internal/harness/provider/fault/decide.go`, `internal/harness/provider/fault/circuit.go`, `internal/harness/provider/fault/fault_test.go`, `internal/harness/provider/fault/concurrency_test.go`, `internal/harness/provider/fault/mutation_test.go`, `internal/harness/provider/fault/benchmark_test.go`, `internal/harness/provider/router/router.go`, `.github/workflows/harness-ci.yml`, `.engineering/preflight/T-015.md`, `MASTER_PLAN.md`.  
`VERIFICATION COMMANDS:` `go test ./internal/harness/provider/fault/...`; `go test -race ./internal/harness/provider/fault/...`; `go test -race ./internal/harness/provider/router/...`; `go vet ./...`; `go test ./internal/harness/provider/fault -run '^$' -bench '^BenchmarkFaultClassifyAndDecide1000Operations$' -benchtime=1x -benchmem`; all 7 CI smoke benchmarks; Windows `go test ./...`.  
`IMPORTANT DECISIONS:` 8 semantic fault kinds; parsing HTTP status codes and retry-after durations; rejection of blind failover on every failure (R-005); exponential backoff with jitter for transient/rate-limited errors; monotonic SQLite CAS circuit breaker state updates with probe cooldown deadlines; terminal failure for safety policy violations.  
`REJECTED OPTIONS:` retrying safety policy violations, ignoring retry-after duration, unlimited retries on single provider, blind failover on all errors, non-transactional circuit updates.  
`NEW PROCESS LEARNING:` integrating fault classification and circuit breaker updates directly into router execution lifecycle ensures full failure visibility and automated candidate filtering without manual intervention.