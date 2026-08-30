# Axiom / ADGO — архитектурное решение для `Antigravity-Progressive-Bootstrap`

> Статус: **REVISED / authoritative decision**  
> Причина пересмотра: `Homiakus/axiom`/ADGO является собственной развиваемой reusable библиотекой и должна стать общим фундаментом, а не внешним независимым runtime.  
> Подробный downstream plan: `AUTONOMOUS_CODING_OS_24X7_PLAN_RU.md`.  
> Подробный upstream plan: `Homiakus/axiom/adgo/AGENT_PLATFORM_PLAN_RU.md`.

---

## 1. Новое решение

Предыдущая версия документа правильно обнаружила главный риск — **два authoritative durable engines нельзя запускать для одного logical workflow**. Ошибочным было только долгосрочное назначение authority.

Правильная стратегия:

```text
CURRENT TRANSITION STATE
agctl Harness = legacy/reference durable runtime
ADGO          = reusable runtime under active upstream hardening

TARGET STATE
ADGO          = single generic durable execution authority
ADGO Agent    = reusable long-running agent/provider/session/quota layer
agctl         = Autonomous Coding OS / domain product layer
```

То есть ADGO не встраивается *внутрь* Harness. Вместо этого generic возможности Harness поэтапно поднимаются upstream в ADGO, после чего дубликаты в `agctl` удаляются.

---

## 2. Почему это лучше

Оба проекта уже имеют или требуют одинаковые классы механизмов:

- durable execution graph;
- tasks/attempts;
- leases/fencing;
- retries;
- budgets;
- timers/signals;
- side-effect idempotency/reconciliation;
- artifacts/history;
- provider routing;
- crash recovery;
- repair/revision;
- long-running agent lifecycle.

Поддержка двух реализаций означает двойную стоимость:

```text
feature design x2
tests x2
fault semantics x2
migration bugs x2
benchmarks x2
security review x2
```

И хуже всего — implementations со временем расходятся.

Если generic feature сначала создаётся в ADGO, её могут использовать `agctl` и другие приложения без копирования.

---

## 3. Целевая граница

### Axiom

- declarative models;
- claims/invariants;
- rules;
- compiler/static validation;
- decision-table frontends.

### ADGO core

- durable DAG/execution;
- coordinator/worker protocol;
- leases/fencing;
- retry/backoff;
- generic budgets/admission/resources;
- waits/signals/human decisions;
- external-effect reconciliation;
- targeted repair;
- stagnation/oscillation protection;
- artifacts/history;
- versions/migration/continue-as-new;
- storage interfaces/backends.

### ADGO Agent

- AgentRun/Generation;
- durable checkpoint contract;
- provider/account/model/session identities;
- capacity/quota observations;
- native quota units;
- demand estimation;
- atomic reservations;
- usage settlement;
- context/session broker;
- provider-neutral AgentRuntime ABI;
- agent-level routing/explainability.

### `agctl`

- `MASTER_PLAN.md`;
- Git/repositories/worktrees;
- Codex/Antigravity/Claude concrete adapters;
- coding roles;
- build/test/race/fuzz/mutation/security gates;
- coding acceptance;
- findings -> plan updates;
- commit coordinator;
- GitHub publication to `main`;
- coding-specific CLI/UI/dashboard.

---

## 4. Что делать с текущим Harness

Текущий Harness не выбрасывается. Он становится одновременно:

1. legacy runtime для уже созданных runs;
2. source implementation для generic capabilities, которых ADGO пока не имеет;
3. characterization oracle;
4. differential-testing reference во время migration;
5. источник тестов/invariants/benchmarks, которые надо перенести upstream.

Для каждого блока выполняется comparison:

```text
ADGO better
  -> consume ADGO

Harness better
  -> generalize + upstream to ADGO

both useful
  -> merge contracts upstream

coding-specific
  -> keep in agctl
```

После parity и drain соответствующий generic Harness package удаляется.

---

## 5. Что особенно важно перенести upstream

Наиболее ценные разработки `agctl`, которые должны стать частью `adgo/agent`:

- explicit Provider/Account/Model/Session model;
- native quota metrics `TOKENS / REQUESTS / COST / FRACTION / OPAQUE`;
- conservative scoped demand estimation;
- complete active-claim accounting;
- atomic multi-window quota reservation;
- classified actual usage settlement;
- session/context broker `REUSE / NEW / CHECKPOINT_AND_NEW / UNAVAILABLE`;
- no-inference affinity rules;
- context hysteresis;
- AgentRun/Generation fencing;
- durable context checkpoint/rotation.

Это универсальные LLM-agent capabilities, а не coding-only code.

---

## 6. Что не надо переносить upstream

ADGO не должен получить знания о:

- `main` branch;
- Git worktrees;
- GitHub publication;
- `go test`;
- mutation testing commands;
- `MASTER_PLAN.md` format;
- Codex CLI flags;
- Antigravity IDE UI/automation;
- coding-specific Definition of Done.

Для них upstream предоставляет generic activity/runtime/policy contracts, а implementation остаётся downstream.

---

## 7. Migration safety

Главный invariant сохраняется:

```text
one logical run -> one durable engine authority
```

Допустимо временно:

```text
legacy Run A -> Harness-owned
new Run B    -> ADGO-owned
```

Недопустимо:

```text
Run A -> Harness coordinator + ADGO coordinator одновременно
```

Также запрещена необходимость distributed transaction между двумя stores для одного logical transition.

---

## 8. Storage decision

Нельзя просто заменить зрелый Harness SQLite path на существующий ADGO Pebble path и потерять operational semantics.

Правильный путь:

1. вывести generic storage contracts из Harness;
2. сохранить ADGO capability-interface design;
3. создать/усилить SQLite backend upstream;
4. перенести нужные migration/crash/CAS/query invariants;
5. не тащить coding-specific schema;
6. затем переключить `agctl` на ADGO store contracts.

---

## 9. Version/toolchain decision

Axiom pre-v1 и использует Go 1.26; `agctl` исторически Go 1.24.2.

Так как оба проекта развиваются согласованно:

- toolchain нужно унифицировать после CI qualification;
- public ADGO Agent API нужно выпускать через reviewed tag/frozen commit;
- `agctl` pin-ит конкретную версию;
- floating dependency на `main` запрещена;
- breaking pre-v1 changes сопровождаются migration notes и external consumer tests.

---

## 10. Итог

Финальная архитектурная формула:

```text
Axiom       = formal/declarative foundation
ADGO        = reusable durable workflow kernel
ADGO Agent  = reusable long-running AI/agent kernel
agctl       = autonomous coding product
```

Поэтому текущая задача — **не интегрировать ADGO как второй runtime и не переписывать ADGO-возможности внутри Harness**, а провести контролируемую upstream extraction + strangler migration.

Подробная последовательность находится в:

- `AUTONOMOUS_CODING_OS_24X7_PLAN_RU.md` — downstream migration в `agctl`;
- `Homiakus/axiom/adgo/AGENT_PLATFORM_PLAN_RU.md` — upstream development в ADGO.
