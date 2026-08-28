# Remote Control implementation progress

## Current baseline

- [x] R0 — capability probe scaffold/matrix; runtime verification on installed Antigravity remains pending.
- [x] R1 — typed remote domain model and validation contracts.
- [x] R2 — durable schema v11 in Harness `state.db`; repository/instance/conversation/session stores.
- [x] R3 — repository registry + `agctl remote repo` CLI.
- [x] R4 — Cockpit `working_dir` gap localized and patch contract documented; upstream application pending because `jlcodes99/cockpit-tools` is `push:false`.
- [ ] R5 — `cockpit-control` physical binary in writable Cockpit fork; protocol v1 is defined.
- [x] R6 — strict Go Cockpit adapter; managed launch context carries Bridge bootstrap only via child environment, never argv.
- [x] R7 — loopback-only Antigravity Bridge command-fallback scaffold.
- [x] R8 — strict Go Bridge client + authenticated registration discovery/re-attachment.
- [x] R9 — single-instance RemoteSession provisioning saga implemented with repository/account/workspace/capability/conversation gates and rollback-stop on failure.
- [ ] R9 — real-machine E2E requires physical `cockpit-control` build + Bridge VSIX install.
- [ ] R10+ — reconciler, multi-conversation, multi-instance, worktree isolation, Telegram.

## Enforced decisions

1. Cockpit credentials are absent from remote DTOs and durable state.
2. Bridge token is per-launch, passed only in environment, never argv/registration/SQLite.
3. Registration is not trusted by itself: discovery performs authenticated `/v1/health` and verifies instance + boot nonce.
4. Session becomes `READY` only after repo, account, process, Bridge, workspace, capabilities and conversation all agree.
5. R9 does not silently open a mismatched workspace; it fails closed. R4 Cockpit launch must open the declared `working_dir`.
6. Worktree isolation is intentionally rejected by R9 until R13 uses the existing Harness Workspace Manager.

## Next critical path

`R10 reconciler → R11 multi-conversation dispatch gates → R12 multi-instance resolver → R13 Harness worktree isolation`, while external R4/R5 and runtime Antigravity verification remain environment-dependent gates.
