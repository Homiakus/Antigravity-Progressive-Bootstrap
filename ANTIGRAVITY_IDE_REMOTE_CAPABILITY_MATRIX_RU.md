# Antigravity IDE Remote Capability Matrix

Статус: **R0 capability scaffold implemented; runtime verification must be executed against the installed Antigravity IDE.**

Цель матрицы — не предполагать наличие внутренних API Antigravity. Каждый пункт должен быть подтверждён probe/Bridge integration test и иметь один из статусов: `SUPPORTED`, `UNSUPPORTED`, `FALLBACK`.

| Capability | Требование | Fallback | Runtime status |
|---|---|---|---|
| health / extension presence | обязательно | stop provisioning | PENDING_RUNTIME |
| stable instance identity | обязательно | boot nonce + Cockpit instance id | PENDING_RUNTIME |
| workspace context | обязательно | VS Code workspace folders | PENDING_RUNTIME |
| open workspace | обязательно | `vscode.openFolder` | PENDING_RUNTIME |
| list conversations | обязательно | none | PENDING_RUNTIME |
| stable conversation id | обязательно | local alias only, no continuity guarantee | PENDING_RUNTIME |
| create conversation | обязательно | focus UI-compatible command | PENDING_RUNTIME |
| focus conversation | обязательно | serialized UI focus | PENDING_RUNTIME |
| send prompt | обязательно | focus-then-send gate | PENDING_RUNTIME |
| direct send by conversation id | желательно | `InstanceCommandGate` | PENDING_RUNTIME |
| message history | желательно | remote-only mirror | PENDING_RUNTIME |
| local user-message event | желательно | `REMOTE_ONLY` mirror | PENDING_RUNTIME |
| agent delta event | желательно | final-message polling/event | PENDING_RUNTIME |
| agent final event | обязательно | status polling | PENDING_RUNTIME |
| tool start/finish events | желательно | verbose status unavailable | PENDING_RUNTIME |
| approval requested event | обязательно | session `NEEDS_ATTENTION` | PENDING_RUNTIME |
| approval decision | обязательно | local IDE approval only | PENDING_RUNTIME |
| cancel current generation | обязательно | conversation remains attached | PENDING_RUNTIME |
| native fork | опционально | handoff fork | PENDING_RUNTIME |
| conversation survives IDE restart | обязательно исследовать | handoff/re-attach | PENDING_RUNTIME |
| conversation survives account switch | обязательно исследовать | explicit handoff | PENDING_RUNTIME |

## Probe

В репозитории используется `tools/antigravity-probe`. После установки собственного Bridge:

```bash
go run ./tools/antigravity-probe -url http://127.0.0.1:<port>
```

При защищённом Bridge токен передаётся через `AGCTL_BRIDGE_TOKEN` или `-token`. Probe не сохраняет токен в файл.

## Exit criteria R0

R0 считается полностью закрытым только после запуска на реальной Antigravity IDE и фиксации для каждой capability фактического статуса, версии IDE и версии Bridge. До этого дальнейшая реализация обязана использовать capability negotiation и не считать `directConversationSend`, `fullMirror` или `nativeFork` доступными по умолчанию.
