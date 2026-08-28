# Remote Control implementation progress

Этот файл является кратким журналом выполнения `REMOTE_CONTROL_IMPLEMENTATION_PLAN_RU.md`.

## Current baseline

- [x] R0 — capability probe scaffold и capability matrix добавлены.
- [ ] R0 — runtime capability verification на установленной Antigravity IDE.
- [x] R1 — типизированная remote domain model, ID kinds, state enums и validation contracts.
- [x] R2 — migration v11 с durable remote-control schema в существующем Harness `state.db`.
- [x] R2 — repository store contract и SQLite implementation.
- [x] R3 — repository registry + CLI `agctl remote status`, `remote repo add|list|enable|disable`.
- [x] R4 — фактический Cockpit `working_dir` gap локализован; reproducible patch contract зафиксирован.
- [ ] R4 — patch применён в writable Cockpit fork/upstream (`jlcodes99/cockpit-tools` доступен `push:false`).
- [ ] R5 — `cockpit-control` binary реализован в writable Cockpit fork; protocol v1 зафиксирован.
- [x] R6 — Go `cockpit.Client` + strict CLI protocol adapter + secret-field fail-closed tests.
- [x] R7 — собственный loopback-only Antigravity Bridge scaffold с command-fallback conversation control и registration metadata.
- [x] R8 — Go Bridge HTTP client с loopback enforcement, bearer auth, protocol negotiation и strict decoding.
- [ ] R7/R8 — runtime capability verification на реальной Antigravity IDE и packaging/install VSIX.
- [ ] R9+ — single-instance RemoteSession/session reconciliation/Telegram.

## Architectural decisions already enforced

1. Cockpit credentials не входят в remote domain model.
2. Telegram transport не является источником истины.
3. Remote state хранится в существующем durable Harness SQLite, отдельная `remote.db` не создаётся.
4. `remote_events` отделены от workflow `events`, потому что IDE events не обязаны иметь `workflow_run_id`.
5. `(source, source_message_id)` в `remote_commands` является durable idempotency key.
6. Активная Telegram topic binding уникальна по `(chat_id, thread_id)`.
7. Репозиторий регистрируется только через canonical Git root; удалённые transports обязаны работать только с зарегистрированными repo IDs.
8. Внутренние Antigravity capability не считаются доступными без negotiation/runtime verification.
9. `agctl` не читает/пишет Cockpit internal state: интеграция только через versioned `cockpit-control` contract.
10. Antigravity Bridge слушает только loopback; Go client также fail-closed отклоняет non-loopback URL.
11. Command-fallback dispatch сериализует `focus → send`, поэтому параллельные Telegram/remote sends не могут перескочить между conversations внутри одного Bridge.

## Next critical path

`R9 single-instance RemoteSession → R10 reconciler → R11 multi-conversation → R12 multi-instance`, параллельно требуется writable Cockpit fork для физического R4/R5 и runtime Antigravity verification для расширения capabilities.
