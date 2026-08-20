package agy

import (
	"errors"
	"strings"
	"testing"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
)

func TestParserHandlesResultSplitAcrossChunks(t *testing.T) {
	p := NewParser(1024)
	p.Feed(harnessexecutor.StreamStdout, []byte(`{"event":"res`))
	p.Feed(harnessexecutor.StreamStdout, []byte(`ult","result":{"status":"SUCCESS","error":""}}`+"\n"))
	if err := p.Validate(0); err != nil {
		t.Fatalf("split successful result rejected: %v", err)
	}
	out := p.Finalize()
	if !out.SawResult || out.ResultStatus != "SUCCESS" || out.MalformedLines != 0 {
		t.Fatalf("unexpected parsed outcome: %+v", out)
	}
}

func TestParserTreatsSoftPermissionDenialAsFailureEvenWithExitZero(t *testing.T) {
	p := NewParser(4096)
	p.Feed(harnessexecutor.StreamStdout, []byte(`{"event":"step_update","step_update":{"step_type":"tool","tool_info":{"error":{"type":"permission","message":"soft-denied: approval required"}}}}`+"\n"))
	p.Feed(harnessexecutor.StreamStdout, []byte(`{"event":"result","result":{"status":"SUCCESS"}}`+"\n"))
	if err := p.Validate(0); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("permission denial error=%v want ErrPermissionDenied", err)
	}
}

func TestParserRequiresTerminalResult(t *testing.T) {
	p := NewParser(1024)
	p.Feed(harnessexecutor.StreamStdout, []byte(`{"event":"step_update","step_update":{"step_type":"text"}}`+"\n"))
	if err := p.Validate(0); !errors.Is(err, ErrMissingResult) {
		t.Fatalf("missing terminal result error=%v want ErrMissingResult", err)
	}
}

func TestParserReportsNonSuccessResult(t *testing.T) {
	p := NewParser(1024)
	p.Feed(harnessexecutor.StreamStdout, []byte(`{"event":"result","result":{"status":"FAILED","error":"tests failed"}}`+"\n"))
	err := p.Validate(0)
	if !errors.Is(err, ErrResultFailed) || !strings.Contains(err.Error(), "tests failed") {
		t.Fatalf("failure result error=%v", err)
	}
}

func TestMalformedAndOversizedLinesAreBoundedAndRecoverable(t *testing.T) {
	p := NewParser(64)
	p.Feed(harnessexecutor.StreamStdout, []byte("not-json\n"))
	p.Feed(harnessexecutor.StreamStdout, []byte(strings.Repeat("x", 128)))
	p.Feed(harnessexecutor.StreamStdout, []byte("still-discarded\n"))
	p.Feed(harnessexecutor.StreamStdout, []byte(`{"event":"result","result":{"status":"SUCCESS"}}`+"\n"))
	out := p.Finalize()
	if out.MalformedLines != 1 || out.OversizedLines != 1 || !out.SawResult {
		t.Fatalf("parser failed to recover after malformed/oversized line: %+v", out)
	}
	if err := p.Validate(0); err != nil {
		t.Fatalf("valid result after parser recovery rejected: %v", err)
	}
}

func TestStderrPermissionTextIsObservedWithoutJSONParsing(t *testing.T) {
	p := NewParser(1024)
	p.Feed(harnessexecutor.StreamStderr, []byte("tool permission denied; approval required\n"))
	p.Feed(harnessexecutor.StreamStdout, []byte(`{"event":"result","result":{"status":"SUCCESS"}}`+"\n"))
	out := p.Finalize()
	if !out.PermissionDenied || out.MalformedLines != 0 {
		t.Fatalf("unexpected stderr outcome: %+v", out)
	}
}

func TestNonZeroProcessExitTakesPrecedence(t *testing.T) {
	p := NewParser(1024)
	p.Feed(harnessexecutor.StreamStdout, []byte(`{"event":"result","result":{"status":"SUCCESS"}}`+"\n"))
	if err := p.Validate(7); err == nil || !strings.Contains(err.Error(), "code 7") {
		t.Fatalf("non-zero exit unexpectedly accepted: %v", err)
	}
}
