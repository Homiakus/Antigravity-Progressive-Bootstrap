package cas

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

var (
	ErrNotFound      = errors.New("harness cas: content not found")
	ErrCorruptDigest = errors.New("harness cas: content digest mismatch")
)

type CAS struct {
	rootDir string
}

func New(rootDir string) (*CAS, error) {
	if strings.TrimSpace(rootDir) == "" {
		return nil, fmt.Errorf("cas root directory is required")
	}
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve cas root: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(abs, "temp"), 0o755); err != nil {
		return nil, fmt.Errorf("create cas temp dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(abs, "sha256"), 0o755); err != nil {
		return nil, fmt.Errorf("create cas sha256 dir: %w", err)
	}
	return &CAS{rootDir: abs}, nil
}

func (c *CAS) RootDir() string {
	return c.rootDir
}

func (c *CAS) Write(ctx context.Context, r io.Reader) (digest string, size int64, err error) {
	if r == nil {
		return "", 0, fmt.Errorf("reader is required")
	}
	tempDir := filepath.Join(c.rootDir, "temp")
	var randBytes [8]byte
	_, _ = rand.Read(randBytes[:])
	tempName := fmt.Sprintf("cas_tmp_%d_%s", time.Now().UnixNano(), hex.EncodeToString(randBytes[:]))
	tempPath := filepath.Join(tempDir, tempName)

	f, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return "", 0, fmt.Errorf("create temp cas file: %w", err)
	}

	hasher := sha256.New()
	multi := io.MultiWriter(f, hasher)

	written, copyErr := io.Copy(multi, r)
	if copyErr != nil {
		_ = f.Close()
		_ = os.Remove(tempPath)
		return "", 0, fmt.Errorf("write cas content: %w", copyErr)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tempPath)
		return "", 0, fmt.Errorf("fsync temp cas file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", 0, fmt.Errorf("close temp cas file: %w", err)
	}

	computedHex := hex.EncodeToString(hasher.Sum(nil))
	digest = "sha256:" + computedHex

	targetDir := filepath.Join(c.rootDir, "sha256", computedHex[:2])
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		_ = os.Remove(tempPath)
		return "", 0, fmt.Errorf("create target cas dir: %w", err)
	}

	targetPath := filepath.Join(targetDir, computedHex)
	// Atomic rename to final CAS path
	if err := os.Rename(tempPath, targetPath); err != nil {
		// On Windows or concurrent writers, target may already exist
		if _, statErr := os.Stat(targetPath); statErr == nil {
			_ = os.Remove(tempPath)
			return digest, written, nil
		}
		_ = os.Remove(tempPath)
		return "", 0, fmt.Errorf("commit cas file: %w", err)
	}

	return digest, written, nil
}

func (c *CAS) WriteBytes(ctx context.Context, data []byte) (digest string, size int64, err error) {
	return c.Write(ctx, strings.NewReader(string(data)))
}

func (c *CAS) Open(digest string) (io.ReadCloser, int64, error) {
	path, err := c.FilePath(digest)
	if err != nil {
		return nil, 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, ErrNotFound
		}
		return nil, 0, err
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, err
	}
	return f, fi.Size(), nil
}

func (c *CAS) ReadBytes(digest string) ([]byte, error) {
	rc, _, err := c.Open(digest)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func (c *CAS) Exists(digest string) bool {
	path, err := c.FilePath(digest)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func (c *CAS) FilePath(digest string) (string, error) {
	if !harnessmodel.ValidateDigest(digest) {
		return "", fmt.Errorf("invalid digest %q", digest)
	}
	hexPart := strings.TrimPrefix(digest, "sha256:")
	return filepath.Join(c.rootDir, "sha256", hexPart[:2], hexPart), nil
}

func (c *CAS) Verify(digest string) error {
	rc, size, err := c.Open(digest)
	if err != nil {
		return err
	}
	defer rc.Close()

	hasher := sha256.New()
	actualSize, err := io.Copy(hasher, rc)
	if err != nil {
		return fmt.Errorf("read cas file for verification: %w", err)
	}
	if actualSize != size {
		return ErrCorruptDigest
	}
	actualDigest := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	if actualDigest != digest {
		return ErrCorruptDigest
	}
	return nil
}

func (c *CAS) GC(ctx context.Context, reachableDigests map[string]struct{}, gracePeriod time.Duration) (reclaimedBytes int64, removedCount int, err error) {
	if gracePeriod < 0 {
		gracePeriod = 0
	}
	now := time.Now()
	threshold := now.Add(-gracePeriod)

	// 1. Clean old temp files
	tempDir := filepath.Join(c.rootDir, "temp")
	tempEntries, _ := os.ReadDir(tempDir)
	for _, entry := range tempEntries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(threshold) {
			_ = os.Remove(filepath.Join(tempDir, entry.Name()))
		}
	}

	// 2. Scan sha256 entries
	shaDir := filepath.Join(c.rootDir, "sha256")
	prefixes, err := os.ReadDir(shaDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("scan cas sha256 dir: %w", err)
	}

	for _, p := range prefixes {
		if !p.IsDir() {
			continue
		}
		prefixDir := filepath.Join(shaDir, p.Name())
		files, err := os.ReadDir(prefixDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			fullPath := filepath.Join(prefixDir, f.Name())
			info, err := f.Info()
			if err != nil {
				continue
			}
			// If file is recent (within grace period), do not collect it yet
			if gracePeriod > 0 && info.ModTime().After(threshold) {
				continue
			}

			digest := "sha256:" + f.Name()
			if reachableDigests != nil {
				if _, ok := reachableDigests[digest]; ok {
					continue
				}
			}

			// File is unreferenced / orphaned and past grace period -> remove
			size := info.Size()
			if err := os.Remove(fullPath); err == nil {
				reclaimedBytes += size
				removedCount++
			}
		}
	}
	return reclaimedBytes, removedCount, nil
}
