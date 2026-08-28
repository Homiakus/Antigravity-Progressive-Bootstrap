# MASTER PLAN — Multi-Runtime Agent Control Plane

> Repository: `Homiakus/Antigravity-Progressive-Bootstrap`  
> Baseline main: `2c10ec8131c5cb9a7dc875d316532d1393bf0e4b`  
> Plan status: ACTIVE / LIVING  
> Source of truth: this file. SQLite and generated indexes are execution caches/ledgers, never competing semantic plans.

## 1. Mission

Evolve `agctl` from an Antigravity-centered adaptive control plane into a quota-aware, multi-runtime agent control plane that can dispatch atomic work across Antigravity and Codex, preserve a single living `MASTER_PLAN.md`, use real provider-capacity signals where available, reserve capacity before parallel dispatch, safely hand work between runtimes, and allow only a fenced commit coordinator to integrate verified changes into `main`.

Primary execution loop:

`OBSERVE -> RECONCILE PLAN -> SELECT READY TASK -> BUILD ENVELOPE -> ESTIMATE -> REFRESH CAPACITY -> SELECT PROVIDER/MODEL/SESSION -> RESERVE -> CLAIM WORKER/WORKSPACE -> EXECUTE -> VERIFY -> SELF-REVIEW -> RECONCILE PLAN -> SETTLE USAGE -> COMMIT COORDINATOR -> VERIFIED PUSH MAIN -> CHECKPOINT -> NEXT`

## 2. Current State

The repository already contains most of the durable orchestration substrate and must be evolved incrementally rather than rewritten:

- adaptive DAG replanning and dynamic recovery chains;
- durable SQLite/WAL harness with immutable migrations;
- two-level fair READY-node scheduler;
- physical resource feasibility (`CPU`, memory, GPU/VRAM, disk, build/browser slots);
- durable worker registry;
- fenced execution leases with epochs and heartbeats;
- effect intent tracking and `IN_DOUBT` transitions for uncertain side effects;
- retries, circuit breakers, waits, approvals, artifacts and workspaces;
- Antigravity/AGY executor and live IDE bridge;
- generic documentation path for Codex CLI, but not a first-class Codex provider runtime;
- workflow budget tracking for steps/model calls/tool calls/tokens/cost/failures;
- CI on Linux and Windows with `go test ./...`, race detector, vet and scheduler/store benchmarks.

The missing layer is logical provider capacity and provider assignment. Current scheduler feasibility is about local hardware, not model-account quota/context availability.

## 3. Architecture Map

Target layering:

```text
USER GOAL
   |
MASTER_PLAN.md  <--------------------------+
   |                                       |
Plan Compiler / Reconciler                 | findings
   |                                       |
Durable DAG                                |
   |                                       |
Fair Scheduler                             |
   |                                       |
Task Envelope + Demand Estimator           |
   |                                       |
Provider Selector / Capacity Broker         |
   |                                       |
   +--------------+------------------------+
                  |
      +-----------+-----------+
      |                       |
Antigravity account       Codex account
models/quota/sessions     models/quota/sessions
      |                       |
      +-----------+-----------+
                  |
        capacity reservation
                  |
           worker selection
                  |
             fenced lease
                  |
          isolated workspace
                  |
              execution
                  |
     usage/events/effect intents
                  |
 verify -> review -> plan reconcile
                  |
          Commit Coordinator
                  |
         fast-forward-only main
```

Orthogonal concepts MUST remain separate:

- `Worker`: where work executes physically;
- `ProviderAccount`: whose external model/quota pool is consumed;
- `ProviderModel`: which model is used;
- `ProviderSession`: which conversational/context state is reused;
- `Workflow budget`: what this workflow is allowed to spend;
- `Provider capacity`: what the external account can currently supply;
- `Execution lease`: who currently owns an attempt;
- `Provider reservation`: capacity held for a not-yet-settled assignment;
- `Workspace lease`: who may mutate a worktree/workspace;
- `Commit lease`: who may integrate/push `main`.

## 4. Baseline

### Code baseline

- main SHA: `2c10ec8131c5cb9a7dc875d316532d1393bf0e4b`.
- SQLite schema version: `13`.
- Provider-domain abstraction: absent.
- Provider quota reservation ledger: absent.
- Codex first-class adapter: absent.
- Task-envelope/plan-digest gate: absent.
- Commit coordinator: absent.

### CI baseline

`.github/workflows/harness-ci.yml` currently enforces:

- `go mod tidy` cleanliness;
- JS bridge syntax and contract tests;
- `go test ./...`;
- `go test -race ./...`;
- `go vet ./...`;
- compiler/store/scheduler/retry/wait smoke benchmarks;
- Windows `go test ./...`.

Each implementation iteration must preserve those checks. Provider-specific live-account integration tests must be optional and must never make normal CI require secrets.

## 5. System Invariants

### I-001 — Plan authority

`MASTER_PLAN.md` is the sole semantic execution plan. Generated JSON indexes and SQLite projections must carry a source digest/revision and be disposable/rebuildable.

### I-002 — No hidden provider state

Every provider assignment used for execution must be durable and explainable by task, attempt, worker, provider account, model, session and plan revision.

### I-003 — Worker/provider separation

A worker is never assumed to equal a provider account. One worker may expose several providers/accounts; one provider account may be usable from more than one worker if policy permits.

### I-004 — Budget/capacity separation

Workflow budget and external provider capacity are distinct constraints. A task may dispatch only when both permit execution.

### I-005 — Atomic reservation

Provider selection and reservation must be committed atomically before provider execution starts. Parallel schedulers must not oversubscribe a quota snapshot.

### I-006 — Fenced execution

Existing attempt lease epochs remain authoritative for execution ownership. Provider failover must not weaken worker/attempt fencing.

### I-007 — Side-effect uncertainty is fail-closed

If an attempt may have produced a non-blind-retry-safe side effect, provider loss/failover must enter existing `IN_DOUBT` reconciliation instead of replaying blindly.

### I-008 — Main has a single writer

Workers may create local/checkpoint commits in isolated workspaces. Only the Commit Coordinator may integrate and push `main`.

### I-009 — No force push

`main` integration is fast-forward/rebase/reconcile based. `--force` and `--force-with-lease` are not part of normal autonomous operation.

### I-010 — Dynamic discovery

Provider/model identifiers must be discovered from runtime adapters when possible. Core scheduling must not depend on hard-coded transient model names or opaque quota bucket IDs.

### I-011 — Bounded uncertainty

Unknown or stale quota information reduces concurrency/score; it never silently counts as unlimited capacity.

### I-012 — Backward-compatible migration

Provider routing is introduced progressively: observe -> shadow -> advisory -> automatic read-only -> isolated writes -> failover -> commit coordination -> default. Legacy paths are deleted only after proven replacement.

## 6. Findings Registry

### F-001 — Provider capacity is not a scheduler dimension

**Status:** Planned  
**Category:** Architecture / Scheduling  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** `internal/harness/resource/resource.go`, `internal/harness/scheduler/scheduler.go` only classify physical resource capacity.  
**Root cause:** scheduler predates multi-runtime quota-aware dispatch.  
**Impact:** READY tasks cannot be intelligently balanced between Antigravity and Codex based on live external capacity.  
**Affected tasks:** T-001..T-013.

### F-002 — Codex is documented as generic terminal rather than first-class provider

**Status:** Planned  
**Category:** Integration  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** `docs/UNIVERSAL_PLATFORMS_RU.md`.  
**Impact:** no normalized model/capacity/session/usage contract for Codex.  
**Affected tasks:** T-002, T-007, T-012.

### F-003 — Workflow budget cannot substitute for provider quota

**Status:** Planned  
**Category:** Resource Accounting  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** `internal/harness/budget/budget.go` tracks per-workflow allowed consumption, not external account availability.  
**Impact:** reusing it for quota would conflate authorization-to-spend with available supply.  
**Affected tasks:** T-003..T-010.

### F-004 — No durable quota reservation ledger

**Status:** Planned  
**Category:** Concurrency / Correctness  
**Severity:** Critical  
**Confidence:** Confirmed  
**Impact:** parallel dispatchers could observe the same remaining quota and oversubscribe it.  
**Affected tasks:** T-005, T-010, T-011.

### F-005 — Existing worker/lease subsystem is reusable and must not be replaced

**Status:** Resolved by architecture  
**Category:** Architecture  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** `internal/harness/model/worker.go`, `internal/harness/worker/registry.go`, `internal/harness/engine/lease_runtime.go`.  
**Decision:** provider assignment is an orthogonal layer over existing workers/leases.  
**Affected tasks:** T-001, T-012, T-016.

### F-006 — Existing `IN_DOUBT` + effect intents already solve core unsafe-replay detection

**Status:** Resolved by architecture  
**Category:** Reliability  
**Severity:** Critical  
**Confidence:** Confirmed  
**Evidence:** `Engine.ReclaimExpiredAttempt` checks effect intents and moves unsafe attempts/nodes to `IN_DOUBT`.  
**Decision:** safe provider handoff must reuse this machinery rather than create a second uncertainty state machine.  
**Affected tasks:** T-015, T-016.

### F-007 — Direct worker push discipline is unsafe under parallel multi-provider writes

**Status:** Planned  
**Category:** Git / Concurrency  
**Severity:** Critical  
**Confidence:** Strong  
**Impact:** two successful workers can race semantically on `main`.  
**Affected tasks:** T-018, T-019.

### F-008 — No task envelope independent of conversation history

**Status:** Planned  
**Category:** Context / Portability  
**Severity:** High  
**Confidence:** Confirmed  
**Impact:** handoff between runtimes is token-expensive and can lose task invariants.  
**Affected tasks:** T-013, T-014.

### F-009 — No durable plan digest on attempts/assignments

**Status:** Planned  
**Category:** Plan Consistency  
**Severity:** High  
**Confidence:** Confirmed  
**Impact:** an agent can continue executing against a stale plan revision after a material finding changes dependencies/scope.  
**Affected tasks:** T-013, T-014, T-016.

### F-010 — No session/context broker

**Status:** Planned  
**Category:** Efficiency  
**Severity:** Medium  
**Confidence:** Confirmed  
**Impact:** naive quota balancing can spend more tokens by moving a task away from a high-affinity session.  
**Affected tasks:** T-008, T-013.

### F-011 — Routing decisions are not durable/explainable at provider level

**Status:** Planned  
**Category:** Observability  
**Severity:** Medium  
**Confidence:** Confirmed  
**Affected tasks:** T-009, T-020, T-021.

### F-012 — Released SQLite migrations are immutable

**Status:** Resolved by architecture  
**Category:** Persistence  
**Severity:** High  
**Confidence:** Confirmed  
**Evidence:** `internal/harness/store/sqlite/migrations.go` verifies stored checksums and schema is currently v13.  
**Decision:** provider persistence begins at v14; no previous migration is modified.  
**Affected tasks:** T-003, T-005.

## 7. Risk Register

| Risk | Level | Mitigation |
|---|---|---|
| quota oversubscription | Critical | atomic reservation transaction + expiry + settlement |
| duplicate provider side effects during failover | Critical | existing effect intents + `IN_DOUBT` + replay-safety gate |
| simultaneous `main` pushes | Critical | fenced Commit Coordinator |
| stale plan execution | High | plan revision/digest on envelope and assignment |
| provider API/schema drift | High | adapter boundary + tolerant parsing + capability discovery |
| opaque quota bucket semantics | High | preserve provider-native IDs, confidence, metric kind; no guessed conversions |
| session context exhaustion | High | session snapshots + checkpoint/compact/new-session policy |
| routing flaps between providers | Medium | hysteresis / switch penalty / minimum reservation lifetime |
| poor usage estimation | Medium | conservative p80 estimator + uncertainty penalty + online settlement |
| unavailable credentials in CI | Medium | fake adapters/contract fixtures; live probes opt-in |
| runtime regression from large rewrite | High | shadow-first migration and backward-compatible phases |

## 8. Pareto Improvements

Highest-leverage first slice:

1. establish provider-domain model without runtime behavior changes;
2. persist normalized accounts/models/capacity/sessions;
3. ingest real Antigravity and Codex capacity signals;
4. run shadow selection and collect prediction-vs-actual evidence;
5. add atomic reservations before enabling automatic dispatch;
6. enforce task envelopes/plan digests before write failover;
7. add Commit Coordinator before parallel multi-provider writes become default.

These changes unlock most of the desired behavior while minimizing early risk.

## 9. Dependency DAG

```text
T-001 provider model primitives
   |
   +--> T-002 provider interfaces/registry
   |       |
   |       +--> T-006 Antigravity adapter
   |       +--> T-007 Codex adapter
   |
   +--> T-003 persistence v14
           |
           +--> T-004 provider stores
           |       |
           |       +--> T-008 session broker
           |       +--> T-009 capacity normalizer
           |
           +--> T-005 assignment/reservation persistence
                   |
                   +--> T-010 demand estimator
                   +--> T-011 atomic reservation service
                           |
                           +--> T-012 shadow provider selector
                                   |
                                   +--> T-013 TaskEnvelope + plan digest
                                   |       |
                                   |       +--> T-014 automatic read-only routing
                                   |       +--> T-015 provider-aware failures/retry
                                   |               |
                                   |               +--> T-016 safe handoff/IN_DOUBT reuse
                                   |
                                   +--> T-017 isolated write routing
                                           |
                                           +--> T-018 Commit Coordinator
                                                   |
                                                   +--> T-019 enforce single-writer main

T-020 explain routing ---- depends T-012
T-021 dashboard ---------- depends T-004,T-005,T-012
T-022 concurrency/property tests -- grows alongside T-005..T-019
T-023 mutation tests ----- depends critical policies implemented
T-024 rollout/default ---- depends T-014..T-023
T-025 debt deletion ------ depends T-024
T-026 final re-audit ------ depends T-025
```

## 10. Implementation Phases

### Phase A — Domain foundation

T-001..T-005. No provider routing changes.

### Phase B — Observe external capacity

T-006..T-009. Capacity is ingested and visible but does not control execution.

### Phase C — Predict and shadow

T-010..T-012. Produce explainable decisions; legacy execution remains authoritative.

### Phase D — Portable task execution

T-013..T-016. Task envelope, plan freshness and safe failover.

### Phase E — Multi-provider writes

T-017..T-019. Isolated writes and one fenced path to `main`.

### Phase F — Operability and proof

T-020..T-026. Explainability, dashboard, adversarial tests, rollout, legacy deletion and re-audit.

## 11. Atomic Tasks

### T-001 — Add provider-domain primitives

**Status:** VERIFYING  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

**Problem**  
No stable provider/account/model/quota/session vocabulary exists in the durable harness.

**Files / symbols**

- `internal/harness/model/ids.go`
  - `ProviderAccountID`
  - `ProviderSessionID`
  - `ProviderAssignmentID`
  - `ProviderReservationID`
  - new generated `IDKind`s
- `internal/harness/model/provider.go`
  - `ProviderKind`
  - `ProviderHealth`
  - `ProviderAccountState`
  - `ProviderAccount`
  - `ProviderModelDescriptor`
  - `QuotaMetricKind`
  - `QuotaWindow`
  - `ProviderCapacitySnapshot`
  - `ProviderSessionState`
  - `ProviderSessionSnapshot`
- `internal/harness/model/provider_test.go`

**Implementation**

Introduce provider-neutral data types only. Do not modify scheduler, worker, executor or persistence behavior.

**Invariants**

- no hard-coded concrete model list;
- context remains session-scoped, not quota-scoped;
- partial/opaque quota metrics are representable;
- confidence and observation timestamps are first-class.

**Tests**

- provider enum validation;
- quota fraction bounds;
- session context overflow rejection;
- generated provider ID kinds validate.

**Acceptance criteria**

- model package compiles;
- existing IDs remain compatible;
- `go test ./...` and race/vet CI remain green;
- no runtime behavior change.

**Verification commands**

`go test ./internal/harness/model/...`  
`go test ./...`  
`go test -race ./...`  
`go vet ./...`

**Dependencies:** none  
**Rollback:** remove new provider file/types and ID kinds; no persisted schema affected.

### T-002 — Define provider adapter and registry contracts

**Status:** TODO  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

**Files / symbols**

- new `internal/harness/provider/provider.go`
- new `internal/harness/provider/registry.go`
- new `internal/harness/provider/provider_test.go`

Define small interfaces: `CapacitySource`, `ModelSource`, `SessionSource`, `UsageSource`, `AgentRuntime`. Registry resolves capabilities by `ProviderKind`/account and supports fakes. Do not let adapters import scheduler internals.

**Tests:** fake-provider registration, duplicate account rejection, capability discovery, context cancellation.  
**Acceptance:** no provider-specific logic in core registry; no execution path changed.  
**Dependencies:** T-001.

### T-003 — Persist provider observation state in immutable SQLite v14

**Status:** TODO  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

**Files / symbols**

- new `internal/harness/store/sqlite/migration_v14_provider_observation.go`
- update `internal/harness/store/sqlite/migrations.go` (`SchemaVersion=14`, append migration)
- new migration contract test

Tables:

- `provider_accounts`
- `provider_models`
- `provider_capacity_snapshots`
- `provider_quota_windows`
- `provider_sessions`

Use foreign keys and indexes for latest account/model/session observations. Released migrations 1..13 remain byte-identical.

**Tests:** fresh DB v14, upgrade v13->v14, migration checksum immutability, FK rejection, duplicate snapshot semantics.  
**Dependencies:** T-001.

### T-004 — Add provider observation store contracts and SQLite implementation

**Status:** TODO  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

**Files / symbols**

- extend `internal/harness/store/store.go` Reader/Tx
- new `internal/harness/store/sqlite/provider_store.go`
- new `provider_store_test.go`

Operations include upsert/get account/model/session, append capacity snapshot, get latest capacity, list usable accounts/models/sessions.

**Tests:** read/write roundtrip, update conflicts, stale observation ordering, concurrent readers.  
**Dependencies:** T-003.

### T-005 — Persist assignments, reservations and usage in SQLite v15

**Status:** TODO  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

**Files / symbols**

- new model `provider_assignment.go`
- new `migration_v15_provider_execution.go`
- `SchemaVersion=15`
- new store methods and `provider_execution_store.go`

Tables:

- `provider_assignments`
- `provider_reservations`
- `provider_usage_samples`
- `provider_circuit_state`

Reservation states: `ACTIVE`, `SETTLED`, `RELEASED`, `EXPIRED`. Assignment must reference attempt + provider account + model; session may be nullable before one exists.

**Tests:** only one active assignment per attempt generation, reservation expiry, settlement idempotency, concurrent reservation CAS.  
**Dependencies:** T-003,T-004.

### T-006 — Add Antigravity capacity/model/session adapter

**Status:** TODO  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

**Files / symbols**

- new `internal/harness/provider/antigravity/`
- integrate with existing AGY/Antigravity bridge/config paths without modifying low-level `executor.Executor`
- CLI/status-line ingestion command in `cmd/agctl`

Normalize status-line/headless observable data into provider-neutral snapshots. Preserve raw provider-native bucket IDs and timestamps. Never scrape UI pixels.

**Tests:** JSON fixtures, missing fields, unknown future fields, reset timestamp, fractional quota, malformed input fail-closed.  
**Dependencies:** T-002,T-004.

### T-007 — Add Codex App Server provider adapter

**Status:** TODO  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

**Files / symbols**

- new `internal/harness/provider/codex/`
- App Server transport/client boundary
- optional CLI execution bridge as fallback, not quota source of truth when App Server is available

Consume account/rate-limit snapshot/update events through a tolerant adapter. Do not infer semantic model mapping from opaque bucket IDs unless explicitly supplied by the provider.

**Tests:** recorded protocol fixtures, rolling updates merged with last snapshot, disconnect/reconnect, unknown bucket preservation, auth/rate-limit classification.  
**Dependencies:** T-002,T-004.

### T-008 — Build session/context broker

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH

**Files / symbols**

- new `internal/harness/provider/session/`
- provider session store methods from T-004

Policy outputs: `REUSE`, `NEW`, `CHECKPOINT_AND_NEW`, `UNAVAILABLE`. Inputs include context headroom, workspace fingerprint, task affinity, provider health and model requirements.

**Tests:** context boundaries, stale session, different workspace, closed/exhausted session, anti-thrashing hysteresis.  
**Dependencies:** T-004,T-006,T-007.

### T-009 — Normalize provider capacity and calculate effective headroom

**Status:** TODO  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

**Files / symbols**

- new `internal/harness/provider/capacity/`

Keep absolute tokens/requests/cost/fraction/opaque metrics separate. Derive only values justified by source data. Compute staleness and confidence penalties.

**Tests:** known/unknown limits, reset passed, stale snapshots, contradictory provider updates, zero/negative invalid input.  
**Dependencies:** T-004,T-006,T-007.

### T-010 — Add conservative task-demand estimator

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH

**Files / symbols**

- new `internal/harness/provider/estimate/`
- usage sample queries from T-005

Start deterministic/statistical: task class + provider/model + repository/context features -> p50/p80 estimate and confidence. Default reservation uses conservative p80; low-confidence cold starts use configurable floors.

**Tests:** no-history fallback, percentile monotonicity, outlier resistance, no negative estimates.  
**Dependencies:** T-005,T-009.

### T-011 — Implement atomic provider capacity reservations

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

**Files / symbols**

- new `internal/harness/provider/reservation/`
- T-005 SQLite store methods

One store transaction must read latest capacity + active reservations, verify effective headroom and create reservation/assignment. Never do read-select-write in separate transactions.

**Tests:** 100+ concurrent reservers cannot oversubscribe; expiry releases capacity; settlement is idempotent; crash-before-start cleanup.  
**Dependencies:** T-005,T-009,T-010.

### T-012 — Add explainable shadow provider selector

**Status:** TODO  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

**Files / symbols**

- new `internal/harness/provider/select/`
- minimal hook from `internal/harness/scheduler/scheduler.go` after physical feasibility, initially shadow-only

Hard filter first: provider/account/model health, required capabilities, context requirements, risk policy. Soft score second: projected quota pressure, reset horizon, session affinity, reliability, switch penalty and uncertainty. No pseudo-precision is exposed as truth; store factor explanations.

**Tests:** unsuitable model never wins, exhausted account rejected, reset-soon capacity favored when otherwise safe, low-confidence capacity penalized, deterministic tie-break.  
**Dependencies:** T-008..T-011.

### T-013 — Introduce portable TaskEnvelope and plan digest

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

**Files / symbols**

- new `internal/harness/model/task_envelope.go`
- new `internal/harness/planindex/`
- compiler/replan integration points

Envelope contains task ID, plan revision/digest, objective, evidence, scope/non-goals, invariants, findings, capabilities, acceptance criteria, verification, risk, replay safety, workspace and checkpoint reference.

Generated plan index is disposable and must include `sourceDigest`. A stale digest invalidates affected assignments before dispatch.

**Tests:** deterministic digest, plan edit invalidates envelope, unrelated generated cache can be rebuilt, missing plan fails closed for write tasks.  
**Dependencies:** T-012.

### T-014 — Enable automatic read-only provider routing

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH

Use shadow evidence to allow only explicitly read-only/replay-safe nodes to be dispatched through provider selector/reservation. Existing worker lease remains authoritative.

**Tests:** no filesystem mutation permissions, provider failover replay is safe, reservations settle on success/failure/cancel.  
**Dependencies:** T-011..T-013.

### T-015 — Make retry/failure taxonomy provider-aware

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

**Files / symbols**

- extend harness failure taxonomy
- `internal/harness/executor/agy/failure.go`
- provider adapters
- `internal/harness/retry/`

Add classes for quota exhausted/low, rate limit, spend control, auth, model unavailable, provider overload/protocol/transient, context low/exhausted and handoff unsafe.

Policy: deterministic code/test failure is not provider failover; quota exhaustion is generally reassignment; short rate limit may wait when cheaper than switching.

**Tests:** decision table and mutation-resistant branch tests.  
**Dependencies:** T-014.

### T-016 — Implement safe provider handoff using existing `IN_DOUBT`

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

**Files / symbols**

- `internal/harness/engine/lease_runtime.go`
- effect reconciliation paths
- new provider handoff coordinator

Do not duplicate uncertainty state. Before redispatching an interrupted write attempt, inspect existing effect intents/checkpoint/workspace state. Unsafe ambiguity remains `IN_DOUBT` until reconciled.

**Tests:** provider dies before tool use; after blind-retry-safe effect; after unsafe dispatched effect; recovered effect; stale lease epoch; crash during checkpoint.  
**Dependencies:** T-013,T-015.

### T-017 — Enable isolated write routing in worktrees/workspaces

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH

Integrate provider assignment with existing workspace subsystem. Provider workers may mutate only their assigned isolated workspace and create local checkpoint commits. Direct `main` push remains disabled.

**Tests:** two providers write independent worktrees, same workspace conflict blocked, cleanup after cancel, base SHA recorded.  
**Dependencies:** T-016.

### T-018 — Add fenced Commit Coordinator

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

**Files / symbols**

- new `internal/harness/commitcoord/`
- new commit-coordinator lease/store state if existing workspace lease cannot safely express repository integration ownership

Collect successful checkpoint commits, fetch current main, integrate/rebase/cherry-pick conservatively, run required verification, reconcile `MASTER_PLAN.md`, then perform fast-forward-only push.

**Tests:** two competing coordinators -> one fenced winner; remote main moved -> no force push; semantic conflict -> blocked/replan; verification failure -> no push.  
**Dependencies:** T-017.

### T-019 — Enforce single-writer main policy

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

Remove/generated instructions that tell ordinary workers to push `main`; replace with checkpoint-commit contract and coordinator handoff. Add policy/static tests that worker prompts/configuration cannot grant direct main push in autonomous multi-runtime mode.

**Dependencies:** T-018.

### T-020 — Add `agctl route explain` and provider diagnostics

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** MEDIUM

Commands:

- `agctl providers list`
- `agctl providers capacity`
- `agctl providers sessions`
- `agctl route explain <node|task>`
- extend `doctor` with adapter health/staleness/circuit state

Output must distinguish hard rejection from soft score factors and show observation age/confidence.

**Dependencies:** T-012.

### T-021 — Add dashboard provider/quota view

**Status:** TODO  
**Priority:** P2  
**Type:** IMPROVE  
**Leverage:** MEDIUM

Expose provider health, quota windows/reset, active reservations, predicted vs actual burn, sessions/context headroom, circuit state and routing explanations. Never expose secrets/tokens.

**Dependencies:** T-004,T-005,T-012,T-020.

### T-022 — Build multidimensional concurrency/property/chaos tests

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

Cover:

`provider x account x model x quota state x reservation state x worker lease x session context x failure timing x replay safety x plan revision x remote-main movement`.

Use table tests, pairwise/high-risk N-wise, fuzz/property tests and crash/restart store tests. Explicitly test reset boundary races and stale snapshots.

**Dependencies:** incremental T-005..T-019.

### T-023 — Add mutation testing for provider policy

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

Mutation targets: quota bound comparisons, stale-plan checks, replay-safe classification, circuit transitions, reservation settlement, single-writer push gate. Surviving mutants require stronger observable contracts, not coverage-only patches.

**Dependencies:** T-011..T-019.

### T-024 — Progressive rollout to automatic multi-runtime mode

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH

Modes:

`OBSERVE -> SHADOW -> ADVISORY -> AUTO_READ_ONLY -> AUTO_ISOLATED_WRITE -> AUTO_FAILOVER -> DEFAULT`.

Promotion requires measured thresholds for routing stability, reservation accuracy, no duplicate effects, no main-write races and acceptable provider error rate.

**Dependencies:** T-014..T-023.

### T-025 — Delete legacy duplicate routing/push paths

**Status:** TODO  
**Priority:** P2  
**Type:** REMOVE  
**Leverage:** MEDIUM

After default rollout, remove superseded compatibility adapters, stale flags, direct-worker-main instructions, duplicate model/account concepts and obsolete tests. Keep explicit compatibility paths only when they still serve supported platforms.

**Dependencies:** T-024.

### T-026 — Re-audit and convergence pass

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

Repeat architecture, correctness, concurrency, security, persistence, API, CI, performance and mutation audits. Add any delta findings/tasks before declaring convergence.

**Dependencies:** T-025.

## 12. Testing Strategy

For each atomic task use the narrowest meaningful proof first, then the broader suite:

1. characterization/contract test;
2. targeted package tests;
3. integration/store tests;
4. concurrency/race tests where shared state changes;
5. property/fuzz tests for normalization, reservations and state machines;
6. full `go test ./...`;
7. `go test -race ./...`;
8. `go vet ./...`;
9. existing smoke benchmarks;
10. optional live-provider probes only when credentials are explicitly available.

Provider adapters require recorded fixtures so normal CI remains deterministic and secret-free.

## 13. Mutation Testing Strategy

Critical mutation surfaces:

- quota threshold `<`/`<=` changes;
- reservation subtraction omitted;
- stale observation accepted;
- plan digest check bypassed;
- replay-safe/unsafe branch inverted;
- `IN_DOUBT` bypassed;
- circuit breaker OPEN/HALF_OPEN transition altered;
- commit coordinator fencing check removed;
- force-push prohibition bypassed.

A surviving mutant becomes a new finding if the observable contract is unclear.

## 14. Performance Baselines

Do not regress existing scheduler/store benchmarks materially. Add provider benchmarks after T-011/T-012:

- 10k provider capacity snapshots latest-read;
- 10k active reservations aggregate;
- 1k READY nodes x 8 provider/model candidates selection;
- 100 concurrent reservation attempts;
- routing explanation generation without N+1 store queries.

Target: provider selection overhead should remain small relative to LLM/network latency and bounded independently of historical snapshot count through indexed latest-state projections.

## 15. Security Hardening

- credentials remain in provider-specific secure configuration, never SQLite telemetry or plan files;
- provider adapters expose capability/health, not secrets;
- untrusted remote workers do not inherit provider credentials unless policy explicitly allows it;
- direct `main` push capability is restricted to Commit Coordinator;
- provider model/session strings are treated as external input and validated/bounded before persistence;
- quota/status ingestion parsers must cap payload size and reject malformed required fields;
- logs redact authentication and provider bearer/session tokens.

## 16. Migration Strategy

`characterize -> introduce provider model -> persist observations -> observe live sources -> shadow select -> reserve -> portable envelope -> auto read-only -> safe handoff -> isolated writes -> commit coordinator -> enforce single writer -> default -> delete legacy`.

Each phase must keep old execution usable until the replacement path proves itself.

## 17. Deferred Work

Until evidence justifies it:

- ML/LLM-based demand predictor (start with robust statistics);
- cross-machine distributed provider credential vault;
- provider marketplace/plugin ABI;
- automatic monetary arbitrage across paid APIs;
- opaque bucket reverse-engineering;
- force-push recovery automation.

## 18. Rejected Decisions

### R-001 — Treat provider as worker

Rejected: violates topology; one worker can expose multiple provider accounts/models and provider health is not physical worker health.

### R-002 — Reuse `budget.Tracker` as quota manager

Rejected: workflow budget is permission-to-spend; provider quota is external supply.

### R-003 — Put context remaining inside quota window

Rejected: context belongs to a provider session, not to an account quota bucket.

### R-004 — Hard-code model names / opaque quota IDs

Rejected: provider catalogs and bucket semantics change independently of agctl releases.

### R-005 — Retry another provider immediately after any failure

Rejected: deterministic implementation/test errors are not provider failures; unsafe side effects require reconciliation.

### R-006 — Let every successful worker push `main`

Rejected: creates semantic races in parallel multi-provider execution.

## 19. Completed Tasks

None yet. T-001 is in verification.

## 20. Iteration Log

### Iteration 1

**Task:** T-001  
**Findings addressed:** F-001, F-005 foundation  
**Unexpected findings:** F-006 (existing `IN_DOUBT`/effect machinery reduces future handoff scope), F-012 (v14+ immutable migration requirement)  
**Changes:** provider-domain primitives, generated provider IDs, contract tests, initial living master plan  
**Tests:** isolated model-package compilation/tests completed before repository commit; repository CI pending  
**Plan changes:** provider/worker/budget separation made explicit; handoff design changed to reuse existing effect reconciliation  
**Commit:** pending  
**Push:** pending verified fast-forward to `main`  
**Result:** VERIFYING

## 21. Definition of Final Done

Convergence requires all of the following:

- no known Critical/High provider-routing findings remain open;
- Antigravity and Codex are first-class provider adapters behind the same core contracts;
- live/stale/unknown capacity is represented without guessed semantics;
- parallel reservations cannot oversubscribe provider capacity under tested races;
- task envelopes make provider handoff independent of full conversation history;
- stale plan revisions cannot start affected write attempts;
- unsafe provider interruption reaches existing `IN_DOUBT` reconciliation instead of blind replay;
- automatic read-only and isolated-write routing have passed progressive rollout gates;
- only the fenced Commit Coordinator can autonomously push `main`;
- no force-push path exists in normal autonomous operation;
- CI, race, vet, mutation and relevant benchmarks are green;
- provider routing is explainable and observable;
- legacy duplicate routing/push paths are deleted after migration;
- final re-audit produces no new fundamental Critical/High finding;
- final verified state and synchronized `MASTER_PLAN.md` are on `main`.
