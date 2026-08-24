---
name: agctl-dsh
description: DSH-адаптированный вызов agctl control-plane как sidecar-бинаря через PowerShell. Покрывает /agctl-doctor, /agctl-run, /agctl-replan, /agctl-dashboard, /agctl-loop, /agctl-tasks, /agctl-mcp, /agctl-security, /agctl-plan, /agctl-init. Рекомендованная точка входа при работе agctl внутри DeepSeek Harness вместо AGY-специфичного навыка agctl.
---

# agctl-dsh — agctl через DeepSeek Harness

Это DSH-вариант навыка `agctl`. В отличие от `agctl` (который рассчитан на
`run_command` и slash-модель Antigravity), этот навык:

- вызывает бинарь как **внешний sidecar-процесс** через инструмент
  оболочки DSH (на Windows — `pwsh`);
- опирается на уже установленный `agctl.exe` в корне workspace или в PATH;
- не зависит от присутствия `agy` (Antigravity CLI): план/цикл/задачи/бюджеты
  живут в `agctl` (SQLite harness), а не в AGY.

> правил AGY-специфичных поверхностей (`~/.gemini/config/*`, hook-контракты,
> custom-agent frontmatter) этот навык касается только для чтения через
> `agctl doctor`/`agctl security audit` — он не обязан их изменять.

## Когда использовать

Вызывайте этот навык, когда задача относится к управлению агентной
инфраструктурой: диагностика окружения, запуск плана с adaptive-DAG,
перепланирование, локальная веб-панель, режим автономного цикла, очередь
задач, проверка MCP, аудит безопасности, просмотр плана или инициализация
workspace. Для обычной разработки в DSH он не нужен.

## Базовый вызов

Исполнитель DSH на Windows — `pwsh`. Каждый раз сначала подтвердите, что
бинарь доступен, затем выполните подкоманду и красиво отформатируйте вывод
(статусные таблицы + Markdown-предупреждения `[!NOTE]`/`[!TIP]`/`[!WARNING]`).

```pwsh
# Разрешаем бинарь: PATH или корень workspace
$agctl = (Get-Command agctl -ErrorAction SilentlyContinue)?.Source
if (-not $agctl) { $agctl = Join-Path $PWD 'agctl.exe' }
"agctl = $agctl"
& $agctl <subcommand> ...
```

## Карта команд (DSH-вызов)

| Поверхность | Команда | Что даёт |
|---|---|---|
| Диагностика | `agctl doctor --self-test` | PASS/FAIL по агентам, capability, hooks, loop, MCP, permissions, plans, plugins |
| Диагностика + MCP probe | `agctl doctor --probe-mcp` | живой пинг MCP-серверов (2026-07-28, fallback 2025-11-25) |
| Запуск плана | `agctl plan run --workspace . --prompt "<task>"` | запуск с adaptive-DAG replanning |
| Перепланирование | `agctl replan run` | supervisor перепланирования |
| Веб-панель | `agctl dashboard serve --listen 127.0.0.1:8787` | локальная панель → `http://127.0.0.1:8787` (запускать в фоне) |
| Автономный цикл | `agctl loop status` / `agctl loop enable <profile>` | профили `standard\|deep\|until-done\|unrestricted` |
| Очередь задач | `agctl tasks list` / `agctl tasks run-pending` | задачи и запуск обработки |
| MCP | `agctl mcp list` / `agctl mcp probe` | зарегистрированные серверы и их доступность |
| Безопасность | `agctl security audit` | Security Score 0..100 |
| План | `agctl plan list` | текущий DAG/планы |
| Инициализация | `agctl project init` | настройка правил и скиллов в workspace |

## Рекомендации по выводу

1. **Диагностика**: выводите компактную таблицу статусов `OK/WARN/FAIL` с
   группой (agents/mcp/hooks/loop/permissions/plans/plugins) и следующее
   рекомендованное действие. `WARN prereq agy missing` — ожидаемо в DSH:
   AGY не требуется для control-plane функций.
2. **План / replan**: показывайте `revision`, `status`, число динамических
   узлов и список узлов с их `dependsOn`. Если план в `blocked` — всегда
   выводите `blockReason` и не перезапускайте вслепую.
3. **Dashboard**: стартуйте фоном (`Start-Process` или фоновый job), дайте
   кликабельную ссылку `http://127.0.0.1:8787` и не блокируйте ввод.
4. Никогда не трактуйте exit code 0 как успех для `plan run` — сверяйтесь с
   терминальным `result.status` плана, а не только с кодом процесса.

## Заметки по переносу (для стратегии A → B)

- Этот навык — «мост»: `agctl.exe` остаётся единственным исполняющим ядром.
- В целевой архитектуре (стратегия B) функции заменяются нативные:
  `agctl loop` → `dsh-goal` + `goal-round-driver`;
  `agctl replan`/`plan` → `dsh-workflow` + `dsh-plan`;
  `agctl tasks` → `dsh-jobs`;
  `agctl permissions` → `dsh-interaction` + `dsh-guard`;
  `agctl mcp` → `dsh-mcp`;
  `agctl dashboard` → `apps/web` (session-лог + SSE).
- До перехода этот навык даёт рабочую поверхность без переписывания
  Go-движка и без зависимости от AGY.
