# MASTER PLAN — Multi-Runtime Agent Control Plane

> Repository: `Homiakus/Antigravity-Progressive-Bootstrap`  
> Plan status: ACTIVE / LIVING  
> Semantic source of truth: this file. SQLite and generated indexes are execution ledgers/caches only.

## 1. Mission

Evolve `agctl` into a quota-aware multi-runtime control plane that can distribute atomic software-engineering tasks between Antigravity and Codex according to capabilities, provider/account quota, reset horizon, session context, reliability, risk and estimated demand, while preserving one living plan and one fenced path to verified `main`.

Target loop:

`OBSERVE -> RECONCILE PLAN -> READY TASK -> TASK ENVELOPE -> ESTIMATE -> REFRESH PROVIDERS -> SELECT PROVIDER/MODEL/SESSION -> RESERVE -> CLAIM WORKER/WORKSPACE -> EXECUTE -> VERIFY -> REVIEW -> RECONCILE PLAN -> SETTLE USAGE -> COMMIT COORDINATOR -> VERIFIED PUSH MAIN -> CHECKPOINT -> NEXT`

## 2. Current State

The existing durable harness is the foundation, not a rewrite target. It already has adaptive DAG replanning, SQLite/WAL durability, fair READY scheduling, physical resource feasibility, worker registry, fenced attempt leases, effect intents, `IN_DOUBT`, retries, waits, approvals, artifacts/workspaces, AGY/Antigravity execution, a generic agent executor, workflow budgets and Linux/Windows CI.

Current missing layer: provider-account/model/session capacity and assignment. Scheduler feasibility currently models CPU/RAM/GPU/disk/build/browser resources, not external LLM capacity.

Completed foundation:

- T-001 provider-domain primitives are on `main` as `bf060a70833504fc5dc979181d59453276a56e0c`.
- T-002 observation adapter/registry is verified and complete in the current iteration.

## 3. Architecture Map

```text
USER GOAL
   |
MASTER_PLAN.md
   |
Plan Compiler / Reconciler
   |
Durable DAG
   |
Fair Scheduler
   |
TaskEnvelope + Demand Estimator
   |
Provider Selector / Capacity Broker
   |
   +----------------------+----------------------+
   |                                             |
Antigravity account                            Codex account
models / quota / sessions                     models / quota / sessions
   |                                             |
   +----------------------+----------------------+
                          |
                  capacity reservation
                          |
                    worker selection
                          |
                     fenced lease
                          |
                 isolated workspace
                          |
                       execute
                          |
              usage / effects / findings
                          |
          verify -> review -> plan reconcile
                          |
                  Commit Coordinator
                          |
                 fast-forward-only main
```

Orthogonal concepts MUST stay separate:

- `Worker`: physical execution location/capabilities.
- `ProviderAccount`: external auth/quota pool.
- `ProviderModel`: model offered by an account/provider.
- `ProviderSession`: reusable conversational/context state.
- workflow `budget`: what the workflow is permitted to spend.
- provider `capacity`: what the external service can currently supply.
- execution lease: fenced attempt ownership.
- provider reservation: held quota/capacity.
- workspace lease: write ownership for a workspace/worktree.
- commit lease: authority to integrate/push `main`.

## 4. Baseline

Repository CI (`.github/workflows/harness-ci.yml`) enforces module tidiness, JS bridge syntax/contracts, `go test ./...`, `go test -race ./...`, `go vet ./...`, compiler/store/scheduler/retry/wait smoke benchmarks and Windows tests.

Persistence baseline before provider work: SQLite schema v13 with immutable checksummed migrations. Provider persistence must begin at v14; migrations 1..13 must remain byte-identical.

## 5. System Invariants

- **I-001 Plan authority.** `MASTER_PLAN.md` is the semantic source of truth; generated projections carry a source digest/revision and are rebuildable.
- **I-002 Explainable assignment.** Every provider-backed attempt must be attributable to task, attempt, worker, provider account, model, session and plan revision.
- **I-003 Worker/provider separation.** A worker is not a provider account. One worker may expose multiple accounts/providers.
- **I-004 Budget/capacity separation.** Workflow spending permission and external provider supply are separate hard constraints.
- **I-005 Atomic reservation.** Selection and quota reservation must commit atomically before provider execution starts.
- **I-006 Existing execution fencing remains authoritative.** Provider failover never bypasses attempt lease epochs.
- **I-007 Fail closed on uncertain effects.** Unsafe interrupted writes reuse existing effect-intent reconciliation and `IN_DOUBT`; no blind replay.
- **I-008 Single writer to main.** Workers may make isolated checkpoint commits; only Commit Coordinator may autonomously integrate/push `main`.
- **I-009 No force push.** Normal autonomous operation never uses `--force` or `--force-with-lease`.
- **I-010 Dynamic discovery.** Core logic does not hard-code transient provider model names or reverse-engineer opaque quota IDs.
- **I-011 Bounded uncertainty.** Unknown/stale provider capacity lowers score/concurrency; it is never treated as unlimited.
- **I-012 Incremental migration.** Observe -> shadow -> advisory -> auto read-only -> isolated writes -> failover -> commit coordination -> default -> legacy deletion.

## 6. Findings Registry

### F-001 — Provider capacity is not a scheduler dimension
**Status:** Planned. **Severity:** High. **Confidence:** Confirmed.  
Physical feasibility exists; provider quota/model/session feasibility does not. Affects T-001..T-013.

### F-002 — Codex is not a first-class provider
**Status:** Planned. **Severity:** High. **Confidence:** Confirmed.  
Current generic-terminal support does not expose normalized account/model/quota/session contracts. Affects T-002,T-007,T-012.

### F-003 — Workflow budget cannot substitute for provider quota
**Status:** Planned. **Severity:** High. **Confidence:** Confirmed.  
`internal/harness/budget` tracks allowed task/workflow consumption, not external service supply. Affects T-003..T-011.

### F-004 — No durable quota reservation ledger
**Status:** Planned. **Severity:** Critical. **Confidence:** Confirmed.  
Parallel dispatchers can otherwise observe the same headroom and oversubscribe it. Affects T-005,T-010,T-011.

### F-005 — Existing worker/lease subsystem must be reused
**Status:** Resolved by architecture. **Severity:** High.  
Provider assignment is orthogonal to `Worker` and existing fenced leases. Affects T-001,T-012,T-016.

### F-006 — Existing `IN_DOUBT` + effect intents already provide unsafe-replay detection
**Status:** Resolved by architecture. **Severity:** Critical.  
`ReclaimExpiredAttempt` already uses effect intent state to decide retry vs `IN_DOUBT`. Safe provider handoff must reuse it. Affects T-015,T-016.

### F-007 — Direct worker pushes are unsafe with parallel providers
**Status:** Planned. **Severity:** Critical. **Confidence:** Strong.  
Parallel successful workers can race semantically on `main`. Affects T-018,T-019.

### F-008 — No TaskEnvelope independent of conversation history
**Status:** Planned. **Severity:** High.  
Provider handoff is token-expensive and can lose invariants without a portable task contract. Affects T-013,T-014.

### F-009 — No plan digest on provider assignments
**Status:** Planned. **Severity:** High.  
An attempt can otherwise continue after a material plan revision. Affects T-013,T-014,T-016.

### F-010 — No session/context broker
**Status:** Planned. **Severity:** Medium.  
Naive quota balancing can waste tokens by abandoning a high-affinity context. Affects T-008,T-013.

### F-011 — Provider routing decisions are not durable/explainable
**Status:** Planned. **Severity:** Medium.  
Affects T-009,T-020,T-021.

### F-012 — Released SQLite migrations are immutable
**Status:** Resolved by architecture. **Severity:** High.  
Provider persistence starts at v14; v1..v13 are untouched. Affects T-003,T-005.

### F-013 — Unified provider execution interface is premature before TaskEnvelope
**Status:** Planned. **Severity:** Medium. **Confidence:** Strong.  
Evidence: `internal/harness/executor/agent/agent.go` uses a `ModelClient` loop while AGY/process execution has a distinct protocol/executor path. Freezing `AgentRuntime` during T-002 would couple observation to incomplete portable execution semantics.  
**Decision:** T-002 is observation-only; portable provider execution is defined with/after T-013 TaskEnvelope and activated in T-014. Affects T-002,T-013,T-014.

## 7. Risk Register

| Risk | Level | Mitigation |
|---|---|---|
| quota oversubscription | Critical | atomic reservation transaction + expiry + settlement |
| duplicate side effects during failover | Critical | effect intents + existing `IN_DOUBT` + replay-safety gate |
| simultaneous `main` writes | Critical | fenced Commit Coordinator |
| stale plan execution | High | plan revision/digest on TaskEnvelope/assignment |
| provider API drift | High | tolerant adapter boundary + dynamic discovery + fixtures |
| opaque quota semantics | High | preserve native IDs/metric type/confidence; never guess mapping |
| session context exhaustion | High | session broker + checkpoint/compact/new-session policy |
| provider flapping | Medium | hysteresis + switch penalty + circuit breaker |
| poor demand estimate | Medium | conservative p80 + uncertainty penalty + online settlement |
| CI requiring secrets | Medium | deterministic fixtures; live probes opt-in only |

## 8. Pareto Improvements

Highest leverage sequence: provider model -> live observation registry -> durable observations -> Antigravity/Codex adapters -> normalized capacity -> conservative estimator -> atomic reservations -> shadow selector -> TaskEnvelope/plan digest -> safe read-only routing -> provider-aware retry/handoff -> isolated writes -> Commit Coordinator.

## 9. Dependency DAG

```text
T001 provider primitives
  -> T002 observation adapter/registry
      -> T006 Antigravity adapter
      -> T007 Codex adapter
  -> T003 SQLite v14 observations
      -> T004 observation store
          -> T008 session broker
          -> T009 capacity normalization
      -> T005 assignment/reservation/usage v15
          -> T010 estimator
          -> T011 reservation service
              -> T012 shadow selector
                  -> T013 TaskEnvelope + plan digest
                      -> T014 auto read-only routing
                      -> T015 provider-aware failures
                          -> T016 safe handoff / IN_DOUBT reuse
                              -> T017 isolated write routing
                                  -> T018 Commit Coordinator
                                      -> T019 enforce single-writer main
T020 explain route <- T012
T021 dashboard <- T004,T005,T012,T020
T022 adversarial/property/chaos grows with T005..T019
T023 mutation <- T011..T019
T024 rollout <- T014..T023
T025 legacy deletion <- T024
T026 final re-audit <- T025
```

## 10. Implementation Phases

- **A Domain foundation:** T-001..T-005; no routing behavior change.
- **B Observe:** T-006..T-009; collect provider state, legacy execution remains authoritative.
- **C Predict/shadow:** T-010..T-012; explain decisions without controlling execution.
- **D Portable execution:** T-013..T-016; envelope, plan freshness, read-only routing, safe failover.
- **E Multi-provider writes:** T-017..T-019; worktree isolation and one fenced `main` writer.
- **F Proof/operations:** T-020..T-026; diagnostics, dashboard, adversarial testing, rollout, deletion, re-audit.

## 11. Atomic Tasks

### T-001 — Add provider-domain primitives
**Status:** DONE. **Priority:** P0. **Leverage:** HIGH.  
Files: `internal/harness/model/provider.go`, `ids.go`, provider tests. Added provider/account/session/quota vocabulary and provider IDs without runtime behavior changes.  
Verification: full Linux/Windows Harness CI PASS.  
Commit on main: `bf060a70833504fc5dc979181d59453276a56e0c`.

### T-002 — Define observation adapter and live registry
**Status:** DONE. **Priority:** P0. **Leverage:** HIGH.  
Files: `internal/harness/provider/provider.go`, `registry.go`, `registry_test.go`.  
Implemented account-scoped `CapacitySource`, `ModelSource`, `SessionSource`, composite observation `Adapter`, and concurrency-safe deterministic `Registry`. Registry binds durable account identity to the live refresh adapter; it does not persist observations.  
Non-goal: portable provider execution is deferred; T-013/T-014 own that boundary after TaskEnvelope exists.  
Tests: register/get/list, deterministic ordering, duplicate rejection, provider-kind mismatch, invalid state/identity, concurrent register/read under race detector.  
Verification: full Linux/Windows Harness CI PASS, including unit/integration, race, vet and all configured smoke benchmarks.  
Acceptance: no scheduler/execution behavior changes; no provider-specific logic in core registry.  
Dependencies: T-001.

### T-003 — Add immutable SQLite v14 provider-observation schema
**Status:** READY. **Priority:** P0.  
Add `provider_accounts`, `provider_models`, `provider_capacity_snapshots`, `provider_quota_windows`, `provider_sessions`; bump SchemaVersion only by appending v14. Tests: fresh DB, v13->v14 upgrade, FK/integrity, immutable prior checksum contract. Dependencies: T-001.

### T-004 — Add provider observation store contracts
**Status:** TODO. **Priority:** P0.  
Extend Store Reader/Tx and SQLite implementation for account/model/session upsert, capacity append, latest capacity and usable-account queries. Tests: roundtrip, ordering/staleness, conflicts, concurrent readers. Dependencies: T-003.

### T-005 — Persist assignments/reservations/usage/circuit state in v15
**Status:** TODO. **Priority:** P0.  
Tables: `provider_assignments`, `provider_reservations`, `provider_usage_samples`, `provider_circuit_state`. Reservation states ACTIVE/SETTLED/RELEASED/EXPIRED. Tests include idempotent settlement and concurrent CAS. Dependencies: T-003,T-004.

### T-006 — Implement Antigravity observation adapter
**Status:** TODO. **Priority:** P0.  
Normalize supported status/headless signals into provider-neutral account/model/quota/session snapshots. Preserve native bucket IDs and timestamps; no UI-pixel scraping. Fixtures cover missing/unknown fields and malformed input. Dependencies: T-002,T-004.

### T-007 — Implement Codex App Server observation adapter
**Status:** TODO. **Priority:** P0.  
Consume account/rate-limit snapshots and rolling updates through tolerant protocol boundary. Preserve opaque buckets; never infer model mapping without provider evidence. Tests: fixtures, rolling update merge, reconnect, auth/rate-limit classification. Dependencies: T-002,T-004.

### T-008 — Build session/context broker
**Status:** TODO. **Priority:** P1.  
Outputs REUSE/NEW/CHECKPOINT_AND_NEW/UNAVAILABLE based on context headroom, workspace fingerprint, affinity, health and model requirements. Add anti-thrashing hysteresis. Dependencies: T-004,T-006,T-007.

### T-009 — Normalize capacity and effective headroom
**Status:** TODO. **Priority:** P0.  
Keep TOKENS/REQUESTS/COST/FRACTION/OPAQUE metrics separate; derive only justified values. Add staleness/confidence penalties. Dependencies: T-004,T-006,T-007.

### T-010 — Add conservative demand estimator
**Status:** TODO. **Priority:** P1.  
Use robust historical statistics by task/provider/model/repository/context class. Produce p50/p80 + confidence; reserve conservative p80. No ML dependency initially. Dependencies: T-005,T-009.

### T-011 — Implement atomic provider reservations
**Status:** TODO. **Priority:** P0.  
In one store transaction read latest capacity + active reservations, test headroom, insert reservation + assignment. Add expiry/release/settlement and 100+ concurrent-reserver proof. Dependencies: T-005,T-009,T-010.

### T-012 — Add explainable shadow selector
**Status:** TODO. **Priority:** P0.  
Hard filter: health/capabilities/model/context/risk. Soft ranking: projected quota pressure, reset horizon, session affinity, reliability, switch penalty, uncertainty. Shadow-only initially; deterministic tie-break and recorded rejection factors. Dependencies: T-008..T-011.

### T-013 — Introduce TaskEnvelope and plan digest
**Status:** TODO. **Priority:** P0.  
Envelope carries task/plan revision+digest, objective, evidence, scope/non-goals, invariants, findings, required capabilities, acceptance/verification, risk, replay safety, workspace and checkpoint. Generated plan index is disposable and source-digest bound. This task also defines the provider-neutral execution request contract deferred from T-002. Dependencies: T-012.

### T-014 — Enable automatic read-only provider routing
**Status:** TODO. **Priority:** P1.  
Only explicitly read-only/replay-safe nodes use selector/reservation/provider execution. Existing worker lease remains authoritative. Dependencies: T-011..T-013.

### T-015 — Make failures/retries provider-aware
**Status:** TODO. **Priority:** P1.  
Add quota exhausted/low, rate limit, spend control, auth, model unavailable, overload, protocol/transient and context low/exhausted classes. Deterministic code/test failure does not trigger provider failover. Dependencies: T-014.

### T-016 — Implement safe provider handoff via existing `IN_DOUBT`
**Status:** TODO. **Priority:** P0.  
Reuse `ReclaimExpiredAttempt`, effect intents and reconciliation. Unsafe interrupted writes never blind-replay. Test provider death before effects, after safe effect, after unsafe dispatched effect, recovered effect, stale epoch and checkpoint crash. Dependencies: T-013,T-015.

### T-017 — Enable isolated write routing
**Status:** TODO. **Priority:** P1.  
Bind provider assignments to existing worktree/workspace isolation. Workers may create local checkpoint commits but not push main. Test parallel independent writes and same-workspace conflict. Dependencies: T-016.

### T-018 — Add fenced Commit Coordinator
**Status:** TODO. **Priority:** P0.  
Single coordinator collects verified checkpoint commits, fetches main, integrates conservatively, runs required verification, reconciles plan and performs fast-forward-only push. Tests: competing coordinators, moved remote main, conflict, verification failure. Dependencies: T-017.

### T-019 — Enforce single-writer main policy
**Status:** TODO. **Priority:** P0.  
Remove/generated direct-worker-main instructions in multi-runtime mode; policy/static tests must prove ordinary workers lack autonomous main-push authority. Dependencies: T-018.

### T-020 — Add provider diagnostics and `agctl route explain`
**Status:** TODO. **Priority:** P1.  
Commands: provider list/capacity/sessions and route explanation; doctor shows staleness/health/circuit state. Distinguish hard rejection from soft scoring. Dependencies: T-012.

### T-021 — Add dashboard provider/quota view
**Status:** TODO. **Priority:** P2.  
Show health, windows/reset, reservations, predicted-vs-actual burn, context headroom, circuits and routing reasons; never secrets. Dependencies: T-004,T-005,T-012,T-020.

### T-022 — Add multidimensional concurrency/property/chaos tests
**Status:** TODO. **Priority:** P0.  
Space: provider x account x model x quota x reservation x worker lease x session context x failure timing x replay safety x plan revision x remote-main movement. Use boundary/pairwise/high-risk N-wise/fuzz/crash-restart tests. Dependencies: grows with T-005..T-019.

### T-023 — Add mutation testing for critical provider policy
**Status:** TODO. **Priority:** P1.  
Mutate quota bounds, reservation subtraction, stale-plan gate, replay classification, circuit transitions, settlement and commit fencing. Surviving mutants require stronger observable contracts. Dependencies: T-011..T-019.

### T-024 — Progressive rollout
**Status:** TODO. **Priority:** P1.  
`OBSERVE -> SHADOW -> ADVISORY -> AUTO_READ_ONLY -> AUTO_ISOLATED_WRITE -> AUTO_FAILOVER -> DEFAULT`. Promotion requires measured routing stability, reservation accuracy, zero duplicate effects/main races and acceptable provider error rate. Dependencies: T-014..T-023.

### T-025 — Delete superseded legacy paths
**Status:** TODO. **Priority:** P2.  
Remove obsolete direct-worker push rules, duplicate provider/model concepts, stale flags/adapters and tests after default rollout proves replacement. Dependencies: T-024.

### T-026 — Full re-audit and convergence pass
**Status:** TODO. **Priority:** P0.  
Repeat architecture/correctness/concurrency/security/persistence/API/testing/mutation/performance/CI audit; add delta findings before final convergence. Dependencies: T-025.

## 12. Testing Strategy

For every atomic task: characterization/contract -> targeted package -> store/integration -> concurrency/race when shared state changes -> property/fuzz for normalization/state machines -> full `go test ./...` -> `go test -race ./...` -> `go vet ./...` -> existing benchmarks. Provider live probes remain opt-in; normal CI uses recorded deterministic fixtures and no secrets.

## 13. Mutation Testing Strategy

Mandatory mutation targets once implemented: quota `<`/`<=`, omitted reservation subtraction, stale observation acceptance, plan-digest bypass, replay-safe branch inversion, `IN_DOUBT` bypass, circuit OPEN/HALF_OPEN changes, settlement idempotency and commit-fencing bypass. A surviving mutant becomes a finding if contract ambiguity caused it.

## 14. Performance Baselines

Do not materially regress current harness benchmarks. Add: latest-capacity read with 10k historical snapshots; aggregate 10k reservations; selection over 1k READY nodes x 8 candidates; 100 concurrent reservation attempts; route explanation without N+1 queries. Provider-selection overhead must remain bounded and small relative to network/model latency.

## 15. Security Hardening

Credentials stay provider-specific and never enter plan/telemetry. Adapters expose health/capabilities, not secrets. Remote worker trust does not imply credential access. Provider strings are external input and bounded/validated. Ingestion parsers cap payload size. Logs redact auth/session tokens. Direct autonomous `main` write belongs only to Commit Coordinator.

## 16. Migration Strategy

`characterize -> provider model -> persistence -> live observation -> shadow -> reservations -> TaskEnvelope -> auto read-only -> safe handoff -> isolated writes -> Commit Coordinator -> single-writer enforcement -> default -> delete legacy`.

Every intermediate state must leave `main` buildable and compatible.

## 17. Deferred Work

Until evidence justifies it: ML/LLM demand predictor, distributed credential vault, provider marketplace ABI, monetary arbitrage, reverse-engineering opaque quota buckets, force-push recovery automation.

## 18. Rejected Decisions

- **R-001 Provider == Worker:** rejected; topology and lifecycles differ.
- **R-002 `budget.Tracker` as quota manager:** rejected; allowed spend != external supply.
- **R-003 Context inside quota window:** rejected; context is session-scoped.
- **R-004 Hard-coded model/bucket IDs:** rejected; catalogs and opaque IDs drift.
- **R-005 Failover on any failure:** rejected; implementation/test failures are not provider failures.
- **R-006 Every worker pushes main:** rejected; causes semantic races.
- **R-007 Define unified AgentRuntime in T-002:** rejected after F-013; portable execution waits for TaskEnvelope.

## 19. Completed Tasks

- **T-001:** DONE on `main`, commit `bf060a70833504fc5dc979181d59453276a56e0c`; full Harness CI passed.
- **T-002:** DONE in this verified iteration; observation adapter/registry contracts and concurrency tests passed full Linux/Windows Harness CI.

## 20. Iteration Log

### Iteration 1
**Task:** T-001  
**Findings:** F-001/F-005 foundation; unexpected F-006 and F-012.  
**Changes:** provider primitives, IDs, validation tests, initial plan.  
**Verification:** Linux/Windows Harness CI PASS including tests, race, vet and benchmarks.  
**Commit:** `bf060a70833504fc5dc979181d59453276a56e0c`.  
**Push:** main, fast-forward, no force.  
**Result:** PASS.

### Iteration 2
**Task:** T-002  
**Findings:** F-002 boundary foundation; unexpected F-013.  
**Changes:** observation-only `CapacitySource`/`ModelSource`/`SessionSource`, composite account `Adapter`, concurrency-safe deterministic live `Registry`, contract/race tests.  
**Plan changes:** unified execution contract deferred to T-013/T-014 because TaskEnvelope is required to avoid coupling AGY/process and `ModelClient` semantics.  
**Verification:** Linux/Windows Harness CI PASS: module tidy, bridge contracts, unit/integration, race, vet and configured smoke benchmarks.  
**Commit/Push:** this verified iteration is to be published as one fast-forward commit on `main`, no force.  
**Result:** PASS.

## 21. Definition of Final Done

Convergence requires: no open Critical/High routing findings; Antigravity and Codex first-class behind common observation/portable execution contracts; no guessed quota semantics; reservations proven non-oversubscribing under races; TaskEnvelope makes handoff independent of full chat history; stale plan cannot start affected writes; unsafe interruption reaches existing `IN_DOUBT`; read-only and isolated-write rollout gates pass; only fenced Commit Coordinator may autonomously push main; no force push path in normal mode; CI/race/vet/mutation/benchmarks pass; routing is explainable; legacy duplicates are deleted; final re-audit finds no new fundamental Critical/High issue; final verified tree and synchronized `MASTER_PLAN.md` are on `main`.
