package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	// db is the authoritative writer/admin handle. SQLite has one physical
	// writer at a time, so a single connection plus BEGIN IMMEDIATE converts
	// in-process writer races into queueing instead of BUSY_SNAPSHOT upgrades.
	db *sql.DB
	// readDB keeps WAL reads concurrent and deliberately uses the driver's
	// default deferred transaction mode rather than write intent.
	readDB *sql.DB
	path   string
	opts   Options
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

	writer, err := sql.Open("sqlite", buildWriterDSN(abs, opts))
	if err != nil {
		return nil, fmt.Errorf("open SQLite writer: %w", err)
	}
	// SQLite serializes writers internally. Keeping exactly one writer
	// connection provides deterministic in-process backpressure while the WAL
	// reader pool below remains independently concurrent.
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)

	closeWriter := func(openErr error) (*DB, error) {
		_ = writer.Close()
		return nil, openErr
	}
	if err := writer.PingContext(ctx); err != nil {
		return closeWriter(fmt.Errorf("ping SQLite writer: %w", err))
	}
	got, err := readPragmas(ctx, writer)
	if err != nil {
		return closeWriter(err)
	}
	if err := verifyPragmas(got, opts); err != nil {
		return closeWriter(err)
	}
	if err := migrate(ctx, writer); err != nil {
		return closeWriter(fmt.Errorf("migrate SQLite: %w", err))
	}
	if SchemaVersion >= 5 {
		if err := backfillReadyDeadlineNS(ctx, writer); err != nil {
			return closeWriter(fmt.Errorf("backfill SQLite deadlines: %w", err))
		}
	}

	reader, err := sql.Open("sqlite", buildDSN(abs, opts))
	if err != nil {
		return closeWriter(fmt.Errorf("open SQLite reader: %w", err))
	}
	reader.SetMaxOpenConns(opts.MaxOpenConns)
	reader.SetMaxIdleConns(opts.MaxIdleConns)
	closeBoth := func(openErr error) (*DB, error) {
		_ = reader.Close()
		_ = writer.Close()
		return nil, openErr
	}
	if err := reader.PingContext(ctx); err != nil {
		return closeBoth(fmt.Errorf("ping SQLite reader: %w", err))
	}
	readerPragmas, err := readPragmas(ctx, reader)
	if err != nil {
		return closeBoth(err)
	}
	if err := verifyPragmas(readerPragmas, opts); err != nil {
		return closeBoth(fmt.Errorf("verify SQLite reader pragmas: %w", err))
	}

	return &DB{db: writer, readDB: reader, path: abs, opts: opts}, nil
}

func buildDSN(path string, opts Options) string {
	return buildDSNWithTxLock(path, opts, "")
}

func buildWriterDSN(path string, opts Options) string {
	return buildDSNWithTxLock(path, opts, "immediate")
}

func buildDSNWithTxLock(path string, opts Options, txLock string) string {
	opts = opts.normalized()
	u := &url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	q := url.Values{}
	for _, pragma := range pragmaValues(opts) {
		q.Add("_pragma", pragma)
	}
	if txLock != "" {
		q.Set("_txlock", txLock)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (d *DB) Close() error {
	if d == nil {
		return nil
	}
	var writerErr, readerErr error
	if d.db != nil {
		writerErr = d.db.Close()
	}
	if d.readDB != nil {
		readerErr = d.readDB.Close()
	}
	return errors.Join(writerErr, readerErr)
}

// SQLDB exposes the authoritative writer/admin handle for diagnostics,
// migrations tests and backup primitives. Runtime read paths should use View.
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
