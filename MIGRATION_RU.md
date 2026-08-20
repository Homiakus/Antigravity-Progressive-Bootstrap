# Миграция agctl 3.2.0 → 3.2.1

Версия 3.2.1 — compatibility/audit patch. Форматы task queue, plans, adaptive replan, provenance и telemetry обратно совместимы с 3.2.0; удалять существующую конфигурацию не нужно.

## Рекомендуемый путь

Распакуйте новый release поверх отдельного каталога и запустите:

```powershell
.\agctl.exe migrate v3.2
.\agctl.exe doctor --self-test
.\agctl.exe doctor --probe-mcp
```

`migrate v3.2`:

- обновляет установленный `agctl.exe`;
- переустанавливает managed hooks/CLI plugin по текущей схеме;
- обновляет embedded custom agents;
- перестраивает Capability Registry;
- сохраняет MCP, remote skill packs, сторонние plugins, task queue, plans, replan history и telemetry.

## Что изменилось семантически

### `/goal`

`agctl goal native` по-прежнему формирует `/goal ...` для Antigravity 2.0 UI. Headless runner (`agy -p`) больше не отправляет `/goal` как CLI-команду; он передаёт эквивалентный until-done contract обычным prompt-текстом.

### MCP probe

Основной probe — MCP 2026-07-28 stateless. Для старых совместимых серверов поддержан fallback на MCP 2025-11-25. Deep probe WebSocket и deprecated HTTP+SSE 2024-11-05 остаются диагностируемыми ограничениями, а не имитируются.

### Go / gopls MCP

`gopls mcp` больше не ставится автоматически через Go project profile. Установите явно, если хотите использовать experimental upstream MCP mode:

```powershell
agctl mcp add gopls --workspace .
```

## После обновления

```powershell
agctl agents doctor
agctl plugins doctor
agctl provenance verify
agctl security audit
agctl doctor --self-test --probe-mcp
```

Полный список результатов аудита: `AUDIT_RU.md`.
