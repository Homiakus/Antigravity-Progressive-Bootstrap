# agctl 3.2.1 — Antigravity Control Plane

<p align="center">
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go Version" /></a>
  <a href="https://github.com/charmbracelet/bubbletea"><img src="https://img.shields.io/badge/TUI-Bubble%20Tea%20%2B%20Lip%20Gloss-38BDF8?style=for-the-badge&logo=terminal&logoColor=white" alt="TUI Architecture" /></a>
  <a href="https://modelcontextprotocol.io/"><img src="https://img.shields.io/badge/MCP-2026--07--28%20%7C%202025--11--25-22C55E?style=for-the-badge&logo=codeforces&logoColor=white" alt="MCP Protocol" /></a>
  <a href="#"><img src="https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-A855F7?style=for-the-badge&logo=windows&logoColor=white" alt="Platform" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-F59E0B?style=for-the-badge" alt="License" /></a>
</p>

---

**`agctl`** — современный высокопроизводительный **Control Plane и оркестратор на Go** для Google Antigravity (AGY) и автономных AI-агентов.

Проект объединяет в себе **мощный CLI-движок автоматизации** (headless queues, MCP probes, permissions, DAG replanning) и **интерактивный 3-колоночный TUI (Terminal User Interface)** в эстетике **Raycast × Linear × Modern IDE**.

> 📖 **Документация и спецификации:**
> - [🇷🇺 Подробное руководство на русском (README_RU.md)](file:///d:/Programms/Antigravity-Progressive-Bootstrap/README_RU.md)
> - [🏛 Архитектурное руководство (ARCHITECTURE_RU.md)](file:///d:/Programms/Antigravity-Progressive-Bootstrap/ARCHITECTURE_RU.md)
> - [🔍 Аудит совместимости Antigravity 2.0 / MCP (AUDIT_RU.md)](file:///d:/Programms/Antigravity-Progressive-Bootstrap/AUDIT_RU.md)
> - [📜 История изменений (CHANGELOG.md)](file:///d:/Programms/Antigravity-Progressive-Bootstrap/CHANGELOG.md)
> - [🔄 Руководство по миграции (MIGRATION_RU.md)](file:///d:/Programms/Antigravity-Progressive-Bootstrap/MIGRATION_RU.md)

---

## 📸 3-Колоночный Терминальный Интерфейс (TUI)

Запуск без аргументов (`agctl` или `Start-agctl.cmd`) открывает полноэкранный терминальный командный центр:

```text
┌────────────────────────────────────────────────────────── agctl 3.2.1 ──────────────────────────────────────────────────────────┐
│ ╭── ◈ НАВИГАЦИЯ ─────────╮ ╭── 01 ПАНЕЛЬ УПРАВЛЕНИЯ / РАБОЧАЯ ЗОНА ─────╮ ╭── > ЖИВАЯ КОНСОЛЬ И ЛОГИ ─────────────────────────╮ │
│ │                        │ │                                             │ │                                                  │ │
│ │   01 ● Панель          │ │ ╭───────────── СИСТЕМНЫЕ МЕТРИКИ ─────────╮ │ │ [00:53:49] [INFO] agctl 3.2.1 initialized.       │ │
│ │ › 02 + Установка       │ │ │ Router: true (balanced)                 │ │ │ [00:53:49] [NAV]  Переход в раздел: 02 Setup   │ │
│ │   03 ◆ Возможности     │ │ │ Loop: false (guarded, max=50)           │ │ │ [00:53:51] [CMD]  Запуск аудита окружения...   │ │
│ │   04 ↺ Автономия       │ │ │ Здоровье: ● ВСЁ В ПОРЯДКЕ (14/14 OK)    │ │ │ [00:53:52] [OK]   Doctor: все тесты пройдены   │ │
│ │   05 ■ Безопасность    │ │ ╰─────────────────────────────────────────╯ │ │                                                  │ │
│ │   06 ⚙ Настройки       │ │                                             │ │                                                  │ │
│ │                        │ │   ДЕЙСТВИЯ:                                 │ │                                                  │ │
│ │ 1..6 Быстрый переход   │ │    › Рекомендованная установка (Quick)      │ │                                                  │ │
│ │ Tab / ←→ Смена панели  │ │      Полная стабильная установка (Full)     │ │                                                  │ │
│ ╰────────────────────────╯ ╰─────────────────────────────────────────────╯ ╰──────────────────────────────────────────────────╯ │
└─ NORMAL   Tab / Shift+Tab панели   ← / → колонки   ↑↓ выбор   Enter запуск   Ctrl+K палитра   ? помощь   q выход ───────────────┘
```

### Возможности интерфейса:
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

### 1. Запуск в 1 клик (Windows CMD скрипты)
* **`Start-agctl.cmd`** — запуск интерактивного 3-колоночного TUI.
* **`Doctor.cmd`** — быстрая диагностика окружения и путей.
* **`Doctor-Live-MCP.cmd`** — живой пинг и опрос всех MCP-серверов.
* **`Install-Recommended.cmd`** — установка базовых meta-skills и хуков.
* **`Install-Full.cmd`** — полная стабильная установка всех пакетов и утилит.
* **`Run-Adaptive-Queue.cmd`** — запуск воркера очереди фоновых задач.
* **`Init-Current-Project.cmd`** — инициализация правил и скиллов в текущем репозитории.

### 2. Сборка из исходников
```powershell
# Клонирование репозитория
git clone https://github.com/Homiakus/Antigravity-Progressive-Bootstrap.git
cd Antigravity-Progressive-Bootstrap

# Сборка бинарника
go build -o agctl.exe ./cmd/agctl

# Диагностика и самотестирование
.\agctl.exe doctor --self-test
```

---

## 🚀 Ключевые возможности

### 🔄 1. Adaptive DAG Replanning
- Динамическое перестроение графа задач агентов на лету в зависимости от результатов тестов и исследований.
- Прием структурированных replan-предложений с валидацией `evidence`, `confidence` и `semantic risk`.
- Автоматическая вставка фаз `diagnose → repair → reverify` при ошибках.
- **No-progress detector** для защиты от бесконечных циклов.

### 🩺 2. Диагностический движок (Doctor)
- Проверка системных зависимостей, путей Antigravity 2.0 (`.gemini/antigravity-ide`), хуков и правил.
- **MCP Live Probe**: опрос серверов по протоколу `2026-07-28` (`server/discover`) и fallback на `2025-11-25` (latency, tool availability).
- Самодиагностика и аудит целостности конфигурации.

### 🧠 3. Менеджер навыков (Skills & Packs)
- Управление встроенными навыками (`agctl`, `agctl-doctor`, `agctl-run`, `agctl-loop`, `agctl-replan`, `agctl-dashboard`, `agctl-tasks`).
- Синхронизация проверенных наборов навыков: `superpowers`, `agent-skills`, `no-ai-slop`, `gemini-skills`.
- Полный расчет SHA-256 provenance для всех директорий навыков.

### 🎯 4. Адаптивный роутер (Adaptive Router)
- Анализирует пользовательский запрос и выбирает **минимально достаточный набор** инструментов (MCP, навыки, субагенты).
- Режимы: `silent` | `balanced` | `transparent` | `maximum`.

### 🔁 5. Автономный цикл (Autonomous Loop)
- Контракт непрерывного исполнения: `plan -> implement -> test -> diagnose -> fix -> reverify -> DoD`.
- Профили: `standard` | `deep` | `until-done` | `unrestricted`.
- Guardrails и автоматические лимиты итераций.

### 📋 6. Фоновая очередь задач (Headless Task Queue)
- Неблокирующее выполнение фоновых задач с контролем ресурсов (CPU-лимиты, слоты сборки, браузер).
- Защита от race conditions через атомарные task claims.

### 🛡 7. Безопасность и Governance
- Профили разрешений: `safe` | `balanced` | `autonomous` | `yolo`.
- Аудит безопасности control plane с оценкой Security Score (0..100).
- Создание и восстановление снимков конфигурации (`agctl backup`).

---

## 🛠 Справочник CLI команд

```powershell
# --- Установка и миграция ---
agctl install recommended          # Рекомендованный стабильный набор
agctl install full --prereqs       # Полный набор (skillpacks, sidecars, prereqs)
agctl install self                 # Инсталляция в PATH и регистрация хуков

# --- Диагностика (Doctor) ---
agctl doctor                       # Базовая проверка среды
agctl doctor --probe-mcp           # Диагностика с живым пингом MCP серверов
agctl doctor --self-test           # Комплексный тест хуков и состояния

# --- Навыки (Skills) ---
agctl skills list                  # Список установленных навыков
agctl skills install embedded      # Установка встроенных meta-skills
agctl skills sync-recommended      # Синхронизация рекомендованных библиотек
agctl skills sync-pack <pack-id>   # Синхронизация конкретного набора

# --- MCP серверы ---
agctl mcp list                     # Список зарегистрированных серверов
agctl mcp probe                    # Живая проверка задержки и доступности tools
agctl mcp add context7 playwright  # Добавление серверов из каталога
agctl mcp remove <id>              # Удаление сервера

# --- Адаптивный роутер ---
agctl router status                # Текущий статус роутера
agctl router enable balanced       # Включение режима (silent|balanced|transparent|maximum)
agctl router disable               # Отключение перехвата

# --- Автономный цикл (Loop) ---
agctl loop status                  # Статус цикла
agctl loop enable standard         # Профили: standard|deep|until-done|unrestricted
agctl loop disable                 # Выключение

# --- Адаптивное планирование (Plan & DAG) ---
agctl plan run --prompt "..."      # Выполнение задачи с адаптивным перепланированием
agctl replan status                # Статус подсистемы replanning
agctl replan enable                # Включение Dynamic DAG

# --- Очередь задач (Tasks) ---
agctl tasks add --prompt "..."     # Добавление задачи в очередь
agctl tasks list                   # Список задач
agctl tasks run-pending            # Запуск обработки очереди
agctl tasks cancel <task-id>       # Отмена задачи

# --- Безопасность и бэкапы ---
agctl permissions apply autonomous # Применение профиля разрешений
agctl permissions audit            # Аудит политик выполнения
agctl security audit               # 0..100 Control-Plane Security Score
agctl backup create                # Создание снимка конфигурации
agctl backup list                  # Список сохраненных бэкапов
```

---

## 📂 Структура проекта

```text
├── .agents/skills/          # Встроенные slash-command навыки для Antigravity
├── cmd/
│   └── agctl/               # Точка входа CLI / TUI (main.go)
├── internal/
│   ├── tui/                 # Bubble Tea 3-колоночный TUI
│   │   ├── app.go           # Root Model, layout, диспетчер фокуса
│   │   ├── keymap.go        # Централизованная карта клавиш (bubbles/key)
│   │   ├── theme/           # Цветовые темы (Dark/Light/Dimmed, Accent Colors)
│   │   ├── i18n/            # Локализация интерфейса (RU / EN)
│   │   ├── components/      # Sidebar, Console, Header, StatusBar, Palette, Modal, Toast
│   │   └── views/           # Dashboard, Setup, Capabilities, Autonomy, Governance, Settings
│   ├── agents/              # Оркестрация субагентов
│   ├── backup/              # Снимки и восстановление конфигураций
│   ├── capability/          # Индексация и ранжирование возможностей
│   ├── dashboard/           # Веб-панель мониторинга
│   ├── doctor/              # Диагностический движок (Doctor)
│   ├── installer/           # Установщик и миграции версий
│   ├── loop/                # Autonomous Completion Loop
│   ├── mcp/                 # Менеджер конфигураций MCP
│   ├── mcpprobe/            # Live probe протокола MCP (latency & tools ping)
│   ├── permissions/         # Политики разрешений и песочницы
│   ├── planner/             # DAG планировщик задач
│   ├── replan/              # Adaptive DAG Replanning Engine
│   ├── risk/                # Семантический анализ рисков
│   ├── router/              # Адаптивный роутер инструментов
│   ├── securityaudit/       # Аудит безопасности и provenance
│   ├── skills/              # Синхронизация и установка скиллов
│   ├── tasks/               # Очередь фоновых задач и scheduler
│   └── worktree/            # Изоляция задач через Git worktrees
├── .gitignore               # Правила игнорирования сборки и окружения
└── go.mod                   # Зависимости Go 1.24+
```

---

## 🧪 Тестирование и верификация

Все модули проекта полностью покрыты автоматическими тестами:

```powershell
# Запуск полного набора unit-тестов
go test ./...

# Запуск тестов TUI, компонентов и темы
go test -v ./internal/tui/...

# Проверка форматирования и сборки
go vet ./...
go build ./...
```

---

## 📄 Лицензия

Распространяется под лицензией [MIT](LICENSE). Разработано для максимальной продуктивности автономной разработки в среде Google Antigravity.