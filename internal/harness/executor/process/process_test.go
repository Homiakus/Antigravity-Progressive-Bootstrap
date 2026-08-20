package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
)

const helperEnv = "GO_WANT_HARNESS_PROCESS_HELPER"

type captureSink struct {
	mu       sync.Mutex
	stdout   bytes.Buffer
	stderr   bytes.Buffer
	signal   []byte
	signaled chan struct{}
	once     sync.Once
}

func newCaptureSink(signal string) *captureSink {
	return &captureSink{signal: []byte(signal), signaled: make(chan struct{})}
}

func (s *captureSink) WriteChunk(_ context.Context, chunk harnessexecutor.LogChunk) error {
	s.mu.Lock()
	if chunk.Stream == harnessexecutor.StreamStderr {
		_, _ = s.stderr.Write(chunk.Data)
	} else {
		_, _ = s.stdout.Write(chunk.Data)
	}
	matched := len(s.signal) > 0 && (bytes.Contains(s.stdout.Bytes(), s.signal) || bytes.Contains(s.stderr.Bytes(), s.signal))
	s.mu.Unlock()
	if matched {
		s.once.Do(func() { close(s.signaled) })
	}
	return nil
}

func (s *captureSink) strings() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stdout.String(), s.stderr.String()
}

type failingSink struct{ err error }

func (s failingSink) WriteChunk(context.Context, harnessexecutor.LogChunk) error { return s.err }

func helperPrepared(t *testing.T, executor *Executor, id harnessexecutor.ExecutionID, action string, args ...string) harnessexecutor.Prepared {
	t.Helper()
	argv := []string{"-test.run=TestProcessHelperProcess", "--", action}
	argv = append(argv, args...)
	prepared, err := executor.Prepare(context.Background(), harnessexecutor.Request{
		ID: id, Command: os.Args[0], Args: argv,
		Env: map[string]string{helperEnv: "1"},
		GracePeriod: 100 * time.Millisecond,
		OutputTailBytes: 4096,
		StreamQueue: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	return prepared
}

func waitRuntimeState(t *testing.T, executor *Executor, id harnessexecutor.ExecutionID, want harnessexecutor.RuntimeState) harnessexecutor.RuntimeStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, err := executor.Reconcile(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if status.State == want {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("execution %s never reached %s", id, want)
	return harnessexecutor.RuntimeStatus{}
}

func TestExecuteStreamsStdoutAndStderrWithBoundedTail(t *testing.T) {
	executor := New(Options{})
	prepared := helperPrepared(t, executor, "stream-basic", "echo")
	sink := newCaptureSink("")
	result, err := executor.Execute(context.Background(), prepared, sink)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr := sink.strings()
	if stdout != "stdout-data" || stderr != "stderr-data" {
		t.Fatalf("unexpected streams stdout=%q stderr=%q", stdout, stderr)
	}
	if result.ExitCode != 0 || result.StdoutBytes != int64(len(stdout)) || result.StderrBytes != int64(len(stderr)) {
		t.Fatalf("unexpected result accounting: %+v", result)
	}
	if string(result.StdoutTail) != stdout || string(result.StderrTail) != stderr || result.StdoutTruncated || result.StderrTruncated {
		t.Fatalf("unexpected output tails: %+v", result)
	}
	status, err := executor.Reconcile(context.Background(), prepared.Request.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != harnessexecutor.RuntimeUnknown {
		t.Fatalf("finished in-process execution should no longer be live: %+v", status)
	}
}

func TestExecutionTimeoutTerminatesProcess(t *testing.T) {
	executor := New(Options{})
	prepared := helperPrepared(t, executor, "execution-timeout", "sleep", "3s")
	prepared.Request.Timeouts.Execution = 150 * time.Millisecond
	result, err := executor.Execute(context.Background(), prepared, nil)
	if err == nil {
		t.Fatal("execution timeout unexpectedly succeeded")
	}
	if !result.TimedOut || result.TimeoutClass != "EXECUTION_TIMEOUT" {
		t.Fatalf("unexpected execution timeout result: %+v err=%v", result, err)
	}
}

func TestIdleTimeoutTerminatesSilentProcess(t *testing.T) {
	executor := New(Options{})
	prepared := helperPrepared(t, executor, "idle-timeout", "sleep", "3s")
	prepared.Request.Timeouts.Idle = 150 * time.Millisecond
	prepared.Request.Timeouts.Execution = 2 * time.Second
	result, err := executor.Execute(context.Background(), prepared, nil)
	if err == nil {
		t.Fatal("idle timeout unexpectedly succeeded")
	}
	if !result.TimedOut || result.TimeoutClass != "IDLE_TIMEOUT" {
		t.Fatalf("unexpected idle timeout result: %+v err=%v", result, err)
	}
}

func TestRegularOutputPreventsIdleTimeout(t *testing.T) {
	executor := New(Options{})
	prepared := helperPrepared(t, executor, "idle-active", "paced", "8", "50ms")
	prepared.Request.Timeouts.Idle = 150 * time.Millisecond
	prepared.Request.Timeouts.Execution = 2 * time.Second
	result, err := executor.Execute(context.Background(), prepared, nil)
	if err != nil {
		t.Fatalf("active process hit idle timeout: result=%+v err=%v", result, err)
	}
	if result.TimedOut || result.StdoutBytes == 0 {
		t.Fatalf("unexpected active result: %+v", result)
	}
}

func TestSinkFailureDoesNotDeadlockOutputDrain(t *testing.T) {
	executor := New(Options{})
	prepared := helperPrepared(t, executor, "sink-failure", "emit", "stdout", strconv.Itoa(16*1024*1024))
	prepared.Request.Timeouts.Execution = 5 * time.Second
	result, err := executor.Execute(context.Background(), prepared, failingSink{err: errors.New("sink boom")})
	if err == nil || !strings.Contains(err.Error(), "sink boom") {
		t.Fatalf("sink failure error=%v result=%+v", err, result)
	}
	if result.StdoutBytes != 16*1024*1024 || !result.StdoutTruncated || len(result.StdoutTail) != prepared.Request.OutputTailBytes {
		t.Fatalf("output was not fully drained/bounded after sink failure: %+v", result)
	}
}

func TestStderrFloodIsBoundedButFullyCounted(t *testing.T) {
	executor := New(Options{})
	prepared := helperPrepared(t, executor, "stderr-flood", "emit", "stderr", strconv.Itoa(8*1024*1024))
	result, err := executor.Execute(context.Background(), prepared, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.StderrBytes != 8*1024*1024 || !result.StderrTruncated || len(result.StderrTail) != prepared.Request.OutputTailBytes {
		t.Fatalf("unexpected stderr flood accounting: %+v", result)
	}
}

func TestDuplicateExecutionIDRejectedBeforeSecondProcessSideEffect(t *testing.T) {
	executor := New(Options{})
	first := helperPrepared(t, executor, "duplicate-id", "sleep", "5s")
	firstDone := make(chan error, 1)
	go func() {
		_, err := executor.Execute(context.Background(), first, nil)
		firstDone <- err
	}()
	waitRuntimeState(t, executor, first.Request.ID, harnessexecutor.RuntimeRunning)

	marker := filepath.Join(t.TempDir(), "duplicate-started")
	second := helperPrepared(t, executor, first.Request.ID, "marker-now", marker)
	if _, err := executor.Execute(context.Background(), second, nil); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("duplicate execution error=%v want ErrAlreadyRunning", err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("duplicate process side effect occurred before identity conflict: stat=%v", err)
	}
	if err := executor.Cancel(context.Background(), first.Request.ID, harnessexecutor.CancelHard); err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstDone:
	case <-time.After(5 * time.Second):
		t.Fatal("first duplicate test process did not stop")
	}
}

func TestHardCancelKillsFullDescendantTree(t *testing.T) {
	executor := New(Options{})
	marker := filepath.Join(t.TempDir(), "grandchild-survived")
	prepared := helperPrepared(t, executor, "tree-kill", "spawn-marker-child", marker)
	sink := newCaptureSink("child-started:")
	done := make(chan error, 1)
	go func() {
		_, err := executor.Execute(context.Background(), prepared, sink)
		done <- err
	}()
	select {
	case <-sink.signaled:
	case <-time.After(5 * time.Second):
		t.Fatal("helper parent never spawned descendant")
	}
	if err := executor.Cancel(context.Background(), prepared.Request.ID, harnessexecutor.CancelHard); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled process tree did not terminate")
	}
	// The descendant writes the marker after 1.2s if it escaped ownership.
	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("grandchild survived process-tree cancellation: stat=%v", err)
	}
}

func TestRingTailKeepsConstantMemoryShapeAcrossSynthetic500MB(t *testing.T) {
	const tailSize = 64 * 1024
	tail := newRingTail(tailSize)
	chunk := bytes.Repeat([]byte{'z'}, 64*1024)
	const total = 500 * 1024 * 1024
	for written := 0; written < total; written += len(chunk) {
		tail.Write(chunk)
	}
	out := tail.Bytes()
	if len(out) != tailSize {
		t.Fatalf("tail grew with total output: len=%d want=%d", len(out), tailSize)
	}
	if !bytes.Equal(out, chunk) {
		t.Fatal("tail does not contain the last synthetic output chunk")
	}
}

func TestProcessHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "1" {
		return
	}
	args := os.Args
	separator := -1
	for i, arg := range args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator < 0 || separator+1 >= len(args) {
		os.Exit(90)
	}
	action := args[separator+1]
	params := args[separator+2:]
	exit := 0
	switch action {
	case "echo":
		_, _ = io.WriteString(os.Stdout, "stdout-data")
		_, _ = io.WriteString(os.Stderr, "stderr-data")
	case "sleep":
		d, err := time.ParseDuration(params[0])
		if err != nil {
			exit = 91
			break
		}
		time.Sleep(d)
	case "paced":
		count, _ := strconv.Atoi(params[0])
		interval, _ := time.ParseDuration(params[1])
		for i := 0; i < count; i++ {
			_, _ = fmt.Fprintf(os.Stdout, "tick-%d\n", i)
			time.Sleep(interval)
		}
	case "emit":
		total, _ := strconv.Atoi(params[1])
		var writer io.Writer = os.Stdout
		if params[0] == "stderr" {
			writer = os.Stderr
		}
		chunk := bytes.Repeat([]byte{'x'}, 32*1024)
		for total > 0 {
			n := len(chunk)
			if n > total {
				n = total
			}
			if _, err := writer.Write(chunk[:n]); err != nil {
				exit = 92
				break
			}
			total -= n
		}
	case "marker-now":
		if err := os.WriteFile(params[0], []byte("started"), 0o644); err != nil {
			exit = 93
		}
	case "spawn-marker-child":
		child := exec.Command(os.Args[0], "-test.run=TestProcessHelperProcess", "--", "delayed-marker", params[0])
		child.Env = append(os.Environ(), helperEnv+"=1")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			exit = 94
			break
		}
		_, _ = fmt.Fprintf(os.Stdout, "child-started:%d\n", child.Process.Pid)
		time.Sleep(30 * time.Second)
	case "delayed-marker":
		time.Sleep(1200 * time.Millisecond)
		if err := os.WriteFile(params[0], []byte("survived"), 0o644); err != nil {
			exit = 95
		}
	default:
		exit = 96
	}
	os.Exit(exit)
}
