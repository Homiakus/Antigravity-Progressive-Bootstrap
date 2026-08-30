# 24/7 AUTONOMOUS CODING OS — подробный план эволюции `agctl`

> Репозиторий: `Homiakus/Antigravity-Progressive-Bootstrap`  
> Статус: **implementation blueprint / living architecture extension**  
> Назначение: превратить существующий Durable Harness + multi-runtime control plane в restart-safe систему автономной разработки, способную выполнять инженерные mission/workflow сутками без зависимости от жизни одного LLM-контекста, IDE-процесса или worker-процесса.  
> Связь с `MASTER_PLAN.md`: этот документ **не заменяет** `MASTER_PLAN.md`. Текущая продуктовая цепочка T-012…T-030 остаётся основной. Этапы A24/T-031+ ниже должны включаться в living plan атомарно по мере разблокировки и не обходить существующие pre-flight/publication gates.

---

## 0. Краткий итог

Проект уже содержит большую часть правильного фундамента:

- durable Workflow/Node/Attempt state;
- SQLite Store с транзакциями и CAS-переходами;
- scheduler, retry budgets, timers/signals/approvals;
- worker leases и fencing epochs;
- side-effect intents и `IN_DOUBT`;
- artifact CAS/provenance;
- workspace lifecycle и Git worktrees;
- process executor с `Prepare/Execute/Cancel/Reconcile`;
- supervised agent loop с budget + external validator;
- provider observation/runtime ledger;
- provider capacity normalization, demand, reservation;
- session/context broker с `REUSE / NEW / CHECKPOINT_AND_NEW / UNAVAILABLE`;
- remote session recovery;
- typed coordinator/worker authority;
- machine-observed publication proof;
- repository-resident context compression checkpoint.

Поэтому **big-bang rewrite запрещён**. Целевой путь — эволюционно сделать Harness единственным durable execution kernel и привести Antigravity, Codex и будущие runtime/providers к одному lifecycle поверх `TaskEnvelope`.

Главная формула:

```text
LLM session lifetime != workflow lifetime
IDE lifetime         != workflow lifetime
worker lifetime      != workflow lifetime
daemon lifetime      != workflow lifetime
provider lifetime    != workflow lifetime

Workflow lifetime = durable state + Git + artifacts + verified checkpoints
```

Главная проверка:

```text
kill -9 agctl
kill worker
kill IDE
exhaust provider quota
rotate context
restart host

=> новый supervisor восстанавливает состояние и продолжает
   без потери принятой работы, без двойных side effects,
   без ложного DONE и без несанкционированного push в main.
```

---

# 1. Цель системы

Создать **Autonomous Coding OS**, в которой пользователь задаёт Mission высокого уровня, а система:

1. компилирует Mission в durable DAG;
2. выбирает атомарные задачи;
3. назначает подходящего runtime/provider/model/session;
4. резервирует quota/capacity до выполнения;
5. выделяет изолированный workspace;
6. запускает worker;
7. собирает durable execution events и artifacts;
8. делает checkpoints до исчерпания контекста;
9. при необходимости завершает поколение агента и создаёт новое;
10. независимо проверяет работу;
11. создаёт новые findings/tasks при обнаружении проблем;
12. безопасно интегрирует только qualified tree;
13. переживает process/IDE/provider/host failure;
14. продолжает цикл до доказанного Definition of Done.

Целевой автономный цикл:

```text
MISSION
  |
  v
PLAN/RECONCILE
  |
  v
READY NODE
  |
  v
TASK ENVELOPE
  |
  +--> demand estimate
  +--> provider observation
  +--> session/context decision
  +--> quota reservation
  |
  v
WORKSPACE + LEASE
  |
  v
EXECUTION GENERATION
  |
  +--> events
  +--> artifacts
  +--> usage
  +--> checkpoint
  |
  +--> context healthy ------> continue
  |
  +--> context rotate -------> checkpoint -> fresh generation
  |
  +--> provider unavailable -> safe handoff/failover
  |
  v
INDEPENDENT VERIFY
  |
  +--> reject/finding -> replan
  |
  v
INTEGRATION GATE
  |
  v
FENCED MAIN PUBLICATION
  |
  v
MISSION RECONCILE
  |
  +--> more work -> next READY node
  |
  `--> final acceptance -> DONE
```

---

# 2. Не-цели

Система **не должна**:

- держать один чат/LLM session 24 часа;
- считать transcript единственным источником памяти;
- доверять `agent says done`;
- позволять implementation worker проверять и принимать собственный результат;
- позволять ordinary worker писать `main`;
- автоматически повторять side effect с неизвестным результатом;
- суммировать несовместимые quota units;
- угадывать provider/session/model affinity;
- хранить незашифрованные provider credentials в plan/checkpoint/events;
- зависеть от GUI automation, если есть более стабильный CLI/API/bridge контракт;
- считать uptime процесса доказательством durability workflow;
- скрывать failed attempts ради «красивого» результата;
- использовать force push в автономном режиме;
- выполнять big-bang migration существующего Harness.

---

# 3. Текущее состояние и точки расширения

## 3.1 Что оставить без архитектурного дублирования

Существующие подсистемы должны остаться authority для своих областей:

| Область | Существующий authority | Действие |
|---|---|---|
| workflow runtime | `internal/harness/engine` + Store | расширять |
| scheduler | `internal/harness/scheduler` | расширять policy inputs |
| attempts | durable Attempt model | переиспользовать |
| ownership | worker lease + fencing | переиспользовать |
| process execution | `internal/harness/executor` | сделать нижним physical-runtime слоем |
| agent supervision | `internal/harness/executor/agent` | вынести durable state и provider-neutral lifecycle |
| side effects | EffectIntent + `IN_DOUBT` | переиспользовать |
| artifacts | CAS + provenance | переиспользовать |
| workspace | workspace manager/worktrees | переиспользовать |
| provider observations | `internal/harness/provider/*` | расширять |
| provider sessions | `provider/session` | расширять generation decisions |
| provider reservations | reservation ledger | переиспользовать |
| policy | harness policy engine | расширять privileges |
| telemetry | events + metrics + explainer | расширять |
| publication | typed roles + publication verifier | довести до Commit Coordinator |

## 3.2 Основной архитектурный разрыв

Сейчас есть два разных уровня execution:

### Physical process execution

```go
Prepare(...)
Execute(...)
Cancel(...)
Reconcile(...)
```

Это правильный низкоуровневый ABI для ОС-процессов.

### Agent loop

Есть ModelClient/ToolDispatcher/Validator и in-memory `AgentRunState`, но он не является полным durable multi-provider lifecycle.

### Provider adapter

Provider adapter наблюдает capacity/models/sessions, но намеренно не содержит portable execution.

Нужно связать эти слои так:

```text
TaskEnvelope
    |
    v
AgentExecutionService
    |
    +--> Provider/Runtime Selector
    |
    +--> Execution Adapter
            |
            +--> Antigravity Bridge/IDE
            +--> Codex App Server/CLI
            +--> generic process
            +--> future Claude/OpenHands/Factory/etc.
    |
    v
Durable AgentRun / Generation / Event ledger
    |
    v
Harness Attempt
```

---

# 4. Источники истины

## 4.1 Semantic truth

`MASTER_PLAN.md`:

- mission intent;
- tasks/findings;
- architecture decisions;
- current checkpoint;
- accepted/rejected approaches.

## 4.2 Runtime truth

Harness Store:

- WorkflowRun;
- GraphRevision;
- NodeRun;
- Attempt;
- Lease;
- Worker;
- retries/timers/signals;
- provider assignments/reservations;
- AgentRun/Generation/Execution durable state;
- effect intents;
- acceptance records.

## 4.3 Code truth

Git:

- commit graph;
- worktrees;
- qualified tree;
- published `main`.

## 4.4 Artifact truth

CAS/provenance:

- logs;
- patches;
- reports;
- test evidence;
- context summaries;
- generated files;
- mutation reports;
- screenshots/other bounded artifacts where applicable.

## 4.5 Что не является truth

Не являются authoritative:

- chat transcript;
- terminal scrollback;
- текущая память Go object;
- PID;
- IDE tab;
- provider UI;
- mutable free-form status message;
- worker self-report без внешней проверки.

---

# 5. Новые системные инварианты 24/7

Следующие инварианты должны быть добавлены в living plan по мере реализации.

### I24-001 — Process independence

Ни один daemon/worker/IDE/CLI процесс не требуется для реконструкции workflow state.

### I24-002 — Session independence

Ни одна LLM/session conversation не является обязательным хранилищем незавершённой логической работы.

### I24-003 — Durable-before-dispatch

Любая externally observable execution сначала получает durable Attempt + assignment + reservation + lease/generation identity и только потом dispatch.

### I24-004 — Generation fencing

События старого поколения не могут завершить или мутировать новое поколение после context rotation/failover.

### I24-005 — One active generation

Для одного logical AgentRun/Attempt одновременно может существовать не более одного ACTIVE write-capable generation, если специально не создан read-only fan-out.

### I24-006 — Checkpoint-before-rotation

Контекст не ротируется как «обычный happy path» без durable checkpoint или явного доказательства, что checkpoint невозможен и работа безопасно пересоздаваема.

### I24-007 — External acceptance

`SUCCESS` от LLM/runtime означает только «execution finished». Logical node становится accepted только через внешние gates.

### I24-008 — No self-acceptance

Implementation generation не может быть единственным verifier/reviewer собственного изменения.

### I24-009 — Safe failover

Provider failover создаёт новую generation/assignment и не перезаписывает историю предыдущей.

### I24-010 — No blind effect replay

После uncertain failure side-effectful operations проходят reconciliation и `IN_DOUBT`, а не обычный retry.

### I24-011 — Lease epoch authority

Любое completion/event/result обязано быть связано с ожидаемым lease/fencing epoch.

### I24-012 — Workspace identity

Generation знает immutable workspace identity/fingerprint; смена physical path не должна подменять logical workspace.

### I24-013 — Plan binding

Write-capable TaskEnvelope связывается с plan digest/revision. Несовпадение блокирует stale write.

### I24-014 — Artifact-addressed handoff

Большие handoff данные передаются через CAS refs/digests, а не копируются целиком в prompt/state rows.

### I24-015 — Bounded context

Supervisor никогда не предполагает бесконечный context. Context utilization — управляемый resource.

### I24-016 — Bounded retries

Нет бесконечного restart/retry loop без budget, backoff, circuit breaker и reason classification.

### I24-017 — Durable wait

Ожидание CI/quota/timer/user approval не занимает живой LLM session.

### I24-018 — Deterministic recovery

При одинаковом durable snapshot reconciliation принимает одинаковое действие либо явно фиксирует внешний nondeterministic observation.

### I24-019 — No hidden authority transfer

Смена provider/model/session не расширяет filesystem/git/network/secret/main-write capabilities.

### I24-020 — Main is qualified output only

Ни один intermediate generation/worktree не считается delivered state до integration qualification.

### I24-021 — No false DONE

Workflow не переходит DONE, пока Definition of Done не подтверждён независимыми evidence gates.

### I24-022 — Event auditability

Каждый существенный state transition имеет immutable event с causal IDs.

### I24-023 — Idempotent daemon restart

Повторный запуск supervisor/reconciler не создаёт дубликаты logical execution/effects.

### I24-024 — Degraded mode is explicit

При stale/unknown quota, missing recovery data или unavailable provider система входит в объяснимый WAIT/NEEDS_ATTENTION/UNAVAILABLE, а не делает оптимистичные догадки.

### I24-025 — 24h qualification is destructive-test driven

Заявление «24/7 ready» запрещено до прохождения restart/fault/endurance matrix.

---

# 6. Целевая доменная модель

Нельзя создавать параллельную «вторую систему задач». Новые сущности должны ссылаться на существующие Workflow/Node/Attempt.

## 6.1 `TaskEnvelope`

Immutable snapshot входных данных для provider-neutral agent execution.

Предлагаемые поля:

```go
type TaskEnvelope struct {
    SchemaVersion       int
    ID                  TaskEnvelopeID

    WorkflowRunID       WorkflowRunID
    NodeRunID           NodeRunID
    AttemptID           AttemptID
    GraphRevision       int

    PlanDigest          string
    TaskID              string
    FindingIDs          []string

    Objective           string
    DefinitionOfDone    []AcceptanceCriterion
    Constraints         []Constraint
    NonGoals            []string

    Role                EngineeringRole
    RiskClass           RiskClass
    RequiredCaps        []Capability
    ForbiddenCaps       []Capability

    Workspace           WorkspaceBinding
    BaseRevision        GitRevision
    TargetBranch        string

    ContextInputs       []ArtifactRef
    RequiredEvidence    []EvidenceRequirement

    Budget              ExecutionBudget
    ProviderPolicy      ProviderPolicyRef
    ValidationPolicy    ValidationPolicyRef

    IdempotencyKey      string
    CreatedAt           time.Time
}
```

Правила:

- immutable после dispatch;
- canonical serialization;
- content digest;
- schema-versioned;
- не содержит raw secrets;
- large context только ArtifactRef;
- write envelope обязательно содержит PlanDigest + BaseRevision;
- любой adapter получает один и тот же semantic contract.

## 6.2 `AgentRun`

Logical execution внутри Attempt.

```text
Attempt
  |
  `-- AgentRun
        |
        +-- Generation 1
        +-- Generation 2
        `-- Generation N
```

`AgentRun` живёт дольше отдельных provider sessions.

Поля:

- AgentRunID;
- AttemptID;
- EnvelopeDigest;
- desired/observed state;
- current generation;
- accepted result ref;
- cumulative budget/usage;
- final classification.

## 6.3 `AgentGeneration`

Одна ограниченная контекстом попытка выполнения AgentRun.

Поля:

- GenerationID;
- AgentRunID;
- generation number;
- lease epoch;
- provider assignment ID;
- provider account/model/session;
- runtime adapter kind;
- checkpoint parent;
- context limit/used/estimate;
- started/last activity/finished;
- state;
- terminal reason;
- output artifact refs;
- usage totals.

Состояния:

```text
PLANNED
-> DISPATCHING
-> RUNNING
-> CHECKPOINTING
-> COMPLETED
-> ROTATED
-> FAILED
-> CANCELLED
-> LOST
-> IN_DOUBT
```

## 6.4 `GenerationCheckpoint`

Не transcript dump, а компактный handoff package:

- objective status;
- completed facts;
- current Git HEAD/tree;
- modified files;
- unresolved issues;
- assumptions;
- test evidence;
- artifact refs;
- next actions;
- known blockers;
- plan digest;
- workspace fingerprint;
- provider-independent semantic memory;
- token/context stats.

Checkpoint имеет digest и immutable provenance.

## 6.5 `ExecutionEvent`

Единая нормализованная лента:

```text
STARTED
HEARTBEAT
MODEL_USAGE
TOOL_STARTED
TOOL_FINISHED
ARTIFACT
PATCH_OBSERVED
CHECKPOINT
CONTEXT_PRESSURE
WAITING
VALIDATION_RESULT
COMPLETION_CANDIDATE
FAILURE
CANCELLED
FINISHED
```

Adapter-specific payload может храниться отдельно, но core state machine читает нормализованные types.

---

# 7. Execution ABI

## 7.1 Не объединять observer и executor в один огромный interface

Сохранить разделение:

```go
type ProviderObserver interface {
    Capacity(...)
    Models(...)
    Sessions(...)
}
```

и ввести отдельный execution boundary.

## 7.2 Предлагаемый `AgentExecutionAdapter`

```go
type AgentExecutionAdapter interface {
    Kind() RuntimeKind
    Capabilities() ExecutionCapabilities

    Prepare(
        context.Context,
        TaskEnvelope,
        ExecutionBinding,
    ) (PreparedExecution, error)

    Start(
        context.Context,
        PreparedExecution,
    ) (ExecutionHandle, error)

    Resume(
        context.Context,
        ResumeRequest,
    ) (ExecutionHandle, error)

    Cancel(
        context.Context,
        ExecutionID,
        CancelMode,
    ) error

    Observe(
        context.Context,
        ExecutionID,
    ) (ExecutionObservation, error)

    Reconcile(
        context.Context,
        ExecutionID,
    ) (ExecutionObservation, error)
}
```

`ExecutionHandle` не должен быть authority. Это live convenience handle поверх durable IDs.

## 7.3 Связь с существующим process `Executor`

Существующий process `Executor` остаётся physical execution substrate:

```text
AgentExecutionAdapter
        |
        +--> ProcessExecutor
        +--> Antigravity Bridge
        +--> Codex App Server
        `--> future remote RPC
```

Не нужно превращать process `Request{Command,Args,...}` в TaskEnvelope.

## 7.4 Связь с `executor/agent`

Существующий supervised loop нужно декомпозировать:

- pure budget logic оставить;
- ModelClient/ToolDispatcher adapters переиспользовать, где удобно;
- `AgentRunState` перестать считать durable state;
- checkpoint persistence вынести в Store/CAS;
- validation gate заменить/расширить через отдельный acceptance subsystem;
- loop должен уметь восстановиться из durable checkpoint.

---

# 8. Provider/runtime adapters

## 8.1 Antigravity

Использовать authoritative Bridge/IDE capabilities:

- workspace;
- conversation focus/send;
- session/model/context, если реально предоставлены;
- controlled restart;
- recovery через persisted remote session identity.

Нельзя:

- считать window/tab title identity;
- хранить bridge token в durable store;
- делать GUI click automation частью core correctness.

## 8.2 Codex

Использовать App Server/CLI execution surface.

До появления authoritative thread->model/session linkage:

- не создавать фиктивный reusable session;
- допускается `NEW`;
- handoff через TaskEnvelope + checkpoint;
- model selection связывать только с authoritative execution response/assignment.

## 8.3 Generic CLI/process

Для внешних coding agents, имеющих CLI:

- adapter генерирует bounded config/prompt package;
- запускает через process executor;
- нормализует stdout/event protocol;
- completion не принимается без external validation;
- crash recovery опирается на durable execution ID + workspace state, а не только PID.

## 8.4 Future providers

Claude Code/OpenHands/Factory/иные runtime должны подключаться через adapter, не менять Harness state machine.

---

# 9. Context Generation Manager

Это ключевой слой для работы сутками.

## 9.1 Context pressure

Supervisor отслеживает:

- authoritative used/limit;
- estimated next-step demand;
- minimum checkpoint budget;
- tool/output growth;
- provider/session health.

## 9.2 Политика

Пример:

```text
SAFE
  continue

PRESSURE
  ask generation to converge current micro-step
  persist artifacts more aggressively

ROTATE_SOON
  forbid starting large new subtask
  create checkpoint

ROTATE_NOW
  checkpoint
  close/drain current generation
  allocate NEW session/context
  resume from checkpoint
```

Пороговые значения — configuration/policy, не hard-coded provider truth.

## 9.3 Rotation protocol

```text
1. generation -> CHECKPOINTING
2. freeze acceptance of new side-effect plans
3. flush logs/artifacts/usage
4. reconcile outstanding effects
5. persist GenerationCheckpoint
6. terminalize old generation as ROTATED
7. release/supersede provider assignment/reservation
8. create new generation number
9. run session broker
10. reserve new provider capacity
11. bind same TaskEnvelope + checkpoint parent
12. start fresh execution
```

Если шаг 4 возвращает uncertain external effect — не продолжать обычную rotation; использовать `IN_DOUBT`.

## 9.4 Semantic handoff

Новый контекст получает:

- TaskEnvelope;
- latest GenerationCheckpoint;
- relevant artifact refs;
- current repository state;
- acceptance failures;
- delta plan/findings.

Он **не обязан** получать полный transcript прошлого поколения.

---

# 10. Supervisor / Reconciler

## 10.1 Модель

Supervisor должен быть reconciliation loop, а не long-running prompt.

```text
Observe durable desired state
Observe external/runtime state
Compute delta
Apply one fenced/idempotent action
Persist event/state
Repeat
```

## 10.2 Основные reconcilers

- WorkflowReconciler;
- AttemptReconciler;
- AgentRunReconciler;
- GenerationReconciler;
- ProviderAssignmentReconciler;
- WorkspaceReconciler;
- EffectReconciler;
- ValidationReconciler;
- IntegrationReconciler.

## 10.3 Restart bootstrap

После запуска daemon:

1. открыть Store;
2. проверить migrations/integrity;
3. reconcile ACTIVE workers/leases;
4. найти RUNNING/DISPATCHING generations;
5. запросить adapters;
6. потерянные процессы классифицировать;
7. outstanding effects reconcile;
8. due timers/retries восстановить;
9. READY nodes вернуть scheduler;
10. восстановить remote sessions, где это безопасно;
11. продолжить execution.

Никакой «restore chat first» не требуется.

---

# 11. Failure taxonomy

Минимальные классы:

```text
TASK_LOGIC
VALIDATION_FAILED
MODEL_TRANSIENT
MODEL_FATAL
PROVIDER_QUOTA
PROVIDER_UNAVAILABLE
PROVIDER_AUTH
CONTEXT_EXHAUSTED
SESSION_LOST
RUNTIME_CRASH
WORKER_LOST
HOST_RESTART
NETWORK_TRANSIENT
TOOL_TRANSIENT
TOOL_FATAL
WORKSPACE_CONFLICT
GIT_CONFLICT
CI_FAILURE
CI_INFRA
POLICY_DENIED
SIDE_EFFECT_UNCERTAIN
STALE_PLAN
STALE_BASE
BUDGET_EXHAUSTED
```

Для каждого класса определить:

- retryable?;
- same provider?;
- same session?;
- same generation?;
- requires checkpoint?;
- requires new Attempt?;
- requires reconciliation?;
- backoff;
- circuit key;
- user attention threshold.

---

# 12. Recovery semantics

## 12.1 Worker lost

Если lease истёк:

- stale worker больше не authority;
- live process observation выполняется adapter-specific;
- non-side-effectful task можно повторить по policy;
- side-effectful uncertain attempt -> `IN_DOUBT`;
- новый worker получает новый fencing epoch.

## 12.2 Daemon crash

После restart:

- state из Store;
- физический runtime reconcile;
- no duplicate dispatch;
- due work continuation.

## 12.3 IDE crash

Antigravity remote session recovery:

- verify instance/workspace/conversation;
- restart managed instance при разрешённой policy;
- issue fresh ephemeral credentials;
- preserve logical AgentRun identity;
- generation может быть resumed или replaced.

## 12.4 Host restart

Host-level qualification должна проверять:

- SQLite WAL recovery;
- workspace/worktree reconciliation;
- orphan PID absence;
- stale leases;
- artifacts integrity;
- remote provider state;
- recovery latency.

## 12.5 Provider outage/quota

Не держать worker в busy loop.

```text
provider unavailable
  |
  +--> alternative eligible provider -> safe handoff
  |
  `--> no provider -> durable WAIT with timer/reset horizon
```

---

# 13. Scheduling для 24/7

Scheduler должен учитывать кроме существующих resource constraints:

- provider capacity reservation;
- context reuse value;
- expected demand;
- reset horizon;
- reliability/circuit state;
- workspace write exclusion;
- verification independence;
- urgency/critical path;
- expected execution duration;
- checkpoint cost;
- switch cost.

## 13.1 Нельзя максимизировать только throughput

Целевая функция должна учитывать:

```text
progress value
- quota risk
- context loss risk
- switch cost
- failure probability
- workspace contention
- stale-plan risk
```

## 13.2 Fairness

Длительная mission не должна навсегда вытеснять короткие critical tasks и наоборот.

Использовать существующий fair workflow scheduling как основу.

---

# 14. Multi-agent decomposition

## 14.1 Роли

Минимально:

- Planner/Architect;
- Researcher;
- Implementer;
- Test Engineer;
- Security Reviewer;
- Performance Reviewer;
- Mutation/Edge-space Reviewer;
- Code Reviewer;
- Integration Coordinator.

## 14.2 Независимость

Для high-risk write node:

```text
Implementer
    |
    v
fresh verifier generation
    |
    +--> tests
    +--> edge-space
    +--> mutation where applicable
    |
    v
fresh reviewer generation
    |
    v
Commit Coordinator
```

## 14.3 Fan-out

Read-only review/research может параллелиться.

Write fan-out — только в отдельных worktrees с явной integration strategy.

---

# 15. Dynamic replanning

Существующий GraphRevision механизм должен быть единственным путём изменения active DAG.

При finding:

```text
F-XXX
  |
  v
ReplanProposal
  |
  v
validate
  |
  v
new GraphRevision
```

Нельзя позволять worker скрыто менять цель workflow только через prompt.

High/Critical finding:

- может блокировать dependent publication;
- создаёт explicit task;
- обновляет plan;
- требует acceptance before final done.

---

# 16. Git/workspace модель

## 16.1 По умолчанию

```text
repo main
  |
  +-- worktree implementation
  +-- worktree experiment
  +-- optional read-only verification view
```

## 16.2 Write worker

Не имеет main-write capability.

Результат:

- commit/patch/tree в worktree;
- evidence artifacts;
- handoff descriptor.

## 16.3 Integration Coordinator

Единственный автономный writer:

1. fresh fetch/observe remote main;
2. verify base ancestry;
3. materialize candidate;
4. rerun required gates на candidate tree;
5. ensure clean worktree;
6. fast-forward only;
7. observe remote HEAD;
8. persist publication proof;
9. update checkpoint.

## 16.4 Stale base

Если main ушёл вперёд:

- не force;
- rebase/merge strategy только через explicit integration workflow;
- после изменения tree повторная qualification обязательна.

---

# 17. Acceptance Engine

## 17.1 Completion levels

Разделить:

```text
EXECUTION_FINISHED
CANDIDATE_READY
VERIFIED
REVIEWED
QUALIFIED
PUBLISHED
ACCEPTED
```

LLM может инициировать только `CANDIDATE_READY`.

## 17.2 Evidence types

- targeted tests;
- full tests;
- race;
- static;
- lint;
- fuzz/property;
- mutation;
- security;
- compatibility;
- performance;
- build;
- UI/browser;
- external service;
- publication proof;
- plan reconciliation.

## 17.3 Policy by risk

Read-only analysis не требует тех же gates, что изменение auth/runtime/store.

Risk profile должен выбирать mandatory evidence.

## 17.4 Evidence freshness

Evidence привязано к:

- tree digest;
- plan digest/revision;
- tool/version/environment where relevant.

После изменения tree старые tests не автоматически действительны.

---

# 18. Validation of validations

Критические gates должны иметь test-of-tests:

- удалить required evidence -> completion fails;
- подменить tree digest -> fails;
- stale checkpoint -> fails;
- stale lease epoch completion -> fails;
- quota oversubscription mutant -> killed;
- context threshold mutant -> killed;
- ordinary worker main-write bit -> killed;
- `IN_DOUBT` blind retry mutant -> killed;
- force push acceptance mutant -> killed;
- old generation event accepted after rotation -> killed.

---

# 19. Observability

## 19.1 Пользователь должен видеть

Для каждой mission:

- current phase;
- completed/remaining nodes;
- active workers;
- provider/model/session;
- quota/reset;
- context headroom;
- workspace;
- last checkpoint;
- current validation;
- blockers;
- retries/failovers;
- estimated critical path;
- latest accepted/published commit.

## 19.2 `agctl explain`

Нужны объяснения:

```text
why waiting?
why provider X?
why new session?
why context rotated?
why retry?
why failover?
why not main?
why task reopened?
why workflow not done?
```

Ответ строится из durable reason codes, не генеративного post-hoc объяснения.

## 19.3 Metrics

Минимально:

- workflow success rate;
- node success/failure/reopen;
- attempts per node;
- generations per attempt;
- context rotations;
- checkpoint latency;
- recovery latency;
- provider failovers;
- quota utilization;
- wasted tokens after invalid result;
- validation reject rate;
- time-to-qualified-tree;
- publication conflicts;
- `IN_DOUBT` count/time;
- duplicate-effect prevention count;
- orphan recovery count;
- scheduler wait reasons.

---

# 20. Security

## 20.1 Secrets

Durable state хранит только `SecretRef`, не secret value.

Ephemeral provider/runtime tokens:

- short-lived;
- memory/process env/secure OS storage;
- redacted from logs;
- not embedded in checkpoints/artifacts.

## 20.2 Capability preservation

Provider switch не меняет permissions.

TaskEnvelope содержит required + forbidden capabilities.

## 20.3 Remote workers

Remote worker:

- не имеет прямой SQLite authority;
- не получает main-write;
- получает только scoped workspace/capabilities;
- results fenced by lease epoch;
- untrusted remote не получает secrets без explicit policy.

## 20.4 Prompt injection / repository instructions

External/repository content считается untrusted data относительно system policy.

Agent не может сам повысить свои capabilities через найденный текст.

---

# 21. Performance и resource bounds

24/7 runtime обязан быть bounded.

## 21.1 Bounded collections

- logs streamed to CAS; bounded tail in DB;
- events paginated;
- checkpoint size cap;
- prompt package size cap;
- artifact metadata bounded;
- ready queue queries indexed;
- reservation accounting set-based in SQL where возможно.

## 21.2 Backpressure

Если consumer telemetry/log sink медленный:

- execution не должен бесконечно накапливать RAM;
- bounded queues;
- spill/drop policy только для non-authoritative data;
- authoritative events не терять.

## 21.3 Targets

Первоначальные qualification targets, не product SLA:

- restart reconciliation: no lost logical state;
- 10k workflow-state rows remain queryable without O(N) scans in hot path;
- 100 concurrent read-only workers do not violate races;
- 10+ concurrent write worktrees preserve exclusivity;
- 24h RSS trend bounded after warm-up;
- no monotonic goroutine/file-descriptor leak;
- event/CAS growth explainable by workload.

---

# 22. Endurance / chaos test architecture

Нужен отдельный test harness, способный запускать контролируемые faults по seed.

## 22.1 Fault injector

Поддержать:

- kill daemon;
- kill worker;
- kill child process;
- kill IDE;
- network deny/restore;
- provider HTTP 429/5xx;
- auth failure;
- quota exhausted/reset;
- delayed response;
- duplicated response/event;
- out-of-order event;
- truncated stream;
- disk full;
- SQLite busy;
- artifact write failure;
- Git remote movement;
- Git conflict;
- CI timeout/failure;
- context pressure;
- stale checkpoint;
- corrupted non-authoritative cache;
- clock skew simulation where safe.

## 22.2 Oracle

После fault система проверяет не только «процесс снова работает», но:

- state machine valid;
- no duplicate accepted effects;
- one active lease/generation;
- no stale generation completion;
- no lost artifact referenced by accepted result;
- no false DONE;
- main only qualified;
- plan/checkpoint coherent;
- reservations eventually settled/released;
- due retry/timer restored.

---

# 23. 24-часовой qualification scenario

Минимальный сценарий должен быть deterministic/replayable по seed.

```text
H00  bootstrap, create mission, baseline
H01  normal multi-agent work
H02  kill one worker during read-only task
H03  kill write worker after local change before completion
H04  transient provider 5xx
H05  quota exhaustion -> alternate provider / WAIT
H06  force context pressure -> checkpoint + generation rotation
H07  kill daemon during idle wait
H08  restart daemon -> reconcile
H09  delayed duplicate provider event
H10  validation rejection -> replan
H11  inject failing test caused by candidate
H12  recover/fix
H13  move remote main externally
H14  candidate integration detects stale base
H15  requalification after integration update
H16  kill IDE/session runtime
H17  session recovery/new generation
H18  simulate uncertain side effect -> IN_DOUBT
H19  reconciliation resolves effect
H20  artifact/log pressure
H21  scheduler concurrency/fairness stress
H22  final independent verification
H23  publication gate
H24  full state/audit/invariant reconstruction
```

Pass conditions:

- zero false DONE;
- zero duplicate non-idempotent effects;
- zero unauthorized main writes;
- zero force pushes;
- zero accepted stale lease/generation result;
- every restart reconstructs correct desired/observed state;
- every rotation has checkpoint lineage;
- every publication has observed proof;
- final Git tree passes required gates;
- full event history can explain every retry/failover/rotation.

---

# 24. 72-часовой soak

После 24h fault qualification:

- variable workload;
- repeated provider reset windows;
- repeated context generations;
- many workflows;
- artifact GC cycles;
- worktree GC cycles;
- remote worker churn;
- intermittent CI/service latency.

Основные leak gates:

- RSS slope;
- goroutine count;
- open FDs/handles;
- SQLite WAL growth;
- DB size by event class;
- orphan worktrees;
- orphan reservations;
- stale leases;
- stale sessions;
- unresolved `IN_DOUBT`;
- timer/retry backlog.

---

# 25. Миграция состояния

## 25.1 Не удалять legacy loop сразу

Старый `internal/loop` оставить compatibility shell до завершения rollout.

## 25.2 Путь

```text
legacy conversation TaskState JSON
        |
        v
read-only projection/bridge
        |
        v
Harness Workflow/Attempt/AgentRun authority
        |
        v
deprecate new writes to legacy state
        |
        v
migration audit
        |
        v
remove legacy execution authority
```

## 25.3 Dual-write запрещён как длительная архитектура

Если временно нужен dual-write:

- один authority;
- второй projection;
- reconciliation;
- bounded migration window;
- explicit deletion task.

---

# 26. Rollout modes

Добавить независимый feature mode:

```text
OFF
OBSERVE
SHADOW
ADVISORY
AUTO_READ_ONLY
AUTO_ISOLATED_WRITE
AUTO_FAILOVER
AUTO_INTEGRATE
ENDURANCE_QUALIFIED
DEFAULT
```

### OFF
Новая execution orchestration не влияет.

### OBSERVE
Только snapshots/diagnostics.

### SHADOW
Решения считаются, execution идёт старым путём.

### ADVISORY
Показываем оператору выбранный provider/generation/recovery action.

### AUTO_READ_ONLY
Автоматическое выполнение безопасных read-only nodes.

### AUTO_ISOLATED_WRITE
Write только в worktree, без autonomous main.

### AUTO_FAILOVER
Safe provider/runtime handoff.

### AUTO_INTEGRATE
Fenced Commit Coordinator.

### ENDURANCE_QUALIFIED
Режим разрешён только после 24h/72h evidence.

### DEFAULT
Legacy execution больше не основной.

---

# 27. Детальный implementation roadmap

Ниже новые задачи **не должны обгонять** существующие T-012…T-030. Перед фактическим включением каждой задачи её необходимо перенести/синхронизировать в `MASTER_PLAN.md`, создать `.engineering/preflight/T-XXX.md`, выполнить существующий qualification contract и только затем менять production code.

## T-031 — Formalize Autonomous Coding OS architecture
**Priority:** P0  
**Dependencies:** T-030 желательно, документационно может быть подготовлено раньше.  
**Scope:**
- зафиксировать I24 invariants;
- target state ownership;
- TaskEnvelope/AgentRun/Generation boundaries;
- legacy migration boundary;
- no-big-bang ADR.
**Acceptance:**
- structural audit не видит conflicting authority;
- все новые сущности имеют owner/store/lifecycle;
- нет дублирования Attempt/Lease/Workspace/EffectIntent.

## T-032 — Introduce canonical `TaskEnvelope`
**Priority:** P0  
**Dependencies:** существующий T-013 должен быть завершён; T-031.  
**Scope:**
- schema version;
- canonical serialization/digest;
- plan/graph/base revision binding;
- role/capabilities;
- artifact refs;
- acceptance/evidence policy refs.
**Tests:**
- deterministic digest/property;
- incompatible schema fail closed;
- stale plan/base sentinels;
- no secrets serialization.

## T-033 — Durable AgentRun + Generation model
**Priority:** P0  
**Dependencies:** T-032.  
**Scope:**
- append-only generation history;
- desired/observed state;
- generation fencing;
- cumulative usage.
**Persistence:**
- append-only migration;
- FK/CHECK/UNIQUE;
- CAS transitions.
**Critical test:**
- race creates at most one ACTIVE write generation.

## T-034 — Provider-neutral execution adapter ABI
**Priority:** P0  
**Dependencies:** T-032,T-033.  
**Scope:**
- Prepare/Start/Resume/Cancel/Observe/Reconcile;
- normalized capabilities/events;
- adapter registry;
- preserve physical process Executor.
**Acceptance:**
- fake deterministic adapter drives full lifecycle;
- core imports no Antigravity/Codex-specific semantics.

## T-035 — Durable execution event ingestion
**Priority:** P0  
**Dependencies:** T-033,T-034.  
**Scope:**
- idempotency key;
- sequence/revision rules;
- out-of-order/duplicate behavior;
- event->state reducer.
**Tests:**
- duplicate replay;
- reordered events;
- stale generation events;
- concurrent ingestion race.

## T-036 — Durable generation checkpoints
**Priority:** P0  
**Dependencies:** T-033,T-035; artifact CAS existing.  
**Scope:**
- compact provider-neutral schema;
- CAS-backed large payloads;
- workspace/tree/plan digest;
- checkpoint lineage.
**Acceptance:**
- restart/resume without transcript;
- stale/mismatched checkpoint rejected.

## T-037 — Context pressure and generation rotation
**Priority:** P0  
**Dependencies:** T-008 existing broker,T-036.  
**Scope:**
- SAFE/PRESSURE/ROTATE_SOON/ROTATE_NOW policy;
- checkpoint-before-rotation;
- generation terminalization;
- new assignment/reservation.
**Mutation sentinels:**
- unknown context != unlimited;
- no rotation without checkpoint;
- old generation completion rejected.

## T-038 — Antigravity execution adapter
**Priority:** P0  
**Dependencies:** T-034,T-036,T-037; existing remote recovery.  
**Scope:**
- Bridge execution lifecycle;
- managed restart;
- normalized events;
- resume/new generation semantics.
**Fault tests:**
- IDE kill;
- bridge token loss;
- workspace mismatch;
- missing conversation.

## T-039 — Codex execution adapter
**Priority:** P0  
**Dependencies:** T-034,T-036,T-037.  
**Scope:**
- App Server/CLI execution;
- authoritative result/session data only;
- normalized events/usage;
- no thread->model guessing.
**Acceptance:**
- provider outage/429;
- new context handoff;
- no fabricated reuse.

## T-040 — AgentRun supervisor/reconciler
**Priority:** P0  
**Dependencies:** T-033..T-039.  
**Scope:**
- desired/observed reconciliation;
- daemon restart bootstrap;
- lost execution handling;
- wait/resume.
**Acceptance:**
- repeated reconcile idempotent;
- kill daemon at every transition property suite.

## T-041 — Provider-aware recovery/failover state machine
**Priority:** P0  
**Dependencies:** existing T-015/T-016 + T-040.  
**Scope:**
- failure taxonomy mapping;
- same/new generation policy;
- safe handoff;
- circuit/backoff/reset horizon.
**Acceptance:**
- no failover bypasses `IN_DOUBT`;
- no active reservation leak;
- history append-only.

## T-042 — Independent Acceptance Engine
**Priority:** P0  
**Dependencies:** T-032,T-040.  
**Scope:**
- EXECUTION_FINISHED != ACCEPTED;
- evidence policy by risk;
- verifier separation;
- tree/plan digest freshness.
**Acceptance:**
- implementation self-report cannot pass alone;
- stale test evidence rejected.

## T-043 — Multi-generation role orchestration
**Priority:** P1  
**Dependencies:** T-042; existing planner/replan.  
**Scope:**
- implementer -> verifier -> reviewer;
- read-only fan-out;
- worktree write isolation;
- findings -> GraphRevision.
**Acceptance:**
- high-risk node requires independent verifier identity.

## T-044 — Fenced integration end-to-end
**Priority:** P0  
**Dependencies:** existing T-018/T-019 + T-042,T-043.  
**Scope:**
- candidate handoff;
- fresh main;
- qualified-tree rerun;
- fast-forward-only publish;
- observed remote proof.
**Acceptance:**
- main movement and stale-base matrix;
- no-force mutation tests.

## T-045 — Legacy completion loop migration
**Priority:** P1  
**Dependencies:** T-040,T-042,T-044.  
**Scope:**
- Harness becomes sole execution authority;
- legacy conversation state projection;
- stop new execution-critical JSON state writes.
**Acceptance:**
- compatibility commands still work;
- restart state comes only from Harness truth.

## T-046 — 24/7 observability and explainability
**Priority:** P1  
**Dependencies:** T-035,T-040,T-041.  
**Scope:**
- generation/context/failover views;
- `route/execution explain`;
- metrics/SSE/API.
**Acceptance:**
- every WAIT/ROTATE/FAILOVER has stable reason code.

## T-047 — Deterministic fault-injection framework
**Priority:** P0  
**Dependencies:** T-040,T-041,T-044.  
**Scope:**
- seeded faults;
- fake providers;
- process/daemon kills;
- event duplication/reorder;
- Git movement;
- storage faults.
**Acceptance:**
- scenario replay by seed;
- invariant oracle.

## T-048 — 24-hour endurance qualification
**Priority:** P0  
**Dependencies:** T-047.  
**Scope:**
- scenario from section 23;
- resource trend capture;
- recovery evidence.
**Pass gate:**
- all I24 invariants;
- zero false DONE/duplicate effects/unauthorized main writes.

## T-049 — 72-hour soak and leak qualification
**Priority:** P1  
**Dependencies:** T-048.  
**Scope:**
- repeated generations/provider resets;
- churn/GC;
- leak metrics.
**Pass gate:**
- bounded RSS/goroutines/handles/WAL/orphans.

## T-050 — Default-mode convergence and delete legacy authority
**Priority:** P0  
**Dependencies:** T-049 + existing T-024/T-025/T-026 convergence intent.  
**Scope:**
- make durable execution path default;
- remove superseded runtime authority;
- final architecture/security/performance audit.
**Acceptance:**
- no Critical/High unresolved durability finding;
- docs and recovery runbook current;
- fresh clone/install can run qualification smoke.

---

# 28. Dependency DAG для T-031+

```text
existing:
T-012 -> T-013 -> T-014 -> T-015 -> T-016 -> T-017 -> T-018 -> T-019
   |        |
   |        `----------------------+
   +-> T-020 -> T-021              |
T-022/T-023/T-024/T-025/T-026      |
T-027 -> T-028 -> T-029 -> T-030   |
                                    |
24/7 extension:                     |
T-030 -> T-031 ---------------------+
          |
          v
T-032 -> T-033 -> T-034 -> T-035 -> T-036 -> T-037
                    |                         |
                    +-> T-038 ----------------+
                    +-> T-039 ----------------+
                                               |
                                               v
                                             T-040
                                               |
                          existing T-015/T-016-+
                                               v
                                             T-041
                                               |
T-032 --------------------------------------> T-042
                                               |
                                               v
                                             T-043
                                               |
                    existing T-018/T-019 ------+
                                               v
                                             T-044
                                               |
                         +---------------------+------------------+
                         v                                        v
                       T-045                                    T-046
                         |                                        |
                         +---------------------+------------------+
                                               v
                                             T-047
                                               |
                                             T-048
                                               |
                                             T-049
                                               |
                                             T-050
```

---

# 29. Порядок реализации без риска

## Wave A — Contracts, no autonomous writes

- T-031;
- T-032;
- T-033;
- T-034;
- T-035;
- T-036.

Режим: tests/shadow.

Выход: durable execution grammar без реального risky routing.

## Wave B — Context generations

- T-037;
- T-038;
- T-039;
- T-040.

Режим: read-only сначала.

Выход: agent может пережить context/session/process смену.

## Wave C — Safe failover + independent acceptance

- T-041;
- T-042;
- T-043.

Выход: provider failure не равен workflow failure, а LLM completion не равен accepted result.

## Wave D — Write/integration migration

- T-044;
- T-045.

Выход: isolated autonomous writes + единственный fenced main path.

## Wave E — Prove 24/7

- T-046;
- T-047;
- T-048;
- T-049.

Выход: не декларация, а измеренная durability evidence.

## Wave F — Convergence

- T-050;
- синхронизация с T-024/T-025/T-026;
- delete legacy authority;
- final re-audit.

---

# 30. Pre-flight template для каждой задачи

Перед production edit:

```text
Task:
Status: IN_PROGRESS
Base main:
Plan digest:
Dependencies:

Root cause / capability gap:
Target invariant:
Protected surfaces:
Change surface:
Non-goals:
Rollback:
Migration impact:
Security impact:
Compatibility impact:

Characterization:
Targeted tests:
Race/concurrency:
Property/fuzz:
Mutation/test-of-tests:
Performance:
Endurance/fault cases:

Publication strategy:
```

Этот marker должен быть первым task-specific publication-lineage write.

---

# 31. Definition of Done для каждой атомарной задачи

Минимально:

1. задача существует в living plan;
2. pre-flight persisted до production edit;
3. existing behavior characterized;
4. root-cause change минимален;
5. unit/targeted tests;
6. edge-space;
7. race, если concurrency/state;
8. property/fuzz, если state machine/parser/arithmetic;
9. mutation/test-of-tests для critical boundaries;
10. static analysis;
11. security review;
12. compatibility review;
13. performance evidence или reasoned N/A;
14. findings reconciled;
15. plan reconciled;
16. final diff adversarial review;
17. qualified tree;
18. publication через разрешённую authority;
19. observed remote proof;
20. context checkpoint.

---

# 32. Definition of Done для «24/7 ready»

Система может называться 24/7-ready только если одновременно выполнено:

### Architecture
- Harness — единственный runtime execution authority;
- TaskEnvelope provider-neutral;
- AgentRun survives session/runtime changes;
- generations fenced;
- checkpoints durable/provider-neutral;
- external acceptance separate.

### Recovery
- daemon restart;
- worker loss;
- IDE loss;
- provider outage;
- quota exhaustion;
- context rotation;
- stale main;
- uncertain effects.

### Safety
- no ordinary main writers;
- no force;
- no blind replay;
- stale lease/generation rejected;
- no secret persistence;
- stale plan/base blocked.

### Verification
- tests/race/vet;
- property/fuzz;
- mutation critical paths;
- 24h seeded fault qualification;
- 72h soak;
- leak/orphan audit.

### Auditability
- every logical decision explainable by durable reason;
- every accepted result bound to evidence/tree/plan;
- every generation has lineage;
- every publication has observed proof.

---

# 33. Success metrics

После внедрения измерять:

| Metric | Цель |
|---|---|
| false workflow DONE | 0 |
| duplicate non-idempotent effect | 0 |
| unauthorized main writes | 0 |
| force pushes by autonomous path | 0 |
| stale lease completion accepted | 0 |
| stale generation completion accepted | 0 |
| unrecoverable task after daemon restart | 0 для supported class |
| rotations without checkpoint | 0 в normal rotation |
| provider failover without recorded rationale | 0 |
| accepted result without external evidence | 0 |
| orphan active reservation after convergence | 0 |
| orphan write worktree after GC horizon | 0 |
| unexplained WAIT | 0 |
| unbounded retry loop | 0 |
| 24h invariant violations | 0 |

Efficiency metrics не могут отменять correctness metrics.

---

# 34. Основные риски реализации

## Risk 1 — Дублирование существующего Harness

Mitigation: новые сущности только как child/extension существующих Attempt/Lease/Store.

## Risk 2 — Слишком большой универсальный executor interface

Mitigation: observer, physical process executor и agent execution adapter остаются раздельными.

## Risk 3 — Durable transcript explosion

Mitigation: normalized events + CAS artifacts + compact checkpoints; transcript optional artifact.

## Risk 4 — Provider semantics leak в core

Mitigation: reason/capability normalization; provider-specific metadata behind adapter.

## Risk 5 — Rotation в опасный момент

Mitigation: checkpoint + outstanding effect reconciliation + generation fencing.

## Risk 6 — False acceptance

Mitigation: completion levels + independent acceptance engine.

## Risk 7 — Retry storm при outage

Mitigation: retry budgets + circuit breaker + reset-aware durable WAIT.

## Risk 8 — Main conflicts

Mitigation: worktrees + single Commit Coordinator + requalification.

## Risk 9 — План и runtime расходятся

Mitigation: plan digest/GraphRevision binding + structural plan audit + reconcile.

## Risk 10 — Тест 24h сам нестабилен

Mitigation: seeded deterministic fault schedule, virtualized fake providers для основной oracle suite, реальные providers как отдельная non-deterministic qualification layer.

---

# 35. Что делать первым после текущей T-012 цепочки

Ближайший порядок **не меняется**:

1. закончить T-012 shadow selector;
2. T-013 TaskEnvelope/plan digest — реализовать уже с учётом требований этого документа;
3. T-014..T-019 довести read-only/write/failover/main fencing;
4. T-030 structural plan enforcement;
5. затем формально открыть T-031 и переносить 24/7 extension в основной living DAG.

Практически главный архитектурный выигрыш появится после комбинации:

```text
T-013 TaskEnvelope
+
T-033 durable AgentRun/Generation
+
T-036 checkpoint
+
T-037 rotation
+
T-040 reconciler
```

После неё один LLM context перестаёт быть единицей долговечности.

---

# 36. Финальная целевая архитектура

```text
                         USER / API / TELEGRAM
                                  |
                                  v
                              MISSION
                                  |
                                  v
                         MASTER_PLAN.md
                                  |
                         Plan/Replan Compiler
                                  |
                                  v
                       Durable Workflow DAG
                                  |
                    +-------------+-------------+
                    |                           |
                    v                           v
              Fair Scheduler              Acceptance Policy
                    |
                    v
                 Attempt
                    |
                    v
               TaskEnvelope
                    |
        +-----------+------------+
        |                        |
        v                        v
 Demand/Quota                 Workspace
 Session Broker               + Lease
        |                        |
        +-----------+------------+
                    v
               AgentRun
                    |
              Generation N
                    |
         Execution Adapter Registry
        +-----------+------------+-------------+
        |           |            |             |
        v           v            v             v
 Antigravity      Codex       Generic CLI    Future
        |           |            |             |
        +-----------+------------+-------------+
                    |
          normalized durable events
                    |
        +-----------+------------+
        |                        |
        v                        v
    CAS/Artifacts          Usage/Provider ledger
        |                        |
        +-----------+------------+
                    |
             Context Manager
                    |
          pressure? / failure?
             |             |
             |             +--> safe failover
             |
             +--> checkpoint
                    |
             fresh Generation
                    |
                    v
          Independent Verification
                    |
                    v
             Reviewer / Judge
                    |
                    v
           Integration Coordinator
                    |
           verified fast-forward
                    |
                    v
                   main
                    |
                    v
             Mission Reconcile
```

---

# 37. Главный архитектурный критерий

Если удалить все live процессы и оставить только:

```text
SQLite Store
Git repositories/worktrees metadata
MASTER_PLAN.md
Artifact CAS
provider-observable external state
```

новый экземпляр `agctl` должен суметь ответить:

- какая Mission выполнялась;
- какой DAG актуален;
- какая GraphRevision;
- какие nodes завершены/приняты;
- какие attempts/generations существовали;
- какой worker/lease потерян;
- какие side effects определённы/неопределённы;
- какие checkpoints доступны;
- какой provider quota доступен;
- какие workspaces безопасны;
- какие validations уже действительны;
- какой candidate tree существует;
- что именно можно сделать следующим безопасным действием.

Если это доказано fault/endurance tests — система действительно durable. Если для ответа требуется «тот самый чат ещё должен быть открыт», система 24/7-ready не является.

---

# 38. Решение

`Antigravity-Progressive-Bootstrap` следует развивать как **главный durable control plane**, а Antigravity/Codex/прочие coding runtimes считать заменяемыми execution backends.

Ключевой путь:

```text
existing durable Harness
-> T-012/T-013 provider selection + TaskEnvelope
-> fenced provider execution
-> durable AgentRun/Generation
-> compact checkpoints
-> context generation rotation
-> restart reconciler
-> safe failover
-> independent acceptance
-> fenced integration
-> deterministic chaos harness
-> 24h qualification
-> 72h soak
-> default mode
```

Это сохраняет уже реализованные сильные стороны проекта и закрывает главный оставшийся разрыв: **долговечность разработки должна принадлежать Harness, а не агенту или его контексту**.
