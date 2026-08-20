# agctl 3.2.1 — Antigravity Control Plane

[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![TUI Architecture](https://img.shields.io/badge/TUI-Bubble%20Tea%20%2B%20Lip%20Gloss-38BDF8?style=flat)](https://github.com/charmbracelet/bubbletea)
[![MCP Protocol](https://img.shields.io/badge/MCP-2026--07--28%20%7C%202025--11--25-22C55E?style=flat)](https://modelcontextprotocol.io/)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-A855F7?style=flat)]()

**`agctl`** — современный высокопроизводительный **Control Plane и оркестратор на Go** для Google Antigravity (AGY) и автономных AI-агентов.

Проект сочетает в себе **полнофункциональный CLI для автоматизации** и **профессиональный 3-колоночный TUI (Terminal User Interface)** в эстетике **Raycast × Linear × Modern IDE**.

---

## 📸 3-Колоночный Терминальный Интерфейс (TUI)

Запуск без аргументов (`agctl` или `Start-agctl.cmd`) открывает интерактивную среду управления:

```text
┌─────────────────────────────────────────────────── agctl 3.2.1 ────────────────────────────────────────────────────────┐
│ ╭── ◈ NAVIGATION ───────╮ ╭── 01 DASHBOARD / WORKSPACE ───────────────╮ ╭── > LIVE CONSOLE & LOGS ─────────────────────╮ │
│ │                       │ │                                           │ │                                              │ │
│ │   01 ● Dashboard      │ │ ╭───────────── SYSTEM METRICS ──────────╮ │ │ [00:53:49] [INFO] agctl 3.2.1 initialized.   │ │
│ │ › 02 + Setup & Doctor │ │ │ Router: true (balanced)               │ │ │ [00:53:49] [NAV] Switched to 02 Setup      │ │
│ │   03 ◆ Capabilities   │ │ │ Loop: false (guarded, max=50)         │ │ │ [00:53:51] [CMD] Starting Doctor Audit...   │ │
│ │   04 ↺ Autonomy & Loop│ │ │ Health: ● HEALTHY • 14 Skills • 6 MCP │ │ │ [00:53:52] [OK] Doctor audit passed (10/10)│ │
│ │   05 ■ Governance     │ │ ╰───────────────────────────────────────╯ │ │                                              │ │
│ │                       │ │                                           │ │                                              │ │
│ │                       │ │   ACTIONS:                                │ │                                              │ │
│ │ 1..5 Jump • Tab Move  │ │    › Recommended Install                  │ │                                              │ │
│ │                       │ │      Full Stable Setup                    │ │                                              │ │
│ ╰───────────────────────╯ ╰───────────────────────────────────────────╯ ╰──────────────────────────────────────────────╯ │
└─ NORMAL   Tab / Shift+Tab cycle panels   ← / → switch   ↑↓ move items   Enter execute   ctrl+k commands   ? help ──────┘
```

### Архитектура интерфейса:
1. **Левая колонка (`◈ НАВИГАЦИЯ`)**: боковое меню с 6 разделами (`01` Панель управления, `02` Установка и Doctor, `03` Возможности и MCP, `04` Автономия и Loop, `05` Безопасность и Ops, `06` Настройки программы) и мгновенным переходом по цифрам `1`..`6`.
2. **Центральная колонка (`РАБОЧАЯ ЗОНА`)**: интерактивная рабочая область (метрики, переключатели режимов, таблицы MCP, задачи, аудит и параметры программы).
3. **Правая колонка (`> ЖИВАЯ КОНСОЛЬ И ЛОГИ`)**: встроенный живой терминал логов с подсветкой статусов (`[OK]`, `[CMD]`, `[INFO]`, `[WARN]`, `[ERROR]`) и возможностью очистки (`c`).
4. **Многоязычность (RU / EN, по умолчанию Русский)**:
   - Переключение в реальном времени в разделе `06 Настройки` или через палитру команд `Ctrl+K`.
   - Автоматическое сохранение выбранного языка, темы оформления (Dark/Light) и акцентных цветов в `tui_settings.json`.
5. **Keyboard-First управление**:
   - `1`..`6` — мгновенный переход в любой раздел.
   - `Tab` / `Shift+Tab` или `←` / `→` — удобный переход фокуса между 3 колонками.
   - `↑` / `↓` или `j` / `k` — навигация по элементам активного окна.
   - `Enter` / `Space` — запуск действия или переключение опции.
   - `Ctrl+K` — глобальная палитра команд (Command Palette с fuzzy search).
   - `?` — всплывающая шпаргалка горячих клавиш.
   - `q` / `Esc` — возврат / закрытие модальных окон / выход.

---

## ⚡ Быстрый старт

### 1. Сборка из исходников
```powershell
go build -o agctl.exe ./cmd/agctl
.\agctl.exe doctor --self-test
```

### 2. Запуск в 1 клик (Windows скрипты)
* **`Start-agctl.cmd`** — запуск интерактивного TUI.
* **`Doctor.cmd`** — быстрая диагностика окружения.
* **`Doctor-Live-MCP.cmd`** — живой пинг и опрос всех MCP-серверов.
* **`Install-Recommended.cmd`** — установка базовых meta-skills и хуков.
* **`Install-Full.cmd`** — полная стабильная установка всех пакетов.
* **`Run-Adaptive-Queue.cmd`** — запуск воркера очереди фоновых задач.

---

## 🛠 Ключевые возможности и CLI команды

`agctl` спроектирован по стандарту Unix: **вся бизнес-логика изолирована от TUI** и доступна через CLI-команды для CI/CD и скриптов.

### 1. Установка и обновление
```powershell
agctl install recommended          # Рекомендованный стабильный набор
agctl install full --prereqs       # Полный набор (все skillpacks, sidecars, prereqs)
agctl install self                 # Компиляция бинарника в PATH и регистрация хуков
```

### 2. Диагностика (Doctor)
```powershell
agctl doctor                       # Базовая проверка среды и путей
agctl doctor --probe-mcp           # Полная диагностика с живым пингом MCP
agctl doctor --self-test           # Самотестирование хуков и сохранения состояния
```

### 3. Навыки (Skills & Packs)
```powershell
agctl skills list                  # Список установленных навыков
agctl skills install embedded      # Установка встроенных meta-skills
agctl skills sync-recommended      # Синхронизация superpowers, agent-skills, no-ai-slop
agctl skills sync-pack <pack-id>   # Синхронизация конкретного набора
```

### 4. Model Context Protocol (MCP)
Поддержка протоколов MCP `2026-07-28` и `2025-11-25`:
```powershell
agctl mcp list                     # Список серверов
agctl mcp probe                    # Живой опрос задержки и проверка инструментов
agctl mcp add context7 playwright  # Подключение серверов из каталога
agctl mcp remove <id>              # Удаление сервера
```

### 5. Адаптивный роутер (Adaptive Router)
Анализирует запросы пользователя и подбирает **минимально достаточный набор инструментов** (навыки, MCP, агенты):
```powershell
agctl router status                # Текущее состояние
agctl router enable balanced       # Режимы: silent | balanced | transparent | maximum
agctl router disable               # Отключение перехвата
```

### 6. Автономный цикл (Autonomous Loop)
Движок непрерывного исполнения `plan -> implement -> test -> fix -> retest` до достижения верифицированного критерия готовности:
```powershell
agctl loop status                  # Статус цикла
agctl loop enable standard         # Профили: standard | deep | until-done | unrestricted
agctl loop disable                 # Выключение
```

### 7. Фоновая очередь задач (Headless Tasks Queue)
Асинхронное выполнение задач с контролем лимитов ресурсов (CPU, слоты сборки, браузер):
```powershell
agctl tasks add --prompt "Напиши unit-тесты" --priority 10 --cpu 25
agctl tasks list                   # Список задач и их статусы
agctl tasks run-pending            # Запуск обработки очереди
agctl tasks cancel <task-id>       # Отмена задачи
```

### 8. Безопасность и управление (Governance & Audit)
```powershell
agctl permissions apply autonomous # Профили: safe | balanced | autonomous | yolo
agctl permissions audit            # Аудит политик выполнения инструментов
agctl security audit               # 0..100 Control-Plane Security Score
agctl backup create                # Создание снимка конфигурации
agctl backup list                  # Список бэкапов
```

---

## 📂 Структура проекта

```text
├── cmd/
│   └── agctl/               # Точка входа CLI / TUI (main.go)
├── internal/
│   ├── tui/                 # Bubble Tea TUI
│   │   ├── app.go           # Root Model, 3-колоночный layout, диспетчер фокуса
│   │   ├── keymap.go        # Централизованная карта клавиш (bubbles/key)
│   │   ├── theme/           # Семантическая палитра, TrueColor / NO_COLOR fallback
│   │   ├── components/      # Sidebar, Console, Header, StatusBar, Palette, Help
│   │   └── views/           # Dashboard, Setup, Capabilities, Autonomy, Governance
│   ├── agents/              # Оркестрация субагентов
│   ├── doctor/              # Диагностический движок (Doctor)
│   ├── installer/           # Установщик и миграции версий
│   ├── loop/                # Autonomous Completion Loop
│   ├── mcp/                 # Менеджер MCP серверов
│   ├── mcpprobe/            # Live probe протокола MCP (latency & tools ping)
│   ├── permissions/         # Политики разрешений и песочницы
│   ├── router/              # Адаптивный роутер инструментов
│   ├── securityaudit/       # Аудит безопасности и provenance
│   ├── skills/              # Синхронизация и установка скиллов
│   └── tasks/               # Очередь фоновых задач
└── .gitignore               # Стандартные правила игнорирования Go
```

---

## 🧪 Тестирование и верификация

Все компоненты покрыты автоматическими тестами:

```powershell
# Запуск всех тестов проекта
go test ./...

# Запуск тестов TUI верстки и адаптивности
go test -v ./internal/tui/...
```

---

## 📄 Лицензия

MIT License. Сделано для максимальной продуктивности автономной разработки в среде Google Antigravity.