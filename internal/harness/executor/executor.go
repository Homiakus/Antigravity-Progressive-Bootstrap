package executor

import (
	"context"
	"fmt"
	"time"
)

type ExecutionID string

type CancelMode string

const (
	CancelSoft     CancelMode = "SOFT_CANCEL"
	CancelGraceful CancelMode = "GRACEFUL_TERMINATE"
	CancelHard     CancelMode = "HARD_KILL"
)

type Stream string

const (
	StreamStdout Stream = "stdout"
	StreamStderr Stream = "stderr"
)

type Timeouts struct {
	Start     time.Duration `json:"startTimeout,omitempty"`
	Execution time.Duration `json:"executionTimeout,omitempty"`
	Idle      time.Duration `json:"idleTimeout,omitempty"`
}

type Request struct {
	ID              ExecutionID       `json:"id"`
	Command         string            `json:"command"`
	Args            []string          `json:"args,omitempty"`
	Dir             string            `json:"dir,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	Timeouts        Timeouts          `json:"timeouts,omitempty"`
	GracePeriod     time.Duration     `json:"gracePeriod,omitempty"`
	OutputTailBytes int               `json:"outputTailBytes,omitempty"`
	StreamQueue     int               `json:"streamQueue,omitempty"`
}

type Prepared struct {
	Request      Request `json:"request"`
	ResolvedPath string  `json:"resolvedPath"`
}

type LogChunk struct {
	At     time.Time `json:"at"`
	Stream Stream    `json:"stream"`
	Data   []byte    `json:"data"`
}

// LogSink receives bounded-stream chunks. Implementations must apply their own
// persistence/backpressure policy and MUST return promptly after ctx is
// cancelled. The process runtime cancels this context during termination so a
// log consumer cannot prevent the child process tree from being drained.
type LogSink interface {
	WriteChunk(context.Context, LogChunk) error
}

type Result struct {
	ExecutionID     ExecutionID `json:"executionId"`
	PID             int         `json:"pid"`
	ExitCode        int         `json:"exitCode"`
	StartedAt       time.Time   `json:"startedAt"`
	FinishedAt      time.Time   `json:"finishedAt"`
	StdoutBytes     int64       `json:"stdoutBytes"`
	StderrBytes     int64       `json:"stderrBytes"`
	StdoutTail      []byte      `json:"stdoutTail,omitempty"`
	StderrTail      []byte      `json:"stderrTail,omitempty"`
	StdoutTruncated bool        `json:"stdoutTruncated,omitempty"`
	StderrTruncated bool        `json:"stderrTruncated,omitempty"`
	Cancelled       bool        `json:"cancelled,omitempty"`
	TimedOut        bool        `json:"timedOut,omitempty"`
	TimeoutClass    string      `json:"timeoutClass,omitempty"`
	Error           string      `json:"error,omitempty"`
}

type RuntimeState string

const (
	RuntimeUnknown  RuntimeState = "UNKNOWN"
	RuntimeStarting RuntimeState = "STARTING"
	RuntimeRunning  RuntimeState = "RUNNING"
	RuntimeFinished RuntimeState = "FINISHED"
)

type RuntimeStatus struct {
	ExecutionID ExecutionID  `json:"executionId"`
	State       RuntimeState `json:"state"`
	PID         int          `json:"pid,omitempty"`
	StartedAt   time.Time    `json:"startedAt,omitempty"`
}

type Capabilities struct {
	ProcessTree       bool `json:"processTree"`
	SoftCancel        bool `json:"softCancel"`
	GracefulTerminate bool `json:"gracefulTerminate"`
	HardKill          bool `json:"hardKill"`
	Streaming         bool `json:"streaming"`
	ReconcileLive     bool `json:"reconcileLive"`
}

type Executor interface {
	Prepare(context.Context, Request) (Prepared, error)
	Execute(context.Context, Prepared, LogSink) (Result, error)
	Cancel(context.Context, ExecutionID, CancelMode) error
	Reconcile(context.Context, ExecutionID) (RuntimeStatus, error)
	Capabilities() Capabilities
}

func (r Request) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("execution id is required")
	}
	if r.Command == "" {
		return fmt.Errorf("command is required")
	}
	if r.Timeouts.Start < 0 || r.Timeouts.Execution < 0 || r.Timeouts.Idle < 0 || r.GracePeriod < 0 {
		return fmt.Errorf("timeouts and grace period must be non-negative")
	}
	if r.OutputTailBytes < 0 || r.StreamQueue < 0 {
		return fmt.Errorf("output bounds must be non-negative")
	}
	return nil
}

type NopSink struct{}

func (NopSink) WriteChunk(context.Context, LogChunk) error { return nil }
