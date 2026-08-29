# MASTER PLAN — Multi-Runtime Agent Control Plane

> Repository: `Homiakus/Antigravity-Progressive-Bootstrap`  
> Plan status: ACTIVE / LIVING  
> Semantic source of truth: this file. Git history is the audit trail for superseded verbose iteration detail; SQLite/generated indexes are execution ledgers or rebuildable projections, never semantic authority.

## 1. Mission

Evolve `agctl` into a quota-aware, multi-runtime engineering control plane that distributes atomic work between Antigravity and Codex according to capability, provider/account capacity, reset horizon, session context, reliability, risk and estimated demand while preserving one living plan and one fenced path to verified `main`.

Target loop:

`OBSERVE -> RECONCILE PLAN -> READY TASK -> TASK ENVELOPE -> ESTIMATE -> REFRESH PROVIDERS -> SELECT PROVIDER/MODEL/SESSION -> RESERVE -> CLAIM WORKER/WORKSPACE -> EXECUTE -> VERIFY -> REVIEW -> RECONCILE PLAN -> SETTLE USAGE -> COMMIT COORDINATOR -> VERIFIED FAST-FORWARD MAIN -> CHECKPOINT -> NEXT`

Current qualified milestones:

- Provider observation/runtime foundation T-001..T-007 and T-009 is published.
- T-027 makes living-plan completion executable and workspace-bound.
- T-028 separates coordinator and ordinary headless-worker authority; published as `595f0d002aaf97ee74472d0bdab84af14fcd0bab`.
- T-029 makes publication/checkpoint evidence machine-verifiable; published as `d6fc97d19b2161db4256aafa839715d929a6eb95`.
- T-010 conservative native-unit demand estimation is published as `9d327e7e530eb940021c7a3dce11b9f33944d53f`; post-push Linux/Windows CI, race, vet and benchmarks PASS.
- T-011 atomic provider reservations are implemented and qualified on `918ac96f8c19d840cda078fef46c6008731c1257`; reconciliation/publication is the current final action. T-008 becomes the highest-leverage READY task after publication because it is the only remaining prerequisite before T-012.

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
Provider Selector / Capacity Broker
      |
  +---+------------------------------+
  |                                  |
Antigravity                        Codex
status/headless                    App Server JSON-RPC
  |                                  |
coherent observations             rate/model/context observations
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

## 4. Findings Registry

### F-001 — Provider capacity is not a scheduler dimension
**Status:** Partially resolved in T-009/T-011. **Severity:** High. **Confidence:** Confirmed.  
T-009 provides normalized headroom and T-011 provides atomic feasibility/reservation; T-012 still integrates those signals into selector policy.

### F-002 — Codex is not yet a complete first-class execution provider
**Status:** Partially resolved. **Severity:** High. **Confidence:** Confirmed.  
Observation is first-class; portable execution waits for T-013/T-014.

### F-003 — Workflow budget cannot substitute for provider quota
**Status:** Resolved by T-010/T-011 boundary. **Severity:** High. **Confidence:** Confirmed.  
T-010 models demand in provider-native units independently of workflow spend; T-011 checks and atomically claims external provider supply without consulting workflow-budget allowance as capacity.

### F-004 — Durable claims exist but atomic quota reservation is incomplete
**Status:** Resolved in qualified T-011; publication pending. **Severity:** Critical. **Confidence:** Confirmed.  
T-011 now performs latest-capacity read, normalization, complete non-expired ACTIVE claim accounting, native-unit feasibility, assignment and all applicable-window reservations inside one existing writer transaction. Rollback, idempotent replay and 100-way oversubscription tests prove the race gap is closed in the qualified tree; publication makes this authoritative on `main`.

### F-005 — Existing worker/lease subsystem must be reused
**Status:** Resolved by architecture. **Severity:** High.  
Provider routing stays orthogonal to workers/fenced attempt leases.

### F-006 — Existing `IN_DOUBT` + effect intents already provide unsafe-replay protection
**Status:** Resolved by architecture. **Severity:** Critical.  
T-015/T-016 reuse existing effect reconciliation rather than duplicate it.

### F-007 — Direct worker pushes are unsafe with parallel providers
**Status:** Planned / narrowed by T-028. **Severity:** Critical. **Confidence:** Strong.  
Typed worker authority denies publication semantically; T-018/T-019 still must enforce it independently of agent compliance.

### F-008 — No TaskEnvelope independent of conversation history
**Status:** Planned. **Severity:** High.  
Affects T-013/T-014.

### F-009 — No plan digest on provider assignments
**Status:** Planned. **Severity:** High.  
Affects T-013/T-014/T-016.

### F-010 — No session/context broker
**Status:** Planned. **Severity:** Medium.  
T-008 READY.

### F-011 — Provider routing decisions are not fully explainable
**Status:** Partially resolved in T-005. **Severity:** Medium.  
Durable history exists; selector rationale/explanation remains T-012/T-020/T-021.

### F-012 — Released SQLite migrations are immutable
**Status:** Resolved. **Severity:** High.  
T-003/T-005 appended v14/v15 without rewriting prior released migrations.

### F-013 — Unified provider execution interface before TaskEnvelope would couple to conversation state
**Status:** Planned boundary. **Severity:** Medium.  
Deferred to T-013/T-014.

### F-014 — Concurrent autonomous work can move `main` during an iteration
**Status:** Partially resolved in T-029. **Severity:** High. **Confidence:** Confirmed.  
T-029 prevents a moved/stale/dirty/non-fast-forward publication from being falsely recorded as completed. T-018/T-019 still provide fenced publication authority and hard single-writer enforcement.

### F-015 — Delayed provider observations can regress durable state
**Status:** Resolved in T-004. **Severity:** High.  
Monotonic timestamps and account/session fencing reject stale reconnect data.

### F-016 — Independent provider reads can create a torn observation
**Status:** Resolved in T-006. **Severity:** High.  
Coherent `SnapshotSource` boundary introduced.

### F-017 — Codex rolling rate-limit updates are sparse and model/bucket mapping is not authoritative
**Status:** Resolved in T-007. **Severity:** High.  
Baseline+sparse merge preserves unavailable fields; no inferred bucket->model mapping.

### F-018 — Codex thread/context telemetry does not identify selected model
**Status:** Resolved by boundary / follow-up T-008. **Severity:** High.  
No fabricated session/model affinity; generic reuse degrades to NEW/UNAVAILABLE until authoritative linkage exists.

### F-019 — Partial model pagination can masquerade as model removal
**Status:** Resolved in T-007. **Severity:** High.  
Catalog replacement requires a complete successful pagination cycle.

### F-020 — Base quota validation is insufficient for routing arithmetic
**Status:** Resolved in T-009. **Severity:** High.  
Derived-policy validation rejects non-finite/inconsistent telemetry.

### F-021 — Absolute remaining without a limit is useful but cannot safely become a fraction
**Status:** Resolved in T-009. **Severity:** High.  
Native absolute headroom is preserved without invented denominator.

### F-022 — Provider reset boundaries invalidate otherwise fresh pre-reset capacity
**Status:** Resolved in T-009. **Severity:** High.  
Reset-aware expiry implemented.

### F-023 — Paged ACTIVE-reservation reads can undercount claims
**Status:** Resolved in T-005/T-011. **Severity:** Critical.  
`ListAllActiveProviderReservations` remains the correctness fallback; T-011 may instead use an equivalent complete transactional SQL aggregate and never a paged diagnostic subset.

### F-024 — Mutating one assignment record would destroy handoff history
**Status:** Resolved in T-005. **Severity:** High.  
Assignment history is append/supersede with at most one ACTIVE assignment per Attempt.

### F-025 — Provider usage redelivery can double-count settlement
**Status:** Resolved in T-005. **Severity:** High.  
Usage persistence is semantic-key idempotent under sequential/concurrent replay.

### F-026 — Runtime FRACTION quantities need explicit bounds
**Status:** Resolved in T-005. **Severity:** High.  
Domain and SQL constraints bound FRACTION and reject OPAQUE arithmetic.

### F-027 — Autonomous completion was prompt-only rather than executable
**Status:** Resolved in T-027. **Severity:** High.  
Typed categorized completion gate introduced.

### F-028 — Completion policy inferred repository identity from ambient cwd
**Status:** Resolved in T-027. **Severity:** High.  
Task workspace is authority; independent multi-plan work fails closed.

### F-029 — Monolithic full-suite gate delayed process failure localization
**Status:** Resolved in T-027. **Severity:** Medium.  
Cheap engineering/loop/hooks/doctor gates precede full CI.

### F-030 — One process prompt cannot safely govern coordinator and ordinary workers
**Status:** Resolved in T-028 at semantic/typed boundary. **Severity:** Critical.  
Separate role contracts/materialization prevent coordinator authority from being delegated by prompt propagation; T-019 remains hard enforcement.

### F-031 — Free-form `push-main` evidence can false-green unpublished or stale state
**Status:** Resolved in T-029. **Severity:** High.  
Structured `PublicationProof` + independent read-only Git observation replaced free-form completion text.

### F-032 — Free-form checkpoint evidence can be stale or unrecoverable
**Status:** Resolved in T-029. **Severity:** High.  
Completion now requires latest complete repository-resident checkpoint bound to current T-XXX.

### F-033 — Durable usage rows do not carry authoritative task/repository/context classification
**Status:** Resolved by boundary in T-010; integration requirement remains. **Category:** Provider Demand / Attribution. **Severity:** High. **Confidence:** Confirmed.  
`ProviderUsageSample` durably and idempotently stores assignment/account/model/metric/amount/time, but not semantic task class, repository identity or context class. T-010 requires explicit `UsageClassification`; T-011/T-012 must not infer those dimensions from opaque identifiers.

### F-034 — Complete Go materialization of ACTIVE claims is too allocation-heavy for the reservation hot path
**Status:** Resolved in T-011. **Category:** Provider Reservation / Performance. **Severity:** Medium. **Confidence:** Measured.  
The correctness-first 10k ACTIVE-claim implementation measured ~48.47 ms/op, ~21.5 MB/op and 490,487 allocs/op. T-011 retained I-025 but added an optional transactional SQL `GROUP BY/SUM` aggregate for SQLite, with the complete unpaged Reader fallback preserved for other Store implementations. Final qualification measured ~7.46 ms/op, ~15.6 KB/op and 439 allocs/op without a schema migration or weaker claim accounting.

## 5. Risk Register

| Risk | Level | Primary mitigation |
|---|---|---|
| quota oversubscription | Critical | T-005 claims + T-010 demand + T-011 atomic capacity/claim/reservation transaction |
| simultaneous main writers | Critical | T-028 typed denial + T-018 fenced coordinator + T-019 hard policy |
| duplicate/uncertain effects | Critical | effect intents + `IN_DOUBT` + T-016 handoff |
| moved/stale main recorded as complete | High | T-029 observed publication proof; T-018/T-019 publication fencing |
| wrong demand population/units | High | I-035 + F-033 + explicit classification + provider/metric isolation |
| incomplete recovery checkpoint | High | T-029 latest complete repository checkpoint contract |
| stale/torn provider state | High | I-014/I-016/I-019/I-022 |
| incompatible quota arithmetic | High | I-020/I-021/I-025/I-027/I-035 + T-011 |
| stale plan execution | High | T-013 plan digest + current I-030 completion digest |
| provider API drift | High | tolerant adapters + dynamic discovery + deterministic fixtures |
| session context exhaustion | High | T-008 broker |
| provider flapping / estimate error | Medium | hysteresis/switch cost + p80 + specificity confidence |

## 6. Dependency DAG and Priority

```text
T001 DONE -> T002 DONE -> T003 DONE -> T004 DONE
                                      |-> T006 DONE
                                      |-> T007 DONE
                                      |-> T005 DONE
T004,T006,T007 -> T008 READY
T004,T006,T007 -> T009 DONE
T005,T009 -> T010 DONE -> T011 DONE -> T012
T008 ---------------------------------> T012
T012 -> T013 -> T014 -> T015
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

Priority after T-011 publication:

1. **T-008 P1 / NEXT BY DEPENDENCY LEVERAGE** — the only remaining prerequisite for P0 T-012 and the direct mitigation for session-context exhaustion.
2. **T-030 P1** — structural plan/process audit; independently READY but lower product-path leverage than T-008.
3. **T-012 P0 / BLOCKED ON T-008** — explainable shadow selector immediately follows once session/context brokering is available.

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
**Status:** DONE / VERIFIED. **Priority:** P0.  
Append-only v15 runtime ledger; typed assignment/reservation/usage/circuit state; FK/CHECK/partial-UNIQUE/CAS invariants; complete ACTIVE-claim correctness read; replay-idempotent usage; concurrency proof. Implementation `be8a786de320b6e017a8da7eb8d74e5f3a119695`, reconciled qualification `7fa9578795a7cf5029e80f618c9b87c3524063a2`, published to `main`.

### T-006 — Implement Antigravity observation adapter
**Status:** DONE. **Priority:** P0. Commit `a46eb75f947f97f62a06129e3469fe18466d6843`.

### T-007 — Implement Codex App Server observation adapter
**Status:** DONE. **Priority:** P0. Commit `cefead407f3ad4a11ac46379f2f9b72a9129d353`.

### T-008 — Build session/context broker
**Status:** READY / NEXT. **Priority:** P1 / DEPENDENCY LEVERAGE.  
REUSE/NEW/CHECKPOINT_AND_NEW/UNAVAILABLE from context headroom, affinity, health/model requirements and hysteresis. No inferred Codex thread->model binding. Dependencies: T-004,T-006,T-007.

### T-009 — Normalize capacity and effective headroom
**Status:** DONE. **Priority:** P0. Commit `47c08fc118513d8b9ddb7cb5fdd6523dd32fd52b`.  
Unit-preserving headroom, freshness/confidence decay, reset expiry, exhaustion-proof separation, malformed telemetry rejection and provider-adapter integration.

### T-010 — Add conservative demand estimator
**Status:** DONE / VERIFIED / PUBLISHED. **Priority:** P1 / HIGH LEVERAGE. Publication `9d327e7e530eb940021c7a3dce11b9f33944d53f`.  
Pure nearest-rank p50/p80 estimator; reservation=p80; bounded history; provider/metric isolation; scoped explicit cold start; usage-classification boundary; HistorySource; FRACTION/OPAQUE safety; fuzz/mutation sentinels; Linux/Windows/race/vet/benchmarks PASS.

### T-011 — Implement atomic provider reservations
**Status:** DONE / VERIFIED, publication pending. **Priority:** P0. **Leverage:** CRITICAL.  
**Dependencies:** T-005,T-009,T-010 are DONE and published.  
**Root cause closed:** normalized provider supply, conservative demand and durable claims previously existed separately; T-011 now makes capacity evidence + complete claims + feasibility + assignment/reservations one writer transaction so parallel selectors cannot oversubscribe the same observed headroom.  
**Protected surface preserved:** no released v15 migration/checksum edit, no scheduler/worker/provider-adapter rewrite, and no parallel mutex/distributed-lock subsystem. `store.Store.Update`, SQLite `MaxOpenConns(1)` writer serialization and existing FK/CHECK/partial-UNIQUE/CAS contracts remain authoritative.  
**Atomic policy:** validate active account + enabled authoritative model; read latest capacity and normalize at the same decision time; select every applicable compatible native-metric window; account for the complete non-expired ACTIVE claim set; require `available >= p80 reservation`; append one ACTIVE assignment and positive claims for every applicable window in the same transaction. Any stale/unusable evidence, incompatible scope/metric, insufficiency or write conflict aborts the transaction.  
**Zero/replay semantics:** zero demand is explicitly assignment-only because the released domain/SQL contract requires positive reservation amount; no fake epsilon claim is created. Exact same active request is replay-idempotent; conflicting assignment/account/model/session/demand or non-canonical claim replay fails closed. Deterministic claim IDs include assignment + provider-native window + model scope + metric.  
**Expiry:** claim lifetime is capped by operational `ClaimTTL`, capacity evidence expiry and provider reset; already-expired claims are excluded from complete accounting. OPAQUE is never reservable; FRACTION remains native and bounded.  
**Complete-accounting implementation:** every Store can use `ListAllActiveProviderReservations`; SQLite additionally implements optional `ActiveProviderReservationAggregator` using a complete transactional `GROUP BY model_id,metric + SUM(amount),COUNT(*)` for the requested provider-native window. Contradictory dimensions fail closed. The fallback characterization deliberately hides this optional capability and proves non-SQLite complete-list correctness remains intact.  
**Concurrency/rollback proof:** with capacity=100 and demand=10, 100 concurrent independent attempts yield exactly 10 successes and never exceed 100 ACTIVE claimed units. Fifty concurrent deliveries of one semantic request yield one physical assignment/claim plus 49 successful replays. A SQLite trigger that fails the second multi-window claim proves the already-written first claim and assignment both roll back.  
**Edge/mutation sentinels:** exact remaining boundary succeeds while over-boundary fails; expired claims do not count; contradictory same-window dimension fails closed; IDs are assignment-scoped; disabled model/account, stale/reset/unavailable capacity, cancellation and invalid native quantities fail closed; FRACTION end-to-end stays native; multi-window claims cover all simultaneous quota windows. These tests kill the material T-011 mutants for paged/omitted claims, raw capacity, broken `>=`, expired-claim inclusion, unit/scope mismatch, partial commit and duplicate replay.  
**Performance finding/resolution:** initial correctness-first 10k ACTIVE-claim benchmark was ~48.47 ms/op, ~21.5 MB/op, 490,487 allocs/op. Complete transactional SQL aggregation reduced the final qualified run to ~7.46 ms/op, ~15.6 KB/op and 439 allocs/op while retaining the unpaged fallback and requiring no migration.  
**Verification:** pre-reconciliation qualification head `918ac96f8c19d840cda078fef46c6008731c1257` — module-tidy guard PASS; bridge/process gates PASS; `go test ./...` PASS; `go test -race ./...` PASS; `go vet ./...` PASS; demand/reservation/compiler/store/scheduler/retry/wait benchmark smoke PASS; Windows targeted/full PASS. Adversarial diff review shows only reservation/store/test/CI/plan scope and no migration/scheduler/router/provider-adapter drift.  
**Acceptance satisfied:** no positive-demand execution-visible ACTIVE assignment commits without all required compatible claims being atomically feasible and persisted; explicit zero-demand assignment-only semantics are bounded by the same account/model/capacity checks; concurrent successful reservations cannot exceed compatible normalized headroom.

### T-012 — Add explainable shadow selector
**Status:** TODO / BLOCKED ON T-008. **Priority:** P0.  
Hard capability/health/model/context/risk filters; soft quota/reset/session/reliability/switch/uncertainty score; deterministic tie-break. Dependencies: T-008..T-011.

### T-013 — Introduce TaskEnvelope and plan digest
**Status:** TODO. **Priority:** P0.  
Portable revision/digest-bound objective/evidence/scope/invariants/findings/capabilities/acceptance/verification/risk/replay/workspace/checkpoint contract. Dependencies: T-012.

### T-014 — Enable automatic read-only provider routing
**Status:** TODO. **Priority:** P1. Dependencies: T-011..T-013.

### T-015 — Make failures/retries provider-aware
**Status:** TODO. **Priority:** P1.  
Quota/rate/spend/auth/model/overload/protocol/context classes; deterministic code/test failures never trigger provider failover. Dependencies: T-014.

### T-016 — Safe provider handoff via existing `IN_DOUBT`
**Status:** TODO. **Priority:** P0.  
Reuse attempt reclaim/effect intents/reconciliation; unsafe writes never blind-replay. Dependencies: T-013,T-015.

### T-017 — Enable isolated write routing
**Status:** TODO. **Priority:** P1.  
Bind provider assignments to existing workspace/worktree isolation; worker may checkpoint delegated work but not publish main. Dependencies: T-016.

### T-018 — Add fenced Commit Coordinator
**Status:** TODO. **Priority:** P0.  
Single coordinator integrates verified checkpoints, rechecks remote main, reconciles if moved and fast-forwards only. Dependencies: T-017.

### T-019 — Enforce single-writer main policy
**Status:** TODO. **Priority:** P0.  
Static/policy/runtime enforcement must prevent ordinary worker main publication even if prompt instructions are ignored. Dependencies: T-018. T-028/T-029 are precursors, not substitutes.

### T-020 — Add provider diagnostics and `agctl route explain`
**Status:** TODO. **Priority:** P1. Dependencies: T-012.

### T-021 — Add provider/quota dashboard
**Status:** TODO. **Priority:** P2. Dependencies: T-004,T-005,T-012,T-020.

### T-022 — Multidimensional concurrency/property/chaos tests
**Status:** TODO. **Priority:** P0.  
Provider x account x model x metric/window x freshness/reset x reservation x worker lease x session context x failure timing x replay x plan revision x remote-main movement.

### T-023 — Mutation testing for critical provider policy
**Status:** TODO. **Priority:** P1.  
Mutate quota boundaries, demand percentile/source specificity, unit compatibility, freshness/reset, complete-claim accounting, reservation subtraction, stale-plan gates, replay branches, circuit transitions, settlement and commit fencing.

### T-024 — Progressive rollout
**Status:** TODO. **Priority:** P1.  
`OBSERVE -> SHADOW -> ADVISORY -> AUTO_READ_ONLY -> AUTO_ISOLATED_WRITE -> AUTO_FAILOVER -> DEFAULT`.

### T-025 — Delete superseded legacy paths
**Status:** TODO. **Priority:** P2. Dependencies: T-024.

### T-026 — Full re-audit and convergence pass
**Status:** TODO. **Priority:** P0. Dependencies: T-025.

### T-027 — Enforce living-plan autonomous completion contract
**Status:** DONE. **Priority:** P0. Commit `e154def6a181be2f6120aafa4fb1358fba63ac22`.  
Categorized evidence, plan/task/finding validation, SHA-256 binding, workspace authority, fail-closed multi-plan resolution and targeted CI feedback ladder.

### T-028 — Role-aware engineering contracts for headless execution
**Status:** DONE. **Priority:** P0. Publication commit `595f0d002aaf97ee74472d0bdab84af14fcd0bab`.  
Typed coordinator/worker authority, worker contract propagation to every planner node, machine-readable role tag and coordinator-authority mutation sentinels. Hard git enforcement remains T-019.

### T-029 — Make publication/checkpoint evidence machine-verifiable
**Status:** DONE. **Priority:** P0. Publication commit `d6fc97d19b2161db4256aafa839715d929a6eb95`.  
Structured publication proof, independent read-only Git verifier, main/head/tree/remote/ancestor/clean/no-force gates and complete latest repository checkpoint. Complements T-018/T-019; never performs publication itself.

### T-030 — Add structural `MASTER_PLAN.md` audit and process metrics
**Status:** TODO. **Priority:** P1.  
Unique T/F IDs, allowed statuses, at most one coordinator `IN_PROGRESS`, dependency validity, unknown-high finding detection, checkpoint recoverability and machine-readable process metrics without Goodhart optimization. Enforce that the first task-specific branch write records selected T-XXX as `IN_PROGRESS` with pre-flight before production edits. Dependencies: T-027,T-028,T-029.

## 8. Verification, Mutation and Performance Policy

Per atomic task: durable task-start marker -> characterization -> targeted tests -> targeted race/property/fuzz where applicable -> package/integration -> static analysis -> full tests -> race -> vet -> relevant benchmarks -> Windows. Live provider probes remain opt-in; normal CI uses deterministic no-secret fixtures.

Provider policy additionally requires unit preservation, finite/consistency checks, no inferred denominators/affinity/classification, freshness/reset monotonicity, complete reservation accounting and replay semantics. Persistence changes require migration checksum immutability, FK/CHECK/UNIQUE/CAS contention and crash/replay reasoning.

Process/provider mutation sentinels are executable requirements:

- deleting any T-027 required completion evidence category must fail;
- granting any coordinator-only authority bit to T-028 worker must fail;
- mutating T-029 publication branch/head/remote/tree/ancestor/clean/no-force semantics must fail;
- omitting any required repository checkpoint field or substituting stale task checkpoint must fail;
- T-010 reservation must remain p80, provider/metric boundaries cannot broaden, equal-time truncation cannot prefer low claims, and FRACTION estimates remain bounded;
- T-011 transaction/complete-claim/native-unit/rollback boundaries are protected by concurrent oversubscription, same-request replay, exact-boundary, expiry, dimension-conflict and partial-write rollback sentinels.

Performance baseline: do not materially regress harness smoke benchmarks. T-010 establishes a 1000-sample estimator smoke benchmark. T-011 establishes a 10k ACTIVE-claim reservation smoke benchmark; qualified aggregate path is ~7.46 ms/op, ~15.6 KB/op and 439 allocs/op on GitHub Linux, versus the initial complete-Go-materialization baseline ~48.47 ms/op, ~21.5 MB/op and 490,487 allocs/op. Later T-012 adds selector-scale benchmarks.

## 9. Security and Compatibility

Credentials never enter plan/telemetry. Provider payloads and demand samples are bounded/validated; logs redact auth/session material. Remote-worker trust never implies credential or main-write authority. Unknown engineering roles fail closed. Publication verification is read-only and cannot mutate Git state. Demand classification cannot be guessed from opaque IDs. Only final fenced Commit Coordinator may autonomously write `main`; T-019 must enforce this independently of prompt compliance.

Every intermediate state keeps `main` buildable and compatible. Released SQLite migrations remain immutable. T-011 uses existing store/transaction and v15 runtime contracts; it adds no migration or public provider-state rewrite. The SQLite aggregate is an optional optimization and non-SQLite Store implementations retain the complete unpaged fallback.

## 10. Migration and Rollout

Provider path:

`provider domain -> observation persistence -> live adapters -> normalization -> runtime ledger -> demand estimator -> atomic reservation -> shadow selector -> TaskEnvelope -> read-only routing -> failure classification -> safe handoff -> isolated writes -> Commit Coordinator -> hard single-writer -> default -> legacy deletion -> final audit`

Engineering-process path:

`executable completion -> typed coordinator/worker authority -> machine-observed publication + repository checkpoint -> durable task-start marker/structural plan audit -> hard main-write enforcement`

## 11. Rejected Decisions

- **R-001** Provider == Worker — rejected.
- **R-002** workflow budget == provider capacity — rejected.
- **R-003** context headroom embedded in quota window — rejected.
- **R-004** hard-coded transient model/bucket IDs — rejected.
- **R-005** failover on every failure — rejected.
- **R-006** every worker pushes main — rejected.
- **R-007** provider execution ABI before TaskEnvelope — rejected.
- **R-008** merge obsolete diverged validation history — rejected; overlay/reconcile against current main.
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
- **R-029** built-in universal token/request cold-start constants — rejected; cold start must be explicit operator policy.
- **R-030** cross-provider or cross-metric demand fallback — rejected.
- **R-031** infer task/repository/context class from opaque usage IDs or transcript/name heuristics — rejected after F-033.
- **R-032** one global history limit shared by all fallback populations — rejected; it can starve broader populations and bias availability.
- **R-033** materialize the complete SQLite ACTIVE-claim set into Go on every hot-path reservation — rejected after F-034 measurement; use the equivalent complete transactional SQL aggregate while retaining the Store fallback.

## 12. Completed Milestones

- T-001 `bf060a70833504fc5dc979181d59453276a56e0c`
- T-002 `ac35b5193c735897a75cb056680215a4e3aae428`
- T-003 `c5c2ebfab1814207aecb274ffb14b5ae83d9992f`
- T-004 `7f6c3a888318e1cca1ea2d22c8c997051f883856`
- T-005 verified/published after reconciled head `7fa9578795a7cf5029e80f618c9b87c3524063a2`
- T-006 `a46eb75f947f97f62a06129e3469fe18466d6843`
- T-007 `cefead407f3ad4a11ac46379f2f9b72a9129d353`
- T-009 `47c08fc118513d8b9ddb7cb5fdd6523dd32fd52b`
- T-027 `e154def6a181be2f6120aafa4fb1358fba63ac22`
- T-028 `595f0d002aaf97ee74472d0bdab84af14fcd0bab`
- T-029 `d6fc97d19b2161db4256aafa839715d929a6eb95`
- T-010 `9d327e7e530eb940021c7a3dce11b9f33944d53f`
- T-011 qualification `918ac96f8c19d840cda078fef46c6008731c1257`; publication commit becomes authoritative after this reconciled plan passes full CI and fast-forward publication.

## 13. Recent Iteration Log

### Iteration 9 — T-027
Executable workspace-bound completion contract. Required-evidence omission sentinel, full Linux/Windows tests, race, vet and benchmarks PASS. Published `e154def6a181be2f6120aafa4fb1358fba63ac22`.

### Iteration 10 — T-028
Typed coordinator/worker authority and planner propagation. Coordinator-authority mutation sentinel, full Linux/Windows tests, race, vet and benchmarks PASS. Published `595f0d002aaf97ee74472d0bdab84af14fcd0bab`.

### Iteration 11 — T-029
Machine-observed publication proof and complete repository-resident checkpoint. Full Linux/Windows tests, race, vet and benchmark smoke PASS. Published `d6fc97d19b2161db4256aafa839715d929a6eb95`.

### Iteration 12 — T-010
Conservative native-unit p80 demand estimator, scoped explicit cold starts, classified history boundary, fuzz/mutation sentinels and benchmark. Qualification/reconciliation passed Linux/Windows full CI, race, vet and benchmarks; one logical fast-forward publication `9d327e7e530eb940021c7a3dce11b9f33944d53f`; post-push CI PASS.

### Iteration 13 — T-011
**Selected task:** T-011 by Critical F-004 risk leverage; durable `IN_PROGRESS` pre-flight was the first task-specific branch commit.  
**Root cause:** capacity normalization, p80 demand and durable claims were separately correct but not one atomic feasibility/write decision, leaving a parallel-selector oversubscription race.  
**Changes:** new provider reservation service; active-account/enabled-model/capacity checks; all applicable simultaneous quota windows; atomic assignment + claims; deterministic assignment-scoped claim IDs; explicit zero-demand assignment-only semantics; exact replay; expiry bounded by TTL/evidence/reset; optional complete SQLite aggregate with unpaged Store fallback; CI benchmark.  
**Unexpected finding:** F-034 — correctness-first 10k claim materialization was allocation-heavy. The equivalent complete transactional SQL aggregate preserved I-025 while cutting final reservation benchmark to ~7.46 ms/op, ~15.6 KB/op and 439 allocs/op from ~48.47 ms/op, ~21.5 MB/op and 490,487 allocs/op.  
**Tests:** multi-window success, zero demand, exact/conflicting replay, exact headroom boundary, expired claims, contradictory dimensions, assignment-scoped IDs, disabled account/model, stale/reset/unavailable evidence, cancellation, FRACTION native units, injected second-claim rollback, 100 independent concurrent reservers and 50-way same-request redelivery, non-aggregate Store fallback, Windows.  
**Mutation/test-of-tests:** sentinels kill omitted/paged claim accounting, raw-capacity comparison, broken boundary, expired-claim inclusion, unit/model/window mismatch, partial multi-window commit and duplicate replay semantics.  
**Verification:** pre-reconciliation head `918ac96f8c19d840cda078fef46c6008731c1257` — module-tidy, targeted process gates, full Linux tests, race, vet, demand/reservation/legacy benchmarks and Windows full tests PASS.  
**Compatibility/security:** no migration, credential, network, scheduler/router or provider-adapter change; released v15 immutable; no new lock subsystem; optional aggregate is internal Store capability with complete-list fallback.  
**Adversarial review:** compare from `main@9d327e7e…` contains only reservation/store/test/CI/plan and the corrected SQLite writer comment; branch is 16 commits ahead/0 behind before reconciliation and no protected-surface drift was found.  
**Plan result:** F-003/F-004/F-023 closed or reduced as recorded; F-034 found/resolved; T-011 DONE/VERIFIED; T-008 READY/NEXT; T-012 blocked only on T-008.  
**Publication:** final reconciled head must pass full CI, then re-read remote `main` and publish one logical fast-forward commit; no force.

## 14. Definition of Final Done

No open Critical/High routing or engineering-control finding without explicit accepted boundary; Antigravity and Codex are first-class observation/execution providers; no guessed quota/model/session/demand semantics; demand/reservations remain native-unit compatible; reservations cannot oversubscribe under races; TaskEnvelope makes handoff conversation-independent; stale plan cannot start affected writes; unsafe effects use existing `IN_DOUBT`; only fenced Commit Coordinator writes `main`; publication proof is machine-observed; recovery checkpoint is complete/repository-resident; CI/race/vet/mutation/benchmarks pass; routing is explainable; obsolete paths removed; final re-audit finds no fundamental Critical/High defect; synchronized plan and verified tree are on `main`.

## 15. Context Compression Checkpoint — after T-011

`CURRENT HEAD:` T-011 final publication commit becomes authoritative only after this reconciled plan passes full CI and fast-forward publication; pre-reconciliation qualification head is `918ac96f8c19d840cda078fef46c6008731c1257`, and observed `main` before reconciliation remains `9d327e7e530eb940021c7a3dce11b9f33944d53f`.  
`CURRENT QUALIFIED MILESTONE:` coherent provider observations, native-unit normalization, durable runtime ledger, conservative p80 demand and atomic complete-claim provider reservation are implemented and qualified; session/context brokering is now the next missing selector prerequisite.  
`ARCHITECTURE:` `provider/demand` owns demand; `provider/reservation` owns atomic feasibility/claim creation; existing `store.Store.Update` and SQLite single writer are transaction authority; SQLite may use optional complete SQL aggregation while other Store implementations retain the complete unpaged fallback; v15 remains unchanged.  
`CRITICAL INVARIANTS:` I-005/I-020/I-021/I-023..I-027/I-035 plus I-008/I-009/I-033/I-034 for publication.  
`COMPLETED THIS ITERATION:` T-011.  
`RESOLVED FINDINGS:` F-003 provider-budget boundary completed; Critical F-004 atomic oversubscription gap closed in the qualified tree; F-023 complete accounting preserved; F-034 performance materialization defect found and resolved.  
`OPEN CRITICAL/HIGH FINDINGS:` F-007 and residual F-014 remain until T-018/T-019; F-001/F-002/F-008/F-009 remain dependency-chain work. F-004 has no remaining implementation blocker and awaits only publication of the qualified/reconciled tree.  
`BLOCKERS:` none for T-008; T-012 is blocked only on T-008.  
`NEXT TASK:` T-008.  
`WHY NEXT:` T-008 is independently READY, directly mitigates session-context exhaustion/F-010/F-018, and is the only remaining prerequisite before P0 T-012 shadow selection.  
`CRITICAL FILES:` `internal/harness/model/provider.go`, `internal/harness/model/provider_runtime.go`, `internal/harness/provider/provider.go`, `internal/harness/provider/antigravity/adapter.go`, `internal/harness/provider/codex/adapter.go`, `internal/harness/store/store.go`, `internal/harness/store/sqlite/provider_store.go`, `internal/harness/store/sqlite/provider_runtime_store.go`, `MASTER_PLAN.md`; released migrations remain protected.  
`VERIFICATION COMMANDS:` first persist T-008 `IN_PROGRESS` + pre-flight on a new branch; characterize current provider-session/context observations and authoritative model affinity; targeted broker decision tests across REUSE/NEW/CHECKPOINT_AND_NEW/UNAVAILABLE; no-inferred-affinity sentinels; concurrency/race/property tests; `go test ./...`; `go test -race ./...`; `go vet ./...`; broker/legacy benchmarks; Windows full tests.  
`IMPORTANT DECISIONS:` reservation demand is p80 in native units; positive demand reserves every applicable simultaneous quota window; zero demand is assignment-only; complete claim accounting may be an equivalent complete SQL aggregate but never paged; exact replay is idempotent; no driver-specific transaction mode or parallel locking; T-008 must not invent Codex thread->model affinity.  
`REJECTED OPTIONS:` paged claims, raw-capacity comparison, partial multi-window reservation, fabricated epsilon claim, cross-unit summing, new lock service, v15 rewrite, hot-path Go materialization of all SQLite claims, inferred session/model affinity.  
`NEW PROCESS LEARNING:` the durable plan-first pre-flight was correctly enforced for T-011; correctness-first implementation plus a mandatory scale benchmark exposed F-034 before publication; optional Store capabilities can optimize a backend without weakening the portable correctness contract; final publication still requires a fresh observed-main guard after reconciled-tree CI.