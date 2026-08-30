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
- T-008 session/context brokering is published in `main` through reconciled commit `9fbd31d0a1348d45acf29c7c38dab3a138e66acb`, with durable pre-flight `5a740b7e8d0b7c4d46d03ff43a9c2963317cb69c` and implementation commit `921f0fbcf4cb0fdcebaac78041f4eaceb50b9192`. T-012 is now fully unblocked and is the next P0 product-path task.

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
**Status:** Partially resolved in T-009/T-011. **Severity:** High. **Confidence:** Confirmed.  
T-009 provides normalized headroom and T-011 atomic feasibility/reservation; T-012 still integrates these into selector policy.

### F-002 — Codex is not yet a complete first-class execution provider
**Status:** Partially resolved. **Severity:** High. **Confidence:** Confirmed.  
Observation is first-class; portable execution waits for T-013/T-014.

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
**Status:** Planned. **Severity:** High.  
Affects T-013/T-014.

### F-009 — No plan digest on provider assignments
**Status:** Planned. **Severity:** High.  
Affects T-013/T-014/T-016.

### F-010 — No session/context broker
**Status:** Resolved and published in T-008. **Severity:** Medium. **Confidence:** Confirmed.  
Provider-neutral `REUSE/NEW/CHECKPOINT_AND_NEW/UNAVAILABLE` policy now uses authoritative account/model/session/context/workspace data and configured hysteresis.

### F-011 — Provider routing decisions are not fully explainable
**Status:** Partially resolved in T-005/T-008. **Severity:** Medium.  
T-008 emits stable reason codes; full selector rationale remains T-012/T-020/T-021.

### F-012 — Released SQLite migrations are immutable
**Status:** Resolved. **Severity:** High.  
T-003/T-005 appended v14/v15; T-008 adds no migration.

### F-013 — Unified provider execution interface before TaskEnvelope would couple to conversation state
**Status:** Planned boundary. **Severity:** Medium.  
Deferred to T-013/T-014.

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
T008,T011 -------------------------> T012 READY/NEXT
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

Priority after T-008 publication:

1. **T-012 P0 / NEXT** — explainable shadow selector; all prerequisites are now published and complete.
2. **T-030 P1** — structural plan/process audit and enforcement of durable task-start ordering.
3. **T-013 P0 / BLOCKED ON T-012** — TaskEnvelope and plan digest.

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
**Status:** IN_PROGRESS. **Priority:** P0.  
Hard capability/health/model/context/risk filters; soft quota/reset/session/reliability/switch/uncertainty score; deterministic tie-break; durable rationale. Dependencies T-008..T-011 are published and complete. Pre-flight established in `.engineering/preflight/T-012.md`.

### T-013 — Introduce TaskEnvelope and plan digest
**Status:** TODO. **Priority:** P0. Dependencies: T-012.

### T-014 — Enable automatic read-only provider routing
**Status:** TODO. **Priority:** P1. Dependencies: T-011..T-013.

### T-015 — Make failures/retries provider-aware
**Status:** TODO. **Priority:** P1. Dependencies: T-014.

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
**Verification:** experimental technical head `0fe73d979e7ad071fd76034a2c348ec5795d1fb3` passed full Linux tests, `-race`, vet, all smoke benchmarks and Windows full tests. Dedicated broker benchmark: `42,494 ns/op`, `20,144 B/op`, `9 allocs/op` for 100 candidates. The clean publication lineage was then independently requalified through full Linux/Windows CI, race, vet and all smoke benchmarks.  
**Security/compatibility:** no migration, credentials, network listeners, provider adapter, scheduler, router or runtime-ledger changes; StoreSource uses one coherent durable View and validates capacity/account/provider consistency.  
**Adversarial review:** published diff from T-011 is only the durable T-008 pre-flight, new session package, three CI lines for broker benchmark and living-plan reconciliation.  
**Process review:** the first experimental branch performed implementation writes before a durable T-008 pre-flight marker. Rather than hide or force-rewrite that history, publication lineage was rebuilt from published `main`: first commit `5a740b7…` is the durable `IN_PROGRESS` pre-flight, second commit `921f0fb…` carries the already-qualified technical tree. This preserves I-009 and makes the publication lineage plan-first without force. T-030 remains responsible for automating this ordering so repair is unnecessary in future.  
**Plan result:** F-010 is resolved/published; F-018 is enforced by broker no-inference behavior; T-012 is READY/NEXT with no remaining prerequisite blocker.  
**Publication:** fresh `main` guard observed T-011 at `7565158…`, qualified lineage was 3 commits ahead / 0 behind, and `main` was fast-forwarded with `force=false` to `9fbd31d0a1348d45acf29c7c38dab3a138e66acb`.

## 14. Definition of Final Done

No open Critical/High routing or engineering-control finding without explicit accepted boundary; Antigravity and Codex are first-class observation/execution providers; no guessed quota/model/session/demand semantics; demand/reservations remain native-unit compatible; reservations cannot oversubscribe under races; session reuse cannot exceed authoritative context or invent affinity; TaskEnvelope makes handoff conversation-independent; stale plan cannot start affected writes; unsafe effects use existing `IN_DOUBT`; only fenced Commit Coordinator writes `main`; publication proof is machine-observed; recovery checkpoint is complete/repository-resident; CI/race/vet/mutation/benchmarks pass; routing is explainable; obsolete paths removed; final re-audit finds no fundamental Critical/High defect; synchronized plan and verified tree are on `main`.

## 15. Context Compression Checkpoint — after T-008

`CURRENT HEAD:` T-008 publication authority is `main@9fbd31d0a1348d45acf29c7c38dab3a138e66acb`; pre-flight is `5a740b7e8d0b7c4d46d03ff43a9c2963317cb69c`; implementation is `921f0fbcf4cb0fdcebaac78041f4eaceb50b9192`. A later plan-only reconciliation may advance `main` without changing T-008 production code; `9fbd31d…` remains the T-008 publication milestone.  
`CURRENT QUALIFIED MILESTONE:` coherent provider observations, normalized native-unit headroom, durable runtime ledger, p80 demand, atomic provider reservations and deterministic authoritative session/context brokering are implemented, qualified and published.  
`ARCHITECTURE:` `provider/demand` owns native-unit demand; `provider/session` owns pure session/context reuse/rotation policy and coherent StoreSource; `provider/reservation` owns atomic feasibility/claim creation; existing Store transactions remain authority; no session schema migration was added.  
`CRITICAL INVARIANTS:` I-005, I-018, I-020, I-021, I-023..I-027, I-033..I-037 plus I-008/I-009 publication fencing.  
`COMPLETED THIS ITERATION:` T-008 implementation, qualification and publication.  
`RESOLVED FINDINGS:` F-010 resolved/published by the broker; F-018 enforced by no-inference semantics; T-011/F-004 is already published.  
`OPEN CRITICAL/HIGH FINDINGS:` F-007 and residual F-014 remain until T-018/T-019; F-001 remains until T-012; F-002/F-008/F-009 remain dependency-chain work.  
`BLOCKERS:` none for T-012; T-013 remains blocked on T-012.  
`NEXT TASK:` T-012 — explainable shadow selector.  
`WHY NEXT:` T-012 is P0, all implementation prerequisites T-008..T-011 are published, and it converts the observation/demand/session/reservation foundations into a deterministic explainable provider decision without yet enabling risky write routing.  
`CRITICAL FILES:` `internal/harness/provider/session/broker.go`, `internal/harness/provider/session/broker_test.go`, `internal/harness/provider/session/concurrency_test.go`, `internal/harness/provider/session/store_source_test.go`, `internal/harness/model/provider.go`, `internal/harness/provider/antigravity/adapter.go`, `internal/harness/provider/codex/adapter.go`, `internal/harness/provider/reservation`, `internal/harness/store/store.go`, `.github/workflows/harness-ci.yml`, `.engineering/preflight/T-008.md`, `MASTER_PLAN.md`.  
`VERIFICATION COMMANDS:` for T-008 historical verification: `go test ./internal/harness/provider/session`; `go test ./...`; `go test -race ./...`; `go vet ./...`; `go test ./internal/harness/provider/session -run '^$' -bench '^BenchmarkEvaluate100Sessions$' -benchtime=1x -benchmem`; all existing harness smoke benchmarks; Windows `go test ./...`. For T-012, begin with a durable plan-first pre-flight before production edits.  
`IMPORTANT DECISIONS:` only authoritative ProviderSessionSnapshot may be reused; Codex thread->model affinity is never inferred; unknown context is never unlimited; same-affinity unusable context rotates; DRAINING may reuse but not acquire; acquire/retain thresholds are caller-configured hysteresis; candidate choice is deterministic; missing capacity health remains UNKNOWN rather than fabricated evidence.  
`REJECTED OPTIONS:` inferred Codex session/model binding, universal reuse percentages, unknown context as unlimited, GUI/transcript heuristics for affinity, new session schema, adapter rewrite, independent torn Store reads, force publication.  
`NEW PROCESS LEARNING:` an experimental branch that starts implementation before durable pre-flight should not be rewritten or silently accepted. Rebuild publication lineage from the published base with an explicit plan-first marker, preserve qualified tree content, re-run full CI on the clean lineage, and only then fast-forward `main`; T-030 should make this automatic.