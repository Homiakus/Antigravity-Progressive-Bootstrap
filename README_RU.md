# agctl 3.2.1 — Antigravity Control Plane

> **3.2.1 Audited Compatibility Patch (19.08.2026).** Эта версия повторно сверена с актуальной официальной документацией Antigravity 2.0 v2.8.1, Antigravity CLI v1.1.14, Antigravity IDE v2.5.5 и MCP specification 2026-07-28. Полная матрица соответствия, исправленные расхождения и сознательно оставленные ограничения находятся в `AUDIT_RU.md`.


`agctl` — standalone Go control-plane для Google Antigravity / AGY. Он не заменяет модель и нативный runtime Antigravity, а управляет расширениями, multi-agent orchestration, MCP, skills/plugins, permissions, verified completion, headless task queue, security governance, supply-chain provenance и observability.

## Что исправлено в 3.2.1 после итогового аудита

- custom agents переведены на актуальный frontmatter (`mainAgent`, `subagent`, `model`, `commandExecutionPolicy`, `tools`); устаревший `permissionMode` больше не генерируется;
- поддержаны обе документированные формы custom agent: `<name>.md` и `<name>/agent.md`, включая agents внутри plugin bundles;
- MCP live probe переведён на stateless MCP 2026-07-28 (`server/discover`, metadata в каждом запросе) с совместимым fallback на 2025-11-25 Streamable HTTP/stdio;
- `serverUrl` считается единственным актуальным remote MCP полем; `url/httpUrl` диагностируются как legacy-invalid;
- headless runner больше не считает exit code 0 достаточным успехом: проверяет terminal `result.status == SUCCESS` и soft permission denials;
- desktop `/goal` отделён от `agy -p`: в headless используется until-done prompt contract без предположения, что IDE slash-command поддерживается CLI;
- Stop gate учитывает обязательный `fullyIdle`;
- CLI managed hooks устанавливаются через документированный `agy plugin install`;
- CLI skill mirrors сохраняют ссылку на полный ресурсный каталог, а provenance хеширует всё дерево skill, а не только `SKILL.md`;
- workspace Rules создаются как обычный Markdown без выдуманного on-disk `activation` frontmatter;
- `gopls mcp` оставлен opt-in, поскольку upstream помечает MCP mode experimental.

## Что нового в 3.2

Главное изменение — **Adaptive DAG Replanning**. План исполнения теперь не обязан оставаться неизменным после старта.

Если агент, тесты или исследование обнаружили материал, которого не было в исходном DAG, `agctl` может:

- принять структурированное replan-предложение от выполняющегося узла;
- проверить evidence, confidence и semantic risk;
- добавить новые nodes в существующий DAG;
- пересчитать зависимости downstream nodes;
- автоматически создать `diagnose → repair → reverify` после окончательного failure;
- пометить восстановленный failed node как `superseded`, а не блокировать весь план;
- обнаружить повторяющуюся ошибку и прекратить бессмысленный цикл через **no-progress detector**;
- ограничить автономное разрастание графа budget-ами;
- при нескольких независимых write-actions из read-only анализа разнести работу по Git worktrees и добавить отдельный integration node;
- хранить revision history DAG;
- показывать revisions/dynamic nodes/replan inbox в doctor и dashboard.

Базовые возможности 3.1 сохранены: resource-aware scheduler, bounded retries, atomic task claims, MCP live probe, MCP Registry security gate, semantic risk engine, plugin/skills/subagent management, capability ranking, provenance и локальный observability dashboard.

---

# Быстрый старт

Распакуйте Windows release и запустите:

```text
Start-agctl.cmd
```

или:

```powershell
.\agctl.exe
```

Для полной настройки:

```powershell
.\agctl.exe install full --prereqs
.\agctl.exe replan enable
.\agctl.exe doctor --self-test
```

Для проекта:

```powershell
cd C:\Projects\my-project

agctl project init
agctl capabilities build --workspace .
agctl security audit --workspace .
agctl doctor --probe-mcp --workspace .
```

---

# Adaptive DAG Replanning

## Основная команда

Создать и выполнять адаптивный multi-agent plan:

```powershell
agctl plan run --workspace . --prompt "Реализуй feature, проверь тестами и исправь все найденные проблемы"
```

Adaptive replanning включён по умолчанию в 3.2. Для старого статического поведения:

```powershell
agctl plan run --static --workspace . --prompt "..."
```

То же для уже существующей очереди:

```powershell
agctl tasks run-pending
```

или статически:

```powershell
agctl tasks run-pending --static
```

Явно запустить adaptive supervisor:

```powershell
agctl replan run
```

## Как меняется DAG

Исходный план:

```text
inspect
   ↓
implement
   ↓
verify
   ↓
review
```

Если `verify` окончательно падает после configured retries:

```text
implement
   ↓
verify [failed → superseded]
   │
   └── diagnose-r1
           ↓
        repair-r1
           ↓
       reverify-r1
           ↓
         review
```

Downstream `review` автоматически rewired с failed task на новый `reverify`.

Если новый repair тоже падает, может быть создан новый recovery chain — но только пока не исчерпаны limits/no-progress budget.

---

# Replan proposal от агента

Каждый plan task получает путь `replanProposalPath` и protocol instruction.

Если во время выполнения обнаружена **обязательная новая работа**, которой нет в DAG, агент может записать JSON:

```json
{
  "version": 1,
  "planId": "plan-...",
  "parentNodeId": "inspect",
  "parentTaskId": "...",
  "reason": "Обнаружено два независимых дефекта, оба нужны для исходного Definition of Done",
  "evidence": [
    "integration test A воспроизводит дефект",
    "schema contract B расходится с реализацией"
  ],
  "confidence": 0.94,
  "actions": [
    {
      "id": "fix-api-validation",
      "title": "Исправить API validation",
      "objective": "Исправить validation и добавить regression tests",
      "agent": "implementer",
      "verification": ["targeted tests pass"],
      "risk": "write-medium",
      "parallelizable": true
    },
    {
      "id": "fix-schema-contract",
      "title": "Исправить schema contract",
      "objective": "Согласовать schema и реализацию, затем проверить compatibility",
      "agent": "implementer",
      "verification": ["contract tests pass"],
      "risk": "write-medium",
      "parallelizable": true
    }
  ]
}
```

`agctl` проверяет:

```text
plan/task identity
        ↓
evidence present?
        ↓
confidence >= threshold?
        ↓
action dependency DAG valid?
        ↓
risk <= autoApplyRiskMax?
        ↓
revision/node budgets available?
        ↓
APPLY / REJECT / BLOCK
```

Опциональная косметика не должна создавать replan proposal.

---

# Bounded autonomy

По умолчанию:

```json
{
  "enabled": true,
  "maxRevisions": 8,
  "maxDynamicNodes": 24,
  "maxRepairDepth": 3,
  "maxSameFailure": 2,
  "minConfidence": 0.65,
  "autoApplyRiskMax": "write-medium",
  "preferWorktrees": true,
  "requireEvidence": true
}
```

Посмотреть:

```powershell
agctl replan config
```

Настроить:

```powershell
agctl replan config `
  --max-revisions 10 `
  --max-dynamic-nodes 30 `
  --max-repair-depth 3 `
  --max-same-failure 2 `
  --min-confidence 0.70 `
  --auto-risk-max write-medium `
  --prefer-worktrees true `
  --require-evidence true
```

Отключить:

```powershell
agctl replan disable
```

Включить:

```powershell
agctl replan enable
```

---

# No-progress detector

`agctl` рассчитывает failure signature из:

```text
task error
+
последних строк execution log
```

Если одна и та же ошибка повторяется слишком много раз или repair chain достигает `maxRepairDepth`, план переводится в:

```text
status = blocked
```

вместо бесконечного:

```text
retry → same failure → retry → same failure → ...
```

Событие записывается в telemetry как `replan.blocked` / `replan.no-progress` context.

---

# Parallel adaptive worktrees

Если предложение пришло от **read-only** node (`architect`/`researcher`) и содержит минимум две независимые `parallelizable` write-actions, 3.2 может создать отдельные Git worktrees.

```text
                inspect/read-only
                      │
          ┌───────────┴───────────┐
          ▼                       ▼
 worktree branch A          worktree branch B
      fix A                      fix B
          │                       │
          └───────────┬───────────┘
                      ▼
                integration node
                      │
                      ▼
                   review
```

Каждый isolated write node получает инструкцию перед успешным завершением создать **локальный commit** в своей ветке. Ничего не push-ится автоматически.

Worktree mode сознательно не используется после arbitrary write-parent, потому что uncommitted base changes могли бы отсутствовать в новом worktree.

---

# Plan history

```powershell
agctl plan list
agctl plan show <PLAN_ID>
agctl plan history <PLAN_ID>
```

План хранит:

```text
revision
status
updatedAt
dynamicNodeCount
blockReason
revisionHistory[]
```

Revision history содержит trigger task/node, reason, type, added nodes, rewired nodes и failure signature.

---

# Replan status и debugging

```powershell
agctl replan status
agctl replan status <PLAN_ID>
agctl replan inbox
agctl replan apply <TASK_ID>
```

`apply` полезен для ручной диагностики proposal-файла или synthetic failed task.

---

# Multi-agent DAG planner

Создать план:

```powershell
agctl plan create --workspace . --prompt "Проведи аудит, исправь ошибки и проверь результат"
```

Planner учитывает:

- project profile;
- task intent;
- Capability Registry;
- research/security/browser/infra signals;
- read/write nature;
- verification profile.

Типичный DAG:

```text
inspect ───────┐
               ├── implement ──┬── verify ─────┐
research ──────┘                └── security ───┼── review
                                               ▼
                                             DONE
```

---

# Resource-aware Task Supervisor

```powershell
agctl tasks config `
  --max-parallel 4 `
  --cpu-weight 100 `
  --build-slots 1 `
  --browser-slots 1 `
  --max-retries 1 `
  --max-task-minutes 120
```

Scheduler учитывает:

```text
DAG dependencies
CPU weight
build slots
browser slots
exclusive workspace
already-running tasks
atomic task claim locks
retry budget
watchdog
```

Один task не может быть одновременно claim-нут двумя процессами `run-pending`.

---

# Native /goal + verified completion

```powershell
agctl goal native "Доведи задачу до полностью проверенного результата"
```

или для plan nodes:

```powershell
agctl plan run --native-goal --prompt "..."
```

Native `/goal` отвечает за непрерывность agent execution, а agctl completion hooks и adaptive supervisor — за Definition-of-Done, verification и recovery/replanning.

---

# Subagents / Orchestrator

```powershell
agctl agents install
agctl agents enable balanced
agctl agents enable parallel
agctl agents enable maximum
agctl agents list
```

Встроенные роли:

```text
architect
researcher
implementer
test-engineer
security-reviewer
code-reviewer
```

Adaptive nodes используют те же first-class custom agents.

---

# Capability Registry

```powershell
agctl capabilities build --workspace .
agctl capabilities list
agctl capabilities search browser
agctl capabilities rank "Исправь мобильную форму и network errors"
```

Нормализуются:

```text
skills
MCP
agents
plugins
workflows
native capabilities
```

---

# MCP

Минимальный development core:

```text
Context7
Playwright
Chrome DevTools
GitHub
```

Go profile:

```text
gopls mcp
```

Live probe:

```powershell
agctl mcp probe
agctl doctor --probe-mcp
```

Probe выполняет:

```text
initialize
→ capabilities
→ tools/list
→ resources/list
→ prompts/list
→ latency
```

---

# MCP Registry security gate

```powershell
agctl registry search postgres
agctl registry info <NAME>
agctl registry install <NAME>
```

Перед install рассчитывается security score. Низкий score требует явного:

```powershell
--allow-low-security-score
```

---

# Semantic Risk Engine

```powershell
agctl risk classify --tool mcp__database__drop_database --args '{"name":"production"}'
```

Классы:

```text
read-low
write-medium
execution-high
external-write-high
destructive-critical
```

Guarded autonomous mode блокирует destructive semantic actions, а не только shell regex.

---

# Security / provenance

```powershell
agctl security audit --workspace .
agctl provenance list
agctl provenance verify
```

Проверяются transport, pinning, scripts/hooks, secrets boundary и hashes remote skills/plugins.

---

# Plugins / Skills / Workflows

```powershell
agctl plugins list
agctl plugins install-git <URL>
agctl plugins doctor

agctl skills list
agctl skills sync-recommended

agctl workflow list --workspace .
agctl workflow install --workspace . verified-goal deep-review release-gate
```

---

# Project initialization

```powershell
agctl project detect
agctl project init
```

Создаётся agent-ready layout:

```text
AGENTS.md
.agents/
├── agents/
├── rules/
├── workflows/
├── skills/
├── plugins/
└── mcp_config.json
```

---

# Observability dashboard

```powershell
agctl dashboard serve
```

По умолчанию:

```text
http://127.0.0.1:8787
```

Dashboard показывает:

- security score;
- plans;
- total plan revisions;
- dynamic nodes;
- replan inbox;
- task states;
- capabilities;
- recent telemetry.

Endpoints:

```text
/api/snapshot
/api/events
/api/tasks
/api/plans
/metrics
```

Prometheus metrics включают:

```text
agctl_plans_total
agctl_plan_revisions_total
agctl_dynamic_nodes_total
agctl_replan_inbox
agctl_tasks{status="..."}
agctl_security_score
agctl_telemetry_events_total
```

---

# Диагностика

```powershell
agctl doctor --self-test
```

Self-test 3.2 проверяет в том числе:

```text
Router PreInvocation
Completion PreInvocation
premature Stop → continue
verified completion → stop
normal tool → allow
destructive MCP → deny
multi-agent planner
adaptive failed-node recovery
failed task → superseded
downstream dependency rewiring
security score
```

Live MCP:

```powershell
agctl doctor --self-test --probe-mcp --workspace .
```

---

# Миграция с 3.1

Запустите:

```text
Migrate-v3.1-to-v3.2.cmd
```

или:

```powershell
agctl migrate v3.1
agctl install full
agctl replan enable
agctl doctor --self-test
```

Существующие 3.1 task/plan JSON совместимы. Новые поля optional и читаются с zero/default values.

---

# Рекомендуемая конфигурация

Для основной dev-машины:

```text
Router                    = Balanced
Completion Loop           = Until-Done или Deep
Adaptive DAG Replanning   = Enabled
Auto replan risk max      = write-medium
Subagent orchestration    = Parallel
Permissions               = Autonomous
Worktrees                 = Enabled
Max revisions             = 8
Max dynamic nodes         = 24
Max repair depth          = 3
Max same failure          = 2
```

`Unrestricted/YOLO` оставляйте только для изолированных сред, где сознательно принимается риск.
