# Axiom / ADGO — оценка применимости в `Antigravity-Progressive-Bootstrap`

> Репозиторий-цель: `Homiakus/Antigravity-Progressive-Bootstrap`  
> Исследованная библиотека: `Homiakus/axiom` (`main@f28c31464c2bf58306f29ddaff32dfe6017d5841`)  
> Цель: определить, какие части Axiom усиливают 24/7 Autonomous Coding OS, а какие создадут дублирование durable runtime и должны быть запрещены.

---

## 1. Решение

**Axiom применим выборочно. ADGO как второй orchestration runtime внутрь Harness встраивать нельзя.**

Правильная схема:

```text
Antigravity-Progressive-Bootstrap / Harness
              = единственный durable control plane

Axiom model/table/compiler
              = опциональный declarative policy / verification frontend

Axiom ADGO
              = reference implementation + источник проверенных алгоритмов
                + возможный differential/model-based test oracle
                НЕ второй production coordinator/store/scheduler
```

Главная причина: текущий Harness и ADGO решают почти одну и ту же фундаментальную задачу. Оба используют committed durable state вместо goroutine/call stack, DAG, coordinator/worker split, leases/fencing, retries, crash recovery, provider routing, budgets, human interaction и side-effect reconciliation. Если запустить один поверх другого, появятся два набора `Execution/Task/Lease/Retry/Store/Coordinator` и нарушится принцип единственного runtime source of truth.

### Итоговая оценка

| Вариант использования | Применимость | Решение |
|---|---:|---|
| `axiom/model` для декларативных invariants/policies | **8/10** | Да, после compatibility gate |
| `axiom/table` для decision tables | **7/10** | Да, опционально для operator/policy layer |
| Axiom compiler как статический verifier | **9/10** | Да, особенно в CI/model tests |
| Axiom history/replay как reference semantics | **7/10** | Да, преимущественно tests/tooling |
| `axiom.Flow` в durable core | **2/10** | Нет |
| `axiom.Engine` как вложенный execution engine | **3/10** | Обычно нет |
| `adgo.Runtime` как embedded production runtime | **2/10** | Нет |
| `adgo.Engine/Host/OpenProduction` внутри Harness | **1/10** | Запретить |
| ADGO как reference architecture | **10/10** | Да |
| ADGO как differential/fault-test oracle | **8/10** | Да |
| Переиспользование targeted-repair алгоритмов | **9/10** | Да, в Harness-native реализации |
| Continue-as-new semantics | **9/10** | Да, перенести в Harness |
| Plan migration compatibility ideas | **8/10** | Да |
| Stagnation/oscillation detection | **9/10** | Да |
| Hedging/ensemble для pure agent work | **7/10** | Позже, только после purity gate |

---

## 2. Почему прямое использование ADGO конфликтует с Harness

### 2.1 Дублирование workflow authority

Harness уже имеет собственные:

- `WorkflowDefinition` / `WorkflowRun`;
- `NodeRun`;
- immutable `Attempt`;
- worker registry;
- leases + fencing epochs;
- ready queue / scheduler;
- retries / retry budgets / circuit breakers;
- timers / signals / approvals;
- `EffectIntent` + `IN_DOUBT`;
- artifact CAS / provenance;
- workspace lifecycle / Git worktrees;
- provider assignment / reservation / usage ledger;
- durable events;
- remote worker protocol.

ADGO также владеет execution/node/task lifecycle, durable coordinator/worker protocol, leases, retries, scheduling, provider routing, budgets, human waits, reconciliation и storage.

Если сделать:

```text
Harness NodeRun
    -> ADGO Execution
        -> ADGO Task
            -> worker
```

возникают вопросы без безопасного однозначного ответа:

- чей task state authoritative;
- чей lease является fencing authority;
- кто решает retry;
- какой circuit breaker главный;
- кто владеет cancellation;
- какой `IN_DOUBT` authoritative;
- чей provider reservation удерживает quota;
- кто решает, что execution завершён;
- кто восстанавливает task после crash;
- какой store является recovery truth.

Поэтому **nested durable execution запрещается**.

### 2.2 Два coordinator loop

ADGO production engine специально имеет resilient coordinator и worker polling. Harness уже имеет durable scheduler/reconciler и проектируется как единственный supervisor 24/7.

Два coordinator-а создадут:

```text
Harness reconciler thinks: retry
ADGO coordinator thinks: wait

или

Harness thinks: failover provider
ADGO thinks: retry current provider
```

Даже если happy path работает, fault path становится недетерминированным.

### 2.3 Два storage lifecycle

`agctl` использует SQLite Harness Store как execution truth. ADGO предоставляет Memory/File/Pebble и extension interfaces. Создание отдельного Pebble state рядом с SQLite означает distributed transaction problem между двумя локальными databases.

Недопустима схема:

```text
SQLite commit succeeded
Pebble commit failed
```

или наоборот.

Нельзя требовать recovery из согласованности двух независимых локальных stores.

---

## 3. Где Axiom полезен без дублирования runtime

## 3.1 Declarative invariant compiler

Это лучший первый use case.

Критические Harness state machines можно описывать в `axiom/model` не как второй runtime, а как **исполняемую спецификацию**.

Примеры:

### AgentGeneration

```text
NEW -> STARTING -> RUNNING -> CHECKPOINTING -> ROTATED
                         \-> FAILED
                         \-> CANCELED
```

Claims:

- одновременно не более одного active write generation;
- `ROTATED` требует durable checkpoint;
- terminal generation не возвращается в RUNNING;
- stale generation не может принять completion;
- provider handoff не обходит `IN_DOUBT`.

### Acceptance

```text
EXECUTION_FINISHED
    !=
ACCEPTED
```

Claims:

- implementer self-report не является acceptance evidence;
- high-risk task требует independent verifier;
- evidence привязано к qualified tree + plan digest;
- stale evidence не проходит.

### Commit Coordinator

Claims:

- только coordinator role имеет publish authority;
- target branch == `main`;
- force запрещён;
- base ancestry должна быть доказана;
- tree должен совпадать с qualified tree;
- observed remote head должен соответствовать proof.

Такой Axiom model может жить в test/tooling package и использоваться для:

1. compile-time diagnostics;
2. generated transition matrix;
3. property tests;
4. mutation-test oracle;
5. documentation/diagram generation.

При этом production transition остаётся в Harness и SQLite.

---

## 3.2 Model-based / differential testing

Axiom особенно полезен как **вторая независимая формализация** части protocol semantics.

Схема:

```text
seeded event sequence
        |
        +-> Harness reducer
        |
        +-> Axiom reference model
                 |
                 v
        compare observable state
```

Хорошие кандидаты:

- generation lifecycle;
- acceptance state;
- publication eligibility;
- retry/failover decision state;
- provider health/circuit transitions;
- operator approval lifecycle.

Это полезнее, чем использовать ADGO как runtime dependency: различия становятся тестовым сигналом вместо production split-brain.

### Ограничение

Axiom и Harness развиваются одним владельцем и используют похожие идеи, поэтому такой oracle не полностью независим концептуально. Его нельзя считать единственным доказательством. Он дополняет property/fuzz/mutation/chaos tests.

---

## 3.3 Decision tables

`axiom/table` потенциально подходит для правил, которые оператор должен видеть и менять декларативно:

- risk -> required acceptance evidence;
- task class -> allowed provider classes;
- privacy class -> provider eligibility;
- repository sensitivity -> remote-worker permission;
- failure class -> retry / repair / failover / human escalation;
- context pressure -> reuse / checkpoint / rotate.

Но таблица **не должна непосредственно менять runtime state**.

Правильная граница:

```text
Axiom decision table
       |
       v
PolicyDecision
       |
Harness validates decision against hard invariants
       |
       v
Harness transition
```

То есть policy frontend может предложить решение, но authority остаётся у Harness.

---

## 3.4 Static validation для Mission / Workflow definitions

Axiom compiler умеет находить:

- missing references;
- cycles;
- conflicting writers;
- invalid policies;
- unsafe external-effect configuration;
- incomplete graph constraints.

Harness уже имеет собственный compiler, поэтому заменять его нельзя. Но можно использовать Axiom в CI как дополнительный **lint/model checker** для высокоуровневой инженерной policy model.

Наиболее полезный вариант — не компилировать весь Harness DAG второй раз, а моделировать только safety-critical control rules, где независимая проверка окупает дублирование спецификации.

---

## 4. Что из ADGO стоит перенести в 24/7 roadmap

ADGO содержит несколько механизмов, которые в текущем 24/7 плане либо отсутствуют, либо описаны менее полно.

## 4.1 Targeted dependency repair

ADGO использует:

```text
failed gate
   -> repair roots
   -> minimal dependency subgraph
   -> invalidate affected outputs
   -> revision epoch
   -> rerun only affected nodes
```

Это хорошо ложится на существующий Harness `GraphRevision`.

### Рекомендация

Не запускать ADGO repair engine. Реализовать Harness-native `RepairPlanner`, который:

1. принимает failed acceptance gate;
2. строит backward dependency cone до repair roots;
3. вычисляет минимальный affected subgraph;
4. сохраняет unaffected qualified outputs;
5. инвалидирует только affected artifacts/facts/cache entries;
6. создаёт новую GraphRevision;
7. ограничивает repair по iterations/cost/time;
8. пишет explainable rationale.

Это существенно уменьшит waste в 24-часовых циклах.

---

## 4.2 Stagnation / oscillation detection

Автоисправление может попасть в цикл:

```text
A -> B -> A -> B -> ...
```

или делать изменения без улучшения quality gate.

Нужно перенести идею ADGO:

- stable strategy signature;
- quality history;
- epsilon improvement;
- repeated-state detection;
- maximum repair iterations;
- maximum repair cost;
- maximum repair duration.

Это следует включить в T-043/T-047/T-048 qualification.

---

## 4.3 Continue-as-new

Для 24/72h runtime важно ограничивать рост одной execution history.

Harness должен получить собственный protocol:

```text
AgentRun / WorkflowRun A
     |
     | quiescent checkpoint
     v
WorkflowRun B
```

Переносить только:

- selected durable facts;
- artifact refs;
- plan/task identity;
- current MASTER_PLAN digest/checkpoint;
- unresolved blockers/findings;
- cumulative mission accounting.

Старый run остаётся immutable audit trail.

Это лучше бесконечного одного event stream.

---

## 4.4 Explicit plan migration

ADGO запрещает скрыто менять plan semantics у уже идущего execution.

Для Autonomous Coding OS это особенно важно, потому что `MASTER_PLAN.md` может изменяться во время работы.

Нужен Harness-native migration contract:

```text
old PlanDigest
      |
compatibility analysis
      |
quiescent point
      |
explicit GraphRevision / migration
      |
new PlanDigest
```

Нельзя просто дать старому worker продолжить по новому плану.

---

## 4.5 Pure-work hedging / ensemble

Полезно позже для:

- architecture review;
- code review;
- research;
- independent acceptance judges;
- flaky reasoning-heavy tasks.

Но только когда node помечен:

```text
Pure = true
No external effects
No mutable shared workspace write
```

Все запущенные варианты должны учитываться в quota/cost budget, а winner должен определяться deterministic acceptance policy.

Для implementation worker с записью в worktree speculative execution следует делать через **разные worktrees**, а не параллельную запись в один workspace.

---

## 5. Что из ADGO НЕ переносить автоматически

### 5.1 Provider router целиком

Harness уже имеет более специализированную модель provider/account/model/session/capacity/reservation и native-unit quota semantics.

ADGO EWMA reliability/quality/latency может быть полезен как дополнительный scoring signal, но не должен заменить существующие hard capacity/reservation contracts.

Правильный порядок:

```text
Harness hard filters
 -> native quota feasibility
 -> session/context feasibility
 -> reservation
 -> optional adaptive reliability utility
```

Не наоборот.

### 5.2 ADGO admission как замена reservation ledger

Generic admission полезен для общих ресурсов, но provider quota уже моделируется отдельными native-unit reservations. Нельзя свести токены/requests/cost/opaque windows к одному generic token bucket.

Admission можно использовать как идею для:

- browser concurrency;
- build slots;
- external search QPS;
- local GPU slots;
- shared CPU-heavy tools.

Provider quota остаётся в provider subsystem.

### 5.3 ADGO Store

Не подключать Pebble/File store для execution truth. Harness SQLite остаётся authoritative.

### 5.4 ADGO worker protocol

Не создавать второй Poll/Claim/Heartbeat/Complete protocol. Использовать существующие Harness Attempt/Lease/Worker contracts.

---

## 6. Технические blockers перед любым runtime import

## 6.1 Go toolchain mismatch

Текущий Axiom требует:

```text
Go 1.26+
```

Текущий `agctl`:

```text
Go 1.24.2
```

Поэтому прямой import сейчас означает toolchain migration всего `agctl`.

### Gate GO-AX-001

Перед подключением dependency:

- отдельная задача upgrade Go toolchain;
- Linux + Windows CI;
- `go test ./...`;
- `go test -race ./...`;
- `go vet ./...`;
- benchmark comparison;
- packaging/install smoke;
- проверить Antigravity integration и native Windows process handling.

До этого Axiom можно использовать только как внешний test/tool process, собранный отдельным module/toolchain, либо не подключать вовсе.

---

## 6.2 Pre-v1 API

Axiom остаётся pre-v1. Minor версии могут менять public API.

На момент исследования GitHub Releases отсутствуют; `main` нельзя считать стабильной production dependency.

### Gate AX-COMPAT-001

Если dependency всё же вводится до v1:

1. pin exact commit/tag;
2. запретить `@latest`;
3. обернуть Axiom собственным `internal/axiombridge`;
4. core packages не импортируют Axiom напрямую;
5. добавить external-consumer/compat test;
6. upgrade Axiom только отдельной квалифицированной задачей.

---

## 6.3 Dependency footprint

Axiom module включает Pebble и связанные зависимости. Даже если production binary не линкует неиспользуемые пакеты, module graph и supply-chain surface увеличатся.

Поэтому предпочтительно сначала использовать Axiom в:

```text
internal/verification/axiommodel
cmd/agctl-modelcheck
CI tooling
```

а не в hot runtime path.

Если этот use case подтвердит ценность, имеет смысл отдельно рассмотреть выделение из Axiom лёгкого pure compiler/model submodule без storage runtime dependencies.

---

## 7. Рекомендуемая архитектура интеграции

```text
                       MASTER_PLAN.md
                              |
                       Harness compiler
                              |
                       Durable Harness DAG
                              |
             +----------------+----------------+
             |                                 |
     production runtime                 verification/tooling
             |                                 |
   SQLite / Scheduler /                Axiom declarative model
 Attempts / Leases / Effects                     |
             |                           compile / claims
             |                                 |
 Provider routing / execution          Reference transitions
             |                                 |
             +----------------+----------------+
                              |
                     differential checks
```

Критическое правило:

> Axiom может проверять или предлагать policy decision, но Axiom execution state никогда не является обязательным условием восстановления Harness workflow.

После `kill -9 agctl` для recovery достаточно Harness SQLite + Git + artifacts + repository checkpoint. Отсутствие Axiom runtime database не должно иметь значения.

---

## 8. Предлагаемый adapter boundary

Если Axiom появляется как dependency, скрыть его за узким интерфейсом:

```go
type SafetyModel interface {
    ValidateTransition(ctx context.Context, in TransitionInput) (TransitionDecision, error)
    Explain(ctx context.Context, in TransitionInput) (Explanation, error)
}
```

или для policy:

```go
type PolicyEvaluator interface {
    Evaluate(ctx context.Context, facts PolicyFacts) (PolicyDecision, error)
}
```

Внутри может быть Axiom, но Harness core знает только собственные domain types.

Запрещено протаскивать наружу:

- `axiom.Plan`;
- `axiom.Engine`;
- ADGO `Execution`;
- ADGO `Task`;
- ADGO Store;
- ADGO worker IDs/leases.

Это сохранит возможность удалить/заменить Axiom без migration execution database.

---

## 9. Pilot plan

### AX-P0 — No-dependency reference study

Статус: можно делать сразу.

- сопоставить Harness invariants с ADGO architecture;
- добавить отсутствующие идеи в 24/7 roadmap;
- не менять `go.mod`.

### AX-P1 — External model-check prototype

- создать маленькую Axiom model для `AgentGeneration` lifecycle;
- запускать как отдельный CI tool/module на Go 1.26;
- generated transition/property vectors подавать Harness tests;
- production binary не зависит от Axiom.

**Success:** модель убивает минимум один намеренный mutant в Harness transition policy, который обычный happy-path test не ловит.

### AX-P2 — Toolchain decision

После текущей multi-provider цепочки определить, оправдан ли upgrade `agctl` до Go 1.26.

Не повышать Go только ради Axiom, если нет измеримой пользы от AX-P1.

### AX-P3 — Pinned library bridge

Если AX-P1 полезен:

- pin exact Axiom version/commit;
- `internal/axiombridge`;
- использовать только compiler/model/table APIs;
- no ADGO production runtime.

### AX-P4 — Optional operator policy frontend

После стабилизации T-012/T-042:

- risk/acceptance/provider eligibility tables;
- Axiom produces decision + explanation;
- Harness revalidates hard invariants before transition.

### AX-P5 — Differential chaos oracle

В T-047 fault framework:

- небольшие synthetic workflows прогоняются через Harness и ADGO reference;
- сравниваются fencing/retry/recovery invariants;
- расхождение сохраняется как replayable seed.

Не использовать ADGO oracle для Git publication/effect semantics, где Harness имеет более специализированные контракты.

---

## 10. Изменения, рекомендуемые в 24/7 roadmap

Без изменения существующих T-ID рекомендуется уточнить scope:

### T-031
Добавить technology decision:

- Harness — единственный durable execution authority;
- nested ADGO runtime запрещён;
- Axiom разрешён только через bounded adapter/tooling boundary.

### T-037
Использовать Axiom model как возможный test oracle для generation rotation invariants.

### T-041
Добавить stagnation/oscillation signatures и bounded repair semantics из ADGO.

### T-042
Рассмотреть declarative acceptance policy через Axiom model/table, но final acceptance transition остаётся Harness-native.

### T-043
Добавить targeted repair subgraph вместо полного повторения workflow.

### T-047
Добавить optional ADGO differential oracle для synthetic durable workflows.

### T-048/T-049
Добавить history-growth gate и Harness-native continue-as-new qualification.

### T-050
Final audit должен доказать отсутствие ADGO execution/store/scheduler authority в production path.

---

## 11. Новые архитектурные инварианты

### AX-I-001 — Single durable engine
В одном `agctl` workflow существует ровно один authoritative durable execution engine: Harness.

### AX-I-002 — No nested ADGO authority
ADGO Execution/Task/Lease не используются как production authority внутри Harness Attempt/NodeRun.

### AX-I-003 — Axiom is optional for recovery
Удаление/недоступность Axiom tooling не мешает восстановлению live Harness workflow.

### AX-I-004 — Bridge isolation
Production core не импортирует Axiom напрямую; integration идёт через узкий adapter boundary.

### AX-I-005 — Harness validates final decision
Любое решение Axiom policy model повторно проверяется Harness hard invariants до state mutation.

### AX-I-006 — Pinned pre-v1 dependency
До стабильного compatibility baseline dependency pin обязателен; floating `main/latest` запрещён.

### AX-I-007 — No second execution store
Axiom Pebble/File store не хранит authoritative `agctl` execution state.

### AX-I-008 — External effects remain Harness-owned
Axiom/ADGO не владеет Git publish, GitHub mutation или другими Harness `EffectIntent` operations.

### AX-I-009 — Model oracle is not acceptance evidence by itself
Совпадение с Axiom reference model не заменяет реальные tests/security/race/mutation/integration evidence.

### AX-I-010 — Toolchain upgrade is independently justified
Go upgrade проходит собственную regression/performance qualification и не скрывается внутри «подключения библиотеки».

---

## 12. Финальная рекомендация

### Делать

1. Использовать ADGO как **reference architecture** для 24/7 evolution.
2. Перенести в Harness-native design:
   - targeted repair;
   - stagnation/oscillation detection;
   - continue-as-new;
   - explicit plan migration;
   - pure-work hedging/ensemble после строгой purity classification.
3. Пилотировать `axiom/model` как **test/model-check layer**, а не production engine.
4. После доказанной пользы — рассмотреть `axiom/table/model` для explainable operator policies через собственный adapter.
5. Сохранять SQLite Harness Store единственным runtime truth.

### Не делать

1. Не запускать `adgo.Engine`, `Host` или `OpenProduction` внутри Harness workflow.
2. Не заводить отдельный ADGO Pebble execution store.
3. Не заменять Harness Attempt/Lease/Retry/EffectIntent моделями ADGO.
4. Не заменять provider native-unit reservation generic ADGO admission/router.
5. Не импортировать unreleased Axiom `main` напрямую в десятки core packages.
6. Не повышать Go toolchain только ради экспериментальной зависимости без pilot evidence.

### Целевое положение

```text
                 Axiom / ADGO
                /            \
       model checker        algorithm/reference source
             |                       |
             v                       v
  declarative safety model     Harness-native features
             \                       /
              +----------+-----------+
                         |
                    agctl Harness
                         |
              single durable authority
                         |
       SQLite + Git + CAS + MASTER_PLAN
```

**Итог:** Axiom способен заметно усилить качество и формальную проверяемость 24/7 Coding OS, но максимальная польза достигается при *неиспользовании ADGO как второго runtime*. Лучшее применение — declarative safety models, decision tables, model-based testing и перенос доказавших себя orchestration algorithms в уже существующий Harness.