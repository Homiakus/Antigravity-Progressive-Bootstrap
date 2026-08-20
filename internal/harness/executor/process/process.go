package process

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
)

const (
	defaultTailBytes = 256 * 1024
	defaultQueueSize = 128
	streamChunkSize  = 32 * 1024
	defaultGrace     = 5 * time.Second
)

var (
	ErrAlreadyRunning = errors.New("process executor: execution already running")
	ErrNotRunning     = errors.New("process executor: execution not running")
)

type Executor struct {
	mu      sync.RWMutex
	running map[harnessexecutor.ExecutionID]*runningProcess
	now     func() time.Time
}

type runningProcess struct {
	id        harnessexecutor.ExecutionID
	cmd       *exec.Cmd
	tree      processTree
	startedAt time.Time
	grace     time.Duration
	done      chan struct{}
	cancelled atomic.Bool
}

type Options struct {
	Now func() time.Time
}

func New(opts Options) *Executor {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Executor{running: make(map[harnessexecutor.ExecutionID]*runningProcess), now: now}
}

func (e *Executor) Capabilities() harnessexecutor.Capabilities {
	return harnessexecutor.Capabilities{
		ProcessTree: true, SoftCancel: true, GracefulTerminate: true,
		HardKill: true, Streaming: true, ReconcileLive: true,
	}
}

func (e *Executor) Prepare(ctx context.Context, req harnessexecutor.Request) (harnessexecutor.Prepared, error) {
	if err := req.Validate(); err != nil {
		return harnessexecutor.Prepared{}, err
	}
	if err := ctx.Err(); err != nil {
		return harnessexecutor.Prepared{}, err
	}
	path, err := exec.LookPath(req.Command)
	if err != nil {
		return harnessexecutor.Prepared{}, fmt.Errorf("resolve command %q: %w", req.Command, err)
	}
	if req.Dir != "" {
		st, err := os.Stat(req.Dir)
		if err != nil {
			return harnessexecutor.Prepared{}, fmt.Errorf("stat working directory: %w", err)
		}
		if !st.IsDir() {
			return harnessexecutor.Prepared{}, fmt.Errorf("working directory is not a directory: %s", req.Dir)
		}
	}
	if req.OutputTailBytes == 0 {
		req.OutputTailBytes = defaultTailBytes
	}
	if req.StreamQueue == 0 {
		req.StreamQueue = defaultQueueSize
	}
	if req.GracePeriod == 0 {
		req.GracePeriod = defaultGrace
	}
	return harnessexecutor.Prepared{Request: req, ResolvedPath: path}, nil
}

func (e *Executor) Execute(ctx context.Context, prepared harnessexecutor.Prepared, sink harnessexecutor.LogSink) (harnessexecutor.Result, error) {
	req := prepared.Request
	if err := req.Validate(); err != nil {
		return harnessexecutor.Result{}, err
	}
	if prepared.ResolvedPath == "" {
		return harnessexecutor.Result{}, fmt.Errorf("request was not prepared: resolved command path is empty")
	}
	if sink == nil {
		sink = harnessexecutor.NopSink{}
	}

	cmd := exec.Command(prepared.ResolvedPath, req.Args...)
	cmd.Dir = req.Dir
	cmd.Env = mergedEnv(req.Env)
	configureCommand(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return harnessexecutor.Result{}, fmt.Errorf("open stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return harnessexecutor.Result{}, fmt.Errorf("open stderr pipe: %w", err)
	}

	startedAt, tree, err := e.start(ctx, cmd, req.Timeouts.Start)
	if err != nil {
		result := harnessexecutor.Result{ExecutionID: req.ID, FinishedAt: e.now().UTC(), Error: err.Error()}
		if errors.Is(err, errStartTimeout) {
			result.TimedOut = true
			result.TimeoutClass = "START_TIMEOUT"
		}
		return result, err
	}
	rp := &runningProcess{id: req.ID, cmd: cmd, tree: tree, startedAt: startedAt, grace: req.GracePeriod, done: make(chan struct{})}
	if err := e.register(rp); err != nil {
		_ = tree.HardKill()
		_ = tree.Close()
		_ = cmd.Wait()
		return harnessexecutor.Result{}, err
	}
	defer e.unregister(req.ID)

	result := harnessexecutor.Result{ExecutionID: req.ID, PID: cmd.Process.Pid, ExitCode: -1, StartedAt: startedAt}
	stdoutTail := newRingTail(req.OutputTailBytes)
	stderrTail := newRingTail(req.OutputTailBytes)
	var stdoutBytes atomic.Int64
	var stderrBytes atomic.Int64
	var lastActivity atomic.Int64
	lastActivity.Store(startedAt.UnixNano())

	streamCtx, streamCancel := context.WithCancel(context.Background())
	defer streamCancel()
	chunks := make(chan harnessexecutor.LogChunk, req.StreamQueue)
	var sinkWG sync.WaitGroup
	var sinkErrMu sync.Mutex
	var sinkErr error
	sinkWG.Add(1)
	go func() {
		defer sinkWG.Done()
		for chunk := range chunks {
			if err := sink.WriteChunk(streamCtx, chunk); err != nil {
				sinkErrMu.Lock()
				if sinkErr == nil {
					sinkErr = err
				}
				sinkErrMu.Unlock()
				// Keep draining the bounded channel even after a sink error so the
				// child cannot deadlock on a full stdout/stderr pipe.
			}
		}
	}()

	var readWG sync.WaitGroup
	readWG.Add(2)
	go streamPipe(stdout, harnessexecutor.StreamStdout, chunks, stdoutTail, &stdoutBytes, &lastActivity, e.now, &readWG)
	go streamPipe(stderr, harnessexecutor.StreamStderr, chunks, stderrTail, &stderrBytes, &lastActivity, e.now, &readWG)

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	var waitErr error
	var timeoutClass string
	var timedOut bool
	var cancelledByContext bool
	execTimer := durationTimer(req.Timeouts.Execution)
	defer stopTimer(execTimer)
	idleTicker := durationTicker(req.Timeouts.Idle)
	defer stopTicker(idleTicker)

waitLoop:
	for {
		select {
		case waitErr = <-waitCh:
			break waitLoop
		case <-ctx.Done():
			cancelledByContext = true
			rp.cancelled.Store(true)
			_ = terminateWithEscalation(ctx, rp, harnessexecutor.CancelGraceful)
			waitErr = <-waitCh
			break waitLoop
		case <-timerChan(execTimer):
			timedOut = true
			timeoutClass = "EXECUTION_TIMEOUT"
			_ = terminateWithEscalation(context.Background(), rp, harnessexecutor.CancelGraceful)
			waitErr = <-waitCh
			break waitLoop
		case <-tickerChan(idleTicker):
			if req.Timeouts.Idle > 0 {
				last := time.Unix(0, lastActivity.Load())
				if e.now().UTC().Sub(last) >= req.Timeouts.Idle {
					timedOut = true
					timeoutClass = "IDLE_TIMEOUT"
					_ = terminateWithEscalation(context.Background(), rp, harnessexecutor.CancelGraceful)
					waitErr = <-waitCh
					break waitLoop
				}
			}
		}
	}
	close(rp.done)
	readWG.Wait()
	close(chunks)
	sinkWG.Wait()
	_ = tree.Close()

	result.FinishedAt = e.now().UTC()
	result.StdoutBytes = stdoutBytes.Load()
	result.StderrBytes = stderrBytes.Load()
	result.StdoutTail = stdoutTail.Bytes()
	result.StderrTail = stderrTail.Bytes()
	result.StdoutTruncated = result.StdoutBytes > int64(len(result.StdoutTail))
	result.StderrTruncated = result.StderrBytes > int64(len(result.StderrTail))
	result.TimedOut = timedOut
	result.TimeoutClass = timeoutClass
	result.Cancelled = cancelledByContext || rp.cancelled.Load()
	result.ExitCode = exitCode(waitErr)

	sinkErrMu.Lock()
	capturedSinkErr := sinkErr
	sinkErrMu.Unlock()
	if timedOut {
		result.Error = timeoutClass
		return result, fmt.Errorf("process %s timed out: %s", req.ID, timeoutClass)
	}
	if cancelledByContext {
		result.Error = ctx.Err().Error()
		return result, ctx.Err()
	}
	if waitErr != nil {
		result.Error = waitErr.Error()
		return result, fmt.Errorf("process %s exited with code %d: %w", req.ID, result.ExitCode, waitErr)
	}
	if capturedSinkErr != nil {
		result.Error = capturedSinkErr.Error()
		return result, fmt.Errorf("process %s log sink failed: %w", req.ID, capturedSinkErr)
	}
	return result, nil
}

func (e *Executor) Cancel(ctx context.Context, id harnessexecutor.ExecutionID, mode harnessexecutor.CancelMode) error {
	rp, ok := e.lookup(id)
	if !ok {
		return ErrNotRunning
	}
	rp.cancelled.Store(true)
	return terminateWithEscalation(ctx, rp, mode)
}

func (e *Executor) Reconcile(_ context.Context, id harnessexecutor.ExecutionID) (harnessexecutor.RuntimeStatus, error) {
	rp, ok := e.lookup(id)
	if !ok {
		return harnessexecutor.RuntimeStatus{ExecutionID: id, State: harnessexecutor.RuntimeUnknown}, nil
	}
	return harnessexecutor.RuntimeStatus{ExecutionID: id, State: harnessexecutor.RuntimeRunning, PID: rp.cmd.Process.Pid, StartedAt: rp.startedAt}, nil
}

func (e *Executor) register(rp *runningProcess) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.running[rp.id]; exists {
		return ErrAlreadyRunning
	}
	e.running[rp.id] = rp
	return nil
}

func (e *Executor) unregister(id harnessexecutor.ExecutionID) {
	e.mu.Lock()
	delete(e.running, id)
	e.mu.Unlock()
}

func (e *Executor) lookup(id harnessexecutor.ExecutionID) (*runningProcess, bool) {
	e.mu.RLock()
	rp, ok := e.running[id]
	e.mu.RUnlock()
	return rp, ok
}

func streamPipe(r io.Reader, stream harnessexecutor.Stream, chunks chan<- harnessexecutor.LogChunk, tail *ringTail, count *atomic.Int64, lastActivity *atomic.Int64, now func() time.Time, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, streamChunkSize)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			at := now().UTC()
			copyChunk := append([]byte(nil), buf[:n]...)
			count.Add(int64(n))
			lastActivity.Store(at.UnixNano())
			tail.Write(copyChunk)
			chunks <- harnessexecutor.LogChunk{At: at, Stream: stream, Data: copyChunk}
		}
		if err != nil {
			return
		}
	}
}

func terminateWithEscalation(ctx context.Context, rp *runningProcess, mode harnessexecutor.CancelMode) error {
	switch mode {
	case harnessexecutor.CancelSoft:
		return rp.tree.SoftCancel()
	case harnessexecutor.CancelHard:
		return rp.tree.HardKill()
	case harnessexecutor.CancelGraceful:
		err := rp.tree.GracefulTerminate()
		if err != nil {
			_ = rp.tree.HardKill()
			return err
		}
		grace := rp.grace
		if grace <= 0 {
			grace = defaultGrace
		}
		timer := time.NewTimer(grace)
		defer timer.Stop()
		select {
		case <-rp.done:
			return nil
		case <-ctx.Done():
			_ = rp.tree.HardKill()
			return ctx.Err()
		case <-timer.C:
			return rp.tree.HardKill()
		}
	default:
		return fmt.Errorf("unknown cancellation mode %q", mode)
	}
}

func mergedEnv(overrides map[string]string) []string {
	env := append([]string(nil), os.Environ()...)
	if len(overrides) == 0 {
		return env
	}
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		env = append(env, key+"="+overrides[key])
	}
	return env
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func durationTimer(d time.Duration) *time.Timer {
	if d <= 0 {
		return nil
	}
	return time.NewTimer(d)
}

func timerChan(t *time.Timer) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

func stopTimer(t *time.Timer) {
	if t != nil {
		t.Stop()
	}
}

func durationTicker(d time.Duration) *time.Ticker {
	if d <= 0 {
		return nil
	}
	interval := d / 4
	if interval < 50*time.Millisecond {
		interval = 50 * time.Millisecond
	}
	if interval > time.Second {
		interval = time.Second
	}
	return time.NewTicker(interval)
}

func tickerChan(t *time.Ticker) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

func stopTicker(t *time.Ticker) {
	if t != nil {
		t.Stop()
	}
}
