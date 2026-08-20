package logsink

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
)

type JSONL struct {
	mu     sync.Mutex
	file   *os.File
	writer *bufio.Writer
	closed bool
}

type record struct {
	Timestamp string                 `json:"ts"`
	Stream    harnessexecutor.Stream `json:"stream"`
	Data      []byte                 `json:"data"`
}

func OpenJSONL(path string) (*JSONL, error) {
	if path == "" {
		return nil, fmt.Errorf("log path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return &JSONL{file: file, writer: bufio.NewWriterSize(file, 64*1024)}, nil
}

func (s *JSONL) WriteChunk(ctx context.Context, chunk harnessexecutor.LogChunk) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("JSONL log sink is closed")
	}
	payload, err := json.Marshal(record{Timestamp: chunk.At.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), Stream: chunk.Stream, Data: chunk.Data})
	if err != nil {
		return fmt.Errorf("encode log chunk: %w", err)
	}
	if _, err := s.writer.Write(payload); err != nil {
		return fmt.Errorf("write log chunk: %w", err)
	}
	if err := s.writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("write log delimiter: %w", err)
	}
	return nil
}

func (s *JSONL) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	if err := s.writer.Flush(); err != nil {
		return fmt.Errorf("flush JSONL log: %w", err)
	}
	return nil
}

func (s *JSONL) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	flushErr := s.writer.Flush()
	syncErr := s.file.Sync()
	closeErr := s.file.Close()
	if flushErr != nil {
		return fmt.Errorf("flush JSONL log: %w", flushErr)
	}
	if syncErr != nil {
		return fmt.Errorf("sync JSONL log: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close JSONL log: %w", closeErr)
	}
	return nil
}
