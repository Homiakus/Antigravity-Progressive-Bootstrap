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

Provider-control-plane foundation completed through T-004:

- T-001 provider-domain primitives: `bf060a70833504fc5dc979181d59453276a56e0c`.
- T-002 observation adapter/registry: `ac35b5193c735897a75cb056680215a4e3aae428`.
- T-003 SQLite v14 provider-observation schema: `c5c2ebfab1814207aecb274ffb14b5ae83d9992f`.
- T-004 durable provider observation Store/SQLite implementation: verified on validation head; publication to `main` is this iteration's final action.

Next highest-leverage work after T-004 is live Antigravity/Codex observation ingestion (T-006/T-007), while T-005 v15 assignment/reservation persistence is also unblocked.

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
  +---+-------------------------+
  |                             |
Antigravity                   Codex
account/model/quota/session   account/model/quota/session
  |                             |
  +-------------+---------------+
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

T-004 implementation verification on head `2b1fa2fe549cf86b854b15cb2fdf584bbf3b06ec`:

- module graph tidy: PASS;
- bridge syntax/contracts: PASS;
- unit/integration: PASS;
- race detector: PASS;
- vet: PASS;
- compiler/store/scheduler/retry/wait smoke benchmarks: PASS;
- Windows tests: PASS.

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

## 6. Findings Registry

### F-001 — Provider capacity is not a scheduler dimension
**Status:** Planned. **Severity:** High. **Confidence:** Confirmed.  
Physical resource feasibility exists; provider quota/model/session feasibility does not. Affects T-009..T-014.

### F-002 — Codex is not a first-class provider
**Status:** Planned. **Severity:** High. **Confidence:** Confirmed.  
Normalized observation and portable execution are still missing. Affects T-007,T-012,T-014.

### F-003 — Workflow budget cannot substitute for provider quota
**Status:** Planned. **Severity:** High. **Confidence:** Confirmed.  
Existing budget tracker is permission-to-spend, not provider supply. Affects T-009..T-011.

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
**Status:** Planned. **Severity:** High.  
Affects T-013,T-014.

### F-009 — No plan digest on provider assignments
**Status:** Planned. **Severity:** High.  
Affects T-013,T-014,T-016.

### F-010 — No session/context broker
**Status:** Planned. **Severity:** Medium.  
Affects T-008,T-013.

### F-011 — Provider routing decisions are not durable/explainable
**Status:** Planned. **Severity:** Medium.  
Affects T-012,T-020,T-021.

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
Reconnect/rolling updates may arrive out of order. Account `updated_at` and model/session `observed_at` now enforce monotonic upserts; stale writes return conflict. Session IDs are fenced to their original account.

## 7. Risk Register

| Risk | Level | Mitigation |
|---|---|---|
| quota oversubscription | Critical | atomic reservation + expiry + settlement |
| duplicate side effects | Critical | effect intents + `IN_DOUBT` |
| simultaneous main writes | Critical | fenced Commit Coordinator |
| moved remote main | High | pre-push ref recheck + fast-forward only |
| stale observations | High | monotonic timestamps + deterministic latest query |
| stale plan execution | High | plan revision/digest gate |
| provider API drift | High | tolerant adapters + fixtures + dynamic discovery |
| opaque quota semantics | High | preserve native IDs/metric/confidence |
| session context exhaustion | High | session broker + checkpoint/new-session policy |
| provider flapping | Medium | hysteresis + switch penalty + circuit breaker |
| poor demand estimate | Medium | conservative p80 + settlement feedback |

## 8. Pareto Improvements

Remaining high-leverage path: Antigravity observation -> Codex observation -> capacity normalization -> v15 assignment/reservation persistence -> estimator -> atomic reservation service -> shadow selector -> TaskEnvelope/plan digest -> safe read-only routing -> provider-aware failover -> isolated writes -> Commit Coordinator.

## 9. Dependency DAG

```text
T001 DONE -> T002 DONE -> T003 DONE -> T004 DONE
                                      |-> T006 Antigravity observation
                                      |-> T007 Codex observation
                                      |-> T005 v15 assignment/reservation persistence
T004,T006,T007 -> T008 session broker
T004,T006,T007 -> T009 capacity normalization
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
**Status:** DONE. **Priority:** P0.  
Commit `bf060a70833504fc5dc979181d59453276a56e0c`.

### T-002 — Define observation adapter and live registry
**Status:** DONE. **Priority:** P0.  
Commit `ac35b5193c735897a75cb056680215a4e3aae428`.

### T-003 — Add immutable SQLite v14 provider-observation schema
**Status:** DONE. **Priority:** P0.  
Commit `c5c2ebfab1814207aecb274ffb14b5ae83d9992f`.

### T-004 — Add provider observation Store contracts
**Status:** DONE. **Priority:** P0. **Leverage:** HIGH.  
Files: `internal/harness/store/store.go`, `internal/harness/store/sqlite/provider_store.go`, provider store tests.  
Implemented account/model/session upsert/read/list, atomic capacity snapshot+window append, deterministic latest capacity reconstruction, provider/account identity checks, monotonic stale-update rejection and WAL concurrent readers.  
Tests: roundtrip, provider/state filtering, model/session reconstruction, latest ordering, atomic rollback, provider mismatch, stale account/model/session conflict, session-account immutability, concurrent readers/writer.  
Verification: full Harness CI PASS on implementation head `2b1fa2fe549cf86b854b15cb2fdf584bbf3b06ec`.  
Non-goal preserved: no scheduler/routing/provider-protocol behavior.

### T-005 — Persist assignments/reservations/usage/circuit state in SQLite v15
**Status:** READY. **Priority:** P0.  
Add `provider_assignments`, `provider_reservations`, `provider_usage_samples`, `provider_circuit_state`. Reservation states ACTIVE/SETTLED/RELEASED/EXPIRED. Tests: FK/CHECK, idempotent settlement foundations, CAS/concurrency. Dependencies: T-004.

### T-006 — Implement Antigravity observation adapter
**Status:** READY. **Priority:** P0. **Leverage:** HIGH.  
Normalize supported status/headless signals into provider-neutral account/model/quota/session observations. Preserve native bucket IDs/timestamps; no UI-pixel scraping. Deterministic fixtures for missing/unknown/malformed fields. Dependencies: T-002,T-004.

### T-007 — Implement Codex App Server observation adapter
**Status:** READY. **Priority:** P0. **Leverage:** HIGH.  
Consume account/rate-limit snapshots and rolling updates through a tolerant boundary. Preserve opaque buckets; never infer model mapping without evidence. Dependencies: T-002,T-004.

### T-008 — Build session/context broker
**Status:** TODO. **Priority:** P1.  
REUSE/NEW/CHECKPOINT_AND_NEW/UNAVAILABLE using context headroom, workspace affinity, health and model requirements; include hysteresis. Dependencies: T-004,T-006,T-007.

### T-009 — Normalize capacity and effective headroom
**Status:** TODO. **Priority:** P0.  
Keep TOKENS/REQUESTS/COST/FRACTION/OPAQUE distinct; derive only justified values; apply staleness/confidence penalties. Dependencies: T-004,T-006,T-007.

### T-010 — Add conservative demand estimator
**Status:** TODO. **Priority:** P1.  
Robust historical p50/p80 by task/provider/model/repository/context class; p80 reservation, no ML initially. Dependencies: T-005,T-009.

### T-011 — Implement atomic provider reservations
**Status:** TODO. **Priority:** P0.  
Single transaction: latest capacity + active reservations -> feasibility -> reservation + assignment. Expiry/release/settlement and 100+ concurrent reserver proof. Dependencies: T-005,T-009,T-010.

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
Provider x account x model x quota x reservation x worker lease x session context x failure timing x replay safety x plan revision x remote-main movement.

### T-023 — Add mutation testing for critical provider policy
**Status:** TODO. **Priority:** P1.  
Mutate quota boundaries, reservation subtraction, stale-plan gate, replay branch, circuit transitions, settlement idempotency and commit fencing.

### T-024 — Progressive rollout
**Status:** TODO. **Priority:** P1.  
`OBSERVE -> SHADOW -> ADVISORY -> AUTO_READ_ONLY -> AUTO_ISOLATED_WRITE -> AUTO_FAILOVER -> DEFAULT`.

### T-025 — Delete superseded legacy paths
**Status:** TODO. **Priority:** P2. Dependencies: T-024.

### T-026 — Full re-audit and convergence pass
**Status:** TODO. **Priority:** P0. Dependencies: T-025.

## 12. Testing Strategy

Per atomic task: characterization/contract -> targeted package -> integration/store -> concurrency/race where shared state changes -> property/fuzz for normalizers/state machines -> full tests -> race -> vet -> existing benchmarks -> Windows. Live provider probes remain opt-in; normal CI uses deterministic no-secret fixtures.

Observation persistence additionally requires atomicity, stale-ordering, identity-boundary and concurrent-reader tests.

## 13. Mutation Testing Strategy

Mandatory once executable routing policy exists: quota `<`/`<=`, reservation subtraction, stale-plan bypass, replay branch inversion, `IN_DOUBT` bypass, circuit transitions, settlement idempotency and commit-fencing. T-004 is persistence plumbing; mutation becomes mandatory starting with T-011 policy logic.

## 14. Performance Baselines

Do not materially regress current harness benchmarks. Add later: latest capacity with 10k snapshots; 10k reservation aggregate; selector over 1k READY x 8 candidates; 100 concurrent reservations; route explanation without N+1 queries.

## 15. Security Hardening

Credentials never enter plan/telemetry. Adapters expose health/capabilities, not secrets. Provider input is validated/bounded. Ingestion payloads are size-capped. Logs redact auth/session tokens. Remote-worker trust does not imply credential access. Only Commit Coordinator may autonomously write `main` in final architecture.

## 16. Migration Strategy

`provider domain -> observation persistence -> live adapters -> normalization -> reservation persistence -> estimator -> shadow -> TaskEnvelope -> read-only -> safe handoff -> isolated writes -> Commit Coordinator -> single-writer enforcement -> default -> legacy deletion`.

Every intermediate state must keep `main` buildable and compatible.

## 17. Deferred Work

ML demand prediction, distributed credential vault, provider marketplace ABI, monetary arbitrage, reverse-engineering opaque buckets, force-push recovery automation.

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

## 19. Completed Tasks

- T-001 DONE — `bf060a70833504fc5dc979181d59453276a56e0c`.
- T-002 DONE — `ac35b5193c735897a75cb056680215a4e3aae428`.
- T-003 DONE — `c5c2ebfab1814207aecb274ffb14b5ae83d9992f`.
- T-004 DONE on verified validation tree; final atomic `main` SHA will be recorded after fast-forward publication.

## 20. Iteration Log

### Iteration 1 — T-001
Provider primitives/IDs. Full Harness CI PASS. Commit `bf060a70833504fc5dc979181d59453276a56e0c`; fast-forward `main`; PASS.

### Iteration 2 — T-002
Observation adapter/registry; execution contract deferred to TaskEnvelope. Full Harness CI PASS. Commit `ac35b5193c735897a75cb056680215a4e3aae428`; fast-forward `main`; PASS.

### Iteration 3 — T-003
SQLite v14 observation schema; F-014 remote-main movement recorded. Full Harness CI PASS. Commit `c5c2ebfab1814207aecb274ffb14b5ae83d9992f`; fast-forward `main`; PASS.

### Iteration 4 — T-004
**Findings addressed:** F-003 durable observation boundary; F-015 stale delivery.  
**Changes:** Store contracts + SQLite account/model/session persistence, atomic capacity snapshots/windows, deterministic latest query, monotonic observation guards, session account fencing.  
**Tests:** roundtrip/filter/latest/rollback/provider mismatch/stale update/session identity/WAL concurrency.  
**Verification:** full Harness CI PASS on implementation head `2b1fa2fe549cf86b854b15cb2fdf584bbf3b06ec`; plan reconciliation is documentation-only and receives final CI before publication.  
**Plan changes:** F-015 added/resolved; I-014/I-015 added; T-004 DONE; T-005/T-006/T-007 READY.  
**Commit/Push:** pending final atomic fast-forward to `main`.  
**Result:** VERIFYING.

## 21. Definition of Final Done

No open Critical/High routing findings; Antigravity and Codex are first-class observation/execution providers; no guessed quota semantics; reservations cannot oversubscribe under races; TaskEnvelope makes handoff conversation-independent; stale plan cannot start affected writes; unsafe interruption reaches existing `IN_DOUBT`; read-only and isolated-write rollout gates pass; only fenced Commit Coordinator writes main autonomously; no force push; CI/race/vet/mutation/benchmarks pass; routing is explainable; legacy duplicates removed; final re-audit finds no fundamental Critical/High issue; final verified tree and synchronized plan are on `main`.