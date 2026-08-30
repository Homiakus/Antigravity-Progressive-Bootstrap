# 24/7 AUTONOMOUS CODING OS — ADGO-first план эволюции `agctl`

> Репозиторий: `Homiakus/Antigravity-Progressive-Bootstrap`  
> Статус: **authoritative architecture/migration blueprint**  
> Стратегическое решение: **ADGO становится единым reusable durable execution kernel; `agctl` становится специализированным Autonomous Coding OS поверх ADGO.**  
> Связанный upstream-план: `Homiakus/axiom/adgo/AGENT_PLATFORM_PLAN_RU.md`.  
> Этот документ заменяет прежнюю цель «сделать `internal/harness` финальным универсальным runtime». Existing Harness сохраняется как квалифицированный legacy/reference runtime до безопасного завершения strangler migration.

---

## 0. Решение

`agctl` уже содержит зрелый durable Harness: Workflow/Node/Attempt, SQLite, leases/fencing, retries, timers/signals, artifacts, side-effect reconciliation, provider accounting, session/context broker и remote workers. Одновременно ADGO в `Homiakus/axiom` решает тот же класс общей задачи и будет развиваться как reusable библиотека.

Развивать обе реализации параллельно запрещается.

Целевая архитектура:

```text
┌──────────────────────────────────────────────────────────────┐
│                     agctl Autonomous Coding OS              │
│ MASTER_PLAN / Git / worktrees / tests / review / publish   │
│ Codex / Antigravity / Claude adapters / coding acceptance  │
├──────────────────────────────────────────────────────────────┤
│                    axiom/adgo/agent                         │
│ agent runs / generations / checkpoints / context/session   │
│ provider-account-model / quota / demand / reservation      │
│ usage / routing / agent-runtime contracts                  │
├──────────────────────────────────────────────────────────────┤
│                       axiom/adgo                            │
│ durable DAG / tasks / leases / retry / effects / repair    │
│ budgets / artifacts / history / migration / storage        │
├──────────────────────────────────────────────────────────────┤
│                         axiom                               │
│ model / claims / rules / compiler / decision tables        │
└──────────────────────────────────────────────────────────────┘
```

Главная формула:

```text
generic durability/orchestration/agent capability -> ADGO
coding/repository/IDE/testing/publication capability -> agctl
```

И главный запрет:

```text
один logical workflow = один authoritative durable engine
```

Во время миграции старые runs могут оставаться Harness-owned, а новые — ADGO-owned. Один и тот же run никогда не управляется двумя coordinator-ами одновременно.

---

# 1. Цель продукта `agctl`

После миграции `agctl` не должен быть второй workflow-библиотекой. Он должен быть продуктовым слоем для автономной разработки, который:

1. читает/ведёт `MASTER_PLAN.md`;
2. анализирует репозиторий и формирует coding mission;
3. компилирует coding-specific работу в ADGO Plan/activities;
4. выбирает Codex/Antigravity/Claude/другие runtime adapters;
5. выделяет Git worktrees и защищает mutable workspace;
6. запускает сборку/тесты/race/fuzz/mutation/security gates;
7. проводит независимый review/acceptance;
8. управляет findings и изменениями инженерного плана;
9. интегрирует только qualified tree;
10. осуществляет fenced publication в `main`;
11. отображает пользователю состояние mission, provider capacity, blockers и evidence.

Все generic механизмы долговечности должны приходить из ADGO.

---

# 2. Ownership matrix

## 2.1 Должно жить в ADGO

| Capability | Target owner | Источник/действие |
|---|---|---|
| durable execution graph | `adgo` | использовать ADGO |
| execution/node/task state | `adgo` | использовать ADGO |
| attempts | `adgo` | унифицировать semantics |
| worker leases/fencing | `adgo` | ADGO authority |
| retry/backoff/failure classes | `adgo` | ADGO authority |
| timers/signals/human waits | `adgo` | ADGO authority |
| generic budgets | `adgo` | ADGO authority |
| generic admission/resources | `adgo` | ADGO authority |
| side-effect idempotency/reconciliation | `adgo` | сохранить честный at-least-once |
| targeted repair | `adgo` | уже есть; расширять только upstream |
| stagnation/oscillation detection | `adgo` | уже есть; не дублировать |
| plan identity/migration | `adgo` | upstream authority |
| continue-as-new | `adgo` | реализовать upstream |
| generic artifact refs/provenance contract | `adgo` | унифицировать |
| durable history/watch/explain | `adgo` | upstream authority |
| provider/account/model domain | `adgo/agent` | перенести лучшие идеи Harness |
| quota/capacity model | `adgo/agent` | перенести native-unit semantics |
| demand estimator | `adgo/agent` | перенести p80/scoped logic |
| atomic reservations/usage settlement | `adgo/agent` | перенести из Harness |
| session/context broker | `adgo/agent` | REUSE/NEW/CHECKPOINT_AND_NEW |
| AgentRun/Generation lifecycle | `adgo/agent` | новый reusable contract |
| context checkpoint/rotation | `adgo/agent` | новый reusable contract |
| generic AgentRuntime ABI | `adgo/agent` | provider-neutral contract |

## 2.2 Должно остаться в `agctl`

- `MASTER_PLAN.md` semantics и engineering process;
- repository discovery и codebase analysis;
- Git/worktree implementation;
- base/head/qualified-tree semantics;
- Codex App Server/CLI adapter;
- Antigravity IDE/Bridge adapter;
- Claude/OpenHands/Factory adapters, если появятся;
- coding roles: planner/implementer/tester/reviewer/acceptance judge;
- `go test`, `-race`, fuzz, mutation testing, linters, security scans;
- project-specific verification matrix;
- commit coordinator и hard single-writer `main` policy;
- GitHub PR/CI/publication workflow;
- coding-specific dashboards/CLI/UI;
- repository-resident checkpoints, которые относятся именно к инженерному плану;
- преобразование findings в `MASTER_PLAN` tasks.

---

# 3. Upstream-first rule

С этого момента действует обязательное правило:

```text
Если новая feature не содержит прямых понятий
Git / repository / code / IDE / build / test / commit / branch /
MASTER_PLAN / developer workflow,
то до реализации в agctl она классифицируется как кандидат в ADGO.
```

Решение для каждой новой capability:

```text
GENERIC      -> реализовать/усилить в Axiom/ADGO -> release -> consume in agctl
CODING-ONLY  -> реализовать в agctl
MIXED        -> generic contract в ADGO, coding adapter в agctl
```

Запрещено добавлять новый generic scheduler, lease, retry, agent-generation, provider-quota или recovery subsystem в `internal/harness` после начала upstream migration.

---

# 4. Миграционные инварианты

### AGM-001 — One authority per run
Каждый run помечен `HARNESS` или `ADGO`; dual authority запрещён.

### AGM-002 — No big-bang rewrite
Переход выполняется capability-by-capability и run-by-run.

### AGM-003 — Upstream before deletion
Дубликат из Harness удаляется только после реализации эквивалентной/лучшей возможности в ADGO, pin/release, integration tests и shadow equivalence.

### AGM-004 — No dual-store transaction
Ни один logical transition не требует атомарного commit одновременно в Harness SQLite и отдельный ADGO store.

### AGM-005 — New generic work upstream-first
После утверждения migration gate новые generic features сначала попадают в ADGO.

### AGM-006 — Coding semantics stay downstream
ADGO не знает `main`, `git worktree`, `go test` или `MASTER_PLAN.md`.

### AGM-007 — Stable adapter boundary
`agctl` не импортирует внутренности ADGO; только public versioned API через тонкий `internal/adgobridge`/coding layer.

### AGM-008 — Exact dependency pin
До стабильного v1 `agctl` использует конкретный tagged/pre-v1 release или exact reviewed commit; floating `main` запрещён.

### AGM-009 — Version migration explicit
Plan/API/storage migration выполняется явно и тестируется; скрытый reinterpretation persisted state запрещён.

### AGM-010 — Legacy drain before deletion
Harness runtime удаляется только когда число active Harness-owned runs равно нулю либо есть доказанный offline importer.

### AGM-011 — Evidence parity
Переход на ADGO не снижает требования к race/property/mutation/fault tests.

### AGM-012 — Restart proof
Каждый перенесённый runtime capability обязан переживать process restart до отключения legacy path.

### AGM-013 — Side effects remain at-least-once honest
Ни один слой не обещает exactly-once network I/O; ambiguity идёт в reconciliation.

### AGM-014 — Provider units stay native
TOKENS/REQUESTS/COST/FRACTION/OPAQUE не смешиваются без явного преобразования.

### AGM-015 — Context is a resource
LLM session/context не является workflow state и может безопасно ротироваться.

---

# 5. Целевая интеграционная модель

## 5.1 ADGO activity boundary

ADGO должен отдавать generic запрос:

```go
type ActivityRequest struct {
    ExecutionID    string
    NodeID         string
    TaskID         string
    Attempt        int
    Inputs         map[string]any
    ArtifactRefs   []ArtifactRef
    Budget         Budget
    IdempotencyKey string
    Metadata       map[string]string
}
```

## 5.2 `adgo/agent` boundary

```go
type AgentTask struct {
    Goal                 string
    ContextRefs          []ArtifactRef
    RequiredCapabilities []Capability
    ForbiddenCapabilities []Capability
    ProviderRequirements ProviderRequirements
    CheckpointRef        *ArtifactRef
    Budget               AgentBudget
}
```

## 5.3 Coding boundary в `agctl`

```go
type CodingTask struct {
    AgentTask
    Repository           RepositoryID
    BaseCommit           string
    WorktreeID           string
    MasterPlanDigest     string
    AcceptanceCriteria   []CodingCriterion
    VerificationCommands []string
    EngineeringRole      EngineeringRole
}
```

ADGO управляет lifecycle, а `agctl` — смыслом coding task.

---

# 6. Provider/runtime boundary

ADGO не должен содержать `RunCodex` или `RunAntigravity`.

Нужен reusable контракт уровня `adgo/agent`:

```go
type AgentRuntime interface {
    Capabilities(context.Context) (RuntimeCapabilities, error)
    Start(context.Context, AgentStartRequest) (RuntimeHandle, error)
    Resume(context.Context, AgentResumeRequest) (RuntimeHandle, error)
    Observe(context.Context, RuntimeID) (RuntimeObservation, error)
    Checkpoint(context.Context, RuntimeID) (RuntimeCheckpoint, error)
    Cancel(context.Context, RuntimeID, CancelMode) error
}
```

Реализации остаются downstream:

```text
agctl/internal/runtime/antigravity
agctl/internal/runtime/codex
agctl/internal/runtime/claude
```

Это позволяет Axiom быть независимой библиотекой и одновременно даёт всем приложениям единый long-running agent lifecycle.

---

# 7. Harness -> ADGO migration matrix

Перед переносом каждого package проводится code-level comparison: `ADGO better / Harness better / merge / downstream-only`.

Предварительная матрица:

| Harness area | Target | Strategy |
|---|---|---|
| `model` workflow/node/attempt | `adgo` | map semantics, удалить после parity |
| `engine` | `adgo.Engine` | новые runs на ADGO |
| `scheduler` | `adgo` scheduler | перенести отсутствующие fairness/resource свойства upstream |
| `lease` | `adgo` worker protocol | сравнить fencing edge cases |
| `retry` | `adgo` | сохранить error taxonomy/budgets |
| `events` | `adgo` history/watch | унифицировать causality IDs |
| `budget` | `adgo` | merge generic limits |
| `resource` | `adgo` admission/resource keys | merge |
| `artifact` | `adgo` artifact contract + downstream CAS where needed | разделить contract/storage |
| `executor/process` | downstream or reusable small package | не смешивать с orchestration |
| `executor/agent` | `adgo/agent` | переработать в durable AgentRun/Generation |
| `provider/demand` | `adgo/agent` | upstream |
| `provider/reservation` | `adgo/agent` | upstream |
| `provider/session` | `adgo/agent` | upstream |
| provider Antigravity/Codex observers | `agctl` adapters | downstream |
| workspace/worktrees | `agctl` | downstream |
| publication verifier | `agctl` | downstream |
| completion loop | удалить после ADGO mission flow | legacy only |

---

# 8. Этапы реализации в `agctl`

## AG-01 — Freeze generic Harness growth
**Priority:** P0.  
Добавить архитектурное правило upstream-first и запрет на новые parallel generic subsystems.

## AG-02 — Capability parity inventory
**P0.** Для каждого `internal/harness/*` package составить public-contract/semantics/test/benchmark matrix против ADGO.

## AG-03 — Go toolchain convergence
**P0.** Поднять `agctl` с Go 1.24.2 до совместимой с Axiom версии (целевой baseline — Go 1.26, если CI/Windows подтверждают совместимость).

## AG-04 — Add `internal/adgobridge`
**P0.** Один downstream compatibility boundary. Никаких scattered imports ADGO по application packages.

## AG-05 — Build dual-runtime test harness
**P0.** Один deterministic scenario runner должен уметь прогнать эквивалентный workflow на legacy Harness и ADGO и сравнить observable terminal/intermediate semantics.

## AG-06 — Adopt upstream agent provider primitives
**P0, blocked by ADGO U-04..U-09.** Переключить provider/account/model/capacity/demand/reservation/session types на `adgo/agent` adapters.

## AG-07 — Runtime adapter bridge
**P0.** Подключить Codex/Antigravity через `adgo/agent.AgentRuntime`, сохранив concrete implementations в `agctl`.

## AG-08 — Agent Generation adoption
**P0.** Logical AgentRun и context rotations управляются ADGO; repository/worktree bindings остаются в coding metadata.

## AG-09 — Coding mission compiler
**P0.** `MASTER_PLAN`/planner -> immutable ADGO definition/plan + coding activity registry.

## AG-10 — ADGO-owned new runs shadow mode
**P0.** Новые non-write missions запускаются через ADGO, legacy Harness параллельно вычисляет только read-only oracle при необходимости.

## AG-11 — Isolated write pilot
**P0.** ADGO управляет lifecycle write activities, но все записи идут в отдельные worktrees; publication ещё остаётся отдельным `agctl` gate.

## AG-12 — Acceptance/repair integration
**P0.** Использовать ADGO targeted repair/revision/oscillation protection; coding gates поставляют violations/repair roots.

## AG-13 — Provider failover + context rotation qualification
**P0.** Kill session/provider, exhaust quota, rotate generation, проверить отсутствие потерянной/двойной работы.

## AG-14 — Legacy run drain
**P1.** Новые runs больше не создаются Harness; старые завершаются на старом engine.

## AG-15 — Delete duplicated provider core
**P1.** После upstream parity удалить local demand/reservation/session generic implementations, оставив adapters.

## AG-16 — Delete duplicated durable runtime
**P1.** По package-группам удалить engine/scheduler/lease/retry generic code после zero-active-run + test parity.

## AG-17 — 24h endurance qualification
**P0.** ADGO-backed `agctl` проходит fault matrix 24h.

## AG-18 — 72h soak + resource leak qualification
**P0.** Memory/file descriptors/goroutines/history growth/quota accounting/recovery стабильны 72h.

## AG-19 — Single-writer main finalization
**P0.** Только coding Commit Coordinator публикует qualified tree; ADGO не получает generic `main` authority.

## AG-20 — Final convergence audit
**P0.** В `agctl` не осталось универсального workflow engine, который дублирует ADGO.

---

# 9. Синхронизация с upstream ADGO roadmap

Cross-repo dependencies:

```text
ADGO U-01..U-03  architecture/store contracts
        |
        +--> AG-03/AG-04

ADGO U-04..U-09  adgo/agent provider/quota/session foundation
        |
        +--> AG-06/AG-07

ADGO U-10..U-12  generation/checkpoint/context lifecycle
        |
        +--> AG-08/AG-13

ADGO U-13..U-15  SQLite + importer/compat + version/release
        |
        +--> AG-09..AG-16

ADGO U-16..U-19  fault/model/mutation/endurance qualification
        |
        +--> AG-17/AG-18
```

`agctl` не должен обходить upstream blocker локальной копией generic feature, кроме временного experiment behind build tag, который не становится production authority.

---

# 10. Migration testing

Для каждой переключаемой capability обязательны:

1. characterization tests старого Harness;
2. ADGO contract tests;
3. deterministic differential tests;
4. property/fuzz where applicable;
5. race tests;
6. mutation tests на критических guards;
7. crash-before-commit / crash-after-effect / stale-worker tests;
8. reopen/restart test;
9. Windows + Linux;
10. benchmark comparison;
11. migration/rollback drill.

Минимальные fault cases:

```text
worker dies before dispatch
worker dies during provider call
provider accepted effect, local commit lost
lease expires, zombie returns
coordinator SIGKILL
host restart
quota observation stale
quota exhausted after reservation
context reaches rotation threshold
checkpoint write fails
session disappears
provider changes model catalog
CI callback duplicated
GitHub unavailable
review rejects result
repair oscillates
remote main moves before publication
```

---

# 11. Rollout states

```text
LEGACY_ONLY
    -> DUAL_TEST
    -> ADGO_SHADOW
    -> ADGO_READ_ONLY
    -> ADGO_ISOLATED_WRITE
    -> ADGO_DEFAULT_NEW_RUNS
    -> LEGACY_DRAIN
    -> LEGACY_DISABLED
    -> LEGACY_DELETED
```

Переход между состояниями требует machine-verifiable evidence. Откат возвращает **новые** runs на предыдущий режим; уже созданный run не меняет engine authority посередине без отдельного migration protocol.

---

# 12. 24/7 Definition of Done

Система считается готовой, когда:

- ADGO — единственный generic durable engine для новых `agctl` missions;
- `agctl` содержит только coding-specific orchestration/adapters/UI;
- provider/session/quota/generation generic logic живёт в `adgo/agent`;
- ни один logical run не зависит от LLM conversation lifetime;
- restart любого `agctl` process восстанавливает работу из durable state;
- stale worker/generation не может commit;
- ambiguous side effects требуют reconciliation;
- context rotation происходит через durable checkpoint;
- provider failover не теряет и не удваивает работу;
- targeted repair не перезапускает unaffected subgraph;
- infinite repair/retry loops bounded;
- new plan revision применяется явно;
- `MASTER_PLAN`/Git остаются coding truth, но не заменяют durable runtime;
- только `agctl` Commit Coordinator имеет право писать `main`;
- 24h destructive endurance и 72h soak проходят без false DONE, lost state, duplicate effect или resource leak;
- legacy Harness disabled/deleted после drain;
- final audit не находит дублирующего general-purpose runtime в `agctl`.

---

# 13. Следующий исполнимый шаг

**Не начинать с удаления Harness.**

Первая практическая итерация:

1. upstream в Axiom: зафиксировать `adgo/agent` architecture и Store capability gaps;
2. в `agctl`: выполнить AG-02 capability parity inventory;
3. согласовать Go 1.26 migration;
4. реализовать маленький `internal/adgobridge` только для одного read-only workflow;
5. построить deterministic Harness-vs-ADGO differential test;
6. только после parity расширять ownership новых runs.

Это превращает уже написанный Harness не в выброшенный код, а в **R&D/reference implementation и migration oracle**, из которого лучшие generic механизмы поднимаются upstream в ADGO один раз и затем используются всеми продуктами.