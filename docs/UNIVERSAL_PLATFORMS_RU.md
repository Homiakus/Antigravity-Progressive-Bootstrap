# Универсальная Кросс-Платформенная Архитектура agctl 3.2.1

## 1. Концепция Универсальности

`agctl` спроектирован как **универсальный Adaptive Control-Plane** для современных агентных сред разработки. Система не привязана жестко к одной платформе и автоматически адаптируется к окружению, в котором запускается:

```text
                             USER GOAL / PROMPT
                                     │
                                     ▼
        ┌─────────────────────────────────────────────────────────┐
        │                 AGENT / IDE ENVIRONMENT                 │
        ├─────────────┬─────────────┬─────────────┬───────────────┤
        │ Antigravity │  DeepSeek   │   Cursor    │  Claude Code  │
        │  IDE / CLI  │ Harness DSH │    Agent    │   Roo / CLI   │
        └──────┬──────┴──────┬──────┴──────┬──────┴───────┬───────┘
               │             │             │              │
               ▼             ▼             ▼              ▼
           GEMINI.md     AGENTS.md    .cursorrules    CLAUDE.md
           hooks.json    Cordis/pwsh    mcp.json      mcp_config
               │             │             │              │
               └─────────────┴──────┬──────┴──────────────┘
                                    │
                                    ▼
                         agctl CONTROL PLANE
              ┌─────────────────────┼─────────────────────┐
              │                     │                     │
          DISCOVERY           ORCHESTRATION          GOVERNANCE
              │                     │                     │
         skills/MCP/IR        Adaptive DAG        Risk & Budgets
              │                     │                     │
              └─────────────────────┼─────────────────────┘
                                    │
                                    ▼
                        AUTONOMOUS EXECUTION LOOP
                            (/agctl-loop)
                                    │
                                    ▼
                       DURABLE SQLITE WAL HARNESS
```

---

## 2. Поддерживаемые Платформы и Интеграция

### 1. Antigravity IDE / AGY CLI
- **Конфигурация**: `~/.gemini/config/agctl/`
- **Файл правил**: `~/.gemini/GEMINI.md` и `.agents/rules/`
- **MCP**: `~/.gemini/config/mcp_config.json`
- **Хуки**: нативная поддержка `hooks.json` (router-pre-invocation, loop-pre-invocation, loop-pre-tool, loop-stop)
- **Слэш-команды**: `/agctl-loop`, `/agctl-run`, `/agctl-doctor`, `/agctl-replan`, `/agctl-dashboard`, `/agctl-tasks`

### 2. DeepSeek Harness (DSH)
- **Конфигурация**: `~/.gemini/config/agctl/` или `$AGCTL_CONFIG_ROOT`
- **Файл правил**: `AGENTS.md` (в корне репозитория и домашней директории)
- **Точка входа**: навык `agctl-dsh` и `agctl-loop` через PowerShell (`pwsh`)
- **Исполнение**: sidecar бинарник `agctl.exe` сопрягается с плагинами Cordis (`dsh-skill`, `dsh-workflow`, `dsh-jobs`, `dsh-goal`)

### 3. Cursor & Cursor Agent
- **Конфигурация**: `~/.cursor/`
- **Файл правил**: `.cursorrules`, `.cursor/rules/`, `AGENTS.md`
- **MCP**: `~/.cursor/mcp.json` или `.cursor/mcp.json`
- **Использование**: запуск команд `agctl loop status`, `agctl plan run`, `agctl doctor` прямо из терминала Cursor или фоновых задач

### 4. Claude Code (Anthropic)
- **Конфигурация**: `~/.claude/`
- **Файл правил**: `CLAUDE.md` и `AGENTS.md`
- **MCP**: `~/.claude/mcp_config.json`
- **Использование**: интеграция через CLAUDE.md инструкции и CLI команды

### 5. Roo Code / Cline
- **Конфигурация**: `~/.cline/`
- **Файл правил**: `.roomodes`, `AGENTS.md`
- **MCP**: `~/.cline/mcp_settings.json`

### 6. Universal / Generic Agentic Terminals (Codex CLI, OpenCode, Aider)
- **Стандарт**: стандартное дерево `.agents/skills/`, `.agents/rules/`, `.agents/workflows/` и `AGENTS.md`
- **Исполнение**: автономный цикл через durable Go/SQLite harness

---

## 3. Переменные Окружения для Тонкой Настройки

| Переменная | Описание | Пример значения |
|---|---|---|
| `AGCTL_PLATFORM` | Принудительный выбор платформы | `antigravity`, `dsh`, `cursor`, `claude`, `cline`, `universal` |
| `AGCTL_CONFIG_ROOT` | Путь к глобальной директории конфигурации | `C:\Custom\agctl_config` |
| `AGCTL_STATE_ROOT` | Путь к хранилищу состояния и SQLite Harness | `C:\Custom\agctl_state` |
| `AGCTL_WORKSPACE` | Путь к целевому рабочему пространству | `D:\Projects\MyApp` |

---

## 4. Команды Управления и Диагностики

### Проверка поддерживаемых и активных сред:
```bash
agctl platforms
# или с JSON-выводом для программной интеграции:
agctl platforms --json
```

### Комплексная диагностика среды:
```bash
agctl doctor
# Проверка со встроенным самотестированием:
agctl doctor --self-test
```

### Управление автономным циклом (`/agctl-loop`):
```bash
# Проверить текущий статус цикла:
agctl loop status

# Включить профиль глубокого автономного завершения (50 итераций):
agctl loop enable deep

# Включить профиль до полного завершения (unlimited):
agctl loop enable until-done
```

### Инициализация проекта:
```bash
# Автоматическое определение профиля проекта и создание универсальных правил:
agctl project init
```

Система автоматически создаст или обновит `.agents/`, `AGENTS.md` и правила, готовые к работе на любой из поддерживаемых агентных платформ.
