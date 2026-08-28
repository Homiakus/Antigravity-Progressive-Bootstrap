# MASTER PLAN — Multi-Runtime Agent Control Plane

> Repository: `Homiakus/Antigravity-Progressive-Bootstrap`  
> Plan status: ACTIVE / LIVING  
> Semantic source of truth: this file. SQLite and generated indexes are execution ledgers/caches only.

## 1. Mission

Evolve `agctl` into a quota-aware multi-runtime control plane that distributes atomic engineering tasks between Antigravity and Codex according to capabilities, provider/account capacity, reset horizon, session context, reliability, risk and estimated demand, while preserving one living plan and one fenced path to verified `main`.

Target loop:

`OBSERVE -> RECONCILE PLAN -> READY TASK -> TASK ENVELOPE -> ESTIMATE -> REFRESH PROVIDERS -> SELECT PROVIDER/MODEL/SESSION -> RESERVE -> CLAIM WORKER/WORKSPACE -> EXECUTE -> VERIFY -> REVIEW -> RECONCILE PLAN -> SETTLE USAGE -> COMMIT COORDINATOR -> VERIFIED PUSH MAIN -> CHECKPOINT -> NEXT`

## 2. Current State

The durable harness remains the foundation: adaptive DAG replanning, SQLite/WAL durability, fair READY scheduling, physical resource feasibility, worker registry, fenced attempt leases, effect intents, `IN_DOUBT`, retries, waits, approvals, artifacts/workspaces, AGY/Antigravity execution, generic agent executor, workflow budgets and Linux/Windows CI already exist.

Provider-control-plane foundation completed/verified through T-009:

- T-001 provider-domain primitives: `bf060a70833504fc5dc979181d59453276a56e0c`.
- T-002 observation adapter/registry: `ac35b5193c735897a75cb056680215a4e3aae428`.
- T-003 SQLite v14 provider-observation schema: `c5c2ebfab1814207aecb274ffb14b5ae83d9992f`.
- T-004 durable provider observation Store/SQLite implementation: `7f6c3a888318e1cca1ea2d22c8c997051f883856`.
- T-006 Antigravity observation adapter: `a46eb75f947f97f62a06129e3469fe18466d6843`.
- T-007 Codex App Server observation adapter: `cefead407f3ad4a11ac46379f2f9b72a9129d353`.
- T-009 provider-neutral capacity normalization/effective headroom: fully verified on validation head `b66d68c18edb3e138dc1acf988d7315fb252871f`; atomic publication is the current iteration's final action.

Next highest-leverage work after publication: T-005 SQLite v15 assignment/reservation/usage/circuit persistence. T-010 then becomes unblocked by T-005 + T-009.

## 3. Architecture Map

```text
MASTER_PLAN.md
      |
Plan Compiler / Reconciler
      |
Durable DAG -> Fair Scheduler
      |
TaskEnvelope + Demand Estimator
      |
Provider Selector / Capacity Broker
      |
  +---+------------------------------+
  |                                  |
Antigravity                        Codex
status-line/headless               App Server JSON-RPC
  |                                  |
coherent Observation               rate-limit baseline + sparse deltas
  |                                  | + complete model/list cycle
  +----------------+-----------------+
                   |
      unit-preserving normalization
                   |
  freshness/confidence/reset policy
                   |
   conservative effective headroom
                   |
       durable provider observations
                   |
          capacity reservation
                   |
         worker + fenced lease
                   |
         isolated workspace
                   |
                execute
                   |
         usage/effects/findings
                   |
    verify -> review -> reconcile
                   |
           Commit Coordinator
                   |
          fast-forward-only main
```

Orthogonal concepts MUST stay separate: `Worker`, `ProviderAccount`, `ProviderModel`, `ProviderSession`, workflow `budget`, provider `capacity`, execution lease, provider reservation, workspace lease and commit authority.

## 4. Baseline

Harness CI enforces module tidiness, bridge syntax/contracts, `go test ./...`, `go test -race ./...`, `go vet ./...`, compiler/store/scheduler/retry/wait smoke benchmarks and Windows tests.

Persistence is SQLite schema v14. Migrations 1..13 are immutable checksummed releases; v14 is append-only provider observation schema.

T-009 verification on `b66d68c18edb3e138dc1acf988d7315fb252871f`:

- module graph tidy: PASS;
- bridge syntax/contracts: PASS;
- unit/integration: PASS;
- race detector: PASS;
- vet: PASS;
- compiler/store/scheduler/retry/wait smoke benchmarks: PASS;
- Windows tests: PASS;
- provider-neutral normalizer adapter integration: PASS;
- fuzz/property seed corpus and 100-window benchmark compile: PASS.

## 5. System Invariants

- **I-001 Plan authority:** `MASTER_PLAN.md` is semantic truth; generated projections are rebuildable and digest-bound.
- **I-002 Explainable assignment:** provider-backed attempts must trace task, attempt, worker, account, model, session and plan revision.
- **I-003 Worker/provider separation:** worker location is not provider quota/account identity.
- **I-004 Budget/capacity separation:** allowed workflow spend is not external provider supply.
- **I-005 Atomic reservation:** selection + quota reservation commit before provider execution.
- **I-006 Execution fencing:** provider failover never bypasses existing lease epochs.
- **I-007 Fail closed on uncertain effects:** unsafe interruption reuses effect intents + `IN_DOUBT`; no blind replay.
- **I-008 Single writer to main:** ordinary workers never autonomously integrate/push `main` in multi-runtime mode.
- **I-009 No force push:** autonomous operation never uses force or force-with-lease.
- **I-010 Dynamic discovery:** no transient model-name or opaque quota-bucket hard-coding in core logic.
- **I-011 Bounded uncertainty:** unknown/stale capacity is penalized, never treated as unlimited.
- **I-012 Incremental rollout:** observe -> shadow -> advisory -> read-only -> isolated writes -> failover -> commit coordination -> default.
- **I-013 Migration immutability:** released migration SQL/checksums are append-only.
- **I-014 Monotonic observations:** older account/model/session observations cannot overwrite newer state after reconnect or delayed delivery.
- **I-015 Durable session identity:** one provider session ID cannot silently migrate between provider accounts.
- **I-016 Coherent provider snapshots:** one upstream snapshot must not be split into capacity/model/session reads from different revisions.
- **I-017 No inferred quota mapping:** native provider quota bucket identity is preserved; bucket-to-model mapping exists only when the provider explicitly supplies it.
- **I-018 Authoritative session/model attribution:** `ProviderSessionSnapshot.ModelID` may only be populated from an authoritative provider/execution linkage. Thread identity, model catalog proximity or naming heuristics are insufficient.
- **I-019 Complete catalog replacement:** a paginated provider model catalog may replace durable/current catalog state only after a complete successful pagination cycle.
- **I-020 Unit-preserving headroom:** TOKENS, REQUESTS, COST, FRACTION and OPAQUE remain distinct. Unlike or potentially overlapping quota windows are never summed or converted merely to obtain one convenient number.
- **I-021 Exhaustion proof separation:** freshness/confidence penalties may reduce effective headroom to zero, but only authoritative provider health/evidence may establish `EXHAUSTED`.
- **I-022 Reset-boundary invalidation:** an observation captured before a quota reset cannot be reused as current capacity after that reset boundary; it becomes expired until refreshed.

## 6. Findings Registry

### F-001 — Provider capacity is not a scheduler dimension
**Status:** Partially resolved. **Severity:** High. **Confidence:** Confirmed.  
T-009 provides provider-neutral effective headroom; scheduler/selector feasibility integration remains T-011/T-012.

### F-002 — Codex is not yet a complete first-class execution provider
**Status:** Partially resolved. **Severity:** High. **Confidence:** Confirmed.  
T-007 makes Codex a first-class observation source; portable execution still waits for T-013/T-014.

### F-003 — Workflow budget cannot substitute for provider quota
**Status:** Planned. **Severity:** High. **Confidence:** Confirmed.  
Existing budget tracker is permission-to-spend, not provider supply. Affects T-010,T-011.

### F-004 — No durable quota reservation ledger
**Status:** Planned. **Severity:** Critical. **Confidence:** Confirmed.  
Parallel dispatchers can oversubscribe the same headroom. Affects T-005,T-011.

### F-005 — Existing worker/lease subsystem must be reused
**Status:** Resolved by architecture. **Severity:** High.  
Provider assignment remains orthogonal to workers and fenced attempt leases.

### F-006 — Existing `IN_DOUBT` + effect intents already provide unsafe-replay protection
**Status:** Resolved by architecture. **Severity:** Critical.  
Safe provider handoff must reuse `ReclaimExpiredAttempt`/effect reconciliation. Affects T-015,T-016.

### F-007 — Direct worker pushes are unsafe with parallel providers
**Status:** Planned. **Severity:** Critical. **Confidence:** Strong.  
Affects T-018,T-019.

### F-008 — No TaskEnvelope independent of conversation history
**Status:** Planned. **Severity:** High. Affects T-013,T-014.

### F-009 — No plan digest on provider assignments
**Status:** Planned. **Severity:** High. Affects T-013,T-014,T-016.

### F-010 — No session/context broker
**Status:** Planned. **Severity:** Medium. Affects T-008,T-013.

### F-011 — Provider routing decisions are not durable/explainable
**Status:** Planned. **Severity:** Medium. Affects T-012,T-020,T-021.

### F-012 — Released SQLite migrations are immutable
**Status:** Resolved. **Severity:** High.  
T-003 appended v14 without modifying v1..v13.

### F-013 — Unified provider execution interface is premature before TaskEnvelope
**Status:** Planned. **Severity:** Medium.  
Portable execution remains deferred to T-013/T-014.

### F-014 — Concurrent autonomous work can move `main` during an iteration
**Status:** Planned. **Severity:** High. **Confidence:** Confirmed.  
Every publication must re-read `main`, reconcile semantics and fast-forward only. Affects T-018/T-019.

### F-015 — Delayed provider observations can regress durable state
**Status:** Resolved in T-004. **Severity:** High. **Confidence:** Confirmed.  
Monotonic account/model/session timestamps reject stale reconnect data; session IDs are fenced to their original account.

### F-016 — Independent provider reads can create a torn observation
**Status:** Resolved in T-006. **Severity:** High. **Confidence:** Confirmed.  
Provider-neutral `SnapshotSource` allows a single upstream payload to produce one coherent observation.

### F-017 — Codex rolling rate-limit updates are sparse and model/bucket mapping is not authoritative
**Status:** Resolved in T-007. **Severity:** High. **Confidence:** Confirmed from current App Server protocol.  
`account/rateLimits/read` creates/replaces the baseline. `account/rateLimits/updated` performs a sparse merge: nullable unavailable fields do not clear prior state. Multi-limit `rateLimitsByLimitId`, `limitId`, primary/secondary windows and reset timestamps are preserved. Bucket-to-model mapping remains intentionally unset without provider evidence. Reconnect baseline replaces pre-disconnect rolling state.

### F-018 — Codex thread/context observations do not identify the selected model
**Status:** Resolved by boundary / follow-up in T-008. **Severity:** High. **Confidence:** Confirmed from current App Server protocol.  
`thread/list` exposes durable thread identity/status/cwd/model provider, and `thread/tokenUsage/updated` exposes thread/turn usage plus `modelContextWindow`, but neither provides the selected model ID required by `ProviderSessionSnapshot`. T-007 therefore returns no generic Codex sessions and preserves token/context signal separately as `ThreadUsageObservation`. T-008/T-014 may create/reuse Codex provider sessions only after authoritative model linkage is available from execution/runtime state.

### F-019 — Partial model pagination can masquerade as model removal
**Status:** Resolved in T-007. **Severity:** High. **Confidence:** Confirmed.  
`model/list` is paginated. T-007 only replaces the in-memory catalog after a complete successful cycle ending with `nextCursor=null`; incomplete/error/duplicate/stale cycles leave the prior catalog intact.

### F-020 — Base quota validation is insufficient for routing arithmetic
**Status:** Resolved in T-009. **Severity:** High. **Confidence:** Confirmed.  
The durable observation type intentionally accepts provider-shaped numeric fields, but routing arithmetic must additionally reject NaN/Inf, `remaining > limit`, impossible zero-limit shapes and contradictory explicit `remainingFraction` versus `remaining/limit`. T-009 validates these at the derived-policy boundary without mutating released v14 persistence semantics.

### F-021 — Absolute remaining without a limit is useful but cannot safely become a fraction
**Status:** Resolved in T-009. **Severity:** High. **Confidence:** Confirmed.  
TOKENS/REQUESTS/COST may provide a trustworthy `remaining` count with no limit. T-009 preserves it as confidence/freshness-discounted `EffectiveRemaining`, counts it as quantitative evidence and deliberately leaves fractional headroom unset. Reservations may compare compatible native units later; selectors may not invent a denominator.

### F-022 — Provider reset boundaries invalidate otherwise fresh pre-reset capacity
**Status:** Resolved in T-009. **Severity:** High. **Confidence:** Confirmed.  
A timestamp can be recent while belonging to the previous quota period. T-009 expires any window observed before its `resetAt` once the reset passes; observations captured at/after the reset remain valid under normal freshness policy.

## 7. Risk Register

| Risk | Level | Mitigation |
|---|---|---|
| quota oversubscription | Critical | atomic reservation + expiry + settlement |
| duplicate side effects | Critical | effect intents + `IN_DOUBT` |
| simultaneous main writes | Critical | fenced Commit Coordinator |
| moved remote main | High | pre-push ref recheck + fast-forward only |
| stale observations | High | monotonic timestamps + freshness/expiry policy |
| pre-reset capacity reused after reset | High | reset-aware expiry (I-022) |
| torn provider observation | High | coherent `SnapshotSource`/single upstream payload |
| sparse delta erases known state | High | merge only available Codex values; baseline on reconnect |
| partial model catalog | High | replace only after complete pagination cycle |
| false thread->model affinity | High | I-018; no inferred ProviderSessionSnapshot |
| incompatible quota units combined | High | I-020; separate metric/window headroom |
| decay-to-zero mistaken for exhaustion | High | I-021; authoritative exhaustion proof only |
| malformed floating telemetry | High | finite/consistency checks at T-009 boundary |
| stale plan execution | High | plan revision/digest gate |
| provider API drift | High | tolerant adapters + deterministic fixtures + dynamic discovery |
| opaque quota semantics | High | native IDs/FRACTION/OPAQUE; no inferred mapping |
| session context exhaustion | High | broker + checkpoint/new-session policy |
| provider flapping | Medium | hysteresis + switch penalty + circuit breaker |
| poor demand estimate | Medium | conservative p80 + settlement feedback |

## 8. Pareto Improvements

Remaining high-leverage path: v15 assignment/reservation/usage persistence -> conservative estimator -> atomic reservation service -> shadow selector -> TaskEnvelope/plan digest -> safe read-only routing -> provider-aware failover -> isolated writes -> Commit Coordinator.

## 9. Dependency DAG

```text
T001 DONE -> T002 DONE -> T003 DONE -> T004 DONE
                                      |-> T006 Antigravity observation DONE
                                      |-> T007 Codex observation DONE
                                      |-> T005 v15 assignment/reservation persistence READY
T004,T006,T007 -> T008 session broker READY
T004,T006,T007 -> T009 capacity normalization DONE
T005,T009 -> T010 estimator -> T011 reservations -> T012 shadow selector
T012 -> T013 TaskEnvelope + digest -> T014 auto read-only -> T015 provider failures
T013,T015 -> T016 safe handoff -> T017 isolated writes -> T018 Commit Coordinator -> T019 single-main-writer enforcement
T012 -> T020 explain route
T004,T005,T012,T020 -> T021 dashboard
T005..T019 -> T022 adversarial/property/chaos
T011..T019 -> T023 mutation
T014..T023 -> T024 rollout -> T025 legacy deletion -> T026 final re-audit
```

## 10. Implementation Phases

- **A Domain/persistence:** T-001..T-005.
- **B Observe/normalize:** T-006..T-009.
- **C Predict/shadow:** T-010..T-012.
- **D Portable execution:** T-013..T-016.
- **E Multi-provider writes:** T-017..T-019.
- **F Proof/operations:** T-020..T-026.

## 11. Atomic Tasks
### T-001 — Add provider-domain primitives
**Status:** DONE. **Priority:** P0. Commit `bf060a70833504fc5dc979181d59453276a56e0c`.

### T-002 — Define observation adapter and live registry
**Status:** DONE. **Priority:** P0. Commit `ac35b5193c735897a75cb056680215a4e3aae428`.

### T-003 — Add immutable SQLite v14 provider-observation schema
**Status:** DONE. **Priority:** P0. Commit `c5c2ebfab1814207aecb274ffb14b5ae83d9992f`.

### T-004 — Add provider observation Store contracts
**Status:** DONE. **Priority:** P0. **Leverage:** HIGH.  
Account/model/session upsert/read/list, atomic capacity snapshot+window append, deterministic latest reconstruction, identity checks, monotonic stale-update rejection and WAL concurrent readers. Commit `7f6c3a888318e1cca1ea2d22c8c997051f883856`.

### T-005 — Persist assignments/reservations/usage/circuit state in SQLite v15
**Status:** READY / NEXT. **Priority:** P0. **Leverage:** HIGH.  
Add `provider_assignments`, `provider_reservations`, `provider_usage_samples`, `provider_circuit_state`. Reservation states ACTIVE/SETTLED/RELEASED/EXPIRED. Tests: FK/CHECK, idempotent settlement foundations, CAS/concurrency. Dependencies: T-004.

### T-006 — Implement Antigravity observation adapter
**Status:** DONE. **Priority:** P0. **Leverage:** HIGH.  
Official status-line/headless normalization, bounded/tolerant parsing, native quota IDs, FRACTION/OPAQUE semantics, no inferred bucket->model mapping, privacy-preserving workspace affinity, separate cumulative usage parser and coherent one-read snapshot.  
Verification: full Linux/Windows Harness CI PASS. Commit `a46eb75f947f97f62a06129e3469fe18466d6843`.

### T-007 — Implement Codex App Server observation adapter
**Status:** DONE. **Priority:** P0. **Leverage:** HIGH.  
Thread-safe Codex observation adapter with baseline+sparse rate-limit state, reconnect replacement, native multi-limit windows, complete model-list cycles and no guessed session/model binding.  
Verification: full Linux/Windows Harness CI PASS. Commit `cefead407f3ad4a11ac46379f2f9b72a9129d353`.

### T-008 — Build session/context broker
**Status:** READY. **Priority:** P1.  
REUSE/NEW/CHECKPOINT_AND_NEW/UNAVAILABLE using context headroom, workspace affinity, health and model requirements; include hysteresis. For Codex, absence of authoritative thread->model binding must degrade to NEW/UNAVAILABLE rather than fabricate reuse affinity. Dependencies: T-004,T-006,T-007.

### T-009 — Normalize capacity and effective headroom
**Status:** DONE / VERIFIED, publication pending. **Priority:** P0. **Leverage:** HIGH.  
Files: `internal/harness/provider/capacity/{normalize.go,normalize_test.go,absolute_test.go,adapter_integration_test.go}`.  
Semantics: preserves TOKENS/REQUESTS/COST/FRACTION/OPAQUE and native ModelID/window IDs; never sums unlike or potentially overlapping windows. Fractional lower bound is the minimum of independently justified effective fractions. Absolute TOKENS/REQUESTS/COST `remaining` without a denominator remains `EffectiveRemaining` in native units and never becomes an invented fraction.  
Uncertainty: configurable `FreshFor/ExpireAfter/MaxFutureSkew`; linear freshness decay multiplies source confidence. Windows observed before `resetAt` expire once the reset passes. Snapshot and per-window age are tracked independently.  
Evidence: UNKNOWN/PARTIAL/QUANTIFIED/STALE/EXHAUSTED/UNAVAILABLE is separate from `ProviderHealth`; an effective zero caused by low confidence/staleness is not an exhaustion proof. Only source provider evidence carries `ProvenExhausted`.  
Validation: rejects NaN/Inf, remaining above limit, invalid zero-limit states, contradictory explicit fraction vs remaining/limit and duplicate native window IDs; deterministic ordering; no model/bucket inference.  
Tests: metric separation, conservative bottleneck, absolute remaining without limit, confidence/freshness decay, reset crossing, zero-vs-exhaustion separation, unavailable/exhausted source states, OPAQUE behavior, contradictory/non-finite telemetry, duplicate IDs, deterministic ordering/model attribution, mixed known/unknown evidence, clock skew, earliest reset, policy validation, Antigravity/Codex integration, fuzz fraction bounds and 100-window benchmark.  
Verification: full Linux/Windows Harness CI PASS on `b66d68c18edb3e138dc1acf988d7315fb252871f`. Dependencies: T-004,T-006,T-007.

### T-010 — Add conservative demand estimator
**Status:** BLOCKED ON T-005 PUBLICATION + T-009 PUBLICATION. **Priority:** P1.  
Robust historical p50/p80 by task/provider/model/repository/context class; p80 reservation, no ML initially. Preserve demand units so native absolute headroom is compared only with compatible estimates. Dependencies: T-005,T-009.

### T-011 — Implement atomic provider reservations
**Status:** TODO. **Priority:** P0.  
Single transaction: latest capacity + active reservations -> feasibility -> reservation + assignment. Expiry/release/settlement and 100+ concurrent reserver proof. Reservation arithmetic must respect I-020 native units and I-021 exhaustion-proof separation. Dependencies: T-005,T-009,T-010.

### T-012 — Add explainable shadow selector
**Status:** TODO. **Priority:** P0.  
Hard filter by health/capabilities/model/context/risk; soft rank by quota pressure/reset/session affinity/reliability/switch cost/uncertainty; deterministic tie-break. Dependencies: T-008..T-011.

### T-013 — Introduce TaskEnvelope and plan digest
**Status:** TODO. **Priority:** P0.  
Portable task contract with revision/digest, objective/evidence/scope/non-goals/invariants/findings/capabilities/acceptance/verification/risk/replay/workspace/checkpoint. Defines provider-neutral execution request. Dependencies: T-012.

### T-014 — Enable automatic read-only provider routing
**Status:** TODO. **Priority:** P1.  
Only explicit read-only/replay-safe nodes initially. Existing worker lease remains authoritative. Dependencies: T-011..T-013.

### T-015 — Make failures/retries provider-aware
**Status:** TODO. **Priority:** P1.  
Quota/rate/spend/auth/model/overload/protocol/context classes; deterministic code/test failure never triggers failover. Dependencies: T-014.

### T-016 — Safe provider handoff via existing `IN_DOUBT`
**Status:** TODO. **Priority:** P0.  
Reuse `ReclaimExpiredAttempt`, effect intents and reconciliation; unsafe writes never blind-replay. Dependencies: T-013,T-015.

### T-017 — Enable isolated write routing
**Status:** TODO. **Priority:** P1.  
Bind assignments to existing worktree/workspace isolation; workers may checkpoint locally but not push main. Dependencies: T-016.

### T-018 — Add fenced Commit Coordinator
**Status:** TODO. **Priority:** P0.  
Single coordinator integrates verified checkpoints, rechecks remote main, verifies/reconciles and fast-forwards only. Dependencies: T-017.

### T-019 — Enforce single-writer main policy
**Status:** TODO. **Priority:** P0.  
Static/policy tests prove ordinary workers lack autonomous main-push authority. Dependencies: T-018.

### T-020 — Add provider diagnostics and `agctl route explain`
**Status:** TODO. **Priority:** P1. Dependencies: T-012.

### T-021 — Add provider/quota dashboard
**Status:** TODO. **Priority:** P2. Dependencies: T-004,T-005,T-012,T-020.

### T-022 — Add multidimensional concurrency/property/chaos tests
**Status:** TODO. **Priority:** P0.  
Provider x account x model x quota metric/window x freshness/reset x reservation x worker lease x session context x failure timing x replay safety x plan revision x remote-main movement.

### T-023 — Add mutation testing for critical provider policy
**Status:** TODO. **Priority:** P1.  
Mutate quota boundaries, unit compatibility, freshness/reset expiry, reservation subtraction, stale-plan gate, replay branch, circuit transitions, settlement idempotency and commit fencing.

### T-024 — Progressive rollout
**Status:** TODO. **Priority:** P1.  
`OBSERVE -> SHADOW -> ADVISORY -> AUTO_READ_ONLY -> AUTO_ISOLATED_WRITE -> AUTO_FAILOVER -> DEFAULT`.

### T-025 — Delete superseded legacy paths
**Status:** TODO. **Priority:** P2. Dependencies: T-024.

### T-026 — Full re-audit and convergence pass
**Status:** TODO. **Priority:** P0. Dependencies: T-025.

## 12. Testing Strategy

Per atomic task: characterization/contract -> targeted package -> integration/store -> concurrency/race where shared state changes -> property/fuzz for normalizers/state machines -> full tests -> race -> vet -> existing benchmarks -> Windows. Live provider probes remain opt-in; normal CI uses deterministic no-secret fixtures.

Observation adapters additionally require unknown-field tolerance, known-field bounds, input-size caps, deterministic ordering, no guessed semantics, stale/reconnect behavior and coherent snapshots. Rolling protocols require baseline+delta merge tests. Paginated catalogs require full-cycle atomic replacement tests.

Capacity normalizers additionally require unit preservation, finite-number checks, fraction/absolute consistency, no inferred denominators, confidence/freshness monotonicity, reset-boundary expiry, zero-vs-exhaustion separation, deterministic bottlenecks and provider-adapter equivalence tests.

## 13. Mutation Testing Strategy

Mandatory once executable routing policy exists: quota `<`/`<=`, metric-unit compatibility, freshness/reset gates, reservation subtraction, stale-plan bypass, replay branch inversion, `IN_DOUBT` bypass, circuit transitions, settlement idempotency and commit-fencing. Mutation becomes mandatory starting with T-011 policy logic.

## 14. Performance Baselines

Do not materially regress current harness benchmarks. T-009 includes a 100-window allocation benchmark for the normalizer. Add later: latest capacity with 10k snapshots; 10k reservation aggregate; selector over 1k READY x 8 candidates; 100 concurrent reservations; route explanation without N+1 queries.

## 15. Security Hardening

Credentials never enter plan/telemetry. Adapters expose normalized health/capabilities, not secrets. Provider input is validated/bounded. Derived routing arithmetic rejects non-finite or internally contradictory telemetry. Ingestion payloads are size-capped. Status-line email/transcript/path data is discarded or one-way fingerprinted where affinity is required. JSON-RPC error/auth payloads are classified but not copied into durable provider state. Logs redact auth/session tokens. Remote-worker trust does not imply credential access. Only Commit Coordinator may autonomously write `main` in final architecture.

## 16. Migration Strategy

`provider domain -> observation persistence -> Antigravity/Codex live adapters -> normalization -> reservation persistence -> estimator -> shadow -> TaskEnvelope -> read-only -> safe handoff -> isolated writes -> Commit Coordinator -> single-writer enforcement -> default -> legacy deletion`.

Every intermediate state must keep `main` buildable and compatible.

## 17. Deferred Work

ML demand prediction, distributed credential vault, provider marketplace ABI, monetary arbitrage, reverse-engineering opaque buckets/model mappings, force-push recovery automation.

## 18. Rejected Decisions

- **R-001 Provider == Worker:** rejected.
- **R-002 budget tracker == quota manager:** rejected.
- **R-003 context inside quota window:** rejected.
- **R-004 hard-coded model/bucket IDs:** rejected.
- **R-005 failover on any failure:** rejected.
- **R-006 every worker pushes main:** rejected.
- **R-007 provider execution interface before TaskEnvelope:** rejected.
- **R-008 merge obsolete diverged validation branch:** rejected; reconcile against current main only.
- **R-009 last-write-wins for provider observations:** rejected after F-015; stale reconnect data must fail closed.
- **R-010 three independent live reads for one provider snapshot:** rejected after F-016; use coherent `SnapshotSource` where available.
- **R-011 infer quota bucket -> active model from naming:** rejected; only explicit provider evidence may establish mapping.
- **R-012 sparse Codex null == clear field:** rejected after F-017; rolling null/unavailable metadata preserves baseline state.
- **R-013 infer Codex thread model from catalog/default/provider:** rejected after F-018; model attribution must be authoritative.
- **R-014 replace model catalog from partial pagination:** rejected after F-019; only complete cycles replace state.
- **R-015 sum/convert unlike quota metrics:** rejected after T-009/F-021; preserve native units and only compare justified fractions.
- **R-016 effective headroom zero == provider exhausted:** rejected after T-009/I-021; decay/uncertainty is not exhaustion proof.
- **R-017 retain pre-reset observation after reset:** rejected after T-009/F-022; reset crossing expires the previous quota-period evidence.

## 19. Completed Tasks

- T-001 DONE — `bf060a70833504fc5dc979181d59453276a56e0c`.
- T-002 DONE — `ac35b5193c735897a75cb056680215a4e3aae428`.
- T-003 DONE — `c5c2ebfab1814207aecb274ffb14b5ae83d9992f`.
- T-004 DONE — `7f6c3a888318e1cca1ea2d22c8c997051f883856`.
- T-006 DONE — `a46eb75f947f97f62a06129e3469fe18466d6843`.
- T-007 DONE — `cefead407f3ad4a11ac46379f2f9b72a9129d353`.
- T-009 DONE/VERIFIED — validation head `b66d68c18edb3e138dc1acf988d7315fb252871f`; publication commit recorded after fast-forward.

## 20. Iteration Log

### Iteration 1 — T-001
Provider primitives/IDs. Full Harness CI PASS. Commit `bf060a70833504fc5dc979181d59453276a56e0c`; fast-forward `main`; PASS.

### Iteration 2 — T-002
Observation adapter/registry; execution contract deferred to TaskEnvelope. Full Harness CI PASS. Commit `ac35b5193c735897a75cb056680215a4e3aae428`; fast-forward `main`; PASS.

### Iteration 3 — T-003
SQLite v14 observation schema; F-014 remote-main movement recorded. Full Harness CI PASS. Commit `c5c2ebfab1814207aecb274ffb14b5ae83d9992f`; fast-forward `main`; PASS.

### Iteration 4 — T-004
Provider Store/SQLite observations, monotonic stale-update guards and session account fencing. Full Harness CI PASS. Commit `7f6c3a888318e1cca1ea2d22c8c997051f883856`; fast-forward main; PASS.

### Iteration 5 — T-006
Antigravity bounded/tolerant observation adapter, coherent snapshot source, privacy-preserving workspace fingerprint and headless usage parser. F-016 resolved; F-017 discovered. Full Linux/Windows CI PASS. Commit `a46eb75f947f97f62a06129e3469fe18466d6843`; fast-forward main; PASS.

### Iteration 6 — T-007
**Findings addressed:** F-017 sparse Codex rate-limit state; F-019 paginated model catalog.  
**Unexpected finding:** F-018 — current Codex thread/context telemetry has no authoritative selected-model ID, so generic session reuse must fail closed.  
**Changes:** thread-safe Codex adapter; baseline + sparse rate-limit merge; reconnect replacement; native multi-limit windows; conservative exhaustion classification; complete model catalog replacement; separate thread token/context observation; bounded JSON-RPC parsing.  
**Tests:** multi/single rate-limit buckets, sparse-null preservation, ambiguity/no-baseline/stale deltas, reconnect, RPC errors, exhaustion, bounds, complete/incomplete/duplicate/stale model pages, no model/session guessing, token usage, oversized payloads, concurrent updates/reads.  
**Verification:** full Linux/Windows Harness CI PASS on implementation head `5b9b8852333620be0857d885bf1db5d8203f6f43` and reconciled head `ccd674547088215ba0deb1c34065f515645a4504`.  
**Plan changes:** I-018/I-019; F-017/F-019 resolved; F-018 boundary decision; T-007 DONE; T-008/T-009 READY.  
**Commit:** `cefead407f3ad4a11ac46379f2f9b72a9129d353`.  
**Push:** `main`, fast-forward, no force.  
**Result:** PASS.

### Iteration 7 — T-009
**Findings addressed:** F-001 provider-neutral capacity semantics; F-020 routing arithmetic validation; F-021 absolute headroom without denominator; F-022 reset-boundary invalidation.  
**Unexpected findings:** durable observation validation is intentionally broader than safe routing arithmetic; absolute remaining can be useful without a limit; a recent timestamp can still belong to the previous quota period after reset.  
**Changes:** pure provider-neutral capacity normalizer; unit-preserving native windows; effective absolute/fractional headroom; configurable freshness/confidence decay; reset-aware expiry; evidence-state separation from provider health; deterministic conservative bottleneck.  
**Tests:** metric separation, absolute remaining, freshness/confidence, resets, zero/exhaustion, OPAQUE, malformed/non-finite telemetry, duplicates, model attribution, mixed evidence, future skew, provider adapter integration, fuzz fraction invariants and 100-window benchmark.  
**Verification:** full Linux/Windows Harness CI PASS on `b66d68c18edb3e138dc1acf988d7315fb252871f`.  
**Plan changes:** I-020/I-021/I-022; F-020/F-021/F-022 resolved; T-009 DONE; T-005 NEXT; T-010 contract tightened to preserve demand units.  
**Commit/Push:** atomic fast-forward publication is current final action; no force.  
**Result:** PASS pending publication checkpoint.

## 21. Definition of Final Done

No open Critical/High routing findings; Antigravity and Codex are first-class observation/execution providers; no guessed quota semantics, denominator, unit conversion or session/model attribution; coherent provider observations cannot tear across revisions; stale/pre-reset capacity cannot masquerade as current supply; reservations cannot oversubscribe under races; TaskEnvelope makes handoff conversation-independent; stale plan cannot start affected writes; unsafe interruption reaches existing `IN_DOUBT`; read-only and isolated-write rollout gates pass; only fenced Commit Coordinator writes main autonomously; no force push; CI/race/vet/mutation/benchmarks pass; routing is explainable; legacy duplicates removed; final re-audit finds no fundamental Critical/High issue; final verified tree and synchronized plan are on `main`.