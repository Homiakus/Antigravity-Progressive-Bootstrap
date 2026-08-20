package logsink

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
)

func TestJSONLSinkPersistsRawChunkBytesAndStreams(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "run.jsonl")
	sink, err := OpenJSONL(path)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Unix(123, 456).UTC()
	chunks := []harnessexecutor.LogChunk{
		{At: at, Stream: harnessexecutor.StreamStdout, Data: []byte("hello\n")},
		{At: at.Add(time.Second), Stream: harnessexecutor.StreamStderr, Data: []byte{0xff, 0x00, 0x01}},
	}
	for _, chunk := range chunks {
		if err := sink.WriteChunk(context.Background(), chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("second close should be idempotent: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var got []record
	for scanner.Scan() {
		var rec record
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			t.Fatalf("decode JSONL record: %v", err)
		}
		got = append(got, rec)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(chunks) {
		t.Fatalf("records=%d want=%d", len(got), len(chunks))
	}
	for i := range chunks {
		if got[i].Stream != chunks[i].Stream || string(got[i].Data) != string(chunks[i].Data) {
			t.Fatalf("record %d mismatch: got=%+v want=%+v", i, got[i], chunks[i])
		}
	}
}

func TestJSONLSinkRejectsWritesAfterClose(t *testing.T) {
	sink, err := OpenJSONL(filepath.Join(t.TempDir(), "run.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sink.WriteChunk(context.Background(), harnessexecutor.LogChunk{At: time.Now(), Stream: harnessexecutor.StreamStdout, Data: []byte("x")}); err == nil {
		t.Fatal("write after close unexpectedly succeeded")
	}
}
