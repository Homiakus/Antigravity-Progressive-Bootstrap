package agy

import (
	"context"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
)

// ObservingSink keeps protocol interpretation separate from raw persistence.
// Every chunk is first observed by the bounded parser, then forwarded unchanged
// to Raw. A raw sink failure remains an execution error; parsing never replaces
// the authoritative raw log stream.
type ObservingSink struct {
	Parser *Parser
	Raw    harnessexecutor.LogSink
}

func (s ObservingSink) WriteChunk(ctx context.Context, chunk harnessexecutor.LogChunk) error {
	if s.Parser != nil {
		s.Parser.Feed(chunk.Stream, chunk.Data)
	}
	if s.Raw != nil {
		return s.Raw.WriteChunk(ctx, chunk)
	}
	return nil
}
