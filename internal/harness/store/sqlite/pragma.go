package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBusyTimeout = 5 * time.Second
	synchronousFull   = 2
)

type Options struct {
	BusyTimeout time.Duration
	MaxOpenConns int
	MaxIdleConns int
}

func (o Options) normalized() Options {
	if o.BusyTimeout <= 0 {
		o.BusyTimeout = defaultBusyTimeout
	}
	if o.MaxOpenConns <= 0 {
		o.MaxOpenConns = 8
	}
	if o.MaxIdleConns <= 0 || o.MaxIdleConns > o.MaxOpenConns {
		o.MaxIdleConns = o.MaxOpenConns
	}
	return o
}

type Pragmas struct {
	JournalMode   string
	ForeignKeys   bool
	BusyTimeoutMS int
	Synchronous   int
}

func pragmaValues(opts Options) []string {
	opts = opts.normalized()
	return []string{
		"busy_timeout(" + strconv.FormatInt(opts.BusyTimeout.Milliseconds(), 10) + ")",
		"foreign_keys(ON)",
		"journal_mode(WAL)",
		"synchronous(FULL)",
	}
}

func readPragmas(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (Pragmas, error) {
	var p Pragmas
	var foreignKeys int
	if err := q.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&p.JournalMode); err != nil {
		return Pragmas{}, fmt.Errorf("read journal_mode: %w", err)
	}
	if err := q.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		return Pragmas{}, fmt.Errorf("read foreign_keys: %w", err)
	}
	if err := q.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&p.BusyTimeoutMS); err != nil {
		return Pragmas{}, fmt.Errorf("read busy_timeout: %w", err)
	}
	if err := q.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&p.Synchronous); err != nil {
		return Pragmas{}, fmt.Errorf("read synchronous: %w", err)
	}
	p.JournalMode = strings.ToLower(strings.TrimSpace(p.JournalMode))
	p.ForeignKeys = foreignKeys == 1
	return p, nil
}

func verifyPragmas(got Pragmas, opts Options) error {
	opts = opts.normalized()
	wantBusy := int(opts.BusyTimeout.Milliseconds())
	if got.JournalMode != "wal" {
		return fmt.Errorf("SQLite journal_mode=%q, want WAL", got.JournalMode)
	}
	if !got.ForeignKeys {
		return fmt.Errorf("SQLite foreign_keys is OFF")
	}
	if got.BusyTimeoutMS != wantBusy {
		return fmt.Errorf("SQLite busy_timeout=%dms, want %dms", got.BusyTimeoutMS, wantBusy)
	}
	if got.Synchronous != synchronousFull {
		return fmt.Errorf("SQLite synchronous=%d, want FULL(%d)", got.Synchronous, synchronousFull)
	}
	return nil
}
