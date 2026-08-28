# cockpit-control protocol v1

`agctl` не читает и не изменяет внутренние SQLite/JSON Cockpit Tools. Все privileged операции с аккаунтами и Antigravity instances проходят через отдельный `cockpit-control`, построенный поверх `cockpit-core`.

## Envelope

Каждый вызов с `--json` обязан писать в stdout ровно один JSON object:

```json
{"protocolVersion":1,"ok":true,"data":{}}
```

Ошибка:

```json
{"protocolVersion":1,"ok":false,"error":{"code":"INSTANCE_NOT_FOUND","message":"instance not found"}}
```

Логи пишутся только в stderr. Credentials запрещены и в stdout, и в stderr.

## Commands

```text
cockpit-control protocol --json
cockpit-control accounts list --json
cockpit-control instances list --json

cockpit-control instance create \
  --name NAME \
  --user-data-dir PATH \
  [--working-dir PATH] \
  [--extra-args ARGS] \
  [--account-id ID] \
  [--copy-source-instance-id ID] \
  [--init-mode MODE] --json

cockpit-control instance update --id ID \
  [--name NAME] [--working-dir PATH] [--extra-args ARGS] \
  [--account-id ID | --unbind-account] --json

cockpit-control instance start --id ID --json
cockpit-control instance stop --id ID --json
cockpit-control instance focus --id ID --json
cockpit-control instance bind-account --id ID --account-id ID --json
```

## Account DTO

Разрешены только:

```json
{"id":"...","email":"masked-or-normal-email","name":"...","plan":"...","disabled":false}
```

Запрещены `access_token`, `refresh_token`, cookies, OAuth blobs, authorization headers и эквивалентные поля. Go client использует `DisallowUnknownFields`, поэтому случайная утечка нового secret field ломает protocol call fail-closed.

## Instance DTO

```json
{
  "id":"...",
  "name":"...",
  "userDataDir":"...",
  "workingDir":"...",
  "bindAccountId":"...",
  "lastPid":1234,
  "running":true,
  "initialized":true,
  "isDefault":false,
  "followLocalAccount":false
}
```

## Compatibility

`protocolVersion` меняется только при breaking change. `agctl` v1 отклоняет любую другую версию и не пытается угадать semantics.
