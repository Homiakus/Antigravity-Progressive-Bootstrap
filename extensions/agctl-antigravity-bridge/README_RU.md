# agctl Antigravity Bridge

Loopback-only VS Code-compatible extension для Antigravity IDE.

## Security boundary

Bridge запускается только при наличии `AGCTL_BRIDGE_TOKEN`, слушает исключительно `127.0.0.1` на случайном порту и никогда не пишет token в registration file. Managed launch передаёт:

```text
AGCTL_INSTANCE_ID
AGCTL_BOOT_NONCE
AGCTL_BRIDGE_TOKEN
AGCTL_BRIDGE_REGISTRY
```

Registration file содержит только instance/boot/PID/port/workspace metadata.

## Реализованный protocol v1

- `GET /v1/health`
- `GET /v1/capabilities`
- `GET /v1/context`
- `GET /v1/conversations`
- `POST /v1/conversations`
- `POST /v1/conversations/{id}/focus`
- `POST /v1/conversations/{id}/messages`
- `POST /v1/workspace/open`

Conversation compatibility использует Antigravity commands, подтверждённые существующим независимым bridge-проектом: `antigravity.getDiagnostics`, `antigravity.startNewConversation`, `antigravity.setVisibleConversation`, `antigravity.sendPromptToAgentPanel`.

Поскольку доступный command fallback не имеет direct send по conversation ID, Bridge сериализует критическую секцию `focus → send`. Capability `conversationDirectSend=false` отражает это явно.

Пока не объявлены поддержанными: full message history, agent delta events, cancel, approval event IDs/decision IDs, native fork. Они должны добавляться только после runtime capability verification; отсутствие этих функций не маскируется.
