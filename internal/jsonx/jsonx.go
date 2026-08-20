package jsonx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

func Read[T any](path string, def T) (T, error) {
	var out = def
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, fmt.Errorf("read %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, fmt.Errorf("parse %s: %w", path, err)
	}
	return out, nil
}

func ReadMap(path string) (map[string]any, error) {
	return Read(path, map[string]any{})
}

func WriteAtomic(path string, v any, backupRoot string) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	b = append(b, '\n')
	return WriteBytesAtomic(path, b, backupRoot)
}

func WriteTextAtomic(path, text, backupRoot string) error {
	return WriteBytesAtomic(path, []byte(text), backupRoot)
}

func WriteBytesAtomic(path string, data []byte, backupRoot string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if backupRoot != "" {
		if _, err := os.Stat(path); err == nil {
			if _, err := BackupFile(path, backupRoot); err != nil {
				return err
			}
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".agctl-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil && runtime.GOOS != "windows" {
		return err
	}

	// os.Rename cannot replace an existing file on every Windows configuration.
	if runtime.GOOS == "windows" {
		_ = os.Remove(path)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func BackupFile(path, root string) (string, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	src, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer src.Close()

	stamp := time.Now().Format("20060102-150405.000")
	base := filepath.Base(path)
	dstPath := filepath.Join(root, fmt.Sprintf("%s.%s.bak", base, stamp))
	dst, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	if err := dst.Sync(); err != nil {
		return "", err
	}
	return dstPath, nil
}

func ListBackups(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".bak") {
			out = append(out, filepath.Join(root, e.Name()))
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out, nil
}

func RestoreBackup(backup, destination string) error {
	b, err := os.ReadFile(backup)
	if err != nil {
		return err
	}
	return WriteBytesAtomic(destination, b, "")
}
