# HARNESS IMPLEMENTATION PLAN — переход `agctl` к Durable Execution Harness

Статус: **implementation blueprint / migration plan**  
Целевая линия: **agctl Harness Runtime 4.x**  
Исходная база: `agctl 3.2.1`  
Приоритеты: **Correctness → Durability → Recoverability → Explicit State → Idempotency → Observability → Extensibility → Performance → Distributed Scaling**

---

# 0. Назначение документа

Этот документ — не общий roadmap и не список пожеланий. Это пошаговый инженерный план миграции существующего `Antigravity-Progressive-Bootstrap` из текущей модели:

```text
ExecutionPlan JSON
      ↓
TaskRecord JSON
      ↓
RunPending scan
      ↓
.run.lock
      ↓
goroutine / AGY process
      ↓
mutable TaskRecord
```

в production-grade durable Harness:

```text
WorkflowDefinition
      ↓
WorkflowRun
      ↓
GraphRevision
      ↓
NodeRun
      ↓
immutable Attempt
      ↓
durable Lease + fencing epoch
      ↓
Executor
      ↓
Result / IN_DOUBT
      ↓
Artifact
      ↓
transactional state + immutable event
```

План специально привязан к существующей кодовой базе, чтобы реализация происходила эволюционно, без big-bang rewrite.

Главный критерий готовности:

> После аварийного завершения `agctl` в любой момент времени новый процесс должен по durable state определить, какие workflow существуют, какие nodes завершены, какие attempts были запущены, какие leases потеряны, какие операции безопасно повторить, какие side effects требуют reconciliation, какие artifacts существуют и какие nodes можно продолжить.

---

# 1. Текущее состояние кодовой базы и точки миграции

## 1.1 `internal/model/types.go`

Сейчас здесь находятся сразу несколько слоёв модели:

- MCP config;
- capability registry;
- orchestration config;
- `ExecutionPlan`;
- `PlanNode`;
- `TaskRecord`;
- `TaskSupervisorConfig`;
- `PlanRevision`;
- `ReplanProposal`.

Проблема: logical workflow definition, runtime state, process execution state и scheduler resource specification смешаны в одном общем package.

### Что делать

Не удалять старые типы сразу. Ввести новый изолированный namespace:

```text
internal/harness/model/
```

и постепенно переводить существующие packages на него.

Старые `model.ExecutionPlan`, `model.TaskRecord`, `model.PlanRevision` оставить как compatibility types до завершения migration stage.

---

## 1.2 `internal/planner/planner.go`

Сейчас `planner.Create()`:

1. принимает prompt/workspace;
2. определяет project profile;
3. строит capability ranking;
4. создаёт `model.ExecutionPlan`;
5. формирует `[]PlanNode`;
6. валидирует DAG;
7. записывает plan как JSON.

### Целевое изменение

Planner не должен создавать runtime entities.

Он должен стать frontend-компилятором:

```text
prompt/project/capabilities
        ↓
planner proposal
        ↓
WorkflowDefinition draft
        ↓
harness/compiler.Validate()
        ↓
harness/compiler.Compile()
        ↓
immutable WorkflowDefinition
```

Planner может оставаться explainable и deterministic, но persistence runtime должна перейти в Harness store.

---

## 1.3 `internal/tasks/tasks.go`

Сейчас package одновременно реализует:

- persistence TaskRecord в JSON-файлах;
- task list;
- dependency resolution;
- READY detection;
- priority ordering;
- resource selection;
- retries;
- concurrent execution goroutines;
- process spawning;
- AGY stream parsing;
- cancellation;
- PID tracking;
- `.run.lock` claim;
- stale lock recovery;
- telemetry.

Это основной архитектурный узел, который нужно декомпозировать.

### Целевое разбиение

```text
internal/harness/store/        persistence
internal/harness/engine/       state transitions
internal/harness/scheduler/    readiness/fairness/resources
internal/harness/lease/        ownership
internal/harness/executor/     physical execution
internal/harness/retry/        retry policy
internal/harness/events/       durable journal
```

`internal/tasks` на переходном этапе становится adapter/facade к новому runtime.

---

## 1.4 `internal/replan/replan.go`

Сейчас replanner:

- читает TaskRecord;
- читает persisted ExecutionPlan;
- проверяет proposal;
- создаёт новые PlanNode;
- создаёт новые TaskRecord;
- rewires TaskRecord dependencies;
- rewires ExecutionPlan dependencies;
- обновляет revision history.

Главный риск — dual-write между plan JSON и task JSON.

### Целевое изменение

Replanner должен выдавать только:

```text
GraphMutation
```

Применение mutation выполняется **одной DB transaction** через `harness/engine`.

```text
replan proposal
   ↓
validate
   ↓
GraphMutation
   ↓
engine.ApplyGraphMutation(tx)
   ├ graph_revision
   ├ new nodes
   ├ dependencies
   ├ node_runs
   ├ dependency counters
   └ event
COMMIT
```

---

## 1.5 `internal/mcpprobe/probe.go`

Существующий код уже поддерживает:

- MCP `2026-07-28`;
- `server/discover`;
- per-request metadata;
- fallback `2025-11-25`;
- legacy `initialize/initialized`;
- stdio/HTTP probing.

Его не нужно переписывать с нуля.

### Целевое изменение

Разделить:

```text
probe = health/discovery diagnostics
runtime adapter = actual durable tool execution
```

Общие protocol helpers вынести в:

```text
internal/harness/executor/mcp/protocol/
```

---

# 2. Принцип миграции

Запрещён big-bang rewrite.

Использовать strangler pattern:

```text
3.2 legacy path
      │
      ├── compatibility adapter
      ▼
new Harness core
      │
      └── постепенно становится default
```

На каждом этапе должны выполняться:

```text
go test ./...
go test -race ./...
go vet ./...
```

Если stage добавляет storage/runtime semantics — дополнительно fault tests и migration tests.

До удаления legacy JSON execution path нужно минимум два релизных цикла compatibility mode.

---

# 3. Целевая структура packages

Создать постепенно:

```text
internal/harness/
    model/
        ids.go
        workflow.go
        node.go
        attempt.go
        event.go
        artifact.go
        worker.go
        policy.go
        errors.go

    ir/
        definition.go
        node.go
        expression.go
        codec.go

    compiler/
        compiler.go
        validate.go
        dag.go
        normalize.go

    state/
        workflow_state.go
        node_state.go
        attempt_state.go
        transitions.go
        invariants.go

    store/
        store.go
        tx.go
        query.go
        errors.go
        sqlite/
            db.go
            pragma.go
            migrations.go
            queries.go
            tx.go
            recovery.go
        postgres/              # V3, интерфейс оставить заранее, реализацию позже

    events/
        event.go
        journal.go
        outbox.go
        inbox.go

    engine/
        engine.go
        workflow.go
        completion.go
        mutation.go
        recovery.go
        reconcile.go

    scheduler/
        scheduler.go
        ready.go
        dependency.go
        fairness.go
        priority.go
        explain.go
        resource.go
        queues.go

    lease/
        lease.go
        heartbeat.go
        fencing.go

    worker/
        registry.go
        local.go
        capabilities.go
        trust.go

    retry/
        policy.go
        backoff.go
        classifier.go
        budget.go
        breaker.go

    timer/
        timer.go
        heap.go
        service.go

    signal/
        signal.go
        service.go
        dedupe.go

    approval/
        approval.go
        policy_bridge.go

    resource/
        spec.go
        pool.go
        allocation.go
        affinity.go

    artifact/
        artifact.go
        store.go
        cas/
            cas.go
            writer.go
            reader.go
            gc.go
        provenance/
            graph.go
        cache/
            key.go
            policy.go

    workspace/
        workspace.go
        lease.go
        local.go
        gitworktree.go

    executor/
        executor.go
        registry.go
        process/
            executor.go
            process_windows.go
            process_unix.go
            logs.go
        agent/
            executor.go
            runtime.go
            budget.go
            checkpoint.go
        mcp/
            executor.go
            registry.go
            protocol/
        validator/
            executor.go

    policy/
        engine.go
        capability.go
        decision.go

    budget/
        manager.go
        reservation.go

    telemetry/
        tracing.go
        metrics.go
        logs.go

    api/
        commands.go
        queries.go
        dto.go
```

Не создавать все директории пустыми заранее. Каждый package появляется только в том stage, где начинает использоваться.

---

# 4. Stage 0 — Architecture Contracts и Harness Model

## Цель

Разделить definition/runtime/attempt semantics до изменения execution path.

## Где делать

Создать:

```text
internal/harness/model/
internal/harness/ir/
internal/harness/state/
internal/harness/compiler/
```

Изменить минимально:

```text
internal/planner/planner.go
internal/model/types.go
```

## 4.1 Ввести typed IDs

Не использовать plain string повсеместно.

```go
type WorkflowDefinitionID string
type WorkflowRunID string
type NodeID string
type NodeRunID string
type AttemptID string
type WorkerID string
type ArtifactID string
type EventID string
type SignalID string
type ApprovalID string
```

Добавить constructors и validation.

### Требования

- ID immutable;
- string representation stable;
- generation monotonic-friendly, предпочтительно UUIDv7/ULID-class semantics;
- тесты не должны зависеть от wall clock;
- генератор injectable.

## 4.2 WorkflowDefinition

Определить immutable definition:

```text
ID
Version
Name
CreatedAt
CompilerVersion
Nodes
Metadata
```

Runtime status туда не помещать.

## 4.3 NodeSpec

Минимальные поля:

```text
NodeID
Kind
ExecutorKind
Dependencies
RetryPolicyRef
TimeoutPolicyRef
ResourceSpec
PolicySpec
CachePolicy
InputRefs
OutputDeclarations
Metadata
```

Не включать:

```text
StartedAt
FinishedAt
PID
Attempts
Status
```

## 4.4 Control-flow kinds

Начать с небольшого набора:

```text
ACTION
DECISION
WAIT
SUBWORKFLOW
MAP
```

Не делать `Agent`, `MCP`, `Shell` отдельными control-flow kinds.

Они являются `ExecutorKind` для `ACTION`.

## 4.5 Runtime types

Создать:

```text
WorkflowRun
GraphRevision
NodeRun
Attempt
```

Отдельно определить identity и lifecycle.

## 4.6 State machines

### WorkflowRun

```text
CREATED
VALIDATING
QUEUED
RUNNING
PAUSING
PAUSED
CANCELLING
SUCCEEDED
FAILED
CANCELLED
BLOCKED
```

### NodeRun

```text
PENDING_DEPENDENCIES
READY
QUEUED
RUNNING
WAITING
RETRY_WAIT
IN_DOUBT
SUCCEEDED
FAILED
TIMED_OUT
CANCELLED
SKIPPED
UNSCHEDULABLE
```

### Attempt

```text
CREATED
CLAIMED
RUNNING
SUCCEEDED
FAILED
TIMED_OUT
CANCELLED
LOST
IN_DOUBT
```

## 4.7 Transition engine

`state/transitions.go` должен иметь единую таблицу допустимых transitions.

Запрещены scattered присваивания:

```go
node.Status = "running"
```

Вместо этого:

```go
state.TransitionNode(current, target)
```

## Тесты Stage 0

### Unit

- каждый допустимый transition;
- каждый запрещённый transition;
- terminal Attempt не возвращается в RUNNING;
- retry не мутирует terminal Attempt;
- terminal WorkflowRun нельзя silently reopen;
- NodeSpec runtime-free.

### Property tests

Генерировать случайные последовательности transitions и проверять invariants.

### DAG validation

- duplicate NodeID;
- missing dependency;
- self-cycle;
- multi-node cycle;
- unreachable nodes;
- invalid ExecutorKind;
- invalid resource values.

### Benchmarks

```text
BenchmarkCompileLinear1K
BenchmarkCompileLinear10K
BenchmarkCompileDAG100K
```

## Definition of Done

- новая модель компилируется без runtime persistence;
- planner может быть адаптирован в WorkflowDefinition;
- legacy execution не сломан;
- все state transitions централизованы;
- отсутствует бизнес-логика, завязанная на строковые status literals в новом Harness package.

---

# 5. Stage 1 — Store abstraction и SQLite WAL

## Цель

Создать durable transactional source of truth.

## Где делать

Создать:

```text
internal/harness/store/
internal/harness/store/sqlite/
internal/harness/events/
```

Изменить:

```text
internal/paths/
cmd/agctl/
```

## 5.1 Driver selection

Сохранить single-binary/cross-platform характер проекта.

До фиксации driver провести мини-ADR:

```text
modernc.org/sqlite
vs
ncruces/go-sqlite3
vs
CGO driver
```

Критерии:

- Windows support;
- no external DLL;
- no mandatory CGO;
- WAL correctness;
- busy handler;
- transaction semantics;
- backup support;
- binary size;
- benchmark write latency.

Предпочтение V1: pure-Go/no-external-runtime вариант, если benchmarks подтверждают достаточную стабильность.

## 5.2 DB location

В `paths.Paths` добавить:

```text
HarnessDB
ArtifactRoot
HarnessBackupRoot
```

Например:

```text
<agctl-state>/harness/state.db
<agctl-state>/harness/artifacts/
```

## 5.3 SQLite pragmas

Инициализация должна явно задавать минимум:

```text
journal_mode=WAL
foreign_keys=ON
busy_timeout
synchronous=NORMAL или FULL — выбрать ADR/benchmark
```

Не полагаться на defaults.

## 5.4 Migrations

Создать versioned migrations.

Минимум таблиц первой версии:

```text
schema_migrations
workflow_definitions
workflow_runs
graph_revisions
nodes
dependencies
node_runs
attempts
events
outbox
```

Каждая migration:

- immutable после release;
- transactional где SQLite позволяет;
- имеет forward test;
- имеет fresh-install test;
- имеет upgrade-from-previous fixture.

## 5.5 Store interface

Не делать repository interface на каждую таблицу.

Сделать transaction-oriented interface:

```text
Store.BeginTx(ctx)
Store.View(ctx, fn)
Store.Update(ctx, fn)
```

и typed operations внутри `Tx`.

Главная цель — атомарность нескольких связанных изменений.

## 5.6 Event journal

Каждый authoritative transition должен в той же transaction:

```text
UPDATE current state
INSERT event
INSERT outbox (если требуется внешняя доставка)
```

Event содержит:

```text
event_id
workflow_run_id
workflow_seq
type
timestamp
entity_type
entity_id
payload_version
payload
```

`workflow_seq` монотонен внутри одного WorkflowRun.

## Тесты Stage 1

### Integration

- fresh DB creation;
- restart/open existing DB;
- concurrent readers;
- busy writer;
- transaction rollback;
- foreign key violation;
- migration replay prevention.

### Crash tests

Отдельный helper subprocess:

1. начинает transition;
2. process убивается в заранее определённой fault point;
3. новый process открывает DB;
4. проверяется atomicity.

Fault points:

```text
before UPDATE
between UPDATE and event
between event and COMMIT
after COMMIT before response
```

### Benchmarks

```text
BenchmarkStateTransition
BenchmarkEventAppend
BenchmarkReadWorkflow1KNodes
BenchmarkReadWorkflow100KNodes
```

## Definition of Done

- committed state переживает process kill;
- partial transition не виден;
- event и state не расходятся;
- DB автоматически создаётся/мигрирует;
- legacy JSON execution всё ещё доступен compatibility mode.

---

# 6. Stage 2 — Legacy import и dual-read compatibility

## Цель

Обеспечить безопасный переход пользователей с `3.2.x`.

## Где делать

Создать:

```text
internal/harness/migratelegacy/
```

Изменить:

```text
internal/planner/
internal/tasks/
cmd/agctl/
```

## 6.1 Import existing plans/tasks

Импортировать:

```text
ExecutionPlan → WorkflowDefinition + WorkflowRun
PlanRevision → GraphRevision history
TaskRecord → NodeRun + synthetic Attempts where possible
```

## 6.2 Нельзя выдумывать историю

Если legacy TaskRecord содержит:

```text
Attempts=3
Status=succeeded
```

но нет истории попыток, не создавать три fake Attempts с выдуманными timestamps.

Создать migration marker:

```text
legacy_attempt_summary
```

или один synthetic imported Attempt с metadata:

```text
history_incomplete=true
legacy_attempt_count=3
```

## 6.3 Legacy running tasks

Если при импорте:

```text
status=running
```

не считать его RUNNING в новом runtime автоматически.

Переводить в:

```text
IN_DOUBT / LEGACY_RECOVERY_REQUIRED
```

с diagnostics.

## 6.4 CLI

Добавить:

```text
agctl harness migrate
agctl harness migrate --dry-run
agctl harness status
```

## Тесты

- fixtures от 3.1/3.2;
- duplicate import idempotency;
- interrupted migration;
- invalid JSON;
- partially missing task dependency;
- legacy running task.

## DoD

Migration можно запускать повторно без дубликатов и потери legacy state.

---

# 7. Stage 3 — Transactional Workflow Engine

## Цель

Сделать `engine` единственным authority для runtime state transitions.

## Где делать

Создать:

```text
internal/harness/engine/
```

Перевести постепенно:

```text
planner.Save
replan mutations
tasks status updates
```

## 7.1 StartWorkflow

Одна transaction:

```text
create WorkflowRun
create GraphRevision #1
create NodeRuns
calculate remaining_dependencies
mark roots READY
append WorkflowStarted/NodeReady events
COMMIT
```

## 7.2 Completion transition

При success NodeRun:

```text
Attempt SUCCEEDED
NodeRun SUCCEEDED
for each outgoing dependency:
    decrement remaining_dependencies
    if reaches 0:
        child READY
append events
COMMIT
```

## 7.3 Failure

Не блокировать downstream произвольными scattered updates.

Engine должен вычислить policy:

```text
retry?
replan?
fail branch?
block dependent?
compensate?
```

## 7.4 Idempotent command handling

Engine commands должны иметь `command_id`/dedupe там, где возможен повтор API call.

## Тесты

### Unit

- StartWorkflow;
- root readiness;
- completion propagation;
- fan-out;
- fan-in;
- terminal workflow detection;
- failed dependency semantics.

### Property

- node никогда не READY раньше dependencies;
- counter никогда < 0;
- duplicate completion command не decrement дважды.

### Fault

kill между parent success и child readiness — после restart данные должны быть согласованы, потому что это одна transaction.

## DoD

Все новые state transitions проходят только через engine.

---

# 8. Stage 4 — Incremental DAG Scheduler

## Цель

Удалить полный `List()` + full dependency scan на каждой scheduling wave.

## Где делать

Создать:

```text
internal/harness/scheduler/
internal/harness/resource/
```

`internal/tasks.RunPendingObserved` оставить compatibility wrapper, но перестать использовать как core.

## 8.1 Dependency counters

Для каждого NodeRun хранить:

```text
remaining_required_dependencies
```

Обновление только по outgoing edges parent.

Общая стоимость propagation:

```text
O(V + E)
```

за run, а не за каждый цикл scheduler.

## 8.2 Durable READY representation

READY должен быть durable state + индексируемые поля:

```text
workflow_run_id
node_run_id
priority
ready_at
not_before
resource_class
```

Не хранить единственный source of truth только в heap RAM.

Heap — cache над DB.

## 8.3 Fairness

Не использовать одну global priority queue.

Реализовать two-level selection:

```text
workflow lane selection
    ↓
node selection inside lane
```

V1 алгоритм: weighted deficit round-robin или equivalent deterministic fair queue.

Поля:

```text
workflow_weight
workflow_deficit
node_priority
age
```

## 8.4 Priority inheritance

Если high-priority node зависит от low-priority unfinished ancestor, вычислять `effective_priority` для blocking chain.

Начать с bounded propagation, не делать сложный general priority inheritance scheduler без benchmark justification.

## 8.5 ExplainNode

Scheduler обязан сохранять/вычислять причину ожидания:

```text
DEPENDENCY
NOT_BEFORE
RESOURCE
CAPABILITY
WORKSPACE_LOCK
CONCURRENCY_LIMIT
RATE_LIMIT
POLICY_APPROVAL
TENANT_LIMIT
NO_ELIGIBLE_WORKER
FAIRNESS
```

CLI:

```text
agctl task explain <id>
```

UI позже использует тот же query API.

## Тесты

### Scale

```text
10k linear
10k random DAG
100k independent
100k fan-out
100k fan-in
```

### Fairness

Workflow A: 100000 READY.  
Workflow B: 3 READY.

Проверить, что B получает service в bounded number scheduling decisions.

### Resource

Hard constraint никогда не нарушается.

Удалить legacy fallback поведения «если batch пуст, запустить первый task всё равно» в новом scheduler.

Если task требует больше доступного ресурса:

```text
UNSCHEDULABLE / WAITING_RESOURCE
```

## Benchmarks

Измерять:

```text
parent commit → child READY
READY → scheduler decision
scheduler decisions/sec
RSS at 100k nodes
```

## DoD

Scheduler больше не перечитывает все task JSON и не пересчитывает весь DAG после каждого completion.

---

# 9. Stage 5 — Attempt model, durable leases и fencing

## Цель

Отделить logical NodeRun от физической попытки execution.

## Где делать

Создать:

```text
internal/harness/lease/
internal/harness/worker/
```

Изменить/адаптировать:

```text
internal/tasks/
```

## 9.1 Attempt creation

Scheduler не переводит NodeRun напрямую RUNNING.

Порядок:

```text
NodeRun READY
↓
create Attempt
↓
Attempt CLAIMED
↓
lease created
↓
NodeRun RUNNING
COMMIT
↓
physical execution
```

## 9.2 Lease schema

```text
lease_id
attempt_id
worker_id
epoch
claimed_at
heartbeat_at
expires_at
state
```

## 9.3 Fencing

Каждый reclaim увеличивает epoch.

Result acceptance:

```text
attempt lease epoch == current epoch
```

Старый worker может закончить computation, но его stale completion не может перезаписать authoritative state.

## 9.4 Heartbeats

Heartbeat не должен создавать full event на каждый tick.

Обновлять compact lease row.

Event писать только на значимые изменения:

```text
lease claimed
lease lost
lease reclaimed
```

## 9.5 Replace `.run.lock`

После переходного периода удалить `.run.lock` как correctness primitive.

Можно временно сохранять compatibility guard для legacy runner, но новый Harness не должен зависеть от lock file age.

## Тесты

- worker dies before execution;
- worker dies during execution;
- heartbeat delayed;
- lease expires;
- second worker reclaims;
- first worker returns stale success;
- duplicate completion;
- clock skew simulation — expiration decisions должны основываться на coordinator/store time where possible.

## Property invariants

- один current lease epoch на Attempt;
- stale epoch не commit-ит result;
- terminal Attempt immutable.

## DoD

Ownership определяется durable lease, а не PID/file lock.

---

# 10. Stage 6 — Process Executor и корректная cancellation

## Цель

Вынести запуск AGY/shell/process из scheduler.

## Где делать

Создать:

```text
internal/harness/executor/
internal/harness/executor/process/
```

Переиспользовать stream parsing из `internal/tasks/tasks.go`, но перенести ответственность.

## 10.1 Executor interface

Минимально:

```text
Prepare
Execute
Cancel
Reconcile
Capabilities
```

Не делать interface огромным.

## 10.2 Process tree

Windows:

- Job Object;
- kill tree;
- graceful termination where possible.

Unix:

- process group;
- TERM;
- grace period;
- KILL tree.

## 10.3 stdout/stderr

- bounded in-memory buffers;
- streaming to log artifact;
- scanner limits не должны silently truncate without metadata;
- backpressure;
- structured parsing AGY отдельно от raw log persistence.

## 10.4 Timeout classes

Ввести отдельно:

```text
start_timeout
execution_timeout
idle_timeout
heartbeat_timeout
```

Не использовать единый `MaxTaskMinutes` как universal semantics.

## 10.5 Cancellation

Поддержать:

```text
SOFT_CANCEL
GRACEFUL_TERMINATE
HARD_KILL
```

State transitions должны отражать, что cancel был requested до фактической остановки процесса.

## Тесты

- child process spawns grandchild;
- cancel kills full tree;
- stdout > 500 MB synthetic stream;
- stderr flood;
- malformed stream-json;
- exit 0 without terminal result;
- AGY permission soft denial;
- timeout;
- executor crash.

## DoD

Scheduler не знает PID/process details.

---

# 11. Stage 7 — Retry Engine, error taxonomy, circuit breaker

## Цель

Сделать retry самостоятельной runtime policy.

## Где делать

Создать:

```text
internal/harness/retry/
```

## 11.1 Error taxonomy

Минимум:

```text
APPLICATION_PERMANENT
APPLICATION_TRANSIENT
INFRA_TRANSIENT
RATE_LIMITED
TIMEOUT
CANCELLED
POLICY_DENIED
UNSCHEDULABLE
PROTOCOL_ERROR
UNKNOWN_EFFECT
```

Executor возвращает typed failure classification + details.

## 11.2 RetryPolicy

```text
max_attempts
max_elapsed_time
initial_delay
backoff_factor
max_delay
jitter
retryable_classes
non_retryable_classes
```

## 11.3 Retry creates new Attempt

Никогда:

```text
FAILED Attempt → RUNNING
```

Всегда:

```text
Attempt #1 FAILED
NodeRun RETRY_WAIT
Timer/NotBefore
Attempt #2 CREATED
```

## 11.4 Retry budgets

Добавить budgets:

```text
per node
per workflow
per provider/service
```

## 11.5 Circuit breaker

Key:

```text
provider/service endpoint
```

State:

```text
CLOSED
OPEN
HALF_OPEN
```

Open breaker должен давать scheduler explain reason.

## Тесты

- deterministic jitter with injectable RNG;
- Retry-After;
- retry storm 10k nodes;
- circuit breaker open/half-open;
- non-retryable failure;
- infra failure separate from semantic repair budget.

## DoD

Retry policy не встроена в goroutine loop и не мутирует старый Attempt.

---

# 12. Stage 8 — Durable Timers, Signals, Pause/Resume, Human Approval

## Цель

Поддержать workflows, живущие часы/дни без удержания goroutines/workers.

## Где делать

Создать:

```text
internal/harness/timer/
internal/harness/signal/
internal/harness/approval/
```

## 12.1 Timers

DB table:

```text
timer_id
workflow_run_id
node_run_id
due_at
kind
payload
state
```

В RAM только ближайший bounded heap.

Restart перестраивает heap из DB.

## 12.2 Signals

Signal persisted **до** доставки waiter.

Это позволяет:

```text
signal arrived before wait node activated
```

не потерять событие.

Добавить dedupe key/message ID.

## 12.3 Pause

Semantics:

```text
PAUSING: no new Attempts
running Attempts drain/checkpoint according policy
PAUSED: scheduler stops new work
```

## 12.4 Human approval

Approval — специализированный durable signal/wait, а не отдельный execution subsystem.

Schema:

```text
approval_id
workflow_run_id
node_run_id
requested_capability
risk
reason
requested_at
expires_at
state
actor
resolved_at
```

## CLI

```text
agctl workflow pause <id>
agctl workflow resume <id>
agctl workflow cancel <id>
agctl approvals list
agctl approvals approve <id>
agctl approvals reject <id>
```

## Тесты

- restart while waiting 24h;
- signal before waiter;
- duplicate signal;
- approval expiration;
- pause while 10 tasks running;
- resume same WorkflowRun;
- cancel paused workflow.

## DoD

Wait 24 часа не занимает worker и не требует живой goroutine.

---

# 13. Stage 9 — Side Effects, Idempotency и `IN_DOUBT`

## Цель

Решить главный crash window external operations.

## Где делать

Создать/расширить:

```text
internal/harness/engine/reconcile.go
internal/harness/model/effect.go
internal/harness/executor/*
```

## 13.1 Effect classification

Каждый side-effectful executor operation должен объявить класс:

```text
PURE
IDEMPOTENT
IDEMPOTENT_WITH_KEY
QUERYABLE
COMPENSATABLE
NON_IDEMPOTENT_UNKNOWN
```

## 13.2 Stable idempotency key

Строить из logical operation identity, не AttemptID:

```text
workflow_run_id
node_run_id
operation namespace
semantic input hash
```

Retries используют тот же key.

## 13.3 Effect intent

До выполнения side effect durable record:

```text
effect_intent_id
node_run_id
attempt_id
operation
idempotency_key
state=PREPARED
```

Затем execution.

После ответа:

```text
CONFIRMED
FAILED
IN_DOUBT
```

## 13.4 Unknown completion

Если соединение потеряно после отправки запроса:

- не классифицировать автоматически как обычный FAILED;
- перейти в `IN_DOUBT`;
- вызвать executor-specific `Reconcile()`.

## 13.5 GitHub/Git/MCP adapters

Для каждого важного provider определить reconciliation contract.

Примеры:

```text
Git commit: query by commit/tree hash
GitHub create operation: query by stable marker/idempotency capability
MCP unknown side effect: server/tool-specific policy
filesystem: content hash / atomic path semantics
```

## Тесты

Инъекция crash:

```text
external effect succeeded
↓
kill before DB success commit
```

Проверить:

- idempotent operation не дублируется семантически;
- queryable operation reconciles;
- unknown operation остаётся IN_DOUBT и не повторяется blindly.

## DoD

Harness не обещает fake exactly-once, но явно различает safe retry и uncertain side effect.

---

# 14. Stage 10 — Artifact Store, CAS и provenance

## Цель

Убрать большие outputs из scheduler/runtime state.

## Где делать

Создать:

```text
internal/harness/artifact/
internal/harness/artifact/cas/
internal/harness/artifact/provenance/
```

## 14.1 Artifact metadata

```text
artifact_id
content_digest
type
uri/path
size
producer_node_run_id
producer_attempt_id
created_at
metadata
```

`ArtifactID` не равен digest: одинаковый content может иметь разную provenance.

## 14.2 CAS layout

```text
artifacts/sha256/ab/abcdef...
```

Write protocol:

```text
temp file
↓
write
↓
fsync
↓
hash verify
↓
atomic rename
↓
DB metadata commit
```

## 14.3 Logs как artifacts

Large stdout/stderr становится artifact.

Node/Attempt хранит только reference + tail summary.

## 14.4 Provenance edges

Хранить:

```text
artifact used_by node
artifact produced_by node
model/tool/executor version
input artifact hashes
prompt/template hash
```

## 14.5 GC

Mark-and-sweep от reachable WorkflowRun/artifact references.

Не удалять orphan немедленно после crash; grace period.

## Тесты

- partial write crash;
- corrupted hash;
- same bytes two artifacts;
- orphan GC;
- 1 GB artifact synthetic test;
- interrupted rename;
- disk full.

## DoD

Scheduler metadata size не зависит линейно от artifact payload size.

---

# 15. Stage 11 — Cache и partial recomputation

## Цель

Добавить идеи Bazel/Nix/DVC без нарушения side-effect semantics.

## Где делать

Создать:

```text
internal/harness/artifact/cache/
```

## 15.1 CachePolicy

```text
DISABLED
RUN_LOCAL
GLOBAL_CONTENT
```

## 15.2 Cache eligibility

Executor/Node объявляет:

```text
DETERMINISTIC
NONDETERMINISTIC
SIDE_EFFECTFUL
```

Default:

- deterministic validator/build → cache allowed;
- LLM → checkpoint, но global memoization disabled;
- side effect → cache disabled.

## 15.3 Cache key

```text
node implementation/version
executor version
canonical inputs
artifact digests
environment fingerprint
tool/model/prompt versions
```

## 15.4 Rerun/invalidation

Rerun B:

```text
A → B → C → D
```

не должен сразу стирать C/D.

Создать new generation/result lineage.

Если B output digest изменился → downstream stale.

Если digest идентичен → downstream может оставаться valid.

## Тесты

- deterministic hit;
- changed tool version miss;
- changed environment miss;
- same artifact content preserves downstream;
- side-effect node never global-cache hit.

## DoD

Partial recomputation основан на explicit content/version identity.

---

# 16. Stage 12 — Adaptive Replanner поверх GraphRevision transaction

## Цель

Сохранить сильный `3.2` adaptive replanning, но убрать dual-write.

## Где делать

Изменить:

```text
internal/replan/replan.go
internal/planner/planner.go
```

Использовать:

```text
internal/harness/engine/mutation.go
```

## 16.1 Новый Replan output

`replan` больше не вызывает `tasks.AddAdvanced()` и `tasks.ReplaceDependency()`.

Он строит:

```text
GraphMutation {
  expected_revision
  trigger_node_run_id
  reason
  evidence
  add_nodes
  replace_dependencies
  supersede_relations
}
```

## 16.2 Optimistic concurrency

Mutation применяется только если:

```text
expected_revision == current_revision
```

Иначе replanner reload/re-evaluate.

## 16.3 Atomic mutation

Одна transaction должна:

- increment revision;
- вставить nodes;
- вставить dependencies;
- создать NodeRuns;
- рассчитать counters;
- rewire только допустимые non-running nodes;
- записать superseded relation;
- append GraphRevision event.

## 16.4 Preserve current safeguards

Сохранить:

```text
max revisions
max dynamic nodes
max repair depth
max same failure
confidence gate
evidence gate
risk gate
no-progress signatures
```

## 16.5 Running downstream

Никогда не mutate execution semantics уже RUNNING/terminal NodeRun.

Если требуется correction — создавать explicit new branch/generation.

## Тесты

- two simultaneous proposals;
- revision conflict;
- concurrent node completions;
- cycle attempt;
- budget exceeded;
- recovery branch;
- no-progress stop;
- crash inside mutation transaction.

## DoD

Plan JSON и Task JSON больше не должны обновляться как две authoritative копии одного graph change.

---

# 17. Stage 13 — Workspace lifecycle и Git worktrees

## Цель

Сделать existing worktree strategy first-class execution resource.

## Где делать

Создать:

```text
internal/harness/workspace/
```

Адаптировать:

```text
internal/worktree/
internal/replan/
```

## 17.1 Workspace identity

```text
workspace_id
kind
base_path
repository_id
branch
owner_workflow_run_id
state
```

Kinds:

```text
PERSISTENT
SHARED_READ
EXCLUSIVE
EPHEMERAL
GIT_WORKTREE
```

## 17.2 Workspace lease

Write executor должен получить exclusive workspace lease или isolated worktree.

## 17.3 Worktree lifecycle

```text
reserve branch
create worktree
execute
validate local commit
publish artifact/commit hash
integration node
cleanup after retention period
```

## 17.4 Crash recovery

После restart:

- scan durable workspace records;
- проверить worktree existence;
- проверить expected branch/HEAD;
- orphan worktree не удалять до reconciliation.

## 17.5 Integration

Integration node должен быть explicit node с validators, а не side effect replanner-а.

## Тесты

- two agents same repo;
- two independent worktrees;
- conflicting commits;
- crash during worktree creation;
- crash after commit before state commit;
- manual repo modification;
- abandoned worktree GC.

## DoD

Concurrent write agents не работают по одной mutable рабочей копии без explicit exclusive policy.

---

# 18. Stage 14 — Agent Executor и durable agent loop

## Цель

Сделать agent supervised executor, а не implicit orchestrator.

## Где делать

Создать:

```text
internal/harness/executor/agent/
internal/harness/budget/
```

Адаптировать:

```text
internal/agents/
internal/goal/
internal/hooks/
```

## 18.1 AgentRun identity

Отдельно от Attempt:

```text
agent_run_id
attempt_id
agent_id
model_policy
step_count
budget_state
checkpoint
```

## 18.2 Hard budgets

Runtime enforced:

```text
max_steps
wall_clock_deadline
max_model_calls
max_tool_calls
max_tokens
max_cost
failure_budget
```

LLM prompt не может увеличить эти пределы.

## 18.3 Checkpoint boundaries

Минимум после:

```text
model response
tool proposal
policy decision
tool result
important artifact creation
```

## 18.4 Agent state separation

Не складывать всё в transcript.

Разделить:

```text
conversation
working memory
structured state
artifact refs
budget
model/tool history
```

## 18.5 Success

Agent message `done` не terminal criterion.

Node success определяется external validator/gate policy.

## Тесты

- infinite plan loop;
- model repeatedly returns same action;
- max token budget;
- tool budget;
- model timeout;
- checkpoint + process kill + resume;
- agent says success while tests fail.

## DoD

Agent не может удерживать root control loop бесконечно и не может объявить workflow completed без Harness validators.

---

# 19. Stage 15 — MCP Runtime Adapter и Tool Registry

## Цель

Поднять существующий MCP probe/discovery код до production execution runtime.

## Где делать

Создать:

```text
internal/harness/executor/mcp/
```

Рефакторить совместно:

```text
internal/mcpprobe/
internal/mcp/
internal/capability/
```

## 19.1 Protocol abstraction

Не дублировать MCP implementation между probe и executor.

Вынести:

```text
modern 2026-07-28 metadata/discover
legacy 2025-11-25 initialize
transport helpers
response validation
```

## 19.2 Tool Registry

Normalized record:

```text
tool_id
server_id
protocol_version
capabilities
input_schema_hash
output_schema_hash
permissions
risk
availability
location
last_discovered_at
TTL
```

## 19.3 Runtime lifecycle

Local stdio:

```text
spawn
health/discover
execute
monitor
restart with backoff
shutdown
```

Remote stateless:

```text
discover/cache
request
validate result
```

## 19.4 MCP failure semantics

Различать:

```text
transport failure before send
transport failure after send
protocol malformed result
tool-level error
policy denial
server unavailable
tool disappeared/schema changed
```

После uncertain side effect использовать `IN_DOUBT`, а не blind retry.

## 19.5 Capability scheduling

Scheduler должен матчить:

```text
required capability
↓
eligible worker/tool/provider
```

## Тесты

- modern MCP server;
- legacy fallback;
- server/discover unavailable;
- stdio server crash;
- malformed JSON-RPC;
- tool disappears after discovery;
- schema hash changed;
- timeout after request send;
- duplicate tool result;
- permission gate.

## DoD

MCP является Executor/ToolProvider над Harness core, а не отдельным параллельным orchestration path.

---

# 20. Stage 16 — Policy Engine и approvals

## Цель

Свести semantic risk, capabilities и approvals в единый pre-execution policy path.

## Где делать

Создать:

```text
internal/harness/policy/
```

Адаптировать:

```text
internal/securityaudit/
existing semantic risk code
internal/capability/
```

## 20.1 Capability namespace

Минимум:

```text
filesystem.read
filesystem.write
process.execute
network.external
github.read
github.write
github.merge
email.send
deployment.staging
deployment.production
secret.use:<name>
```

## 20.2 Policy decision

```text
ALLOW
DENY
REQUIRE_APPROVAL
```

Decision durable и auditable.

## 20.3 SecretRef

Workflow state содержит только reference:

```text
secret://github/default
```

Secret value materialize только внутри executor boundary.

## 20.4 Worker trust

```text
TRUSTED_LOCAL
TRUSTED_REMOTE
UNTRUSTED_REMOTE
```

Policy учитывает trust class при scheduler matching.

## Тесты

- denied write;
- approval-required git push;
- secret never appears in event/log serialization;
- untrusted worker cannot receive secret-bound task;
- policy changes while node waiting.

## DoD

Любая destructive capability проходит один policy interface.

---

# 21. Stage 17 — Observability и Explainability

## Цель

Любой execution должен быть диагностируемым без чтения внутренних JSON/SQLite вручную.

## Где делать

Создать:

```text
internal/harness/telemetry/
```

Адаптировать:

```text
internal/telemetry/
internal/dashboard/
internal/doctor/
```

## 21.1 Trace hierarchy

```text
WorkflowRun span
 ├ NodeRun span
 │  └ Attempt span
 │      ├ AgentRun
 │      │   ├ ModelCall
 │      │   └ ToolCall
 │      └ Process/MCP
 └ Subworkflow
```

## 21.2 Log correlation

Каждый structured log:

```text
workflow_run_id
node_run_id
attempt_id
worker_id
trace_id
span_id
```

## 21.3 Metrics

Минимум:

```text
active_workflows
ready_queue_depth
queue_latency
node_duration
attempt_duration
state_transitions_total
worker_utilization
lease_losses
retry_rate
failure_rate
timeout_rate
rate_limit_wait
artifact_bytes
llm_latency
llm_tokens
llm_cost
tool_latency
sqlite_commit_latency
sqlite_wal_size
```

## 21.4 Explain queries

Doctor/dashboard должны уметь отвечать:

```text
Why not running?
What is it waiting for?
Who owns the attempt?
When lease expires?
Why retry scheduled?
Why workflow blocked?
What changed in graph revision?
```

## Тесты

Observability failure не должна ломать semantic result.

Например OTel exporter down → workflow продолжает выполнение, durable local event остаётся.

## DoD

Для любого non-terminal node есть machine-readable wait reason и human-readable explanation.

---

# 22. Stage 18 — REST/SSE API и Dashboard migration

## Цель

Отделить commands/queries от internal package calls и подготовить remote control without premature microservices.

## Где делать

Создать:

```text
internal/harness/api/
```

Изменить:

```text
internal/dashboard/
cmd/agctl/
```

## 22.1 Command API

```text
CreateWorkflow
StartWorkflow
PauseWorkflow
ResumeWorkflow
CancelWorkflow
RetryNode
RerunNode
RerunFrom
SkipNode
SendSignal
ApproveAction
RejectAction
```

## 22.2 Query API

```text
GetWorkflow
GetNode
GetAttempt
ExplainNode
ListEvents
ListArtifacts
GetProvenance
ListWorkers
GetResources
```

## 22.3 Live updates

V1:

```text
REST = commands/queries
SSE = events/progress/log metadata
```

WebSocket только если появится настоящий bidirectional terminal/session use case.

## 22.4 Dashboard screens

Добавить:

```text
Workflow hierarchy/DAG
Timeline
Node inspector
Attempt history
Artifact inspector
Approval inbox
Worker/resources
Scheduler explanation
Graph revision diff
```

## Тесты

- SSE reconnect by cursor;
- duplicate client command with command ID;
- cancelled HTTP request does not cancel workflow unless explicit command;
- large event history pagination.

## DoD

Dashboard больше не зависит от чтения ad-hoc task JSON как primary state API.

---

# 23. Stage 19 — Remote Workers (V2)

## Цель

Добавить distributed execution без изменения Workflow API.

## Где делать

Создать:

```text
internal/harness/worker/remote/
internal/harness/transport/worker/
cmd/agworker/   # только если отдельный binary действительно оправдан
```

Если требуется сохранить один distribution binary, поддержать режим:

```text
agctl worker --connect ...
```

## 23.1 Protocol

Предпочтительно mTLS gRPC.

Worker:

```text
register
advertise capabilities/resources/trust
pull attempt
receive lease epoch
heartbeat
execute
upload artifact
commit result
```

## 23.2 Не давать worker DB access

Remote worker общается только с control plane protocol.

SQLite остаётся local authority.

## 23.3 Backpressure

Worker pull model.

Coordinator не push-ит бесконечное число tasks.

## 23.4 Network partition

Обязательные tests:

```text
partition after claim
partition after side effect
partition during artifact upload
old worker reconnect after reclaim
```

## DoD

Remote workers работают при single authoritative control plane и не требуют Postgres/Kafka/Kubernetes.

---

# 24. Stage 20 — Postgres и HA Control Plane (V3)

## Цель

Добавить multiple control-plane instances только когда V2 metrics покажут необходимость.

## Где делать

Реализовать заранее предусмотренный:

```text
internal/harness/store/postgres/
```

## 24.1 Coordination

Использовать:

```text
transactions
row locking
SKIP LOCKED where appropriate
leases/fencing
```

Не писать собственный Raft.

## 24.2 Shared artifacts

Filesystem CAS заменяется/расширяется object-store backend.

Artifact interface должен позволять это без изменения workflow semantics.

## 24.3 HA tests

- kill active scheduler instance;
- two schedulers race for READY work;
- DB failover;
- stale controller resumes;
- concurrent graph mutation.

## DoD

User WorkflowDefinition и API не меняются между V1/V2/V3.

---

# 25. CLI migration plan

Сохранить текущие команды столько, сколько возможно.

## Transitional mapping

```text
agctl plan create
→ compile WorkflowDefinition

agctl plan run
→ Create/Start WorkflowRun

agctl tasks run-pending
→ harness scheduler drain/run

agctl tasks retry
→ RetryNode/new Attempt

agctl replan apply
→ validate proposal + ApplyGraphMutation
```

## Новые команды

```text
agctl harness doctor
agctl harness migrate
agctl harness status

agctl workflow list
agctl workflow show <run>
agctl workflow pause <run>
agctl workflow resume <run>
agctl workflow cancel <run>
agctl workflow events <run>

agctl node show <node-run>
agctl node explain <node-run>
agctl node retry <node-run>
agctl node rerun <node-run>

agctl attempts list <node-run>

agctl approvals list
agctl artifacts list <run>
agctl workers list
```

Не удалять `plan/tasks/replan` команды, пока migration documentation не готова.

---

# 26. Testing pyramid

Каждый stage обязан добавлять не только unit tests.

## 26.1 Unit

- state transitions;
- retry policy;
- scheduler selection;
- DAG compiler;
- policy decisions;
- cache keys.

## 26.2 Integration

- SQLite;
- process executor;
- MCP;
- workspace/worktree;
- artifact CAS;
- API.

## 26.3 Property-based

Проверять invariants на случайных DAG/state sequences.

Минимум properties:

1. node не запускается до required dependencies;
2. terminal Attempt immutable;
3. retry создаёт новый Attempt;
4. dependency counter >= 0;
5. duplicate completion idempotent;
6. graph mutation сохраняет acyclicity;
7. stale lease epoch не commit-ит result;
8. duplicate signal не выполняет transition дважды.

## 26.4 Fuzzing

Цели:

```text
Workflow IR decoder
GraphMutation decoder
MCP responses
AGY stream-json parser
event payload decoder
migration importer
```

## 26.5 Fault injection

Создать reusable failpoint package только для tests/build tags.

Fault points:

```text
before DB transaction
before commit
after commit
before external request
after request send
before result persistence
after artifact write
before artifact metadata
before lease renew
```

Не оставлять production global random kill hooks.

## 26.6 Race testing

`go test -race ./...` — обязательный release gate.

Особенно packages:

```text
scheduler
lease
worker
replan
executor
SSE
```

---

# 27. Benchmark suite

Создать отдельный package/command, чтобы benchmarks были воспроизводимыми.

## Workloads

```text
linear_1k
linear_10k
random_dag_10k
independent_100k
fanout_100k
fanin_100k
map_1m_metadata
10k_timers
10k_workflows
```

## Метрики

```text
compile duration
state transition latency p50/p95/p99
parent-complete → child-ready
ready → claimed
result → durable commit
commit → SSE visible
scheduler decisions/sec
SQLite writes/sec
RSS
bytes metadata/node
WAL growth
```

## Performance acceptance

Не фиксировать произвольные маркетинговые numbers до baseline.

Сначала записать baseline текущей реализации, затем на каждом stage проверять regression.

Главный обязательный algorithmic target:

```text
dependency propagation total O(V + E)
```

а не repeated whole-graph scans.

---

# 28. Failure acceptance matrix

Перед объявлением Harness production-ready обязательно автоматизировать:

| Failure | Expected result |
|---|---|
| orchestrator kill | committed state survives, runtime reconstructs |
| worker kill | lease expires, Attempt LOST/retry policy |
| DB temporarily unavailable | fail closed, no new claims |
| stale worker result | rejected by fencing |
| network partition | explicit lost/uncertain handling |
| external timeout after send | IN_DOUBT/reconcile |
| duplicate command | no duplicate semantic transition |
| duplicate signal | deduped |
| disk full | no false SUCCEEDED |
| corrupted artifact | digest failure |
| process tree cancel | children terminated |
| MCP schema changes | cache invalidated/revalidated |
| rate limit | backoff/throttle, no retry storm |
| LLM infinite loop | budget stop |
| two agents same repo | isolation/workspace lease |
| graph mutation race | expected_revision conflict |

---

# 29. Критические invariants — превратить в executable tests

Минимум:

1. WorkflowDefinition immutable после старта run.
2. Runtime mutation создаёт новую GraphRevision.
3. Required dependency должна завершиться до READY child.
4. Scheduler не dispatch-ит PENDING_DEPENDENCIES.
5. Physical execution начинается только после durable claim.
6. Один Attempt имеет максимум один terminal outcome.
7. Terminal Attempt не возвращается RUNNING.
8. Retry создаёт новый Attempt.
9. SUCCEEDED публикуется только после durable commit.
10. Terminal WorkflowRun не reopen-ится implicit.
11. Cancellation распространяется по hierarchy согласно policy.
12. Lease epoch monotonic.
13. Stale epoch не commit-ит result.
14. Потеря RAM не влияет на correctness.
15. Durable timer переживает restart.
16. Signal не теряется до появления waiter.
17. Duplicate signal безопасен.
18. Duplicate event delivery безопасна.
19. workflow sequence monotonic.
20. State + audit event commit atomic.
21. Dependency counter никогда не отрицателен.
22. Hard resource limit не нарушается scheduler.
23. Secret не сериализуется в normal state/event.
24. Artifact доступен только после verified commit.
25. Scheduler не держит heavy payload.
26. Agent budget enforced outside LLM.
27. Autonomous loops bounded.
28. Unknown non-idempotent effect не retry blindly.
29. Human wait не удерживает worker.
30. Graph mutation не создаёт cycle.
31. Running/terminal node не rewired silently.
32. Tool/model/executor versions входят в provenance.
33. Telemetry failure не меняет semantic result.
34. Durable storage failure → fail closed.
35. Concurrent writes требуют isolation/lease.
36. Каждый WAIT имеет explainable reason.
37. RUNNING workflow без active/ready/waiting work → liveness violation.
38. Large output становится Artifact.
39. Reconciliation state explicit.
40. Legacy migration не выдумывает missing history.

---

# 30. Release gates по версиям

## Harness 4.0-alpha

Должны быть готовы:

```text
new model/IR
SQLite store
engine
incremental scheduler
Attempts
leases
process executor
```

Legacy adapter включён.

## Harness 4.0-beta

Добавить:

```text
timers/signals
retry engine
IN_DOUBT
artifacts
GraphRevision replanner migration
workspace lifecycle
```

Начать long-running fault tests.

## Harness 4.0-rc

Добавить:

```text
agent executor
MCP executor
policy/approval
observability
new dashboard APIs
```

Обязательные soak tests.

## Harness 4.0 stable

Release gate:

```text
go test ./...
go test -race ./...
go vet ./...
all migration fixtures
all crash consistency scenarios
all critical invariants
10k/100k scheduler benchmarks
24h wait restart test
side-effect reconciliation tests
Windows process-tree tests
MCP modern + legacy tests
```

Legacy JSON state остаётся read/import compatibility, но не primary runtime.

---

# 31. Что сознательно НЕ делать до необходимости

Не добавлять в V1:

```text
Kafka
Kubernetes requirement
Raft/Paxos
custom distributed DB
multi-region consensus
service mesh
microservice split
global event ordering
2PC arbitrary tools
generic streaming platform
complex distributed work stealing
huge YAML DSL
```

Каждый такой компонент требует отдельного доказанного bottleneck/use case.

---

# 32. Порядок фактической реализации

Реализовывать именно в таком порядке:

```text
0. Model / IR / invariants
1. SQLite store + migrations + events
2. Legacy import
3. Engine transactional transitions
4. Incremental scheduler
5. Attempts + leases + fencing
6. Process executor + cancellation
7. Retry/circuit breaker
8. Timers/signals/approval
9. Side-effect IN_DOUBT/reconciliation
10. Artifact CAS/provenance
11. Cache/invalidation
12. Replanner GraphRevision migration
13. Workspace/worktree runtime
14. Agent executor/budgets/checkpoints
15. MCP executor/tool registry
16. Policy/secrets/trust
17. OTel/explainability
18. REST/SSE/dashboard
19. Remote workers
20. Postgres/HA only when justified
```

Почему порядок важен:

- нельзя делать remote workers до leases/fencing;
- нельзя делать retries правильно до immutable Attempts;
- нельзя делать agent durability до durable engine;
- нельзя делать human wait без timers/signals;
- нельзя безопасно retry side effects без `IN_DOUBT`/idempotency model;
- нельзя делать massive DAG optimizations поверх full-scan scheduler;
- нельзя делать HA, пока single-node correctness не доказана fault tests.

---

# 33. Первый конкретный implementation slice

Чтобы начать без слишком большого diff, первый PR/commit implementation после этого документа должен ограничиться следующим scope:

## Создать

```text
internal/harness/model/ids.go
internal/harness/model/workflow.go
internal/harness/model/node.go
internal/harness/model/attempt.go
internal/harness/state/transitions.go
internal/harness/state/invariants.go
internal/harness/ir/definition.go
internal/harness/compiler/validate.go
internal/harness/compiler/dag.go
```

## Добавить tests

```text
internal/harness/state/transitions_test.go
internal/harness/state/invariants_test.go
internal/harness/compiler/validate_test.go
internal/harness/compiler/dag_test.go
internal/harness/compiler/dag_benchmark_test.go
```

## Не менять пока

```text
internal/tasks execution semantics
.run.lock
planner persistence
replan persistence
CLI default behavior
```

## Definition of Done первого slice

- новый Harness model существует независимо от legacy;
- DAG validator работает на 100k nodes без quadratic implementation;
- transition table запрещает invalid states;
- `go test ./...` и `go test -race ./...` проходят;
- нет behavior change для пользователя.

Это создаёт безопасный фундамент для Stage 1 без риска сломать `agctl 3.2.1`.

---

# 34. Итоговый критерий завершения всей миграции

Миграция считается завершённой только если следующий тест проходит автоматически:

```text
1. Создать workflow с:
   - linear nodes
   - parallel fan-out/fan-in
   - agent node
   - MCP tool
   - artifact
   - retry
   - approval wait

2. Запустить.

3. Случайно kill orchestrator несколько раз:
   - до claim
   - во время process
   - после model response
   - после external request
   - после artifact bytes
   - до success commit

4. Перезапускать тот же binary.

5. Проверить:
   - completed nodes не вычисляются повторно без необходимости;
   - lost attempts видны как lost;
   - retries имеют новые AttemptID;
   - stale workers не commit-ят result;
   - uncertain side effect находится в IN_DOUBT/reconcile;
   - artifact hashes корректны;
   - approval переживает restart;
   - graph revision history сохраняется;
   - scheduler объясняет все waits;
   - final validators определяют success.
```

Если после этого Harness способен дать точный ответ:

```text
что произошло
что выполнялось
что подтверждено
что потеряно
что безопасно повторить
что нельзя повторять автоматически
какие artifacts существуют
почему каждый node ждёт
с какого места продолжится workflow
```

— тогда `agctl` действительно перешёл от adaptive task supervisor к production-grade Durable Execution Harness.
