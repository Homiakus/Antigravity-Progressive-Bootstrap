package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func TestWriterPoolIsSingleConnectionAndReaderPoolIsIndependent(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{MaxOpenConns: 6, MaxIdleConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := db.SQLDB().Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("writer pool max connections=%d want=1", got)
	}
	if db.readDB == nil {
		t.Fatal("reader pool is nil")
	}
	if got := db.readDB.Stats().MaxOpenConnections; got != 6 {
		t.Fatalf("reader pool max connections=%d want=6", got)
	}
}

func TestWALReaderSeesCommittedSnapshotWhileWriterIsOpen(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := db.SQLDB().Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("writer pool max connections=%d want=1", got)
	}
	if _, err := db.SQLDB().ExecContext(ctx, `CREATE TABLE pool_probe(id INTEGER PRIMARY KEY, value TEXT NOT NULL); INSERT INTO pool_probe(id,value) VALUES(1,'committed')`); err != nil {
		t.Fatal(err)
	}

	writerEntered := make(chan struct{})
	releaseWriter := make(chan struct{})
	writerDone := make(chan error, 1)
	go func() {
		writerDone <- db.Update(ctx, func(tx harnessstore.Tx) error {
			raw := tx.(*transaction)
			if _, err := raw.tx.ExecContext(ctx, `UPDATE pool_probe SET value='uncommitted' WHERE id=1`); err != nil {
				return err
			}
			close(writerEntered)
			<-releaseWriter
			return nil
		})
	}()
	select {
	case <-writerEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("writer did not enter transaction")
	}

	readDone := make(chan error, 1)
	var got string
	go func() {
		readDone <- db.View(ctx, func(reader harnessstore.Reader) error {
			raw := reader.(*transaction)
			return raw.tx.QueryRowContext(ctx, `SELECT value FROM pool_probe WHERE id=1`).Scan(&got)
		})
	}()
	select {
	case err := <-readDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WAL reader was blocked by open writer transaction")
	}
	if got != "committed" {
		t.Fatalf("reader saw %q want last committed snapshot", got)
	}

	close(releaseWriter)
	if err := <-writerDone; err != nil {
		t.Fatal(err)
	}
	if err := db.View(ctx, func(reader harnessstore.Reader) error {
		raw := reader.(*transaction)
		return raw.tx.QueryRowContext(ctx, `SELECT value FROM pool_probe WHERE id=1`).Scan(&got)
	}); err != nil {
		t.Fatal(err)
	}
	if got != "uncommitted" {
		t.Fatalf("reader did not observe committed writer value: %q", got)
	}
}
