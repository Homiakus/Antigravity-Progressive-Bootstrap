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

Provider-control-plane foundation completed/verified through T-009, with T-005 v15 runtime persistence now verified for publication:

- T-001 provider-domain primitives: `bf060a70833504fc5dc979181d59453276a56e0c`.
- T-002 observation adapter/registry: `ac35b5193c735897a75cb056680215a4e3aae428`.
- T-003 SQLite v14 provider-observation schema: `c5c2ebfab1814207aecb274ffb14b5ae83d9992f`.
- T-004 durable provider observation Store/SQLite implementation: `7f6c3a888318e1cca1ea2d22c8c997051f883856`.
- T-005 SQLite v15 provider runtime ledger: implementation `be8a786de320b6e017a8da7eb8d74e5f3a119695`; reconciled against current R18 main on `7fa9578795a7cf5029e80f618c9b87c3524063a2`; publication pending.
- T-006 Antigravity observation adapter: `a46eb75f947f97f62a06129e3469fe18466d6843`.
- T-007 Codex App Server observation adapter: `cefead407f3ad4a11ac46379f2f9b72a9129d353`.
- T-009 provider-neutral capacity normalization/effective headroom: `47c08fc118513d8b9ddb7cb5fdd6523dd32fd52b`.
- T-027 executable living-plan coordinator completion: published to `main` as `e154def6a181be2f6120aafa4fb1358fba63ac22`.
- T-028 role-aware headless worker contract: implemented and qualified; publication is the current iteration action.

Next process dependency is T-029 machine-verifiable publication/checkpoint evidence. Provider dependency-chain work remains T-010 conservative demand estimator; T-008 session/context broker remains READY and is required before T-012.

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
      assignment + reservation ledger
                   |
          capacity reservation
                   |
         worker + fenced lease
                   |
         isolated workspace
                   |
                execute
                   |
        idempotent usage/effects
                   |
    verify -> review -> reconcile
                   |
           Commit Coordinator
                   |
          fast-forward-only main
```

Orthogonal concepts MUST stay separate: `Worker`, `ProviderAccount`, `ProviderModel`, `ProviderSession`, workflow `budget`, provider `capacity`, execution lease, provider reservation, workspace lease and commit authority.

## 4. Baseline

Harness CI enforces module tidiness, bridge syntax/contracts, targeted engineering/loop/hooks/doctor gates, `go test ./...`, `go test -race ./...`, `go vet ./...`, compiler/store/scheduler/retry/wait smoke benchmarks and Windows tests.

Persistence is SQLite schema v15. Migrations 1..14 are immutable checksummed releases; v15 is the append-only provider runtime ledger and does not modify released v14 provider-observation SQL.

T-005 verification:

- implementation head `be8a786de320b6e017a8da7eb8d74e5f3a119695`: full Linux/Windows Harness CI PASS;
- `main` moved during validation to R18 commit `265d947eda0470dd56f775b523277a932276646d`; changed files were remote daemon/session/command only and did not overlap T-005;
- T-005 blobs were overlaid on the current main tree rather than merging the obsolete branch history;
- reconciled head `7fa9578795a7cf5029e80f618c9b87c3524063a2`: full Linux/Windows Harness CI PASS;
- publication remains fast-forward-only and requires a final current-main recheck after plan reconciliation.

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
- **I-023 Durable routing history:** provider handoff appends a new assignment only after the previous assignment becomes terminal/superseded; historical account/model decisions are never rewritten. At most one assignment per Attempt is ACTIVE at a time.
- **I-024 Reservation lifecycle fencing:** reservations begin ACTIVE at revision 1 and reach terminal states only by CAS. An assignment cannot become terminal while any reservation row attached to it remains ACTIVE.
- **I-025 Complete reservation accounting:** capacity feasibility must use the complete untruncated set of non-expired ACTIVE reservations (or an equivalent SQL aggregate). Paged diagnostic reads are never correctness inputs.
- **I-026 Usage replay idempotency:** one provider usage event key produces one immutable semantic sample; equivalent redelivery is a no-op, while the same key with changed semantic data fails closed.
- **I-027 Runtime quantity bounds:** OPAQUE capacity is observable but never reservable/measurable arithmetically; FRACTION reservation/usage values stay within their defined fractional range.

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
**Status:** Partially resolved in T-005. **Severity:** Critical. **Confidence:** Confirmed.  
T-005 adds durable typed assignments/reservations, revision fencing, expiry metadata and a complete ACTIVE-claim read. The critical oversubscription risk is not closed until T-011 atomically performs capacity read + complete active-reservation accounting + feasibility + assignment/reservation commit in one write transaction.

### F-005 — Existing worker/lease subsystem must be reused
**Status:** Resolved by architecture. **Severity:** High.  
Provider assignment remains orthogonal to workers and fenced attempt leases.

### F-006 — Existing `IN_DOUBT` + effect intents already provide unsafe-replay protection
**Status:** Resolved by architecture. **Severity:** Critical.  
Safe provider handoff must reuse `ReclaimExpiredAttempt`/effect reconciliation. Affects T-015,T-016.

### F-007 — Direct worker pushes are unsafe with parallel providers
**Status:** Planned. **Severity:** Critical. **Confidence:** Strong.  
T-028 removes publication authority from the declared worker contract; hard enforcement remains T-018/T-019 and is not claimed complete by prompt/metadata isolation alone.

### F-008 — No TaskEnvelope independent of conversation history
**Status:** Planned. **Severity:** High. Affects T-013,T-014.

### F-009 — No plan digest on provider assignments
**Status:** Planned. **Severity:** High. Affects T-013,T-014,T-016.

### F-010 — No session/context broker
**Status:** Planned. **Severity:** Medium. Affects T-008,T-013.

### F-011 — Provider routing decisions are not durable/explainable
**Status:** Partially resolved in T-005. **Severity:** Medium.  
Historical assignment/account/model/session bindings are now durable and handoff-safe. Selector scoring/rationale and human-readable route explanation remain T-012/T-020/T-021.

### F-012 — Released SQLite migrations are immutable
**Status:** Resolved. **Severity:** High.  
T-003 appended v14 without modifying v1..v13; T-005 appends v15 without modifying v1..v14.

### F-013 — Unified provider execution interface is premature before TaskEnvelope
**Status:** Planned. **Severity:** Medium.  
Portable execution remains deferred to T-013/T-014.

### F-014 — Concurrent autonomous work can move `main` during an iteration
**Status:** Planned. **Severity:** High. **Confidence:** Confirmed again in T-005.  
T-005 observed `main` move to R18 commit `265d947eda0470dd56f775b523277a932276646d`. The validation tree was not published directly; its scoped blobs were overlaid on current main and fully revalidated. Every publication must re-read `main`, reconcile semantics and fast-forward only. Affects T-018/T-019/T-029.

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

### F-023 — Paged ACTIVE-reservation reads can undercount capacity claims
**Status:** Resolved in T-005. **Severity:** Critical. **Confidence:** Confirmed.  
A caller-supplied diagnostic limit can truncate claims and make later feasibility arithmetic unsafe. T-005 therefore provides `ListAllActiveProviderReservations` as the unbounded correctness read; T-011 must use it or an equivalent complete SQL aggregate inside its atomic transaction.

### F-024 — Mutating one assignment record would destroy provider-handoff history
**Status:** Resolved in T-005. **Severity:** High. **Confidence:** Confirmed.  
T-005 stores assignment history as multiple immutable-identity records with a partial unique constraint allowing only one ACTIVE assignment per Attempt. A superseded assignment remains durable before a replacement becomes ACTIVE.

### F-025 — Provider usage redelivery can double-count settlement
**Status:** Resolved in T-005. **Severity:** High. **Confidence:** Confirmed.  
Usage samples are keyed by provider/event idempotency key. Concurrent equivalent redelivery creates exactly one row and returns the same semantic record; reusing a key with changed assignment/reservation/account/model/metric/amount/observation time fails with conflict.

### F-026 — Runtime FRACTION quantities need explicit bounds
**Status:** Resolved in T-005. **Severity:** High. **Confidence:** Confirmed.  
Reservation amounts are finite and positive, usage amounts finite and non-negative, and FRACTION is bounded to its fractional range in both domain validation and v15 SQL CHECK constraints. OPAQUE is deliberately excluded from reservation/usage arithmetic.

## 7. Risk Register

| Risk | Level | Mitigation |
|---|---|---|
| quota oversubscription | Critical | T-005 durable claims + T-011 atomic capacity/claim transaction |
| truncated active-claim accounting | Critical | I-025; unbounded correctness read / complete SQL aggregate |
| duplicate side effects | Critical | effect intents + `IN_DOUBT` |
| duplicate usage settlement | High | semantic idempotency key + conflicting replay rejection |
| simultaneous main writes | Critical | typed worker denial in T-028 + fenced Commit Coordinator T-018 + hard policy T-019 |
| moved remote main | High | pre-push ref recheck + scoped reconcile + fast-forward only; machine proof T-029 |
| stale reservation/assignment mutation | High | revision CAS + terminal transition rules |
| provider handoff history loss | High | append/supersede assignment history; one ACTIVE per Attempt |
| stale observations | High | monotonic timestamps + freshness/expiry policy |
| pre-reset capacity reused after reset | High | reset-aware expiry (I-022) |
| torn provider observation | High | coherent `SnapshotSource`/single upstream payload |
| sparse delta erases known state | High | merge only available Codex values; baseline on reconnect |
| partial model catalog | High | replace only after complete pagination cycle |
| false thread->model affinity | High | I-018; no inferred ProviderSessionSnapshot |
| incompatible quota units combined | High | I-020/I-027; separate metric/window headroom |
| decay-to-zero mistaken for exhaustion | High | I-021; authoritative exhaustion proof only |
| malformed floating telemetry | High | finite/consistency checks at normalization/runtime boundaries |
| stale plan execution | High | plan revision/digest gate |
| provider API drift | High | tolerant adapters + deterministic fixtures + dynamic discovery |
| opaque quota semantics | High | native IDs/FRACTION/OPAQUE; OPAQUE is never reserved |
| session context exhaustion | High | broker + checkpoint/new-session policy |
| provider flapping | Medium | hysteresis + switch penalty + provider circuit CAS |
| poor demand estimate | Medium | conservative p80 + settlement feedback |

## 8. Pareto Improvements

Remaining high-leverage provider path: conservative estimator -> atomic reservation service -> shadow selector -> TaskEnvelope/plan digest -> safe read-only routing -> provider-aware failover -> isolated writes -> Commit Coordinator. Remaining high-leverage process path: machine-verifiable publication/checkpoint proof -> structural plan audit/process metrics -> hard single-main-writer enforcement with T-018/T-019.

## 9. Dependency DAG

```text
T001 DONE -> T002 DONE -> T003 DONE -> T004 DONE
                                      |-> T006 Antigravity observation DONE
                                      |-> T007 Codex observation DONE
                                      |-> T005 v15 assignment/reservation persistence DONE
T004,T006,T007 -> T008 session broker READY
T004,T006,T007 -> T009 capacity normalization DONE
T005,T009 -> T010 estimator READY -> T011 reservations -> T012 shadow selector
T012 -> T013 TaskEnvelope + digest -> T014 auto read-only -> T015 provider failures
T013,T015 -> T016 safe handoff -> T017 isolated writes -> T018 Commit Coordinator -> T019 single-main-writer enforcement
T012 -> T020 explain route
T004,T005,T012,T020 -> T021 dashboard
T005..T019 -> T022 adversarial/property/chaos
T011..T019 -> T023 mutation
T014..T023 -> T024 rollout -> T025 legacy deletion -> T026 final re-audit
T027 DONE -> T028 DONE -> T029 publication/checkpoint proof
                      |-> T030 structural plan/process audit
```

## 10. Implementation Phases

- **A Domain/persistence:** T-001..T-005 complete after T-005 publication.
- **B Observe/normalize:** T-006..T-009 complete except independent T-008 session broker.
- **C Predict/shadow:** T-010..T-012.
- **D Portable execution:** T-013..T-016.
- **E Multi-provider writes:** T-017..T-019.
- **F Proof/operations:** T-020..T-026.
- **G Engineering process:** T-027/T-028 complete; T-029/T-030 remain.

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
**Status:** DONE / VERIFIED. **Priority:** P0. **Leverage:** HIGH.  
Schema: append-only v15 `provider_runtime_ledger` adds `provider_assignments`, `provider_reservations`, `provider_usage_samples`, `provider_circuit_state`; released v1..v14 remain immutable. Assignment/account and reservation/assignment composite FKs preserve durable identity without requiring transient provider-model catalog ordering. Partial unique indexes enforce one ACTIVE assignment per Attempt and one ACTIVE reservation per assignment/window/model/metric dimension.  
Assignments: historical account/model decisions are immutable in identity, optional session binding is one-way, state/revision changes use CAS, terminal assignments cannot strand ACTIVE reservations, and safe handoff can supersede one assignment then append another without erasing history.  
Reservations: native TOKENS/REQUESTS/COST/FRACTION amounts only; OPAQUE is not reservable; FRACTION is bounded; ACTIVE starts at revision 1 and transitions to SETTLED/RELEASED/EXPIRED through CAS; `expires_at_ns` supports deterministic due scans. Diagnostic active reads may be paged, while `ListAllActiveProviderReservations` is deliberately unbounded for correctness-sensitive feasibility.  
Usage: immutable provider/event-keyed samples with assignment/account/model/metric and optional reservation linkage; equivalent sequential or concurrent redelivery is idempotent even if local ingestion `CreatedAt` differs; changed semantic payload under the same key conflicts.  
Circuit: account/model-scoped CLOSED/OPEN/HALF_OPEN state with monotonic revision CAS, separate from the existing generic service circuit breaker.  
Concurrency authority: the existing SQLite single-writer pool serializes in-process writers while SQL FK/CHECK/partial-UNIQUE/CAS/UPSERT invariants provide correctness; T-005 does not introduce a parallel lock subsystem.  
Tests: v14->v15 migration, table/index existence, FK/CHECK and FRACTION bounds, one-ACTIVE constraints, Store round-trip, stale CAS, assignment/reservation lifecycle, terminalization guard, handoff history, complete-vs-paged active set, provider circuit fencing, 32-way concurrent assignment claim, 32-way reservation CAS and 32-way concurrent usage replay exactly-once semantics.  
Verification: implementation head `be8a786de320b6e017a8da7eb8d74e5f3a119695` full Linux/Windows Harness CI PASS; reconciled head `7fa9578795a7cf5029e80f618c9b87c3524063a2` full Linux/Windows Harness CI PASS after overlay on current R18 main. Dependencies: T-004.

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
**Status:** DONE. **Priority:** P0. **Leverage:** HIGH.  
Files: `internal/harness/provider/capacity/{normalize.go,normalize_test.go,absolute_test.go,adapter_integration_test.go}`.  
Semantics: preserves TOKENS/REQUESTS/COST/FRACTION/OPAQUE and native ModelID/window IDs; never sums unlike or potentially overlapping windows. Fractional lower bound is the minimum of independently justified effective fractions. Absolute TOKENS/REQUESTS/COST `remaining` without a denominator remains `EffectiveRemaining` in native units and never becomes an invented fraction.  
Uncertainty: configurable `FreshFor/ExpireAfter/MaxFutureSkew`; linear freshness decay multiplies source confidence. Windows observed before `resetAt` expire once the reset passes. Snapshot and per-window age are tracked independently.  
Evidence: UNKNOWN/PARTIAL/QUANTIFIED/STALE/EXHAUSTED/UNAVAILABLE is separate from `ProviderHealth`; an effective zero caused by low confidence/staleness is not an exhaustion proof. Only source provider evidence carries `ProvenExhausted`.  
Validation: rejects NaN/Inf, remaining above limit, invalid zero-limit states, contradictory explicit fraction vs remaining/limit and duplicate native window IDs; deterministic ordering; no model/bucket inference.  
Tests: metric separation, conservative bottleneck, absolute remaining without limit, confidence/freshness decay, reset crossing, zero-vs-exhaustion separation, unavailable/exhausted source states, OPAQUE behavior, contradictory/non-finite telemetry, duplicate IDs, deterministic ordering/model attribution, mixed known/unknown evidence, clock skew, earliest reset, policy validation, Antigravity/Codex integration, fuzz fraction bounds and 100-window benchmark.  
Verification: full Linux/Windows Harness CI PASS on implementation and reconciled validation heads.  
Commit: `47c08fc118513d8b9ddb7cb5fdd6523dd32fd52b`.  
Push: `main`, fast-forward, no force. Dependencies: T-004,T-006,T-007.

### T-010 — Add conservative demand estimator
**Status:** READY. **Priority:** P1.  
Robust historical p50/p80 by task/provider/model/repository/context class; p80 reservation, no ML initially. Preserve demand units so native absolute headroom is compared only with compatible estimates. Usage-sample idempotency from T-005 is the durable feedback foundation. Dependencies: T-005,T-009.

### T-011 — Implement atomic provider reservations
**Status:** TODO. **Priority:** P0.  
Single write transaction: latest capacity + complete non-expired ACTIVE reservations -> normalization/compatible-unit feasibility -> assignment + reservation. Expiry/release/settlement and 100+ concurrent reserver proof. Must use I-025 complete accounting, respect I-020/I-027 units and preserve I-021 exhaustion-proof separation. Dependencies: T-005,T-009,T-010.

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
Hard static/policy/runtime tests prove ordinary workers lack autonomous main-push authority even if a prompt is ignored. Dependencies: T-018. T-028 is the semantic/typed precursor, not a substitute.

### T-020 — Add provider diagnostics and `agctl route explain`
**Status:** TODO. **Priority:** P1. Dependencies: T-012.

### T-021 — Add provider/quota dashboard
**Status:** TODO. **Priority:** P2. Dependencies: T-004,T-005,T-012,T-020.

### T-022 — Add multidimensional concurrency/property/chaos tests
**Status:** TODO. **Priority:** P0.  
Provider x account x model x quota metric/window x freshness/reset x reservation x worker lease x session context x failure timing x replay safety x plan revision x remote-main movement.

### T-023 — Add mutation testing for critical provider policy
**Status:** TODO. **Priority:** P1.  
Mutate quota boundaries, unit compatibility, freshness/reset gates, complete active-claim accounting, reservation subtraction, stale-plan gate, replay branch, circuit transitions, settlement idempotency and commit fencing.

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

Provider runtime persistence additionally requires immutable migration checksums, FK/CHECK/partial-UNIQUE proof, revision-CAS contention, one-ACTIVE assignment/reservation constraints, handoff-history preservation, explicit reservation terminalization, complete active-claim reads and sequential/concurrent idempotent usage redelivery.

Engineering process changes additionally require negative authority/omission tests. Coordinator and worker authority must be represented as typed data and mirrored in prompts; prompt text alone is not sufficient evidence of hard enforcement.

## 13. Mutation Testing Strategy

Mandatory once executable routing policy exists: quota `<`/`<=`, metric-unit compatibility, freshness/reset gates, complete-claim accounting, reservation subtraction, stale-plan bypass, replay branch inversion, `IN_DOUBT` bypass, circuit transitions, settlement idempotency and commit-fencing. Mutation becomes mandatory starting with T-011 policy logic.

For process policy, semantic mutation sentinels are also required: deleting a mandatory completion evidence category must fail T-027 tests; granting any coordinator-only authority bit to a worker must fail T-028 tests.

## 14. Performance Baselines

Do not materially regress current harness benchmarks. T-009 includes a 100-window allocation benchmark for the normalizer. Add later: latest capacity with 10k snapshots; complete/aggregate active claims with 10k reservations; selector over 1k READY x 8 candidates; 100 concurrent reservations; route explanation without N+1 queries. T-028 changes prompt construction only at task materialization and does not add scheduler/runtime hot-path work.

## 15. Security Hardening

Credentials never enter plan/telemetry. Adapters expose normalized health/capabilities, not secrets. Provider input is validated/bounded. Derived routing arithmetic rejects non-finite or internally contradictory telemetry. Runtime reservation/usage quantities reject non-finite values and OPAQUE arithmetic; fractional values are bounded. Ingestion payloads are size-capped. Status-line email/transcript/path data is discarded or one-way fingerprinted where affinity is required. JSON-RPC error/auth payloads are classified but not copied into durable provider state. Logs redact auth/session tokens. Remote-worker trust does not imply credential access. Only Commit Coordinator may autonomously write `main` in final architecture. T-028 declares and tests this authority split; T-019 must still enforce it independently of agent compliance.

## 16. Migration Strategy

`provider domain -> observation persistence -> Antigravity/Codex live adapters -> normalization -> reservation persistence -> estimator -> shadow -> TaskEnvelope -> read-only -> safe handoff -> isolated writes -> Commit Coordinator -> single-writer enforcement -> default -> legacy deletion`.

Engineering-process migration: `coordinator completion contract -> typed coordinator/worker authority -> worker propagation -> machine-verifiable publication/checkpoint proof -> structural plan audit -> hard main-write enforcement`.

Every intermediate state must keep `main` buildable and compatible. Released SQLite migrations remain immutable; new runtime persistence is additive v15.

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
- **R-018 use a paged reservation list for feasibility:** rejected after F-023; capacity arithmetic requires the complete active set or equivalent aggregate.
- **R-019 rewrite one assignment row during provider handoff:** rejected after F-024; supersede/terminalize then append a new historical assignment.
- **R-020 reserve OPAQUE or accept FRACTION > 1:** rejected after F-026; only compatible measurable native units participate in arithmetic.
- **R-021 duplicate usage rows on event redelivery:** rejected after F-025; usage persistence is semantic-key idempotent under races.
- **R-022 rely on a driver-specific SQLite `_txlock` option for correctness:** rejected; the durable harness uses a single writer connection plus SQL CAS/constraints and WAL readers.
- **R-023 copy coordinator process text into ordinary workers:** rejected after F-030; roles have different authority and require separate typed contracts.
- **R-024 treat worker prompt denial as hard git sandbox:** rejected; T-028 is semantic/typed isolation while T-018/T-019 provide publication enforcement.

## 19. Completed Tasks

- T-001 DONE — `bf060a70833504fc5dc979181d59453276a56e0c`.
- T-002 DONE — `ac35b5193c735897a75cb056680215a4e3aae428`.
- T-003 DONE — `c5c2ebfab1814207aecb274ffb14b5ae83d9992f`.
- T-004 DONE — `7f6c3a888318e1cca1ea2d22c8c997051f883856`.
- T-005 DONE/VERIFIED — implementation `be8a786de320b6e017a8da7eb8d74e5f3a119695`; reconciled `7fa9578795a7cf5029e80f618c9b87c3524063a2`; publication commit recorded after fast-forward.
- T-006 DONE — `a46eb75f947f97f62a06129e3469fe18466d6843`.
- T-007 DONE — `cefead407f3ad4a11ac46379f2f9b72a9129d353`.
- T-009 DONE — `47c08fc118513d8b9ddb7cb5fdd6523dd32fd52b`.
- T-027 DONE — `e154def6a181be2f6120aafa4fb1358fba63ac22`.
- T-028 DONE/VERIFIED — qualification head `720f626355adb40137d116e07aef5360bb5eebf2`; publication commit is the `main` HEAD produced by Iteration 10.

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
**Verification:** full Linux/Windows Harness CI PASS on implementation head `b66d68c18edb3e138dc1acf988d7315fb252871f` and reconciled head `3e24d94f822fe5b8d73c8f6a61282bb65334e8f3`.  
**Plan changes:** I-020/I-021/I-022; F-020/F-021/F-022 resolved; T-009 DONE; T-005 NEXT; T-010 contract tightened to preserve demand units.  
**Commit:** `47c08fc118513d8b9ddb7cb5fdd6523dd32fd52b`.  
**Push:** `main`, fast-forward, no force.  
**Result:** PASS.

### Iteration 8 — T-005
**Findings addressed:** F-004 durable reservation foundation; F-011 durable routing history; F-023 complete active-claim accounting; F-024 handoff history; F-025 usage replay; F-026 runtime quantity bounds.  
**Unexpected findings:** a paged ACTIVE-reservation API is unsafe as a correctness input; provider handoff requires append/supersede history rather than mutation; concurrent usage redelivery needs semantic idempotency; `main` moved to R18 while T-005 was validating.  
**Changes:** SQLite v15 runtime ledger; provider assignment/reservation/usage/circuit domain models; FK/CHECK/partial-UNIQUE schema; revision CAS; terminalization guard; unbounded correctness active-reservation read; immutable/idempotent usage samples; provider-scoped circuit fencing.  
**Tests:** v14->v15 migration, FK/CHECK/FRACTION bounds, assignment and reservation uniqueness, lifecycle/CAS, history handoff, complete claim reads, circuit state, 32-way assignment claims, 32-way reservation CAS, sequential and 32-way concurrent usage replay.  
**Verification:** full Linux/Windows Harness CI PASS on implementation head `be8a786de320b6e017a8da7eb8d74e5f3a119695`; stale validation history was not merged after `main` moved; scoped T-005 blobs were overlaid on R18 main and reconciled head `7fa9578795a7cf5029e80f618c9b87c3524063a2` also passed full Linux/Windows Harness CI.  
**Plan changes:** I-023..I-027; F-004/F-011 partially resolved; F-023..F-026 resolved; T-005 DONE/VERIFIED; T-010 READY; T-011 explicitly requires complete active-claim accounting.  
**Commit/Push:** published to `main` through the normal fast-forward-only sequence; no force.  
**Result:** PASS.

## 21. Definition of Final Done

No open Critical/High routing findings; Antigravity and Codex are first-class observation/execution providers; no guessed quota semantics, denominator, unit conversion or session/model attribution; coherent provider observations cannot tear across revisions; stale/pre-reset capacity cannot masquerade as current supply; durable routing history survives provider handoff; usage redelivery is idempotent; reservation arithmetic accounts for the complete active claim set; reservations cannot oversubscribe under races; TaskEnvelope makes handoff conversation-independent; stale plan cannot start affected writes; unsafe interruption reaches existing `IN_DOUBT`; read-only and isolated-write rollout gates pass; only fenced Commit Coordinator writes main autonomously; no force push; CI/race/vet/mutation/benchmarks pass; routing is explainable; legacy duplicates removed; final re-audit finds no fundamental Critical/High issue; final verified tree and synchronized plan are on `main`.

## 22. Living Engineering Process Integration

This section extends the same execution source of truth with the self-improving engineering control loop. It is not a parallel roadmap. Product/provider tasks and process tasks are prioritized together by blocker severity, dependency leverage, architectural leverage, risk reduction and unlock value.

### Process invariants

- **I-028 Executable living-plan completion:** when a task workspace resolves to `MASTER_PLAN.md`, autonomous completion is accepted only with categorized evidence for task identity, pre-flight, characterization, edge-space, tests, mutation/test-of-tests, race, static analysis, security, compatibility, performance, findings, adversarial self-review, plan reconciliation, process review, verified normal main publication and context checkpoint. The selected `T-XXX` must exist and be `DONE`; referenced `F-XXX` identifiers must exist; bare `n/a`/`none`/`skipped` cannot satisfy a gate.
- **I-029 Workspace-bound process authority:** engineering policy is resolved from the task's persisted workspace identity, never from ambient process cwd. Multiple workspaces below one living plan may share one task; workspaces resolving to multiple independent living plans fail closed and must be split.
- **I-030 Plan revision binding:** successful managed completion stores a SHA-256 digest of the reviewed `MASTER_PLAN.md` in verification evidence so a checkpoint cannot silently refer to another plan revision.
- **I-031 Coordinator/worker authority separation:** coordinator process instructions may reconcile/publish `main`; ordinary headless workers may characterize/implement/verify and report findings/replan proposals but must not receive autonomous task-selection, plan-mutation, commit, main-publication or continuation-checkpoint authority. This specializes, and never weakens, I-008/I-009.
- **I-032 Typed worker authority:** headless DAG work is materialized with an explicit `engineering.RoleWorker` contract and machine-readable `engineering-role:worker` tag. Unknown roles fail closed. Granting a coordinator-only authority bit to a worker must make the role contract invalid. Prompt text mirrors this typed boundary but does not replace T-019 hard enforcement.

### Process findings

### F-027 — Autonomous completion was prompt-only rather than executable
**Status:** Resolved in T-027. **Category:** Engineering Process / Correctness. **Severity:** High. **Confidence:** Confirmed.  
**Evidence:** prior `loop.MarkComplete` accepted any non-empty verification list, including a single arbitrary string.  
**Observed behavior:** an agent could satisfy Stop/Completion after a partial iteration without a living-plan Task ID, characterization, adversarial review, mutation/race/security decision, reconciliation, main publication evidence or checkpoint.  
**Root cause:** process semantics existed in instructions but had no typed/executable completion boundary.  
**Impact/blast radius:** every autonomous interactive engineering iteration could become false-green.  
**Resolution:** `internal/engineering` validates categorized completion evidence, declared DONE task/finding linkage and plan digest; `CompletionInjection` embeds the mandatory living-plan process; semantic omission tests prove every required category is observable.

### F-028 — Completion policy inferred repository identity from ambient cwd
**Status:** Resolved in T-027. **Category:** Engineering Process / Boundary. **Severity:** High. **Confidence:** Confirmed by CI regression.  
**Evidence:** first T-027 CI exposed synthetic task workspaces being governed by the agctl repository's `MASTER_PLAN.md` only because the test process cwd was the repository root.  
**Observed behavior:** unrelated workspaces could inherit or bypass a living-plan policy based on launcher cwd.  
**Expected behavior:** task workspace is the authority.  
**Root cause:** completion resolution used `os.Getwd()` instead of persisted hook workspace identity.  
**Resolution:** `TaskState` persists deduplicated `WorkspacePaths`; legacy active state can backfill them once; explicit unmanaged workspace does not inherit fallback cwd; multiple independent plans fail closed; hook lifecycle tests preserve workspace identity end to end.

### F-029 — Monolithic full-suite gate delayed process failure localization
**Status:** Resolved in T-027. **Category:** Meta Engineering / CI. **Severity:** Medium. **Confidence:** Confirmed.  
**Evidence:** initial failures appeared only as `go test ./...` red after earlier unrelated gates had passed; package identity required log extraction and manual diagnosis.  
**Root cause:** no cheap process-layer package gates preceded the full suite.  
**Resolution:** Linux/Windows CI runs `internal/engineering`, `internal/loop`, `internal/hooks`, and `internal/doctor` contract gates before the full suite, while retaining the authoritative full/race/vet/benchmark/Windows gates.

### F-030 — One process prompt cannot safely govern both coordinator and ordinary workers
**Status:** Resolved in T-028 at semantic/typed boundary. **Category:** Architecture / Authority. **Severity:** Critical. **Confidence:** Confirmed.  
**Evidence:** coordinator contract requires reconciliation and verified publication while ordinary workers are forbidden from writing `main`; previous `planner.Enqueue` stored only node objective/verification and had no explicit process role.  
**Root cause:** engineering authority was implicit in execution context rather than represented as a role contract at DAG materialization.  
**Resolution:** `internal/engineering/roles.go` defines `Role`, `Authority`, fail-closed role validation and separate coordinator/worker contracts. `planner.Enqueue` wraps every node with `RoleWorker`, preserves Plan/Node identity and adds `engineering-role:worker`. Architecture tests reject any coordinator-only authority bit on workers and prove coordinator instructions do not leak into worker prompts.  
**Residual risk:** this resolves authority declaration/propagation, not hard command sandboxing; F-007 remains open until T-018/T-019.

### Process tasks

### T-027 — Enforce living-plan autonomous completion contract
**Status:** DONE. **Priority:** P0. **Leverage:** HIGH. Commit `e154def6a181be2f6120aafa4fb1358fba63ac22`.  
**Root cause:** process requirements were advisory text while the runtime completion gate accepted one arbitrary verification string.  
**Changes:** `internal/engineering` contract/plan resolver; coordinator completion injection; categorized evidence gate; Task/Finding validation; reasoned not-applicable semantics; SHA-256 plan binding; workspace identity persistence; fail-closed multi-plan resolution; targeted CI feedback ladder.  
**Verification:** Linux/Windows targeted and full tests, race, vet and benchmark smoke PASS before fast-forward publication to `main`.

### T-028 — Introduce role-aware engineering contracts for headless execution
**Status:** DONE. **Priority:** P0. **Leverage:** HIGH.  
**Root cause:** headless planner tasks had no typed authority boundary; copying the coordinator process would conflict with I-008 and reintroduce F-007.  
**Pre-flight:** change only `internal/engineering`, `internal/planner`, tests and plan; protect scheduler, task runtime, provider routing, SQLite, permission subsystem and Commit Coordinator. Rollback removes worker wrapping/tag and `roles.go` without persistent-state migration.  
**Changes:** typed `RoleCoordinator`/`RoleWorker`; `Authority` capability set; fail-closed `ContractForRole`/`ValidateRoleContract`; bounded worker engineering process; `WrapWorkerTask`; every `planner.Enqueue` node now carries worker contract, Plan ID, Node ID and `engineering-role:worker` tag while retaining the existing adaptive-replan path.  
**Characterization/regression:** before T-028 `Enqueue` persisted only node objective/verification and `tasks.executionPrompt` applied one generic completion prefix. Tests now prove every node receives worker authority and no coordinator `select T-XXX`/`PUSH MAIN` instructions leak. DAG dependencies and required verification are preserved.  
**Edge-space:** coordinator/worker/unknown role; read-only/write/review/security nodes; worker authority bit combinations; DAG dependency preservation; plan/node identity; replan authority; Linux/Windows execution materialization.  
**Mutation/test-of-tests:** authority mutation sentinel independently flips `SelectPlanTask`, `MutateLivingPlan`, `CommitRepository`, `PublishMain`, and `ContinuationCheckpoint`; every mutant must be rejected.  
**Verification:** qualification head `720f626355adb40137d116e07aef5360bb5eebf2` — engineering/loop/hooks/doctor targeted gates PASS; `go test ./...` PASS; `go test -race ./...` PASS; `go vet ./...` PASS; compiler/store/scheduler/retry/wait smoke benchmarks PASS; Windows targeted gates and full tests PASS.  
**Security:** unknown role fails closed and workers receive no declared publication authority. Hard enforcement independent of agent compliance remains intentionally assigned to T-018/T-019.  
**Compatibility/performance:** TaskRecord schema and scheduler behavior are unchanged; wrapping occurs once at task materialization; existing DAG dependencies, verification and adaptive replan paths remain intact; smoke benchmarks green. Dependencies: T-027. Addresses F-030 and reinforces F-007/I-008/I-031.

### T-029 — Make publication/checkpoint evidence machine-verifiable
**Status:** READY. **Priority:** P0. **Leverage:** HIGH.  
Replace free-form `push-main`/`checkpoint` assertions with repository-aware evidence: expected branch, local/qualified HEAD, observed remote-main HEAD, fast-forward relation, plan digest and durable continuation checkpoint. Fail closed on stale/moved main and preserve no-force policy. Must complement T-018/T-019 rather than create a second Commit Coordinator. Dependencies: T-027,T-028.

### T-030 — Add structural `MASTER_PLAN.md` audit and process metrics
**Status:** TODO. **Priority:** P1.  
Executable audit for unique T/F IDs, allowed statuses, at most one coordinator `IN_PROGRESS` task per repository, declared dependencies, no unknown high findings, checkpoint recoverability and machine-readable engineering-loop metrics (failure classification, time-to-detect, flaky/mutation signals). Do not optimize metrics as targets. Dependencies: T-027,T-028.

### Process dependency delta

```text
T027 DONE -> T028 DONE -> T029 publication/checkpoint proof
                    |-> T030 structural plan/process audit
T028 reinforces T018/T019 single-main-writer work and does not create a second commit coordinator.
```

### Iteration 9 — T-027
**Selected task:** T-027.  
**Why now:** the requested self-improving workflow could not be trusted while completion was advisory and false-green-able.  
**Findings addressed:** F-027, F-028, F-029.  
**Unexpected findings:** ambient cwd was an unsafe repository authority; coordinator process text cannot be reused unchanged by workers because of I-008.  
**Changes:** executable living-plan completion contract, workspace-bound task identity, plan digest binding, mutation omission sentinel, process/hook/doctor feedback ladder.  
**Tests:** targeted engineering/loop/hooks/doctor; full unit/integration; race; vet; existing benchmark smoke; Windows targeted/full.  
**Mutation:** semantic required-gate omission matrix PASS.  
**Plan changes:** I-028..I-031; F-027..F-029 resolved; F-030 recorded; T-027 DONE; T-028 NEXT.  
**Process improvements:** task policy follows explicit workspace identity; process failures localize earlier; completion is plan/task/finding/digest bound.  
**Commit:** `e154def6a181be2f6120aafa4fb1358fba63ac22`.  
**Push:** `main`, normal fast-forward, no force; remote HEAD verified.  
**Result:** PASS.

### Iteration 10 — T-028
**Selected task:** T-028.  
**Why now:** F-030 was Critical; propagating the coordinator contract to ordinary workers would violate the existing single-writer invariant before T-018/T-019 are implemented.  
**Root cause:** worker/coordinator process authority was implicit and not represented at the task-materialization boundary.  
**Findings addressed:** F-030. F-007 is narrowed but deliberately remains open for hard enforcement.  
**Changes:** `internal/engineering/roles.go`, role authority model, separate worker contract, worker task wrapper, planner propagation and role tag.  
**Tests:** role authority/unknown-role/contract tests plus planner end-to-end task-materialization characterization; full Linux/Windows Harness CI.  
**Mutation:** coordinator-authority bit mutation sentinel PASS for all five forbidden worker capabilities.  
**Race:** full `go test -race ./...` PASS.  
**Security:** worker defaults deny coordinator-only process capabilities; unknown roles fail closed; no credential/network surface added.  
**Performance:** existing compiler/store/scheduler/retry/wait benchmarks PASS; no scheduler hot-path modification.  
**Compatibility:** no TaskRecord persistence schema change, DAG/replan/verification behavior preserved.  
**Plan reconciliation:** I-032 added; F-030 resolved at semantic boundary; T-028 DONE; T-029 promoted to READY/P0; T-019 explicitly remains responsible for hard main-write enforcement.  
**Process learning:** high-consequence authority must be represented as typed executable data and negative tests, with prompt language as a projection of that data rather than the source of authority.  
**Commit:** publication commit containing this iteration record; authoritative SHA is the resulting `main` HEAD.  
**Push:** normal fast-forward only, never force.  
**Result:** PASS after final reconciled-head qualification and publication.

### Context Compression Checkpoint — after T-028

`CURRENT HEAD:` publication commit for Iteration 10 becomes authoritative after final fast-forward.  
`CURRENT QUALIFIED MILESTONE:` coordinator and headless worker engineering roles are explicitly separated; every planner DAG node is worker-bound.  
`ARCHITECTURE:` `internal/engineering/process.go` owns managed completion; `roles.go` owns role authority; `internal/loop` is coordinator lifecycle/injection; `planner.Enqueue` materializes `RoleWorker`; `tasks` execution/runtime is unchanged.  
`CRITICAL INVARIANTS:` I-008/I-009/I-028..I-032.  
`COMPLETED THIS ITERATION:` T-028.  
`RESOLVED FINDINGS:` F-030 at semantic/typed boundary.  
`OPEN CRITICAL/HIGH FINDINGS:` F-004 until atomic reservations; F-007 until Commit Coordinator/single-writer hard enforcement; F-014 until publication proof/coordinator fencing.  
`BLOCKERS:` none for T-029.  
`NEXT TASK:` T-029.  
`WHY NEXT:` current completion still accepts free-form claims for push/checkpoint; machine-verifiable publication evidence is the next highest-leverage process correctness boundary and directly reduces F-014 risk.  
`CRITICAL FILES:` `internal/engineering/process.go`, `internal/engineering/roles.go`, `internal/loop/loop.go`, `internal/planner/planner.go`, `internal/tasks/tasks.go`, `MASTER_PLAN.md`.  
`VERIFICATION COMMANDS:` targeted engineering/planner/loop/hooks; `go test ./...`; `go test -race ./...`; `go vet ./...`; existing benchmark smoke; Windows full tests.  
`IMPORTANT DECISIONS:` coordinator and worker are security/process roles, not styles; worker can modify delegated workspace and propose replans but cannot own task selection/plan mutation/commit/publish/checkpoint; prompt denial is not hard sandbox enforcement.  
`REJECTED OPTIONS:` one universal process prompt; coordinator authority in workers; disabling adaptive replan; treating prompt compliance as T-019 completion; force publication.  
`NEW PROCESS LEARNING:` represent authority at the materialization boundary; couple every high-consequence deny rule to a mutant/negative test; state residual enforcement gaps explicitly instead of overclaiming convergence.
