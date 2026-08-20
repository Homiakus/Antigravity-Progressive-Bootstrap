package agy

import (
	"context"
	"fmt"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
)

type Runner struct {
	Process      harnessexecutor.Executor
	MaxLineBytes int
}

type RunResult struct {
	Process  harnessexecutor.Result `json:"process"`
	Protocol Outcome                `json:"protocol"`
}

func (r Runner) Run(ctx context.Context, request harnessexecutor.Request, rawLog harnessexecutor.LogSink) (RunResult, error) {
	if r.Process == nil {
		return RunResult{}, fmt.Errorf("process executor is required")
	}
	prepared, err := r.Process.Prepare(ctx, request)
	if err != nil {
		return RunResult{}, err
	}
	return r.ExecutePrepared(ctx, prepared, rawLog)
}

func (r Runner) ExecutePrepared(ctx context.Context, prepared harnessexecutor.Prepared, rawLog harnessexecutor.LogSink) (RunResult, error) {
	if r.Process == nil {
		return RunResult{}, fmt.Errorf("process executor is required")
	}
	parser := NewParser(r.MaxLineBytes)
	processResult, processErr := r.Process.Execute(ctx, prepared, ObservingSink{Parser: parser, Raw: rawLog})
	protocol := parser.Finalize()
	result := RunResult{Process: processResult, Protocol: protocol}
	if processErr != nil {
		return result, processErr
	}
	if err := parser.Validate(processResult.ExitCode); err != nil {
		return result, err
	}
	return result, nil
}
