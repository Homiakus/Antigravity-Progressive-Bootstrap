# R4/R5 — patch contract для Cockpit Tools

Upstream: `jlcodes99/cockpit-tools`.

Текущее GitHub-подключение имеет к upstream только `pull` и не имеет `push`, поэтому этот этап нельзя честно пометить как применённый upstream. Здесь фиксируется точный patch contract для управляемого fork/build.

## Подтверждённый gap

В `cockpit-core` `InstanceProfile`, `CreateInstanceParams` и `UpdateInstanceParams` уже имеют `working_dir`. Но в `src-tauri/src/commands/instance.rs` текущие Antigravity команды:

```rust
create_instance(...) {
    ... CreateInstanceParams {
        working_dir: None,
        ...
    }
}
```

и:

```rust
update_instance(...) {
    ... UpdateInstanceParams {
        working_dir: None,
        ...
    }
}
```

теряют значение на Tauri boundary. `start_instance` далее вызывает `start_antigravity_with_args(user_data_dir, extra_args)` и не использует сохранённый `working_dir`.

## R4 patch

1. В Tauri `create_instance` добавить аргумент `working_dir: Option<String>` и передавать его в `CreateInstanceParams`.
2. В `update_instance` добавить `working_dir: Option<String>` и передавать его в `UpdateInstanceParams`.
3. Обновить frontend invoke DTO/форму instance editor так, чтобы `workingDir` не терялся.
4. Перед сохранением валидировать: пустая строка → `None`; непустой путь должен быть каталогом.
5. При `start_instance` открывать `working_dir` как workspace Antigravity. Предпочтительный контракт — отдельная функция process layer `start_antigravity_instance(user_data_dir, working_dir, extra_args)`, а не скрытое кодирование workspace в UI state.
6. Не удалять repository/workspace при удалении profile instance.

## R5 binary

Добавить workspace member `crates/cockpit-control`, который зависит от `cockpit-core`, а не от Tauri UI. Он реализует `CONTROL_PROTOCOL_V1.md` и вызывает существующие core функции для:

- sanitized account list;
- instance list/create/update;
- account binding/injection;
- start/stop/focus;
- protocol/version reporting.

### Безопасность

- stdout при `--json` содержит только protocol envelope;
- логи идут в stderr;
- account serializer формируется отдельным allowlist DTO и никогда не сериализует `Account.token`;
- CLI не предоставляет команду чтения raw Cockpit DB/token;
- destructive profile deletion в protocol v1 намеренно отсутствует.

## Required tests in Cockpit fork

- create/update round-trip сохраняет `working_dir`;
- launch получает правильный workspace path, включая пробелы/Unicode;
- два instance с разными `user_data_dir` и workspace стартуют независимо;
- bind-account не меняет другой instance;
- stdout account list не содержит `token`, `refresh`, `cookie`, `oauth`;
- protocol JSON остаётся валидным при внутренних логах;
- Windows/macOS/Linux argument quoting;
- stop/focus адресуют конкретный PID/profile.
