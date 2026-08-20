package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	db   *sql.DB
	path string
	opts Options
}

func Open(ctx context.Context, path string, opts Options) (*DB, error) {
	if path == "" {
		return nil, fmt.Errorf("SQLite database path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve SQLite path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, fmt.Errorf("create SQLite parent directory: %w", err)
	}
	opts = opts.normalized()
	dsn := buildDSN(abs, opts)
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open SQLite: %w", err)
	}
	raw.SetMaxOpenConns(opts.MaxOpenConns)
	raw.SetMaxIdleConns(opts.MaxIdleConns)

	closeWith := func(openErr error) (*DB, error) {
		_ = raw.Close()
		return nil, openErr
	}
	if err := raw.PingContext(ctx); err != nil {
		return closeWith(fmt.Errorf("ping SQLite: %w", err))
	}
	got, err := readPragmas(ctx, raw)
	if err != nil {
		return closeWith(err)
	}
	if err := verifyPragmas(got, opts); err != nil {
		return closeWith(err)
	}
	if err := migrate(ctx, raw); err != nil {
		return closeWith(fmt.Errorf("migrate SQLite: %w", err))
	}
	if SchemaVersion >= 5 {
		if err := backfillReadyDeadlineNS(ctx, raw); err != nil {
			return closeWith(fmt.Errorf("backfill SQLite deadlines: %w", err))
		}
	}
	return &DB{db: raw, path: abs, opts: opts}, nil
}

func buildDSN(path string, opts Options) string {
	opts = opts.normalized()
	u := &url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	q := url.Values{}
	for _, pragma := range pragmaValues(opts) {
		q.Add("_pragma", pragma)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (d *DB) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	return d.db.Close()
}

func (d *DB) SQLDB() *sql.DB {
	if d == nil {
		return nil
	}
	return d.db
}

func (d *DB) Path() string {
	if d == nil {
		return ""
	}
	return d.path
}

func (d *DB) Pragmas(ctx context.Context) (Pragmas, error) {
	if d == nil || d.db == nil {
		return Pragmas{}, fmt.Errorf("SQLite database is not open")
	}
	return readPragmas(ctx, d.db)
}

func (d *DB) SchemaVersion(ctx context.Context) (int, error) {
	if d == nil || d.db == nil {
		return 0, fmt.Errorf("SQLite database is not open")
	}
	return schemaVersion(ctx, d.db)
}
