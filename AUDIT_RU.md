# Итоговый аудит agctl 3.2.1

Дата проверки: **19 августа 2026**  
Статус: **Audited Compatibility Patch**

## 1. Что именно проверено

Аудит проводился не только как code review, а как сверка реальных контрактов `agctl` с текущими публичными контрактами Google Antigravity и MCP.

На дату аудита навигация официальной документации Google показывает:

- Antigravity 2.0 **v2.8.1**;
- Antigravity CLI **v1.1.14**;
- Antigravity IDE **v2.5.5**;
- Antigravity SDK **v0.1.12**.

Проверялись:

1. пути discovery global/workspace customizations;
2. MCP JSON schema и remote transport fields;
3. MCP wire protocol 2026-07-28 и compatibility fallback;
4. Agent Skills и auxiliary resources;
5. custom agents/subagents и frontmatter;
6. plugins и CLI plugin installation;
7. hooks, hook I/O и execution-loop decisions;
8. unified permissions;
9. rules/workflows;
10. sidecars;
11. AGY CLI headless execution;
12. `/goal` и until-done execution;
13. autonomous completion gate;
14. task/DAG/replan supervisor;
15. supply-chain/provenance;
16. project profiles/defaults;
17. diagnostics и failure semantics.

Основные официальные источники:

- https://antigravity.google/docs/mcp
- https://antigravity.google/docs/skills
- https://antigravity.google/docs/subagents/
- https://antigravity.google/docs/plugins
- https://antigravity.google/docs/hooks
- https://antigravity.google/docs/permissions/
- https://antigravity.google/docs/sidecars
- https://antigravity.google/docs/rules-workflows
- https://antigravity.google/docs/cli/features
- https://antigravity.google/docs/cli/headless
- https://antigravity.google/docs/cli/modes
- https://antigravity.google/docs/cli/plugins
- https://modelcontextprotocol.io/specification/2026-07-28/
- https://go.dev/gopls/features/mcp

---

# 2. Итоговый вердикт

После исправлений 3.2.1 основные поверхности, которыми `agctl` **реально управляет**, соответствуют текущей опубликованной документации.

Условная оценка соответствия после patch:

| Область | Статус | Комментарий |
|---|---|---|
| Global/workspace MCP config | ✅ | Актуальные пути и `serverUrl` |
| MCP stdio 2026-07-28 probe | ✅ | Stateless `server/discover` + request metadata |
| MCP HTTP 2026-07-28 probe | ✅ | Stateless headers + request metadata |
| MCP 2025-11-25 compatibility | ✅ | `initialize` + `initialized`, session header для HTTP |
| Deprecated HTTP+SSE 2024 deep probe | ⚠️ Partial | Конфиг Antigravity допускает; отдельный legacy GET/SSE probe не реализован |
| WebSocket MCP deep probe | ⚠️ Partial | Конфиг валиден, но `agctl mcp probe` не реализует WS client |
| IDE/2.0 Skills | ✅ | Полные skill folders + resources |
| CLI skill mirrors | ✅ | Flat `.md` mirror + указатель resource base |
| Custom agents | ✅ | Обе формы discovery и актуальный frontmatter |
| Plugin agents | ✅ | Попадают в Capability Registry |
| IDE/2.0 plugins | ✅ | `~/.gemini/config/plugins`, `.agents/plugins` |
| CLI plugins | ✅ | `agy plugin install/enable/disable` |
| Hooks | ✅ | Актуальные events, matcher и camelCase I/O |
| Stop completion gate | ✅ | `decision=continue`, учитывается `fullyIdle` |
| Unified permissions | ✅ | Resource syntax и Deny > Ask > Allow |
| Project GUI-only permissions | ⚠️ Boundary | Не патчится недокументированный internal storage IDE |
| Rules | ✅ | Обычный Markdown, актуальные paths |
| Rule activation mode | ⚠️ Boundary | Файловая схема activation не опубликована; `agctl` её не выдумывает |
| Workspace workflows | ✅ | Markdown workflows, slash-invocation model |
| Global workflow file location | ⚠️ Boundary | UI поддерживает global, но публичная страница не закрепляет отдельный filesystem path |
| Sidecars | ✅ | `sidecar.json`, config enablement, schedule structure |
| AGY headless | ✅ | `-p`, stream-json, `--agent`, `--mode=accept-edits`, timeout |
| Headless success detection | ✅ | Проверяется terminal `result.status`, а не только exit code |
| Headless soft permission denial | ✅ | Exit 0 больше не считается успехом при фактическом denial |
| Desktop `/goal` | ✅ | Генерируется только для UI surface |
| Headless until-done | ✅ | Обычный prompt contract, без предположения CLI `/goal` |
| `gopls mcp` | ✅ Opt-in | Не stable-default: upstream называет его experimental |
| Skill provenance | ✅ | Hash всего дерева skill |
| Upstream skill pinning | ⚠️ Medium | Sync по HEAD; commit/hash фиксируется после install/update |
| MCP Registry | ⚠️ Preview dependency | Registry-клиент зависит от preview API |

**Практический вывод:** core integration `agctl 3.2.1` корректна для документированных surfaces. Оставшиеся пункты в таблице — явно диагностируемые ограничения или области, где стабильного публичного API/пути нет; программа больше не подменяет отсутствие документации догадками.

---

# 3. Критичные проблемы, найденные в 3.2.0

## 3.1 Custom agent frontmatter

### Было

Генерировалось поле:

```yaml
permissionMode: acceptEdits
```

Для текущего custom-agent schema оно не документировано.

### Исправлено

3.2.1 генерирует:

```yaml
---
name: test-engineer
description: ...
model: flash
mainAgent: false
subagent: true
commandExecutionPolicy: auto
tools:
  - view_file
  - grep_search
  - run_command
  - manage_task
---
```

Поддерживаются актуальные значения:

```text
model:
  inherit | flash | pro

commandExecutionPolicy:
  off | auto | eager | sandbox
```

Doctor дополнительно валидирует точные имена tools, потому что официальная документация отдельно предупреждает, что неизвестный tool name способен привести к зависанию subagent.

## 3.2 Неполное discovery custom agents

### Было

`agctl` фактически ориентировался на:

```text
.agents/agents/name/agent.md
```

### Исправлено

Поддерживаются обе официальные формы:

```text
.agents/agents/name.md
.agents/agents/name/agent.md
```

и соответствующие global/plugin scopes.

---

# 4. MCP audit

## 4.1 Config schema

Текущая Antigravity MCP configuration использует:

```json
{
  "mcpServers": {
    "example": {
      "command": "...",
      "args": [],
      "env": {},
      "cwd": "...",
      "serverUrl": "https://...",
      "headers": {},
      "authProviderType": "...",
      "oauth": {},
      "disabled": false,
      "disabledTools": []
    }
  }
}
```

3.2.1 сохраняет неизвестные будущие JSON fields при merge и меняет только managed entries.

Legacy:

```text
url
httpUrl
```

теперь doctor/security audit помечают как invalid для текущей Antigravity configuration.

## 4.2 MCP 2026-07-28

Это было крупнейшим protocol defect 3.2.0.

MCP 2026-07-28 стал stateless:

- старый обязательный `initialize`/`notifications/initialized` handshake удалён;
- request несёт protocol/client metadata в `_meta`;
- discovery выполняется через `server/discover`;
- результаты current protocol имеют новый result contract.

3.2.1 реализует отдельный modern path для stdio и HTTP.

### Modern stdio

```text
server/discover
    ↓
supportedVersions
capabilities
    ↓
tools/list
resources/list
prompts/list
```

Каждый request включает current metadata.

### Modern HTTP

Каждый POST включает актуальные MCP headers, включая protocol version/method metadata; никакой session state для 2026-era transport не предполагается.

### Legacy fallback

Если server действительно относится к initialize-era protocol, выполняется fallback на:

```text
2025-11-25
initialize
notifications/initialized
lists
```

Для legacy Streamable HTTP сохраняется `Mcp-Session-Id` после initialize.

## 4.3 Сознательные ограничения probe

Antigravity принимает remote SSE, Streamable HTTP и websocket через `serverUrl`.

`agctl` deep live probe сейчас полноценно проверяет:

```text
stdio modern
stdio legacy
HTTP modern
Streamable HTTP legacy 2025-11-25
```

Он **не притворяется**, что проверил:

```text
WebSocket MCP
deprecated HTTP+SSE 2024-11-05 GET/SSE handshake
```

Для них выводится явная limitation.

---

# 5. Skills audit

## IDE / Antigravity 2.0

Используется документированная структура:

```text
~/.gemini/config/skills/<skill>/SKILL.md
<workspace>/.agents/skills/<skill>/SKILL.md
```

Сохраняются:

```text
scripts/
examples/
resources/
references/
assets/
```

## CLI mirror

CLI имеет свою поверхность. Flat mirror содержит путь к canonical full skill folder, чтобы инструкции не теряли доступ к auxiliary resources.

## Provenance

3.2.0 мог фактически ограничить integrity signal главным `SKILL.md`.

3.2.1 хеширует recursively всё установленное дерево skill. Изменение helper script/resource теперь меняет provenance verification.

## Остаточный риск

Remote packs обновляются из upstream branch/HEAD. Lock фиксирует source commit и content hash **после** конкретной синхронизации и обнаруживает tampering, но update policy ещё не требует заранее закреплённого immutable commit.

Это не protocol incompatibility, а supply-chain hardening gap.

---

# 6. Plugins audit

## IDE / 2.0

Поддерживаются:

```text
~/.gemini/config/plugins/<plugin>/
<workspace>/.agents/plugins/<plugin>/
```

Plugin manager сохраняет bundle components и provenance.

## CLI

CLI имеет отдельный root:

```text
~/.gemini/antigravity-cli/plugins/<plugin>/
```

Для managed control-plane hooks 3.2.1 использует документированный lifecycle:

```text
agy plugin install <path>
agy plugin enable <name>
agy plugin disable <name>
agy plugin uninstall <name>
```

Если `agy` недоступен в test/minimal environment, допускается filesystem mirror, но doctor не трактует такой mirror как доказанную runtime-регистрацию.

---

# 7. Hooks audit

Проверены:

```text
PreInvocation
PostInvocation
PreToolUse
PostToolUse
Stop
```

Managed runtime использует:

```text
PreInvocation -> route/completion context
PreToolUse    -> semantic permission/risk gate
Stop          -> verified-completion gate
```

### PreToolUse fail-safe

Повреждённый hook input теперь приводит к:

```json
{
  "decision": "ask"
}
```

а не к автоматическому allow.

### Stop

Финальное завершение допускается только если одновременно:

```text
complete = true
verified = true
verification non-empty
hardBlocker = false
fullyIdle = true
```

Если обязательная работа ещё не завершена, возвращается:

```json
{
  "decision": "continue"
}
```

Malformed Stop input fail-open завершает loop, чтобы ошибка самого control-plane не могла навечно запереть conversation.

---

# 8. Permissions audit

`agctl` использует текущую resource model:

```text
read_file(...)
write_file(...)
read_url(...)
execute_url(...)
command(...)
unsandboxed(...)
mcp(...)
```

и учитывает:

```text
Deny > Ask > Allow
```

CLI settings используют документированные:

```text
toolPermission
artifactReviewPolicy
enableTerminalSandbox
allowNonWorkspaceAccess
```

## Граница автоматизации

Antigravity 2.0 имеет Project-scoped GUI configuration. `agctl` **не патчит недокументированное внутреннее хранилище IDE**, потому что это было бы хрупким и опасным при любом update приложения.

Поэтому `permissions checklist/audit` остаётся необходимым для GUI-only project override, если он конфликтует с глобальной автономной политикой.

---

# 9. Headless AGY audit

3.2.1 использует:

```text
agy --mode=accept-edits -p "..." --output-format stream-json
```

с:

```text
--agent <custom-agent>
--print-timeout <watchdog>
```

когда применимо.

## Критичное исправление success semantics

Официальный headless stream имеет terminal `result` event с `result.status`.

Поэтому exit code процесса сам по себе недостаточен.

Теперь задача считается succeeded только если:

```text
process exit = 0
AND terminal result exists
AND result.status = SUCCESS
AND no soft permission denial observed
```

Это устраняет класс ложных SUCCESS при soft-denied tool action.

---

# 10. `/goal` audit

Есть две разные поверхности:

## Antigravity 2.0 UI

```text
/goal <task>
```

используется как native until-done interaction.

## AGY headless

CLI reference не документирует `/goal` как core CLI slash-command.

Поэтому 3.2.1 больше не отправляет literal `/goal` внутрь `agy -p`.

Вместо этого headless runner передаёт эквивалентный completion contract обычным prompt-текстом.

`agctl goal native` сохранён для UI composition.

---

# 11. Rules и Workflows

Workspace rules:

```text
.agents/rules/*.md
```

Global rule:

```text
~/.gemini/GEMINI.md
```

`agctl` больше не пишет недокументированный YAML вида:

```yaml
activation: always
```

в rule file.

Rule остаётся обычным Markdown.

## Ограничение activation

UI документирует Manual / Always On / Model Decision / Glob, но публичная страница не определяет стабильный file-level metadata schema для этих режимов.

Поэтому `agctl` не придумывает такой schema.

## Workflows

Workspace workflows создаются как Markdown и рассчитаны на `/workflow-name` invocation.

Google UI также поддерживает Global workflows, но проверенная публичная документация не закрепляет отдельный filesystem path, поэтому `agctl` сознательно управляет только документированным workspace storage вместо угадывания внутреннего пути.

---

# 12. Sidecars audit

Используются:

```text
~/.gemini/config/sidecars/<id>/sidecar.json
~/.gemini/config/plugins/<plugin>/sidecars/<id>/sidecar.json
~/.gemini/config/config.json
~/.gemini/antigravity/sidecar_data/<id>/
```

Проверяются:

- mutually exclusive `command` / `builtin`;
- `restart_policy`: `always | on-failure | never`;
- `schedule` builtin;
- 5-field cron shape;
- enable/disable через global config.

---

# 13. `gopls mcp`

Go официально предоставляет:

```text
gopls mcp
```

но прямо называет MCP server **experimental**.

Поэтому 3.2.1:

```text
Recommended = false
```

и Go project profile больше не устанавливает его автоматически.

При необходимости:

```powershell
agctl mcp add gopls --workspace .
```

---

# 14. Что проверено тестами

После audit patch в regression suite присутствуют проверки как минимум следующих классов:

```text
custom agent documented frontmatter
flat + folder agent discovery
invalid agent field/tool detection
plugin-agent capability discovery
CLI-only skill vs IDE mirror deduplication
full skill-tree provenance
legacy MCP url/httpUrl diagnostics
MCP 2026 modern stdio
MCP 2026 modern HTTP
MCP modern result validation
MCP legacy stdio fallback
MCP legacy Streamable HTTP initialize/initialized/session
Stop fullyIdle completion gate
malformed PreToolUse fail-safe
headless soft denial with exit 0
headless terminal ERROR with exit 0
headless missing terminal result
headless --agent / --print-timeout
headless does not inject desktop /goal
sidecar schedule shape
rules without invented activation frontmatter
adaptive DAG recovery/replanning
resource scheduler
security risk classification
provenance tamper detection
```

---

# 15. Что нельзя честно назвать «100% покрытием Antigravity»

`agctl` — control-plane, а не копия всего продукта Google. После аудита остаются следующие сознательные границы:

1. **WebSocket MCP deep probe** не реализован.
2. **Deprecated HTTP+SSE 2024-11-05 deep live probe** не реализован.
3. **Project-specific GUI settings** не патчатся через undocumented IDE internals.
4. **Global workflow filesystem path** не автоматизируется, пока публичная документация не задаёт стабильный путь.
5. **Remote skill pack update pinning** пока post-install lock/provenance, а не обязательный immutable pin до download.
6. **MCP Registry** является внешней evolving/preview surface, поэтому Registry client требует compatibility maintenance.
7. `agctl` не пытается заменить native Antigravity reasoning, browser, `/goal`, subagent runtime или teamwork — он управляет ими и добавляет governance/verification.

Эти пункты должны рассматриваться как roadmap, а не как скрытые «всё работает» утверждения.

---

# 16. Рекомендованная production-конфигурация

Для обычной автономной разработки:

```powershell
agctl install full
agctl router enable balanced
agctl loop enable deep
agctl agents enable parallel
agctl replan enable
agctl permissions apply autonomous
agctl doctor --self-test
agctl security audit
agctl provenance verify
```

Для каждого repository:

```powershell
cd C:\Projects\repo
agctl project init
agctl capabilities build --workspace .
agctl doctor --workspace . --probe-mcp
```

Экспериментальные capability, включая `gopls mcp`, подключать явно.

---

# 17. Финальный вывод

`agctl 3.2.0` содержал несколько реальных несовместимостей с текущими контрактами Antigravity/MCP, поэтому утверждать, что он был полностью корректен, было бы неверно.

`agctl 3.2.1` исправляет найденные protocol/config/runtime defects и добавляет regression tests на эти контракты.

Для **документированных и управляемых `agctl` поверхностей** итоговый статус: **совместимо / проверено**.

Для неподдерживаемых transport/UI internals статус теперь всегда выводится как limitation/partial, а не маскируется под успешную поддержку.
