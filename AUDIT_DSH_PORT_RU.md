# Аудит проекта и исследование портирования агента agctl в DeepSeek Harness

Дата: **2026-08-21**
Автор: агент DSH (проверено по исходному коду обоих репозиториев)
Статус: **аудит выполнен + исследование портирования (3 стратегии, рекомендация)**

> Дисклеймер проверки: выводы ниже основаны на чтении исходников
> `D:\Programms\Antigravity-Progressive-Bootstrap` (agctl) и
> `D:\Programms\deepseek-harness` (DSH), а также на запуске:
> `go build ./...`, `go vet ./...`, `go test ./...` — **все проходят (exit 0)**
> на текущем рабочем дереве (с незакоммиченным WIP Stages 18–20).

---

## 1. Резюме

**Что это за проект.** `Antigravity-Progressive-Bootstrap` — это **agctl**:
control-plane и оркестратор на Go (v3.2.1) для Google Antigravity (AGY),
поверх которого идёт миграция к **собственному Durable Execution Harness**
(реализованы Stages 0–20 по плану `HARNESS_IMPLEMENTATION_PLAN_RU.md`).

**Что значит «переделать агента для DSH».** «Агент» здесь — это не один
файл, а связка из трёх слоёв:

1. **Runtime-ядро**: Go CLI/TUI `agctl.exe` + SQLite durable-движок
   (22 053 строки Go в `internal/harness`).
2. **Интеграция с рантаймом-исполнителем**: сейчас это AGY
   (Antigravity), в будущем — любой исполняющий слой.
3. **Поверхность для LLM**: набор навыков `.agents/skills/*/SKILL.md`
   (формат slash-команд), которые агент вызывает через `run_command`.

DSH (DeepSeek Harness) — это **плагинная агентная среда на Cordis**:
«агент» в DSH — это **композиция плагинов** (preset / bundle), а не
отдельная программа. Ключевой факт исследования: **почти все функции
agctl уже имеют нативные аналоги в DSH** (`goal`, `workflow`, `jobs`,
`plan`, `todo`, `skill`, `mcp`, `interaction`/permissions, `guard`,
`runtime-diagnostics`, `apps/web`). Поэтому «переделка» — это не
переписывание движка, а **перенос управляющих политик и навыков** на
нативные extension points DSH.

**Рекомендация (по возрастанию усилий):**

| Стратегия | Суть | Усилия | Когда |
|---|---|---|---|
| **A. Skill-мост** | Навыки agctl уже в формате DSH (`.agents/skills`), бинарь остаётся sidecar | 0.5–2 дня | немедленно, работает уже сегодня |
| **C. Гибрид** | Go harness как durable sidecar-сервис + DSH-плагин-клиент | 1–2 недели | если нужно сохранить SQLite-движок как есть |
| **B. Нативный порт** | Переписать управляющие политики как DSH-плагины (goal/workflow/jobs/…), Go-движок вывести из эксплуатации | 3–6 недель | целевая архитектура на длинный срок |

Оптимальный путь: **A сегодня → B как целевая линия**, C — только если
durable-семантика `IN_DOUBT`/reconciliation нужна дословно и её нельзя
воспроизвести поверх `dsh-workflow`.

---

## 2. Аудит проекта agctl

### 2.1 Что проверено фактически

| Проверка | Результат |
|---|---|
| `go build ./...` | ✅ exit 0 |
| `go vet ./...` | ✅ exit 0 |
| `go test ./...` | ✅ exit 0 |
| `go test -race ./...` | ✅ exit 0 (полный прогон с детектором гонок) |
| Git-состояние | ветка `main`, **незакоммиченный WIP** Stages 18–20 |
| Структура | `cmd/agctl` (main + harness shim), 34 пакета `internal/`, 26 подпакетов `internal/harness` |

### 2.2 Количественные показатели

```text
Go-код вне harness:        90 файлов, ~16 342 строк
Go-код internal/harness:  195 файлов, ~22 053 строки
Тестовые файлы:           111 шт. (*_test.go)
Зависимости (go.mod):     bubbletea, lipgloss, bubbles (TUI);
                          modernc.org/sqlite (pure-Go SQLite, см. ADR-0001);
                          больше ничего существенного
```

Минимализм зависимостей — сильная сторона: никаких тяжёлых фреймворков,
SQLite встроен нативно (без CGO), всё остальное — стандартная библиотека.

### 2.3 Сильные стороны

1. **Durable Execution Harness уровня production** (Stages 0–20):
   - типизированные ID, централизованные state transitions и инварианты;
   - транзакционный SQLite store (WAL, versioned migrations v1–v10, event
     journal/outbox);
   - immutable Attempts + durable leases + fencing epochs;
   - retry-таксономия, circuit breaker, budgets;
   - durable timers/signals/pause/approvals с TTL;
   - **side effects с идемпотентностью и явным состоянием `IN_DOUBT`** —
     это редкая и ценная семантика, которой нет «из коробки» почти нигде;
   - artifact CAS + provenance (v8), node cache (v9), workspaces (v10);
   - policy engine (доверие воркеров, SecretRef-изоляция), telemetry
     Explainer, REST/SSE API, remote workers, интерфейсы Store/Tx под
     Postgres/HA (Stage 20).
2. **Strangler-миграция без big-bang**: legacy JSON-путь (`ExecutionPlan` →
   `TaskRecord`) живёт рядом, `agctl harness …` — временный shim, всё
   собирается и тестируется на каждом шаге.
3. **Дисциплина верификации**: 100% тестов с `-race`, `go vet` чист,
   fault/race-тесты в каждом stage; критерий завершения миграции (раздел 34
   плана) — это автоматический kill-and-recover тест.
4. **Аудит совместимости** (AUDIT_RU.md): реальная сверка контрактов AGY/MCP,
   честные границы (WebSocket MCP probe, GUI-only permissions — явные
   limitations, а не «всё работает»).
5. **Хорошая документация**: ARCHITECTURE_RU, AUDIT_RU, MIGRATION_RU,
   ADR-0001, и живой 3298-строчный план реализации с authoritative
   checkpoint-блоком.

### 2.4 Слабые места и риски

| Риск | Детали |
|---|---|
| **Незакоммиченный WIP** | Stages 18–20 (`api`, `artifact`, `budget`, `policy`, `telemetry`, `worker/remote`, `workspace`, `executor/agent`, `executor/mcp`, `replan/harness_replan`) лежат в рабочем дереве не закоммиченными. Перенос на другую машину/ветку потеряет их. **Первое действие — закоммитить.** |
| **Двойная жизнь runtime** | Код по-прежнему несёт два слоя: legacy JSON-задачи и durable harness. Пока не закончен переход, каждая фича может жить в двух реализациях. |
| **Всё завязано на AGY** | Executor `agy` (headless Antigravity), пути `~/.gemini/config/*`, hook-контракты AGY. Весь слой интеграции бесполезен вне экосистемы Antigravity — при порте в DSH его нужно заменить на `ctx.subprocess`/`ctx.shell`/`ctx.mcp`. |
| **Одиночный разработчик-паттерн** | Один автор, крупные односоставные коммиты, документация только на русском для внутренних решений (README двуязычный). Для open-source вклада — барьер. |
| **Масштаб** | ~38k строк Go — это уже не «утилита», а продукт. Поддержка требует темпов выпуска и CI (в репо есть `.github`), но внешнего CI-прогона я не проверял. |

### 2.5 Вердикт аудита

Кодовая база в хорошем состоянии: **собирается, проходит vet и все тесты**,
архитектура продумана, durable-ядро — сильный актив. Главные операционные
долги: **незакоммиченный WIP** и **двойная (legacy+harness) runtime-модель**,
которую нужно закрыть либо завершением миграции, либо портом в DSH.

---

## 3. Что такое «агент» в agctl и в DSH

### 3.1 В agctl (сейчас)

```text
Пользователь → Antigravity (AGY) — reasoning/tools/subagents
                    │
                    ▼
              agctl 3.2 (Go control plane)
   discovery │ orchestration │ governance (router/loop/replan/permissions)
                    │
                    ▼
            Durable Harness 4.x (SQLite workflow engine)
                    │
                    ▼
   .agents/skills/*/SKILL.md  (slash-команды: /agctl-run, /agctl-doctor, …)
```

LLM-поверхность агента — это **навыки-обёртки** (`SKILL.md`), которые говорят
агенту «выполни `agctl <subcommand>` через `run_command` и красиво
отформатируй результат». Сам «мозг» (reasoning) живёт в AGY; agctl добавляет
governance и verified completion.

### 3.2 В DSH

```text
dsh --profile web|headless
   = Cordis plugin tree (всё — плагин, нет привилегированного ядра)
   ├─ dsh-base (bundle): llm, session, tools, agent-loop, fs, shell,
   │                     subprocess, mcp, sandbox, permission, credentials…
   ├─ agent preset (packages/preset/agent-presets): per-agent композиция
   │     └─ dsh-persona: строка персоны (может shadow системный промпт)
   ├─ dsh-goal + goal-round-driver: same-session objective (аналог Loop)
   ├─ dsh-workflow (+ worker-thread provider): durable-подобные workflow
   ├─ dsh-jobs (+ jobs-local, tool-jobs): фоновая работа (аналог Task Queue)
   ├─ dsh-plan / dsh-todo: план-режим и todo_write
   ├─ dsh-skill (+ skill-filesystem, tool-skill): каталог навыков
   ├─ dsh-interaction: approvals/commands/ask-user
   └─ apps/web: Web GUI (это GUI, в котором работает текущая сессия)
```

Модель-видимый вход **обязан** быть в session-логе (durable events), а
поведение добавляется через документированные extension points
(`ctx.tools`, `ctx.jobs`, `ctx.goals`, `agent/*` events, waterfall-хуки).
Это фундаментальное отличие: в DSH нет «агента-программы», есть **композиция
плагинов**, которые обслуживают любого агента.

---

## 4. Карта соответствия: agctl → DSH

| Функция agctl | Реализация в agctl | Нативный аналог в DSH | Что делать |
|---|---|---|---|
| Autonomous Loop (`plan→implement→test→diagnose→fix→reverify→DoD`) | `internal/loop`, `internal/harness/budget`, `executor/agent` | `packages/goal` (`ctx.goals`), `goal-round-driver`, `agent-loop`, `guard` (loop-hygiene), `budget`-подобные лимиты | Перенести политику профилей (`standard/deep/until-done/unrestricted`) в goal-round-driver поверх `ctx.goals`; лимиты шагов/токенов — в guard/политику |
| Adaptive DAG Replanning (`GraphRevision`, `ApplyGraphMutation`, diagnose→repair→reverify, no-progress detector) | `internal/replan`, `internal/planner`, `internal/harness/engine` (mutation.go) | `packages/workflow` (workflow + worker-thread provider), `packages/plan` (plan-режим как состояние) | Адаптивная мутация графа — новый плагин поверх `dsh-workflow` (добавление нод, rewiring, отмена ветвей, no-progress detection через события) |
| Task Queue (headless, resource-aware scheduler, retries, `.run.lock`) | `internal/tasks`, `internal/harness/scheduler` | `packages/jobs` (`ctx.jobs` + `jobs-local` + `tool-jobs`) | Переписать producer'ов задач на `JobRegistry`; CPU/слоты-лимиты — admission policy провайдера |
| Planner (initial DAG из prompt+capabilities) | `internal/planner` | `packages/plan`, `packages/todo` | План-режим DSH + todo; capability ranking — контекст-плагин |
| MCP manager + Live Probe (2026-07-28 / 2025-11-25) | `internal/mcp`, `internal/mcpprobe` | `packages/mcp` | DSH уже имеет MCP-капабилити; probe-логика (stateless discover, fallback) переносится в диагностический плагин |
| Skills & Packs manager (provenance SHA-256, sync) | `internal/skills`, `internal/provenance` | `packages/skill` (registry + filesystem provider + tool-skill) | Навыки agctl уже в формате DSH — подключить как есть; provenance — новый плагин поверх `skills/change` |
| Permissions / Security Score / backup | `internal/permissions`, `internal/securityaudit`, `internal/backup` | `packages/interaction` (permission, commands, ask-user), `packages/guard`, `packages/storage` | Политики — в permission-провайдер; security score — новый плагин (аудит composition); backup — поверх session/storage |
| Adaptive Router (выбор инструментов) | `internal/router` | `packages/preset` (agent-presets), `packages/context` | Роутинг «какие инструменты видит агент» — это композиция preset'ов; динамический роутер — контекст/пре-степ плагин |
| Doctor (диагностика окружения) | `internal/doctor` | `packages/runtime-diagnostics` | Перенести проверки (пути, зависимости, hook I/O) как runtime-diagnostics-провайдер |
| Dashboard (веб-мониторинг) | `internal/dashboard`, `internal/harness/api` (REST/SSE) | `apps/web` (Web GUI) + `packages/session` (events) | Аггрегация планов/тасков/телеметрии — веб-панель поверх session-лога и SSE |
| Worktree lanes | `internal/worktree`, `internal/harness/workspace` | `packages/workspace`, `packages/hooks` (git hooks) | Перенести политику «read-only parent + parallel write worktrees» в workspace-плагин |
| Headless AGY runner (`agy -p`, stream-json) | `internal/harness/executor/agy` | `packages/subprocess`, `packages/shell`, `packages/subagent` | Заменить AGY на DSH-subprocess/subagent; success-семантику (terminal result.status, soft denial) перенести в validator |
| Сlash-команды (`/agctl-*`) | `.agents/skills/*` (SKILL.md → `run_command agctl …`) | `packages/interaction` (commands), `packages/skill` | **Прямой перенос**: формат уже совместим; slash → DSH commands или навыки |
| Durable side effects / `IN_DOUBT` / reconciliation | `internal/harness/engine/effect_runtime.go`, `executor/reconciler.go` | нет прямого аналога (dsh-workflow проще) | Это **уникальный актив**; либо обернуть sidecar'ом (стратегия C), либо воспроизвести как workflow-семантику |

---

## 5. Стратегии переделки

### Стратегия A — Skill-мост (0.5–2 дня, работает уже сейчас)

**Факт из исследования:** DSH `skill-filesystem` обнаруживает навыки из
`<projectRoot>/.agents/skills` — это **тот же путь и тот же формат**
(`<name>/SKILL.md` с frontmatter `name`/`description`), в котором лежат
навыки agctl. То есть в проекте, открытом под DSH, навыки agctl **уже
видны** агенту DSH без единого изменения формата.

Что нужно:
1. Адаптировать тела навыков: заменить «выполнить через `run_command`» на
   «выполнить через shell/subprocess-инструмент DSH» и переписать
   ожидания вывода под конвенции DSH (model-visible ⟺ logged).
2. Слash-команды AGY (`/agctl-run`) → зарегистрировать как DSH-команды
   (`ctx.commands`) или оставить навыками (DSH каталог навыков + tool-skill).
3. `agctl.exe` остаётся **sidecar-бинарем** в workspace; DSH его вызывает
   как внешний процесс. Никакого портирования Go-кода.

Плюсы: мгновенный эффект, нулевой риск для DSH-репо (всё живёт в проекте).
Минусы: две системы управления (DSH и agctl), AGY-специфичные пути
(`~/.gemini`) и hook-контракты в agctl остаются мёртвым грузом; «мозг»
реально исполняет DSH, а governance — Go-бинарь: дублирование контуров.

### Стратегия B — Нативный порт как DSH preset/bundle (3–6 недель, целевая)

Переписать **управляющие политики** agctl на нативных extension points DSH.
Go-движок выводится из эксплуатации (или остаётся только как архив).

```text
Новый bundle: @agctl/dsh-agctl (packages/… или out-of-tree plugin)
├─ preset/agctl.cordis.yml     — композиция: dsh-goal + dsh-workflow +
│                                dsh-jobs + dsh-plan + dsh-todo + dsh-mcp +
│                                dsh-skill + dsh-interaction + guard + …
├─ plugin: agctl-loop          — профили цикла поверх ctx.goals + goal-round-driver
├─ plugin: agctl-replan        — адаптивная мутация workflow-графа (no-progress detector)
├─ plugin: agctl-security      — Security Score 0..100 поверх composition/session
├─ plugin: agctl-provenance    — SHA-256 дерева навыков/плагинов (skills/change)
├─ plugin: agctl-doctor        — runtime-diagnostics-провайдер
├─ plugin: agctl-router        — пре-степ выбор инструментов (context/preset)
└─ skills/agctl-*              — портированные SKILL.md (см. стратегию A)
```

Ключевые принципы DSH, которые надо соблюсти при порте:
- **Model-visible ⟺ logged**: любой новый вход для модели — новое session
  event; replay/fork/telemetry обязаны работать.
- **Capability seam = Service Definition + Provider + Consumer** — не одна роль.
- **Плагины, не изменения loop**: политики вешаются на extension points;
  изменение `agent-loop` = изменение docs/architecture.md.
- **Registrations are effects**: `ctx.effect()`/`ctx.on()`, disposer'ы.
- **Snapshots**: каждое model-видимое поведение — keyless snapshot
  (runnable example) в том же PR.

Плюсы: одна среда, один контур, session-лог как единый durable источник,
Web GUI и SDK (TS+Python) бесплатно. Минусы: переписывание ~22k строк
движка не требуется (движок заменяется на `dsh-workflow`/`dsh-jobs`), но
нужно переписать ~15–20k строк политик; уникальная семантика
`IN_DOUBT`/reconciliation теряется, если не воспроизвести её в workflow.

### Стратегия C — Гибрид: Go harness как durable sidecar (1–2 недели)

Если нужна **дословная** durable-семантика (`IN_DOUBT`, effect intents,
reconciliation, CAS-артефакты) и нет желания её воспроизводить на TS:
- Go harness поднимается как **локальный сервис** (`agctl harness serve`,
  Stage 18 уже даёт REST/SSE API);
- DSH-плагин-клиент: инструменты `agctl_*` (run/plan/status/approve) через
  `ctx.jobs`-провайдера + HTTP-клиент к sidecar'у;
- session-лог DSH хранит только «что агент попросил и что получил»
  (model-visible), sidecar хранит физику исполнения.

Плюсы: сохраняет протестированный движок, минимум переписывания.
Минусы: две системы состояния (дублирование), сложнее отладка, и это
**против** архитектурной философии DSH (всё в session-логе); подходит как
переходный этап от A к B, не как финал.

---

## 6. Пошаговый план (рекомендуемый: A → B)

### Фаза 0 — гигиена (0.5 дня)
1. Закоммитить текущий WIP Stages 18–20 (см. риск №1 аудита).
2. Прогнать `go test -race ./...` и зафиксировать результат в плане.
3. Определить, остаётся ли agctl поддерживаться для AGY параллельно, или
   это полный переход на DSH (влияет на стратегию C).

### Фаза 1 — Skill-мост (1–2 дня)
4. Скопировать `.agents/skills/agctl*` в workspace-проект под DSH
   (формат совместим, путь тот же).
5. Адаптировать: `run_command` → shell-инструмент DSH; вывод → маркдаун по
   конвенциям DSH; slash-команды → `ctx.commands`.
6. Подключить `agctl.exe` как sidecar, проверить `dsh-goal`/`dsh-jobs` как
   замену loop/tasks для простых кейсов (пилот на одном проекте).

### Фаза 2 — Порт политик (2–4 недели)
7. **Loop**: плагин `agctl-loop` поверх `ctx.goals` + `goal-round-driver`
   (профили standard/deep/until-done/unrestricted, round-лимиты).
8. **Replan**: плагин `agctl-replan` поверх `dsh-workflow` — адаптивная
   мутация графа, diagnose→repair→reverify, no-progress detector
   (события workflow + goal-фазы).
9. **Jobs**: producer'ы задач на `ctx.jobs` (ресурсные лимиты — admission).
10. **Governance**: permission-провайдер (профили safe/balanced/autonomous/
    yolo), security-score плагин, provenance плагин.
11. **Doctor**: runtime-diagnostics-провайдер; **Dashboard**: веб-панель
    поверх session-лога + SSE.
12. **Skills**: перенести оставшиеся навыки, убрать AGY-специфику.

### Фаза 3 — Вывод AGY/Go из эксплуатации (1–2 недели)
13. Заменить `executor/agy` на `ctx.subprocess`/`subagent`; success-семантику
    (terminal `result.status`, soft denial) — в DSH-валидатор.
14. Решить судьбу `IN_DOUBT`/reconciliation: воспроизвести в `dsh-workflow`
    (или задокументировать как осознанно упрощённую семантику).
15. Снапшоты: keyless snapshots для каждого model-видимого поведения;
    обновить SDK TS/Python проекции.
16. Удалить legacy JSON-путь и `agctl harness` shim.

---

## 7. Риски портирования

| Риск | Митигация |
|---|---|
| **Потеря durable-семантики** (`IN_DOUBT`, idempotency keys, CAS) | Стратегия C как сохранный вариант; или явный документированный упрощённый контракт в `dsh-workflow` |
| **AGY-специфичные контракты в навыках** (пути `~/.gemini`, hook I/O, custom agents frontmatter) | Вырезать при порте; DSH-эквиваленты — session events и hooks DSH |
| **Дублирование состояния** (две БД: SQLite agctl + session-лог DSH) | Не допускать в стратегии B; в C — только переходно |
| **Дисциплина DSH** (coverage 100% per-file, snapshots, bilingual docs, agent notes) | Планировать PR как несколько маленьких; следовать docs/AGENTS.md DSH |
| **Разные «агентные» модели** (у agctl агент=программа, у DSH агент=композиция) | Поменять ментальную модель на раннем этапе; persona — это `dsh-persona` строка, а не бинарь |

---

## 8. Итоговый вывод

1. **Аудит**: agctl — здоровый, хорошо протестированный Go-проект с
   сильным durable-ядром; главные долги — незакоммиченный WIP и двойная
   runtime-модель.
2. **Портирование**: полноценная «переделка агента под DSH» означает
   перенос **политик и навыков**, а не движка: DSH уже покрывает
   loop/replan/workflow/jobs/mcp/skills/governance нативными плагинами.
3. **Быстрый выигрыш**: навыки agctl уже лежат в DSH-совместимом формате и
   пути — стратегия A даёт рабочий результат за день-два.
4. **Целевая архитектура**: стратегия B (bundle `agctl-*` плагинов поверх
   DSH) с фазами A → B; стратегия C — только если нужна дословная
   `IN_DOUBT`/reconciliation-семантика.

Следующий практический шаг (по запросу): закоммитить WIP и собрать
минимальный preset-бандл стратегии A (портированные навыки + sidecar-вызов
`agctl.exe`) в тестовом проекте под DSH.
