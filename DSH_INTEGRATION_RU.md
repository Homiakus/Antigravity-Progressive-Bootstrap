# DSH-интеграция системы agctl (текущее состояние)

Дата: **2026-08-21**
Статус: **система адаптирована под DeepSeek Harness (уровень agent-surface / sidecar)**
Дополняет: `AUDIT_DSH_PORT_RU.md` (аудит + стратегии A/B/C).

---

## 1. Как система теперь работает под DSH

```text
                    DeepSeek Harness (dsh web / headless)
                    ── plugin tree on Cordis ──
   ├─ dsh-goal / goal-round-driver   (same-session objective)
   ├─ dsh-workflow / dsh-jobs        (durable-подобное исполнение)
   ├─ dsh-skill (+ skill-filesystem) ← читает <projectRoot>/.agents/skills
   ├─ dsh-shell (pwsh)               ← выполняет agctl.exe как sidecar
   └─ apps/web                       (GUI, в котором вы работаете)
                              │
                              ▼
                   .agents/skills/  (57 файлов / 23 skills)
   ├─ agctl*            — управление control-plane agctl (sidecar)
   ├─ agctl-dsh         — единая DSH-точка входа в agctl через pwsh
   └─ master-prompt*    — 9 domain-навыков (UI/audit/visual-runtime)
                              │
                              ▼
                     agctl.exe  (sidecar, без AGY)
   Doctor · plan/DAG · replan · loop · tasks · mcp · security · dashboard
                              │
                              ▼
              Durable Execution Harness (Go, SQLite WAL)
```

Ключевой факт: **DSH обнаруживает навыки из `<projectRoot>/.agents/skills`** —
это тот же путь и формат (`<name>/SKILL.md`), в котором живут и agctl-навыки,
и установленные master-prompt-skills. Никакого переписывания формата не нужно.

---

## 2. Что уже сделано и проверено

| Шаг | Факт | Статус |
|---|---|---|
| WIP harness Stages 18–20 закоммичен | `6e1e1ac` | ✅ |
| Аудит + исследование порта | `AUDIT_DSH_PORT_RU.md` (`0c6dc0c`) | ✅ |
| DSH-навык входа `agctl-dsh` | `.agents/skills/agctl-dsh/SKILL.md` | ✅ |
| Установлены 9 domain-навыков v2.1 | `dbded3c` | ✅ |
| Sidecar `agctl.exe doctor --self-test` | PASS (11 групп OК, агенты/MCP/loop/…) | ✅ |
| DSH обнаруживает все навыки | появились в каталоге skills сессии | ✅ |
| `go build ./...` / `go vet ./...` / `go test ./...` (включая `-race`) | exit 0 | ✅ |
| Working tree | чисто | ✅ |

---

## 3. Карта «что sidecar, что нативно» (после адаптации)

| Подсистема agctl | Как сейчас работает под DSH | Полностью нативное DSH-решение (стратегия B) |
|---|---|---|
| Доктор / диагностика | `agctl doctor --self-test` через `pwsh` | `dsh-runtime-diagnostics` |
| План / DAG | `agctl plan list/run` | `dsh-plan` + `dsh-workflow` |
| Adaptive replan | `agctl replan run` | плагин поверх `dsh-workflow` (мутация графа) |
| Автономный цикл | `agctl loop status/enable` | `dsh-goal` + `goal-round-driver` |
| Очередь задач | `agctl tasks list/run-pending` | `dsh-jobs` |
| Permissions / security score | `agctl permissions/security audit` | `dsh-interaction` + `dsh-guard` + плагин score |
| MCP | `agctl mcp list/probe` | `dsh-mcp` |
| Skill/provenance | `agctl skills` | `dsh-skill` + плагин provenance |
| Dashboard | `agctl dashboard serve` (localhost:8787) | `apps/web` + session-лог + SSE |
| Веб-дашборд-качество | `web-visual-runtime-validation` | тот же навык (уже нативно) |

---

## 4. Hardware факты для DSH

- **Sidecar binary**: `D:\Programms\Antigravity-Progressive-Bootstrap\agctl.exe`
  (в PATH или в корне workspace). Проверить: `Get-Command agctl`.
- **Точка входа**: навык **`agctl-dsh`** — единая DSH-обёртка всех подкоманд.
- **Не зависит от AGY**: `agy` может отсутствовать; control-plane функции
  (план/цикл/задачи/бюджеты) работают из SQLite harness.
- **Model-visible ⟺ logged**: вывод agctl попадает в session-лог DSH через
  результаты инструмента `pwsh`, что соответствует правилу DSH.

---

## 5. Нативный порт (стратегия B) — начат

План порта зафиксирован в DSH: `.agents/notes/proposed/architecture/2026-08-21-agctl-to-dsh-plugin-port.md`
(карта agctl → DSH-extension points + порядок создания плагинов). Первый
плагин `@deepseek-ai/dsh-agctl-security` (Security Score 0..100) создан как
scaffold: ядро `src/score.ts` + 13 unit-тестов проходят (`pnpm exec vitest run`).
Cordis-шов (Service Definition + Provider + Consumer) — следующий инкремент.

## 5.1. Что осталось для «полностью нативного» порта (стратегия B)

Текущая адаптация делает систему **полностью рабочей и discoverable под DSH**,
но исполняющее ядро остаётся Go-sidecar. Для полного ухода от Go нужен порт
политик на DSH-плагины:

1. `dsh-goal` + `goal-round-driver` — заменяют `agctl loop`.
2. `dsh-workflow` (+ адаптивная мутация графа) — заменяют replan/plan.
3. `dsh-jobs` — заменяют очередь задач.
4. `dsh-interaction` + `dsh-guard` — заменяют permissions.
5. Новые плагины: `agctl-security` (Score 0..100), `agctl-provenance`,
   `agctl-doctor`, `agctl-router`.

Это отдельная крупная работа (несколько недель), не блокирующая текущий
рабочий статус.

---

## 6. Как проверить интеграцию сейчас

```pwsh
# 1. Sidecar жив и диагностируется
.\agctl.exe doctor --self-test

# 2. Все навыки на месте (23 штуки)
Get-ChildItem .agents\skills -Directory | Select-Object -ExpandProperty Name

# 3. Кодовая база в порядке
go build ./... ; go vet ./... ; go test -race ./...
```

Все три пункта проходят.
