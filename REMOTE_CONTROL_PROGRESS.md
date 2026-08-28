# Remote Control implementation progress

Этот файл является кратким журналом выполнения `REMOTE_CONTROL_IMPLEMENTATION_PLAN_RU.md`.

## Current baseline

- [x] R0 — capability probe scaffold и capability matrix добавлены.
- [ ] R0 — runtime capability verification на установленной Antigravity IDE.
- [x] R1 — типизированная remote domain model, ID kinds, state enums и validation contracts.
- [x] R2 — migration v11 с durable remote-control schema в существующем Harness `state.db`.
- [x] R2 — repository store contract и SQLite implementation.
- [x] R3 — repository registry с canonical path, Git metadata, allowlisted roots и idempotent registration.
- [x] R3 — CLI surface: `agctl remote status`, `remote repo add|list|enable|disable`.
- [x] R4 — фактический Cockpit `working_dir` gap локализован до Tauri boundary/start path; reproducible patch contract зафиксирован.
- [ ] R4 — patch применён в writable Cockpit fork/upstream (текущее подключение `jlcodes99/cockpit-tools` имеет `push:false`).
- [ ] R5 — `cockpit-control` binary реализован в writable Cockpit fork; protocol v1 зафиксирован в этом repo.
- [x] R6 — Go `cockpit.Client` + strict CLI protocol adapter + secret-field fail-closed tests.
- [ ] R7+ — Antigravity Bridge/session/Telegram layers.

## Architectural decisions already enforced

1. Cockpit credentials не входят в remote domain model.
2. Telegram transport не является источником истины.
3. Remote state хранится в существующем durable Harness SQLite, отдельная `remote.db` не создаётся.
4. `remote_events` отделены от workflow `events`, потому что IDE events не обязаны иметь `workflow_run_id`.
5. `(source, source_message_id)` в `remote_commands` является durable idempotency key.
6. Активная Telegram topic binding уникальна по `(chat_id, thread_id)`.
7. Репозиторий регистрируется только через canonical Git root; удалённые transports обязаны работать только с зарегистрированными repo IDs.
8. Внутренние Antigravity capability не считаются доступными без negotiation/runtime verification.
9. Remote CLI использует тот же Harness `state.db` и отдельную domain-store абстракцию поверх него.
10. `agctl` не читает/пишет Cockpit internal state: интеграция только через versioned `cockpit-control` contract.

## Next critical path

`Writable Cockpit fork для R4/R5 || параллельно R7 Bridge → R8 Bridge client → R9 single-instance RemoteSession`.
