package cas

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestCASWriteReadAndDeduplication(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	content := []byte("antigravity durable harness test artifact")
	digest, size, err := c.WriteBytes(ctx, content)
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len(content)) {
		t.Fatalf("size=%d want=%d", size, len(content))
	}
	expectedDigest := harnessmodel.ComputeSHA256Digest(content)
	if digest != expectedDigest {
		t.Fatalf("digest=%s want=%s", digest, expectedDigest)
	}

	// Verify exists
	if !c.Exists(digest) {
		t.Fatal("expected content to exist in CAS")
	}

	// Verify read
	readData, err := c.ReadBytes(digest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readData, content) {
		t.Fatalf("read mismatch: %q vs %q", string(readData), string(content))
	}

	// Write again (deduplication check)
	digest2, size2, err := c.WriteBytes(ctx, content)
	if err != nil {
		t.Fatal(err)
	}
	if digest2 != digest || size2 != size {
		t.Fatalf("duplicate write mismatch: %s (%d)", digest2, size2)
	}

	// Verify integrity
	if err := c.Verify(digest); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
}

func TestCASCorruptionDetection(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	content := []byte("original uncorrupted data")
	digest, _, err := c.WriteBytes(ctx, content)
	if err != nil {
		t.Fatal(err)
	}

	path, err := c.FilePath(digest)
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt file on disk
	if err := os.WriteFile(path, []byte("tampered data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify should report corruption
	if err := c.Verify(digest); err != ErrCorruptDigest {
		t.Fatalf("expected ErrCorruptDigest, got %v", err)
	}
}

func TestCASMarkAndSweepGCWithGracePeriod(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	d1, _, err := c.WriteBytes(ctx, []byte("reachable-1"))
	if err != nil {
		t.Fatal(err)
	}
	d2, _, err := c.WriteBytes(ctx, []byte("reachable-2"))
	if err != nil {
		t.Fatal(err)
	}
	d3, _, err := c.WriteBytes(ctx, []byte("orphan-to-delete"))
	if err != nil {
		t.Fatal(err)
	}

	path3, err := c.FilePath(d3)
	if err != nil {
		t.Fatal(err)
	}

	// Artificial backdate for orphan file so it is outside grace period
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path3, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	reachable := map[string]struct{}{
		d1: {},
		d2: {},
	}

	// Run GC with 1 hour grace period
	reclaimed, count, err := c.GC(ctx, reachable, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || reclaimed == 0 {
		t.Fatalf("expected 1 orphan reclaimed, got count=%d bytes=%d", count, reclaimed)
	}

	if !c.Exists(d1) || !c.Exists(d2) {
		t.Fatal("reachable artifacts were incorrectly deleted")
	}
	if c.Exists(d3) {
		t.Fatal("orphan artifact was not deleted")
	}
}
