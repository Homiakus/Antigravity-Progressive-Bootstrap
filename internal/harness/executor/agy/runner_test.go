package agy

import (
	"context"
	"errors"
	"testing"
	"time"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
)

type fakeProcessExecutor struct {
	chunks []harnessexecutor.LogChunk
	result harnessexecutor.Result
	err    error
}

func (f fakeProcessExecutor) Prepare(_ context.Context, req harnessexecutor.Request) (harnessexecutor.Prepared, error) {
	return harnessexecutor.Prepared{Request: req, ResolvedPath: req.Command}, nil
}

func (f fakeProcessExecutor) Execute(ctx context.Context, prepared harnessexecutor.Prepared, sink harnessexecutor.LogSink) (harnessexecutor.Result, error) {
	for _, chunk := range f.chunks {
		if sink != nil {
			if err := sink.WriteChunk(ctx, chunk); err != nil {
				return f.result, err
			}
		}
	}
	return f.result, f.err
}

func (fakeProcessExecutor) Cancel(context.Context, harnessexecutor.ExecutionID, harnessexecutor.CancelMode) error {
	return nil
}

func (fakeProcessExecutor) Reconcile(context.Context, harnessexecutor.ExecutionID) (harnessexecutor.RuntimeStatus, error) {
	return harnessexecutor.RuntimeStatus{}, nil
}

func (fakeProcessExecutor) Capabilities() harnessexecutor.Capabilities { return harnessexecutor.Capabilities{} }

type countSink struct{ chunks int }

func (s *countSink) WriteChunk(context.Context, harnessexecutor.LogChunk) error {
	s.chunks++
	return nil
}

func TestRunnerRequiresProtocolSuccessNotOnlyExitZero(t *testing.T) {
	now := time.Unix(1, 0).UTC()
	tests := []struct {
		name    string
		lines   []string
		wantErr error
	}{
		{name: "success", lines: []string{`{"event":"result","result":{"status":"SUCCESS"}}`}},
		{name: "missing result", lines: []string{`{"event":"step_update","step_update":{"step_type":"text"}}`}, wantErr: ErrMissingResult},
		{name: "permission denial", lines: []string{`{"event":"step_update","step_update":{"step_type":"tool","tool_info":{"error":{"type":"permission","message":"approval required"}}}}`, `{"event":"result","result":{"status":"SUCCESS"}}`}, wantErr: ErrPermissionDenied},
		{name: "failed result", lines: []string{`{"event":"result","result":{"status":"FAILED","error":"validation failed"}}`}, wantErr: ErrResultFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := make([]harnessexecutor.LogChunk, 0, len(tt.lines))
			for _, line := range tt.lines {
				chunks = append(chunks, harnessexecutor.LogChunk{At: now, Stream: harnessexecutor.StreamStdout, Data: []byte(line + "\n")})
			}
			fake := fakeProcessExecutor{chunks: chunks, result: harnessexecutor.Result{ExecutionID: "agy-test", ExitCode: 0}}
			raw := &countSink{}
			result, err := (Runner{Process: fake}).Run(context.Background(), harnessexecutor.Request{ID: "agy-test", Command: "agy"}, raw)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("success rejected: %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("error=%v want=%v", err, tt.wantErr)
			}
			if raw.chunks != len(chunks) {
				t.Fatalf("raw log chunks=%d want=%d", raw.chunks, len(chunks))
			}
			if result.Protocol.SawResult != (tt.name != "missing result") {
				t.Fatalf("unexpected protocol result: %+v", result.Protocol)
			}
		})
	}
}

func TestRunnerPreservesProcessFailureWithoutInventingProtocolSuccess(t *testing.T) {
	processErr := errors.New("process crashed")
	fake := fakeProcessExecutor{
		result: harnessexecutor.Result{ExecutionID: "agy-crash", ExitCode: 137},
		err:    processErr,
	}
	result, err := (Runner{Process: fake}).Run(context.Background(), harnessexecutor.Request{ID: "agy-crash", Command: "agy"}, nil)
	if !errors.Is(err, processErr) {
		t.Fatalf("process error=%v want %v", err, processErr)
	}
	if result.Protocol.SawResult {
		t.Fatalf("protocol success invented after process failure: %+v", result.Protocol)
	}
}
