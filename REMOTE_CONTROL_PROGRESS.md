# Remote Control implementation progress

Этот файл является кратким журналом выполнения `REMOTE_CONTROL_IMPLEMENTATION_PLAN_RU.md`.

## Current baseline

- [x] R0 — capability probe scaffold и capability matrix добавлены.
- [ ] R0 — runtime capability verification на установленной Antigravity IDE.
- [x] R1 — типизированная remote domain model, ID kinds, state enums и validation contracts.
- [x] R2 — migration v11 с durable remote-control schema в существующем Harness `state.db`.
- [x] R2 — repository store contract и SQLite implementation.
- [x] R3 — repository registry с canonical path, Git metadata, allowlisted roots и idempotent registration.
- [ ] R3 — CLI surface `agctl remote repo ...` (следующий вертикальный шаг).
- [ ] R4+ — Cockpit/Bridge/session/Telegram layers.

## Architectural decisions already enforced

1. Cockpit credentials не входят в remote domain model.
2. Telegram transport не является источником истины.
3. Remote state хранится в существующем durable Harness SQLite, отдельная `remote.db` не создаётся.
4. `remote_events` отделены от workflow `events`, потому что IDE events не обязаны иметь `workflow_run_id`.
5. `(source, source_message_id)` в `remote_commands` является durable idempotency key.
6. Активная Telegram topic binding уникальна по `(chat_id, thread_id)`.
7. Репозиторий регистрируется только через canonical Git root и может быть ограничен allowlisted roots.
8. Внутренние Antigravity capability не считаются доступными без negotiation/runtime verification.

## Next critical path

`R3 CLI → R4 Cockpit working_dir patch → R5 cockpit-control → R6 Go Cockpit adapter → R7 Bridge → R8 Bridge client → R9 single-instance RemoteSession`.
