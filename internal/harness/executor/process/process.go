package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
)

const (
	defaultTailBytes = 256 * 1024
	defaultQueueSize = 128
	defaultGrace     = 5 * time.Second
)

var (
	ErrAlreadyRunning = errors.New("process executor: execution already running")
	ErrNotRunning     = errors.New("process executor: execution not running")
)

type Executor struct {
	mu       sync.RWMutex
	starting map[harnessexecutor.ExecutionID]time.Time
	running  map[harnessexecutor.ExecutionID]*runningProcess
	now      func() time.Time
}

type runningProcess struct {
	id           harnessexecutor.ExecutionID
	cmd          *exec.Cmd
	tree         processTree
	startedAt    time.Time
	grace        time.Duration
	done         chan struct{}
	cancelStream context.CancelFunc
	cancelled    atomic.Bool
}

type Options struct {
	Now func() time.Time
}

func New(opts Options) *Executor {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Executor{
		starting: make(map[harnessexecutor.ExecutionID]time.Time),
		running:  make(map[harnessexecutor.ExecutionID]*runningProcess),
		now:      now,
	}
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

	// Reserve the logical execution identity before Cmd.Start. A duplicate ID
	// must fail before any external process side effect is created.
	if err := e.reserve(req.ID); err != nil {
		return harnessexecutor.Result{}, err
	}
	reserved := true
	defer func() {
		if reserved {
			e.releaseReservation(req.ID)
		}
	}()

	result := harnessexecutor.Result{ExecutionID: req.ID, ExitCode: -1}
	stdoutTail := newRingTail(req.OutputTailBytes)
	stderrTail := newRingTail(req.OutputTailBytes)
	var stdoutBytes atomic.Int64
	var stderrBytes atomic.Int64
	var lastActivity atomic.Int64

	streamCtx, streamCancel := context.WithCancel(context.Background())
	defer streamCancel()
	chunks := make(chan harnessexecutor.LogChunk, req.StreamQueue)
	var sinkWG sync.WaitGroup
	var sinkErrMu sync.Mutex
	var sinkErr error
	sinkWG.Add(1)
	go func() {
		defer sinkWG.Done()
		failed := false
		for chunk := range chunks {
			if failed {
				continue
			}
			if err := sink.WriteChunk(streamCtx, chunk); err != nil {
				sinkErrMu.Lock()
				if sinkErr == nil {
					sinkErr = err
				}
				sinkErrMu.Unlock()
				failed = true
				streamCancel()
			}
		}
	}()

	cmd := exec.Command(prepared.ResolvedPath, req.Args...)
	cmd.Dir = req.Dir
	cmd.Env = mergedEnv(req.Env)
	cmd.Stdout = &streamWriter{
		ctx: streamCtx, stream: harnessexecutor.StreamStdout, chunks: chunks,
		tail: stdoutTail, count: &stdoutBytes, lastActivity: &lastActivity, now: e.now,
	}
	cmd.Stderr = &streamWriter{
		ctx: streamCtx, stream: harnessexecutor.StreamStderr, chunks: chunks,
		tail: stderrTail, count: &stderrBytes, lastActivity: &lastActivity, now: e.now,
	}
	configureCommand(cmd)

	startedAt, tree, err := e.start(ctx, cmd, req.Timeouts.Start)
	if err != nil {
		streamCancel()
		close(chunks)
		sinkWG.Wait()
		result.FinishedAt = e.now().UTC()
		result.Error = err.Error()
		if errors.Is(err, errStartTimeout) {
			result.TimedOut = true
			result.TimeoutClass = "START_TIMEOUT"
		}
		return result, err
	}
	lastActivity.Store(startedAt.UnixNano())
	result.PID = cmd.Process.Pid
	result.StartedAt = startedAt

	rp := &runningProcess{
		id: req.ID, cmd: cmd, tree: tree, startedAt: startedAt, grace: req.GracePeriod,
		done: make(chan struct{}), cancelStream: streamCancel,
	}
	if err := e.activate(rp); err != nil {
		streamCancel()
		_ = tree.HardKill()
		_ = cmd.Wait()
		_ = tree.Close()
		close(chunks)
		sinkWG.Wait()
		return harnessexecutor.Result{}, err
	}
	reserved = false
	defer e.unregister(req.ID)

	waitCh := make(chan error, 1)
	go func() {
		waitErr := cmd.Wait()
		close(rp.done)
		waitCh <- waitErr
	}()

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
			// Break downstream backpressure before waiting for process-tree
			// termination. Writers keep draining child output but drop chunks.
			streamCancel()
			_ = terminateWithEscalation(context.Background(), rp, harnessexecutor.CancelGraceful)
			waitErr = <-waitCh
			break waitLoop
		case <-timerChan(execTimer):
			timedOut = true
			timeoutClass = "EXECUTION_TIMEOUT"
			streamCancel()
			_ = terminateWithEscalation(context.Background(), rp, harnessexecutor.CancelGraceful)
			waitErr = <-waitCh
			break waitLoop
		case <-tickerChan(idleTicker):
			if req.Timeouts.Idle > 0 {
				last := time.Unix(0, lastActivity.Load())
				if e.now().UTC().Sub(last) >= req.Timeouts.Idle {
					timedOut = true
					timeoutClass = "IDLE_TIMEOUT"
					streamCancel()
					_ = terminateWithEscalation(context.Background(), rp, harnessexecutor.CancelGraceful)
					waitErr = <-waitCh
					break waitLoop
				}
			}
		}
	}

	// Cmd.Wait returns only after os/exec has finished copying both configured
	// writers. No child output can be sent after this point, so the bounded
	// stream channel can now be closed without racing a writer.
	close(chunks)
	sinkWG.Wait()
	streamCancel()
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
	if mode != harnessexecutor.CancelSoft && rp.cancelStream != nil {
		rp.cancelStream()
	}
	return terminateWithEscalation(ctx, rp, mode)
}

func (e *Executor) Reconcile(_ context.Context, id harnessexecutor.ExecutionID) (harnessexecutor.RuntimeStatus, error) {
	if rp, ok := e.lookup(id); ok {
		return harnessexecutor.RuntimeStatus{ExecutionID: id, State: harnessexecutor.RuntimeRunning, PID: rp.cmd.Process.Pid, StartedAt: rp.startedAt}, nil
	}
	if startedAt, ok := e.lookupStarting(id); ok {
		return harnessexecutor.RuntimeStatus{ExecutionID: id, State: harnessexecutor.RuntimeStarting, StartedAt: startedAt}, nil
	}
	return harnessexecutor.RuntimeStatus{ExecutionID: id, State: harnessexecutor.RuntimeUnknown}, nil
}

func (e *Executor) reserve(id harnessexecutor.ExecutionID) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.starting[id]; exists {
		return ErrAlreadyRunning
	}
	if _, exists := e.running[id]; exists {
		return ErrAlreadyRunning
	}
	e.starting[id] = e.now().UTC()
	return nil
}

func (e *Executor) releaseReservation(id harnessexecutor.ExecutionID) {
	e.mu.Lock()
	delete(e.starting, id)
	e.mu.Unlock()
}

func (e *Executor) activate(rp *runningProcess) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.running[rp.id]; exists {
		return ErrAlreadyRunning
	}
	if _, reserved := e.starting[rp.id]; !reserved {
		return fmt.Errorf("process executor: execution %s lost its start reservation", rp.id)
	}
	delete(e.starting, rp.id)
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

func (e *Executor) lookupStarting(id harnessexecutor.ExecutionID) (time.Time, bool) {
	e.mu.RLock()
	startedAt, ok := e.starting[id]
	e.mu.RUnlock()
	return startedAt, ok
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
	// Produce one value per key instead of appending duplicate KEY=value pairs.
	// Duplicate environment names have platform-dependent semantics, especially
	// on Windows, and should not be part of executor behavior.
	values := make(map[string]string)
	spelling := make(map[string]string)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		normalized := envKey(key)
		values[normalized] = value
		spelling[normalized] = key
	}
	for key, value := range overrides {
		normalized := envKey(key)
		values[normalized] = value
		spelling[normalized] = key
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, normalized := range keys {
		env = append(env, spelling[normalized]+"="+values[normalized])
	}
	return env
}

func envKey(key string) string {
	if os.PathSeparator == '\\' {
		return strings.ToUpper(key)
	}
	return key
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
