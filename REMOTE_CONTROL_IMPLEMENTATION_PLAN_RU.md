# Remote Control v1 — атомарный план внедрения Antigravity IDE + Cockpit Tools + Telegram

## 0. Цель

Построить в `Antigravity-Progressive-Bootstrap` полноценный Remote Session Control Plane, который позволяет безопасно и детерминированно управлять одним или несколькими экземплярами Antigravity IDE, несколькими разговорами агентов, несколькими Cockpit-профилями/аккаунтами и Telegram-топиками, не смешивая ответственность Cockpit, IDE Bridge, Harness и transport/UI.

Система должна поддерживать:

- запуск, остановку, фокус и переиспользование нескольких Antigravity IDE;
- привязку каждого IDE instance к Cockpit account/profile;
- один или несколько conversations внутри одного IDE;
- несколько IDE параллельно;
- работу с одним и тем же репозиторием через изолированные git worktrees;
- Telegram как удалённый control surface и зеркало conversation events;
- approvals, pause/resume/cancel через существующий Harness/risk layer;
- crash recovery и reconciliation после рестартов `agctl`, Cockpit, IDE и Bridge;
- идемпотентность Telegram-команд и durable event/outbox delivery;
- отсутствие credentials/tokens в RemoteSession, Telegram и логах.

---

# 1. Архитектурные границы

## 1.1. Cockpit Tools

Cockpit отвечает только за:

- `InstanceProfile`;
- `user_data_dir`;
- `working_dir`;
- `bind_account_id`;
- запуск/остановку/focus IDE process;
- подготовку/инъекцию аккаунта в профиль;
- PID и lifecycle instance.

Cockpit не должен становиться владельцем RemoteSession, Telegram state или Harness workflow.

## 1.2. Antigravity Bridge

Bridge Extension отвечает только за runtime IDE:

- capability negotiation;
- current workspace;
- список conversations;
- create/focus conversation;
- send prompt;
- cancel;
- approvals;
- normalized IDE events;
- Bridge heartbeat/discovery.

Bridge не должен читать Cockpit storage и не должен владеть durable remote state.

## 1.3. Harness

Harness остаётся владельцем:

- durable workflows;
- node/attempt state;
- approvals/signals;
- workspace allocation;
- exclusive/persistent workspace locks;
- git worktree isolation;
- workflow event/outbox model.

Remote subsystem должен переиспользовать Harness Workspace Manager вместо собственного второго worktree manager.

## 1.4. Remote Session Manager

`agctl` Remote Session Manager связывает Cockpit instance, workspace, conversation, Telegram binding и при необходимости Harness workflow.

Именно `RemoteSession` является источником истины для удалённого управления.

## 1.5. Telegram

Telegram — только transport/UI.

Telegram handler не должен напрямую:

- запускать процесс;
- переключать аккаунт;
- выполнять git command;
- approve tool action.

Все действия проходят через типизированные RemoteCommand/service/risk interfaces.

---

# 2. Не смешивать четыре вида «сессии»

Ввести четыре отдельные сущности:

1. `CockpitInstance` — профиль + account binding + IDE process.
2. `IDEWorkspace` — открытая рабочая директория/репозиторий.
3. `IDEConversation` — конкретный разговор с агентом.
4. `RemoteSession` — логическая связка instance + workspace + conversation + Telegram + optional workflow.

Целевая связь:

```text
CockpitInstance
├── Account
├── UserDataDir
├── PID
└── IDEWorkspace
    ├── Repository
    ├── Conversation A
    ├── Conversation B
    └── Conversation C

RemoteSession
├── CockpitInstance
├── IDEWorkspace
├── IDEConversation
├── TelegramBinding
└── optional WorkflowRunID
```

---

# 3. Domain model

Создать:

```text
internal/remote/model/
├── ids.go
├── repository.go
├── instance.go
├── workspace.go
├── conversation.go
├── session.go
├── command.go
├── event.go
├── telegram.go
├── state.go
└── validation.go
```

## 3.1. Typed IDs

Не использовать голые `string` по всему коду.

```go
type RemoteSessionID string
type RepositoryID string
type WorkspaceID string
type ConversationID string
type HostID string
type TelegramBindingID string
```

## 3.2. RemoteSession

```go
type RemoteSession struct {
    ID RemoteSessionID

    HostID HostID

    CockpitInstanceID string
    CockpitAccountID  string

    RepositoryID  RepositoryID
    WorkspaceID   WorkspaceID
    WorkspacePath string

    ConversationID ConversationID

    TelegramBindingID TelegramBindingID

    DesiredState SessionDesiredState
    ObservedState SessionObservedState

    IsolationMode IsolationMode

    WorkflowRunID string

    CreatedAt time.Time
    UpdatedAt time.Time
}
```

RemoteSession никогда не хранит OAuth token, refresh token, cookies, Antigravity credentials или Telegram bot token.

Для любых секретов используется только `SecretRef`.

---

# 4. Инварианты

## I-01. Один Cockpit instance = один уникальный `user_data_dir`

Два instance не могут управляться как один профиль.

## I-02. Conversation всегда принадлежит конкретному instance/workspace

Conversation нельзя одновременно привязать к двум instance без явной migration/handoff операции.

## I-03. Account switch не меняет контекст активной RemoteSession молча

Перед account switch обязательны:

1. freeze;
2. checkpoint;
3. suspend/detach conversations;
4. stop IDE;
5. account rebind;
6. restart IDE;
7. bridge reconciliation;
8. conversation reconciliation.

## I-04. Один physical write workspace имеет максимум одного write owner

Для параллельной записи в один repo создаются разные worktrees.

## I-05. Telegram update обрабатывается идемпотентно

Повторный update/callback не может дважды выполнить start/send/approve/stop/push.

## I-06. Credentials не попадают в Telegram и remote logs

Даже debug mode не исключение.

## I-07. Destructive reconciliation запрещён по умолчанию

Несоответствие observed/desired state не должно автоматически удалять, пересоздавать или переключать аккаунты без policy.

---

# 5. Патч Cockpit Tools

Cockpit core уже поддерживает `working_dir`; довести значение от UI/Tauri до core/launcher.

## 5.1. Изменить Tauri boundary

Проверить и изменить:

```text
src-tauri/src/commands/instance.rs
```

В `create_instance` и `update_instance` перестать передавать `working_dir: None` и пробросить реальное значение из DTO.

## 5.2. Обновить frontend DTO/service

Добавить/проверить поле:

```text
workingDir?: string
```

во всех create/update/view моделях.

## 5.3. Validation

Перед сохранением:

- path canonicalization;
- directory exists;
- directory is directory;
- не удалять repository при удалении Cockpit profile.

## 5.4. Тесты

Добавить tests:

- create instance with working_dir;
- update working_dir;
- restart Cockpit and persistence;
- launch uses correct working_dir;
- duplicate profile/user_data_dir rejected.

---

# 6. `cockpit-control` — стабильный integration surface

Не читать/писать Cockpit internal JSON/SQLite напрямую из Go.

Создать Rust CLI поверх существующего `cockpit-core`:

```text
cockpit-tools/
└── crates/
    └── cockpit-control/
        ├── Cargo.toml
        └── src/
            ├── main.rs
            ├── protocol.rs
            ├── account.rs
            ├── instance.rs
            └── error.rs
```

Команды:

```text
cockpit-control protocol --json
cockpit-control accounts list --json
cockpit-control instances list --json
cockpit-control instance get <id> --json
cockpit-control instance create ... --json
cockpit-control instance update ... --json
cockpit-control instance start <id> --json
cockpit-control instance stop <id> --json
cockpit-control instance focus <id> --json
cockpit-control instance bind-account <id> <account> --json
```

Protocol envelope:

```json
{
  "protocolVersion": 1,
  "success": true,
  "data": {}
}
```

Ошибки должны иметь стабильный `code`:

```json
{
  "protocolVersion": 1,
  "success": false,
  "error": {
    "code": "INSTANCE_NOT_FOUND",
    "message": "..."
  }
}
```

Никогда не отдавать account tokens.

---

# 7. Go Cockpit adapter

Создать:

```text
internal/cockpit/
├── client.go
├── cli.go
├── model.go
├── protocol.go
├── errors.go
├── reconcile.go
└── fake.go
```

Интерфейс:

```go
type Client interface {
    Health(ctx context.Context) (Health, error)
    ListAccounts(ctx context.Context) ([]Account, error)
    ListInstances(ctx context.Context) ([]Instance, error)
    GetInstance(ctx context.Context, id string) (Instance, error)
    CreateInstance(ctx context.Context, spec InstanceSpec) (Instance, error)
    UpdateInstance(ctx context.Context, id string, patch InstancePatch) (Instance, error)
    StartInstance(ctx context.Context, id string) (Instance, error)
    StopInstance(ctx context.Context, id string) error
    FocusInstance(ctx context.Context, id string) error
    BindAccount(ctx context.Context, id, accountID string) error
}
```

Обязательные свойства:

- binary discovery;
- protocol version negotiation;
- timeout per command;
- context cancellation;
- strict JSON decode;
- sanitized errors;
- fake client для unit/integration tests.

---

# 8. Antigravity capability spike

До Telegram и production Bridge провести отдельное исследование текущей IDE.

Создать:

```text
tools/antigravity-probe/
docs/ANTIGRAVITY_IDE_REMOTE_CAPABILITY_MATRIX_RU.md
```

Проверить:

| Capability | Требование |
|---|---|
| list conversations | mandatory |
| stable conversation ID | mandatory |
| create conversation | mandatory |
| focus conversation | mandatory |
| send prompt | mandatory |
| direct send by conversation ID | investigate |
| message history | investigate |
| user-message event | investigate |
| agent delta/final event | investigate |
| terminal/tool approval event | mandatory |
| approval decision | mandatory |
| cancel generation | mandatory |
| native fork | optional |
| workspace open | mandatory |
| persistence after IDE restart | mandatory |
| persistence after account switch | investigate |

Для каждой capability зафиксировать ровно одно состояние:

- `supported`;
- `unsupported`;
- `fallback`.

Никаких `unknown` после завершения spike.

---

# 9. Собственный Antigravity Bridge Extension

Создать:

```text
extensions/
└── agctl-antigravity-bridge/
    ├── package.json
    ├── tsconfig.json
    └── src/
        ├── extension.ts
        ├── server.ts
        ├── protocol.ts
        ├── auth.ts
        ├── bridge/
        │   ├── capabilities.ts
        │   ├── discovery.ts
        │   └── heartbeat.ts
        ├── workspace/
        │   ├── current.ts
        │   └── open.ts
        ├── conversation/
        │   ├── list.ts
        │   ├── create.ts
        │   ├── focus.ts
        │   ├── prompt.ts
        │   ├── cancel.ts
        │   └── watcher.ts
        └── events/
            ├── stream.ts
            └── normalize.ts
```

Bridge слушает только:

```text
127.0.0.1:<random-port>
```

Запрещено bind на `0.0.0.0` по умолчанию.

---

# 10. Bridge capability negotiation

Endpoint:

```text
GET /v1/capabilities
```

Пример:

```json
{
  "protocolVersion": 1,
  "capabilities": {
    "workspaceOpen": true,
    "conversationList": true,
    "conversationCreate": true,
    "conversationFocus": true,
    "conversationDirectSend": false,
    "agentEvents": true,
    "messageHistory": true,
    "cancel": true,
    "approvalEvents": true,
    "approvalDecision": true,
    "nativeFork": false
  }
}
```

Если `conversationDirectSend=false`, `focus + send` считается одной критической операцией и сериализуется через `InstanceCommandGate`.

---

# 11. Bridge API v1

```text
GET  /v1/health
GET  /v1/capabilities
GET  /v1/context

GET  /v1/conversations
POST /v1/conversations

POST /v1/conversations/{id}/focus
POST /v1/conversations/{id}/messages
POST /v1/conversations/{id}/cancel

POST /v1/workspace/open

POST /v1/approvals/{id}/approve
POST /v1/approvals/{id}/reject

GET /v1/events
WS  /v1/events/stream
```

Bridge нормализует внутренние Antigravity структуры в стабильный `agctl Bridge Protocol v1`.

---

# 12. Bridge registration/discovery

При запуске instance `agctl` генерирует:

- `bootNonce`;
- `bridgeToken`;
- `instanceID`.

Передать их IDE через launch environment/context:

```text
AGCTL_INSTANCE_ID=...
AGCTL_BOOT_NONCE=...
AGCTL_BRIDGE_TOKEN=...
AGCTL_BRIDGE_REGISTRY=...
```

Bridge atomic-write создаёт:

```text
state/bridges/<boot-nonce>.json
```

Содержимое:

```json
{
  "protocolVersion": 1,
  "instanceId": "...",
  "bootNonce": "...",
  "pid": 15872,
  "port": 51482,
  "workspace": "...",
  "startedAt": "..."
}
```

Token в registration file не записывать.

Stale registration определяется по PID + heartbeat + boot nonce.

---

# 13. Расширение `internal/paths`

Добавить пути:

```go
BridgeRoot     string
RemoteRoot     string
RemoteConfig   string
TelegramConfig string
```

Файловая структура:

```text
state/
├── harness/
│   └── state.db
├── bridges/
└── remote/
```

`bridges/` — discovery transport, не durable source of truth.

---

# 14. Durable storage

Не создавать отдельную `remote.db`.

Использовать существующий Harness SQLite `state.db` с новыми immutable migrations.

Предлагаемый порядок:

```text
migration 11: remote_control_core
migration 12: telegram_transport
migration 13: remote_outbox_and_recovery
```

Никогда не редактировать released migrations 1..10.

---

# 15. Таблицы remote subsystem

## 15.1. `remote_repositories`

Поля:

```text
id
name
canonical_path
git_root
git_remote
default_branch
enabled
created_at
last_seen_at
```

`canonical_path UNIQUE`.

Telegram может открыть только зарегистрированный repository.

## 15.2. `remote_instances`

Mirror Cockpit state:

```text
cockpit_instance_id
name
user_data_dir
working_dir
account_id
pid
desired_state
observed_state
bridge_id
last_reconciled_at
last_error
```

Cockpit остаётся source of truth профиля.

## 15.3. `remote_conversations`

```text
id
provider_conversation_id
cockpit_instance_id
workspace_id
title
state
last_activity_at
mirror_mode
created_at
updated_at
```

Provider conversation ID считать opaque string.

## 15.4. `remote_sessions`

```text
id
instance_id
workspace_id
conversation_id
repository_id
account_id
workflow_run_id
desired_state
observed_state
isolation_mode
created_at
updated_at
```

## 15.5. `telegram_bindings`

```text
id
session_id
chat_id
thread_id
owner_user_id
enabled
created_at
```

Уникальность active binding по `(chat_id, thread_id)`.

## 15.6. `remote_commands`

```text
id
source
source_message_id
session_id
kind
payload
state
requested_at
started_at
completed_at
error
```

Unique `(source, source_message_id)` для идемпотентности.

## 15.7. `remote_events`

```text
event_id
session_id
seq
source
type
source_event_id
payload
timestamp
```

## 15.8. `remote_outbox`

```text
event_id
transport
delivered_at
attempt_count
next_attempt_at
```

---

# 16. Normalized IDE events

В Go:

```go
type EventType string

const (
    EventConversationStarted EventType = "conversation.started"
    EventUserMessage         EventType = "user.message"
    EventAgentDelta          EventType = "agent.delta"
    EventAgentMessage        EventType = "agent.message"
    EventAgentIdle           EventType = "agent.idle"
    EventToolStarted         EventType = "tool.started"
    EventToolFinished        EventType = "tool.finished"
    EventApprovalRequested   EventType = "approval.requested"
    EventApprovalResolved    EventType = "approval.resolved"
    EventWorkspaceChanged    EventType = "workspace.changed"
    EventBridgeConnected     EventType = "bridge.connected"
    EventBridgeDisconnected  EventType = "bridge.disconnected"
)
```

Порядок событий внутри session обеспечивается `seq`.

---

# 17. Mirror modes

Capability-aware режимы:

## `full`

IDE user message → Telegram; Telegram → IDE; IDE agent response → Telegram.

## `remote`

Надёжно зеркалируются Telegram-originated prompts и agent responses, но локально введённые IDE user messages могут быть недоступны.

## `status`

Только lifecycle/status/approval/completion без полного conversation text.

UI всегда показывает активный mirror mode; запрещено выдавать `remote` за `full`.

---

# 18. Go Bridge client

Создать:

```text
internal/antigravityide/
├── client.go
├── capabilities.go
├── model.go
├── conversation.go
├── workspace.go
├── events.go
├── discovery.go
├── health.go
├── errors.go
└── fake.go
```

Интерфейс:

```go
type Client interface {
    Capabilities(ctx context.Context) (Capabilities, error)
    Context(ctx context.Context) (IDEContext, error)
    ListConversations(ctx context.Context) ([]Conversation, error)
    CreateConversation(ctx context.Context) (Conversation, error)
    FocusConversation(ctx context.Context, id string) error
    SendMessage(ctx context.Context, id string, text string) error
    Cancel(ctx context.Context, id string) error
    OpenWorkspace(ctx context.Context, path string) error
    Subscribe(ctx context.Context, afterSeq uint64) (<-chan Event, error)
    DecideApproval(ctx context.Context, approvalID string, decision Decision) error
}
```

---

# 19. State machine Cockpit instance

```text
STOPPED
  ↓ start
PREPARING
  ↓
STARTING
  ↓
PROCESS_RUNNING
  ↓
WAITING_BRIDGE
  ↓
READY
  ├─ stop → STOPPING → STOPPED
  └─ bridge lost → DEGRADED
                     ├─ reconnect → READY
                     └─ process lost → STOPPED
```

Instance state не смешивать с conversation/session state.

---

# 20. RemoteSession state machine

```text
CREATING
  ↓
RESOLVING_INSTANCE
  ↓
WAITING_INSTANCE
  ↓
WAITING_BRIDGE
  ↓
OPENING_WORKSPACE
  ↓
ATTACHING_CONVERSATION
  ↓
READY
  ├─ prompt → RUNNING
  │             ├─ approval → WAITING_APPROVAL → RUNNING
  │             ├─ complete → READY
  │             └─ bridge lost → DEGRADED
  ├─ pause → PAUSED
  └─ close → CLOSING → CLOSED
```

Избегать state explosion; дочерние state machines хранятся отдельно.

---

# 21. SessionSpec

```go
type SessionSpec struct {
    RepositoryID RepositoryID
    AccountID    string

    InstanceStrategy InstanceStrategy

    ConversationStrategy     ConversationStrategy
    ExistingConversationID   string

    IsolationMode IsolationMode

    TelegramBinding TelegramBindingSpec

    AutoRestart bool
}
```

---

# 22. Plan-before-mutate

Добавить dry-run API/CLI:

```text
agctl remote session plan
```

Он возвращает декларативный execution plan, например:

```text
✓ reuse account
+ create Cockpit instance cytology-02
+ create git worktree
+ start Antigravity
+ wait Bridge
+ open workspace
+ create conversation
+ attach Telegram topic
```

После этого отдельная execute operation.

Для high-risk действий Telegram также сначала показывает план/blast radius.

---

# 23. Session Resolver

Создать:

```text
internal/remote/session/
├── resolver.go
├── planner.go
├── provisioner.go
├── reconciler.go
├── lifecycle.go
├── policy.go
└── service.go
```

Алгоритм reuse:

```text
requested repo/account
    ↓
find READY instances
    ↓
same account?
    ├─ no → reject reuse
    ↓
same workspace?
    ├─ yes → reuse
    ↓
instance has pinned active session?
    ├─ yes → new instance
    ↓
workspace replacement allowed?
    ├─ yes → reuse
    └─ no → new instance
```

Default policy: один одновременно активно используемый repository workspace = один Cockpit instance.

---

# 24. Multi-conversation внутри одного IDE

Разрешить несколько conversations одного workspace.

Если Bridge поддерживает direct send by conversation ID — dispatch может быть concurrent.

Если есть только `focus + send`, создать:

```go
type InstanceCommandGate struct {
    instanceID string
    mu sync.Mutex
}
```

Операция `focus + verify + send` выполняется в одной критической секции.

Обязательный concurrency test: prompt A не должен попасть в conversation B при racing focus/send.

---

# 25. Multi-instance

Поддержать параллельно:

```text
IDE #1 — Account A — Repo A
IDE #2 — Account A — Repo B
IDE #3 — Account B — Repo C
```

Никакого глобального `currentInstance`.

Все операции адресуются через `RemoteSessionID`/`CockpitInstanceID`.

Один account может быть привязан к нескольким isolated Cockpit instances, но UI должен показывать, что quota/account identity shared.

---

# 26. Account lifecycle

Account binding существует только на уровне Cockpit instance.

Нельзя иметь разные account identities для conversations внутри одного process/profile.

## 26.1. Cold account switch

Алгоритм:

```text
find child RemoteSessions
→ if RUNNING: block or explicit graceful switch
→ checkpoint conversations
→ suspend RemoteSessions
→ stop IDE
→ Cockpit bindAccount(new)
→ Cockpit prepares/injects account
→ start IDE
→ wait Bridge
→ reopen workspace
→ reconcile conversations
```

## 26.2. Conversation after account switch

Старый provider conversation ID перепроверяется через Bridge.

Если conversation найден — `REATTACH`.

Если не найден — `SUSPENDED_ACCOUNT_MISMATCH` и пользователь выбирает:

- создать новый conversation;
- вернуть старый account;
- создать handoff.

---

# 27. Handoff

Перед потерей native conversation continuity сформировать context checkpoint artifact:

```text
goal
current state
important decisions
files touched
tests
remaining work
known failures
```

Новый conversation получает checkpoint + original objective.

UI явно показывает:

```text
Continuation mode: HANDOFF
```

Не выдавать handoff за native conversation continuation.

---

# 28. Parallel write isolation

Для одного repo с двумя write sessions использовать Harness Workspace Manager.

Алгоритм:

```text
parallel session requested
→ classify access = WRITE
→ workspace.Manager.Allocate()
→ exclusive conflict?
   ├─ no → regular workspace
   └─ yes/parallel → create git worktree
→ unique branch
→ create/reuse Cockpit instance
→ open isolated worktree
→ create conversation
```

Branch naming:

```text
agctl/remote/<session-id>/<slug>
```

Инвариант:

```text
1 write RemoteSession = 1 physical worktree
```

Второй write conversation нельзя привязать к тому же worktree без explicit override policy.

---

# 29. Lifecycle semantics

Разделить операции:

## `Stop agent`

Остановить текущую генерацию/tool execution; IDE остаётся.

## `Pause session`

Session attached, но новые commands не dispatch-ятся.

## `Close remote session`

Отвязать RemoteSession/Telegram, не обязательно закрывая conversation или IDE.

## `Stop IDE`

Остановить Cockpit process; child RemoteSessions переводятся в `SUSPENDED`.

## `Delete Cockpit instance`

Destructive profile operation с отдельным подтверждением и blast-radius preview.

---

# 30. Reconciler

Создать:

```text
internal/remote/reconcile/
├── reconciler.go
├── cockpit.go
├── bridge.go
├── session.go
└── drift.go
```

Сравнивать:

```text
desired state
vs
observed Cockpit/process/Bridge/workspace/conversation state
```

Обнаруживать:

- stale PID;
- PID reuse;
- stale Bridge registration;
- missing Bridge;
- missing workspace;
- missing conversation;
- Cockpit manually stopped;
- changed working_dir/account;
- repo/worktree removed.

No destructive repair by default.

---

# 31. Crash scenarios

## 31.1. IDE crash

READY/RUNNING → DEGRADED/SUSPENDED.

Если `autoRestart=true`:

```text
restart instance
→ wait Bridge
→ reopen workspace
→ find conversation
→ reattach
```

Если conversation отсутствует — `NEEDS_ATTENTION`, не создавать новый молча.

## 31.2. `agctl` restart

На startup:

```text
load remote_sessions
load desired state
query Cockpit
scan Bridge registrations
query live Bridges
reconcile
```

Persisted state никогда не считается реальностью без observed reconciliation.

## 31.3. Cockpit manual changes

Если пользователь вручную остановил instance, `agctl` фиксирует observed drift и применяет policy (`autoRestart` или `NEEDS_ATTENTION`).

---

# 32. Lease/locking model

Использовать три разных механизма:

1. `InstanceLifecycleLock` — start/stop/rebind/delete одного instance.
2. `WorkspaceWriteLease` — exclusive write ownership physical workspace.
3. `ConversationDispatchGate` — atomic focus/send на instance без direct conversation send.

Не использовать один глобальный mutex.

---

# 33. Telegram package

Создать:

```text
internal/telegram/
├── service.go
├── bot.go
├── polling.go
├── update.go
├── router.go
├── auth/
│   ├── pairing.go
│   └── allowlist.go
├── ui/
│   ├── home.go
│   ├── projects.go
│   ├── sessions.go
│   ├── instances.go
│   ├── accounts.go
│   └── approvals.go
├── mirror/
│   ├── inbound.go
│   ├── outbound.go
│   └── coalescer.go
└── callback/
    └── callback.go
```

Default transport — long polling через outbound HTTPS.

Webhook оставить optional future deployment mode.

---

# 34. Telegram pairing/auth

CLI:

```text
agctl telegram pair
```

Генерируется одноразовый short-lived pairing code.

Telegram:

```text
/pair <code>
```

В allowlist сохраняется immutable numeric Telegram user ID, не username.

Минимальные роли:

```text
viewer
operator
admin
```

---

# 35. Telegram command pipeline

Каждый inbound update:

```text
receive update
→ authenticate user
→ resolve topic/session binding
→ deduplicate update ID
→ parse intent
→ authorize role
→ classify risk
→ persist RemoteCommand
→ execute through service layer
→ persist result
→ render response
```

Handler никогда не выполняет process/git/approval напрямую.

---

# 36. Telegram UX

## Home

```text
🛰 Antigravity Remote

PC           🟢 Online
Cockpit      🟢 Connected
IDE          3 running
Agents       5 conversations
Working      2
Approvals    1

[📁 Projects]   [🖥 IDE]
[💬 Sessions]   [👤 Accounts]
[⚠ Approvals]   [📊 Status]
```

## Projects

Показывать repository, число sessions и active state.

## IDE instances

Показывать instance, repo, account, number of conversations и lifecycle state.

## Accounts

Показывать account identity + число running instances; никогда не показывать tokens.

---

# 37. Telegram Topics = conversations

Основной UX для параллельной работы:

```text
Telegram Supergroup
├── CYTOLOGY / segmentation
├── CYTOLOGY / benchmark
├── DocsHub / auth
└── HomeSentinel / scenarios
```

Binding:

```text
chat_id + thread_id
→ TelegramBinding
→ RemoteSession
→ ConversationID
```

Пользователь пишет обычный текст в topic без обязательного `/send session-id`.

---

# 38. Agent output streaming

Bridge может отдавать много `agent.delta`.

Telegram gateway должен:

1. создать одно сообщение;
2. накапливать delta;
3. throttled `editMessage`;
4. финальный flush;
5. при Telegram outage сохранить final/coalesced result в outbox.

Не отправлять сотни отдельных delta сообщений.

---

# 39. Event verbosity

На RemoteSession:

```text
quiet
normal
verbose
debug
```

`quiet`: final + approval + failure.

`normal`: working/test/commit/completion.

`verbose`: tool calls.

`debug`: normalized Bridge diagnostics без secrets.

---

# 40. Telegram outage

Agent execution не зависит от доступности Telegram.

```text
IDE event
→ remote_event
→ remote_outbox
→ Telegram unavailable
→ retry/coalesce
```

После восстановления не replay-ить каждый token delta; доставить final message и важные lifecycle events.

---

# 41. Approvals через Telegram

Flow:

```text
Bridge approval.requested
→ RemoteEvent
→ risk classification
→ Telegram card
→ Approve once / Reject / Details
→ existing Harness/risk decision layer
→ Bridge approval decision
```

Telegram никогда не должен обходить Harness/risk policy прямым `execute`.

Перед high-risk approval показывать:

- repository;
- conversation;
- action;
- risk;
- blast radius.

---

# 42. Fork conversation

Поддержать два режима.

## Native fork

Использовать, только если capability `nativeFork=true`.

## Handoff fork

Source conversation → checkpoint summary → new conversation → context bootstrap.

UI обязательно показывает fork type.

---

# 43. Remote daemon

Добавить:

```text
agctl remote serve
```

Долгоживущие компоненты:

```text
SQLite
├── Cockpit reconciler
├── Bridge discovery/heartbeat
├── Session reconciler
├── Telegram polling
├── RemoteCommand executor
├── Event dispatcher
├── Remote outbox delivery
└── Harness event adapter
```

Windows service/task registration можно добавить после стабилизации daemon mode.

---

# 44. CLI

Не раздувать `cmd/agctl/main.go`; вынести subcommands в отдельные файлы.

Добавить:

```text
agctl remote status

agctl remote repo list
agctl remote repo add
agctl remote repo discover

agctl remote instance list

agctl remote session list
agctl remote session show
agctl remote session plan
agctl remote session create
agctl remote session attach
agctl remote session pause
agctl remote session resume
agctl remote session close

agctl remote conversation list
agctl remote conversation new
agctl remote conversation attach
agctl remote conversation fork
agctl remote conversation send

agctl remote reconcile

agctl cockpit doctor
agctl cockpit accounts
agctl cockpit instances

agctl ide bridge install
agctl ide bridge doctor

agctl telegram enable
agctl telegram disable
agctl telegram pair
agctl telegram status
```

Файлы:

```text
cmd/agctl/
├── remote.go
├── remote_repo.go
├── remote_session.go
├── remote_conversation.go
├── cockpit.go
├── ide.go
└── telegram.go
```

---

# 45. Harness integration

Разделить два режима.

## Direct IDE chat

```text
Telegram
→ RemoteSession
→ Antigravity conversation
```

## Structured autonomous work

```text
Telegram
→ RemoteSession
→ Harness workflow
→ tasks
→ verification
→ approvals
```

При structured mode `RemoteSession.WorkflowRunID != ""`.

В Telegram topic можно объединять IDE conversation events и Harness workflow events.

---

# 46. Secret Store

Создать:

```text
internal/secrets/
```

Интерфейс:

```go
type Store interface {
    Put(name string, secret []byte) error
    Get(name string) ([]byte, error)
    Delete(name string) error
}
```

Backends:

- Windows Credential Manager / DPAPI;
- macOS Keychain;
- Linux Secret Service;
- environment variable fallback.

Telegram bot token и Bridge secrets не хранить plain text в config JSON.

---

# 47. Security hardening

Обязательно реализовать:

- localhost-only Bridge;
- per-instance random Bridge token;
- boot nonce;
- short-lived pairing code;
- numeric Telegram user allowlist;
- role-based permissions;
- risk classification;
- explicit confirmations;
- secret redaction;
- command/event audit trail;
- repository allowlist;
- no arbitrary path open from Telegram;
- no shell-string concatenation для Cockpit control;
- bounded payload sizes;
- timeouts/cancellation;
- replay protection для callbacks.

---

# 48. Атомарные этапы реализации

## R0 — Antigravity capability audit

### R0.1
Создать `tools/antigravity-probe/`.

### R0.2
Создать capability matrix document.

### R0.3
Проверить stable ConversationID после window/IDE restart.

### R0.4
Проверить direct vs focus/send dispatch.

### R0.5
Проверить local user-message events, agent delta/final, tool/approval events.

### R0.6
Зафиксировать `supported/unsupported/fallback` для каждой capability.

**DoD:** нет `unknown`.

---

## R1 — Remote domain model

### R1.1
Создать `internal/remote/model`.

### R1.2
Добавить typed IDs.

### R1.3
Добавить enums/state validation.

### R1.4
Добавить invariants.

### R1.5
Unit tests на invalid combinations.

**DoD:** model не зависит от Telegram, Cockpit или TS Bridge.

---

## R2 — Durable storage

### R2.1
Добавить immutable migration 11.

### R2.2
Создать remote core tables.

### R2.3
Добавить remote store interfaces.

### R2.4
SQLite implementation в той же Harness DB.

### R2.5
Migration tests old→new DB.

### R2.6
Transaction/crash tests.

**DoD:** restart процесса восстанавливает remote state.

---

## R3 — Repository Registry

### R3.1
Создать `internal/remote/repository`.

### R3.2
Canonical path normalization.

### R3.3
Git root detection.

### R3.4
Remote URL/default branch detection.

### R3.5
Allowed roots policy.

### R3.6
CLI add/list/discover.

**DoD:** Telegram не открывает незарегистрированный path.

---

## R4 — Cockpit working_dir vertical patch

### R4.1
Пробросить working_dir в Tauri create/update.

### R4.2
Обновить TS DTO/service/UI.

### R4.3
Validation и canonicalization.

### R4.4
Persistence/launch tests.

**DoD:** Cockpit instance стабильно запускается в выбранном workspace.

---

## R5 — `cockpit-control`

### R5.1
Создать binary crate.

### R5.2
Stable JSON envelope/version.

### R5.3
Accounts list sanitized.

### R5.4
Instances list/get.

### R5.5
Create/update/start/stop/focus.

### R5.6
Account binding.

### R5.7
Stable error codes.

### R5.8
Contract tests.

**DoD:** lifecycle выполняется без GUI automation.

---

## R6 — Go Cockpit adapter

### R6.1
Binary discovery.

### R6.2
Version negotiation.

### R6.3
Strict JSON protocol.

### R6.4
Timeout/cancellation.

### R6.5
Structured errors.

### R6.6
Fake driver.

### R6.7
Contract tests с `cockpit-control`.

---

## R7 — Production Antigravity Bridge

### R7.1
VSIX scaffold.

### R7.2
localhost random port.

### R7.3
Bearer auth.

### R7.4
Capabilities.

### R7.5
Workspace current/open.

### R7.6
Conversation list/create/focus/send.

### R7.7
Cancel.

### R7.8
Approval operations.

### R7.9
Normalized event stream.

### R7.10
Bridge registration/heartbeat.

### R7.11
Compatibility adapter around Antigravity internals.

**DoD:** один conversation полностью управляется без Telegram.

---

## R8 — Go Bridge client

### R8.1
Discovery watcher.

### R8.2
Protocol client.

### R8.3
Capability cache.

### R8.4
Reconnect/resume events by seq.

### R8.5
Fake Bridge server.

### R8.6
Protocol contract tests.

---

## R9 — Single-instance RemoteSession vertical slice

### R9.1
Implement SessionSpec.

### R9.2
Implement Resolver.

### R9.3
Implement Provisioner.

### R9.4
Create/reuse Cockpit instance.

### R9.5
Start IDE.

### R9.6
Wait Bridge readiness.

### R9.7
Open workspace.

### R9.8
Create/attach conversation.

### R9.9
Persist RemoteSession.

### R9.10
Close/recover.

**DoD:** `agctl remote session create --repo X --account Y` поднимает рабочий conversation end-to-end.

---

## R10 — Reconciler

### R10.1
Cockpit state reconciliation.

### R10.2
Bridge state reconciliation.

### R10.3
Workspace reconciliation.

### R10.4
Conversation reconciliation.

### R10.5
Stale PID/registration detection.

### R10.6
Startup recovery.

### R10.7
No destructive repair by default.

---

## R11 — Multi-conversation

### R11.1
Conversation registry.

### R11.2
Attach existing.

### R11.3
Create new.

### R11.4
InstanceCommandGate.

### R11.5
Capability-dependent dispatch.

### R11.6
Conversation-level cancel/focus.

### R11.7
Native/handoff fork.

### R11.8
Cross-talk race tests.

**DoD:** три conversations одного instance не смешивают messages.

---

## R12 — Multi-instance

### R12.1
Instance Resolver.

### R12.2
Instance lifecycle lock.

### R12.3
Reuse-compatible algorithm.

### R12.4
Dedicated instance mode.

### R12.5
Several same-account instances.

### R12.6
Different-account instances.

### R12.7
Blast-radius calculation.

**DoD:** Repo A/Account A + Repo B/Account A + Repo C/Account B работают одновременно.

---

## R13 — Parallel write isolation

### R13.1
Подключить Harness Workspace Manager.

### R13.2
Классифицировать access readonly/write.

### R13.3
Exclusive workspace lease.

### R13.4
Automatic worktree creation.

### R13.5
Unique branch naming.

### R13.6
Worktree cleanup policy.

### R13.7
Dirty-worktree protection.

### R13.8
Integration/merge handoff workflow.

**DoD:** два write agents никогда не работают по одному physical working tree.

---

## R14 — Account lifecycle

### R14.1
Account listing adapter.

### R14.2
Account binding.

### R14.3
Cold switch.

### R14.4
Active-session guard.

### R14.5
Conversation checkpoint.

### R14.6
Post-switch reconciliation.

### R14.7
Account mismatch state.

### R14.8
Handoff continuation.

---

## R15 — Telegram foundation

### R15.1
Bot service.

### R15.2
Long polling.

### R15.3
Durable update offset.

### R15.4
Pairing.

### R15.5
Allowlist/roles.

### R15.6
Callback routing.

### R15.7
RemoteCommand persistence.

### R15.8
Dedupe/replay protection.

**DoD:** duplicate callback выполняется максимум один раз.

---

## R16 — Telegram UI

### R16.1
Home.

### R16.2
Projects.

### R16.3
Accounts.

### R16.4
Instances.

### R16.5
Conversations.

### R16.6
Session actions.

### R16.7
Approvals.

### R16.8
Impact/blast-radius confirmation.

---

## R17 — Conversation mirror

### R17.1
Telegram → conversation.

### R17.2
Bridge → remote event.

### R17.3
Remote event → Telegram.

### R17.4
Delta coalescing.

### R17.5
Throttled message editing.

### R17.6
Full/remote/status mode.

### R17.7
Reconnect from event sequence.

### R17.8
Event dedupe.

---

## R18 — Telegram Topics

### R18.1
Topic binding.

### R18.2
Session↔topic uniqueness.

### R18.3
Automatic topic naming.

### R18.4
Optional rename on conversation title update.

### R18.5
Detach without killing conversation.

---

## R19 — Harness bridge

### R19.1
Link WorkflowRunID.

### R19.2
Harness event → RemoteEvent adapter.

### R19.3
Approval forwarding.

### R19.4
Pause/resume/cancel from Telegram через Harness.

### R19.5
Verification/test status rendering.

---

## R20 — Security hardening

### R20.1
Secret store.

### R20.2
Per-instance Bridge credentials.

### R20.3
Telegram allowlist.

### R20.4
Roles.

### R20.5
Risk integration.

### R20.6
Dangerous operation confirmations.

### R20.7
Audit log.

### R20.8
Secret-redaction tests.

---

## R21 — Failure injection

Проверить минимум:

```text
Cockpit dies
Antigravity dies
Bridge dies
agctl dies
Telegram unavailable
duplicate command
reordered event
lost event
account switch during message
stop during generation
two simultaneous start requests
two simultaneous prompts
two simultaneous write sessions
worktree missing
repo deleted
branch deleted
PID reused
bridge registration stale
wrong instance ID
wrong conversation ID
```

---

## R22 — Test-of-tests

### Go

- unit;
- integration;
- `go test -race`;
- property tests;
- fuzz;
- mutation testing.

### TypeScript Bridge

- unit;
- protocol contract;
- fake IDE adapter;
- mutation testing.

### Rust Cockpit control

- unit;
- CLI contract;
- process lifecycle integration;
- mutation-sensitive critical tests.

Особый mutation focus:

```text
auth
routing
session isolation
account binding
risk
idempotency
```

---

## R23 — Concurrency stress

Synthetic target:

```text
50 logical RemoteSessions
20 conversations
10 fake instances
1000 concurrent Telegram updates
100000 remote events
```

Проверять:

- goroutine leaks;
- deadlocks;
- duplicate prompts;
- cross-session routing;
- out-of-order events;
- SQLite contention;
- lock starvation;
- outbox duplication.

---

## R24 — E2E matrix

| Scenario | Expected |
|---|---|
| 1 account × 1 instance × 1 repo × 1 conversation | PASS |
| 1 account × 1 instance × 1 repo × 3 conversations | PASS |
| 1 account × 3 instances × 3 repos | PASS |
| 3 accounts × 3 instances × 3 repos | PASS |
| same repo × 2 read sessions | PASS |
| same repo × 2 write sessions | separate worktrees |
| account switch idle | PASS |
| account switch working | blocked/checkpoint |
| crash IDE | recover/suspend |
| crash agctl | durable restore |
| duplicate Telegram update | exactly once |
| Bridge loss | reconnect |
| Telegram outage | outbox |
| Cockpit manually stopped | reconcile |
| process restarted with new PID | reconcile |

---

# 49. Ключевые пользовательские сценарии

## S1 — Запуск проекта

```text
Telegram Projects
→ choose repo
→ choose account
→ Start
→ resolve/create instance
→ bind account
→ set working_dir
→ start IDE
→ wait Bridge
→ open repo
→ create conversation
→ create RemoteSession
→ bind Telegram
→ READY
```

## S2 — Подключение к уже открытому IDE

```text
Cockpit running instance
+ Bridge connected
+ workspace discovered
+ conversations listed
→ user selects conversation
→ create only RemoteSession + TelegramBinding
```

IDE не перезапускается.

## S3 — Несколько conversations в одном IDE

Каждому conversation соответствует отдельный RemoteSession/Telegram topic. Dispatch определяется capabilities.

## S4 — Несколько IDE параллельно

Каждый instance имеет собственный profile/account/workspace/process state. Нет глобального active instance.

## S5 — Один account, несколько IDE

Разрешено; UI показывает shared account/quota.

## S6 — Разные accounts

Account binding всегда instance-level.

## S7 — Account switch

Только graceful cold switch с checkpoint/reconcile.

## S8 — Parallel write same repo

Автоматически отдельные worktrees/branches.

## S9 — Stop agent vs stop IDE

Разные lifecycle operations и разный blast radius.

## S10 — Focus on PC

```text
Cockpit focusInstance()
→ Bridge focusConversation()
```

## S11 — IDE crash

RemoteSession → DEGRADED/SUSPENDED → optional controlled restart/rebind.

## S12 — `agctl` crash/restart

Durable state + observed reconciliation.

## S13 — Telegram outage

Execution продолжается; события durable-outbox и coalesce.

## S14 — Approval

Bridge → risk/Harness → Telegram confirmation → decision → Bridge.

## S15 — Fork

Native если capability есть, иначе explicit HANDOFF.

---

# 50. Финальная файловая структура

```text
Antigravity-Progressive-Bootstrap/
├── cmd/
│   └── agctl/
│       ├── main.go
│       ├── remote.go
│       ├── remote_repo.go
│       ├── remote_session.go
│       ├── remote_conversation.go
│       ├── cockpit.go
│       ├── ide.go
│       └── telegram.go
│
├── internal/
│   ├── remote/
│   │   ├── model/
│   │   ├── session/
│   │   ├── repository/
│   │   ├── reconcile/
│   │   ├── events/
│   │   ├── store/
│   │   ├── command/
│   │   └── policy/
│   ├── antigravityide/
│   ├── cockpit/
│   ├── telegram/
│   ├── secrets/
│   └── harness/
│       ├── workspace/
│       ├── store/
│       └── ...
│
├── extensions/
│   └── agctl-antigravity-bridge/
│
├── tools/
│   └── antigravity-probe/
│
├── docs/
│   ├── REMOTE_CONTROL_ARCHITECTURE_RU.md
│   ├── REMOTE_CONTROL_PROTOCOL_RU.md
│   ├── REMOTE_SESSION_MODEL_RU.md
│   ├── ANTIGRAVITY_IDE_REMOTE_CAPABILITY_MATRIX_RU.md
│   ├── COCKPIT_INTEGRATION_RU.md
│   ├── TELEGRAM_REMOTE_CONTROL_RU.md
│   └── REMOTE_SECURITY_MODEL_RU.md
│
└── REMOTE_CONTROL_IMPLEMENTATION_PLAN_RU.md
```

---

# 51. Критический путь внедрения

Не начинать реализацию с Telegram.

Правильная последовательность:

```text
R0  capability research
 ↓
R1  domain contracts
 ↓
R2  durable storage
 ↓
R3  repository registry
 ↓
R4  Cockpit working_dir patch
 ↓
R5  cockpit-control
 ↓
R6  Go Cockpit adapter
 ↓
R7  Antigravity Bridge
 ↓
R8  Bridge Go client
 ↓
R9  single-instance vertical slice
 ↓
R10 reconciliation
 ↓
R11 multi-conversation
 ↓
R12 multi-instance
 ↓
R13 worktree isolation
 ↓
R14 account lifecycle
 ↓
R15 Telegram transport
 ↓
R16 Telegram UI
 ↓
R17 mirror
 ↓
R18 Topics
 ↓
R19 Harness integration
 ↓
R20 security hardening
 ↓
R21–R24 chaos/mutation/concurrency/E2E
```

Первая production-quality вертикаль должна быть:

```text
agctl CLI
→ RemoteSession Manager
→ Cockpit
→ Antigravity IDE
→ Bridge
→ specific conversation
```

Только после её стабильной работы для нескольких параллельных instances подключать Telegram как transport/UI.

---

# 52. Definition of Done Remote Control v1

Функционал считается завершённым только если одновременно выполнено всё ниже:

```text
✓ Telegram lists registered repositories
✓ Telegram lists Cockpit accounts
✓ Telegram lists running/stopped Antigravity instances
✓ User starts repository under selected account
✓ agctl waits for real Bridge readiness
✓ User attaches to existing conversation
✓ User creates new conversation
✓ Several conversations work in one IDE without cross-talk
✓ Several IDE instances work in parallel
✓ Several accounts work in parallel
✓ Telegram topic maps deterministically to one conversation
✓ Agent output is mirrored according to declared mirror mode
✓ User can approve/reject actions through policy layer
✓ User can focus correct IDE/conversation on PC
✓ Account switching cannot corrupt active sessions
✓ Parallel write sessions use separate worktrees
✓ IDE crash is reconciled
✓ agctl restart restores durable remote state
✓ Telegram duplicate updates do not duplicate operations
✓ Telegram outage does not stop agent execution
✓ Secrets never reach Telegram or logs
✓ Cockpit remains credential/profile source of truth
✓ Harness remains workflow/workspace source of truth
✓ RemoteSession remains remote-control source of truth
✓ race/fuzz/mutation/chaos/E2E tests pass
```

---

# 53. Итоговая целевая топология

```text
Telegram
  │
  ├─ CYTOLOGY
  │   ├─ segmentation ──→ Antigravity #1 / Account A
  │   └─ benchmark    ──→ Antigravity #1 / Account A
  │
  ├─ Docs_Hub
  │   └─ auth         ──→ Antigravity #2 / Account A
  │
  └─ Home_Sentinel
      └─ scenarios    ──→ Antigravity #3 / Account B
```

Для одного repository с параллельной write-разработкой:

```text
                 Repository
                    │
          ┌─────────┴─────────┐
          │                   │
      worktree A          worktree B
          │                   │
   Antigravity #1       Antigravity #2
          │                   │
 Conversation A        Conversation B
          │                   │
 Telegram Topic A      Telegram Topic B
```

Cockpit управляет профилями, аккаунтами и процессами. Bridge управляет IDE/workspace/conversation. Harness управляет durable workflows, approvals и workspace isolation. RemoteSession Manager связывает эти слои в одно детерминированное control plane, а Telegram остаётся безопасным удалённым интерфейсом.