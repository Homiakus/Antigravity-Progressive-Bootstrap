# ADR-0001 — SQLite driver для Harness Runtime

- Status: **Accepted**
- Date: **2026-08-20**
- Scope: `internal/harness/store/sqlite`

## Context

Harness Runtime должен оставаться local-first и single-binary, работать на Windows/Linux без внешнего SQLite DLL и без обязательного CGO toolchain. База является authoritative durable state store, поэтому выбор драйвера оценивается прежде всего по correctness, crash-safety, WAL/transaction semantics и эксплуатационной простоте, а не по максимальному synthetic throughput.

Репозиторий сейчас фиксирует `go 1.24.2`. Stage 1 не должен одновременно делать скрытую миграцию toolchain.

## Рассмотренные варианты

### `modernc.org/sqlite`

Плюсы:

- CGo-free `database/sql` driver;
- Windows/Linux/macOS и широкий набор архитектур;
- SQLite встроен в Go binary, внешняя DLL не требуется;
- поддерживает URI/DSN pragmas, что позволяет применять connection-scoped `foreign_keys`, `busy_timeout`, `journal_mode` и `synchronous` ко всем соединениям pool;
- зрелая v1-линия.

Минусы:

- заметный dependency surface;
- upstream отдельно предупреждает, что версия `modernc.org/libc` должна совпадать с версией из `modernc.org/sqlite/go.mod`;
- новые releases уже требуют более новый Go toolchain.

### `github.com/ncruces/go-sqlite3`

Плюсы:

- CGo-free;
- `database/sql` compatible;
- очень широкая cross-platform test matrix;
- современная архитектура проекта.

Минусы:

- текущая `v0.35.x` требует Go 1.25;
- совместимая с Go 1.24 линия `v0.32.x` использует предыдущую wazero-based реализацию, а upstream после неё сменил execution approach на wasm2go;
- каждый SQLite connection имеет дополнительный Wasm-runtime memory overhead;
- pre-v1 API/versioning line.

### `github.com/mattn/go-sqlite3`

Плюсы:

- наиболее известный и зрелый драйвер;
- высокая производительность;
- большой production footprint.

Минусы:

- обязательный CGO;
- Windows build требует C compiler/toolchain;
- ухудшает cross-compilation и single-binary release ergonomics;
- противоречит цели `agctl.exe` без внешней build/runtime инфраструктуры.

## Decision

Для Harness Runtime V1 выбрать:

```text
modernc.org/sqlite v1.46.0
modernc.org/libc   v1.67.6
```

Причины выбора `v1.46.0`, а не latest:

1. `modernc.org/sqlite v1.46.0` объявляет `go 1.24`, поэтому совместим с текущим `go 1.24.2` проекта.
2. Следующие актуальные линии `v1.47+`/`v1.5x` перешли на Go 1.25, поэтому их adoption должен быть отдельным toolchain decision.
3. Версия `modernc.org/libc v1.67.6` pin-ится явно в соответствии с upstream requirement этой версии SQLite driver.

## Durability settings

V1 defaults:

```text
journal_mode = WAL
foreign_keys = ON
busy_timeout = 5000 ms
synchronous = FULL
```

`FULL` выбран сознательно. При WAL SQLite гарантирует ACID durability через OS/power failure при `synchronous=FULL`; `NORMAL` быстрее, но допускает rollback последнего committed transaction после OS crash/power loss. Поскольку приоритет Harness — `Correctness → Durability → Recoverability`, default остаётся `FULL`.

Позже `NORMAL` может быть разрешён только как явный opt-in performance profile после benchmark и с отображением ослабленной durability semantics.

## Pool policy

- `database/sql` pool разрешает concurrent readers.
- SQLite всё равно имеет одного writer; `busy_timeout` сглаживает короткую write contention.
- write transactions должны быть короткими.
- heavy artifact bytes не должны храниться в SQLite.
- connection-scoped pragmas должны задаваться через driver DSN, а не единичными `PRAGMA` после `sql.Open`, чтобы новые connections не теряли `foreign_keys`/`busy_timeout`.

## Consequences

Положительные:

- single Go binary;
- без CGO/DLL;
- корректная Windows release story;
- WAL и transactions остаются embedded;
- Stage 1 можно реализовать через стандартный `database/sql`.

Отрицательные:

- binary станет больше;
- dependency graph увеличится;
- sqlite/libc versions должны обновляться согласованно;
- при будущем переходе проекта на Go 1.25 необходимо повторно оценить current `modernc` и current `ncruces`.

## Validation gate

Решение считается подтверждённым только если Stage 1 проходит:

- fresh/open/reopen integration tests;
- WAL/foreign-key/busy-timeout pragma tests;
- rollback tests;
- concurrent reader + busy writer tests;
- subprocess crash tests вокруг transaction boundaries;
- `BenchmarkStateTransition` и `BenchmarkEventAppend`;
- Windows/Linux CI compilation/tests.

Если выбранная pinned версия не проходит этот gate, ADR пересматривается до включения SQLite store как default runtime path.
