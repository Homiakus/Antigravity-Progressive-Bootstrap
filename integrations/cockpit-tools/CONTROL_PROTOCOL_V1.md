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
cockpit-control instance create --name NAME --user-data-dir PATH [--working-dir PATH] [--extra-args ARGS] [--account-id ID] [--copy-source-instance-id ID] [--init-mode MODE] --json
cockpit-control instance update --id ID [--name NAME] [--working-dir PATH] [--extra-args ARGS] [--account-id ID | --unbind-account] --json
cockpit-control instance start --id ID --json
cockpit-control instance stop --id ID --json
cockpit-control instance focus --id ID --json
cockpit-control instance bind-account --id ID --account-id ID --json
```

## Managed start environment

Для `instance start` `agctl` задаёт per-invocation environment:

```text
AGCTL_INSTANCE_ID
AGCTL_BOOT_NONCE
AGCTL_BRIDGE_TOKEN
AGCTL_BRIDGE_REGISTRY
```

`cockpit-control` не должен логировать эти значения и не должен переносить их в argv. Запускаемый Antigravity process наследует environment, поэтому Bridge extension получает bootstrap context без утечки token в process command line. `AGCTL_BRIDGE_TOKEN` запрещено сохранять в Cockpit profile/store.

## Account DTO

Разрешены только `id`, `email`, `name`, `plan`, `disabled`. Запрещены `access_token`, `refresh_token`, cookies, OAuth blobs и authorization headers. Go client использует strict decoding и fail-closed отклоняет неожиданные поля.

## Compatibility

`protocolVersion` меняется только при breaking change. `agctl` v1 отклоняет любую другую версию.
