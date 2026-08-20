package process

import (
	"context"
	"sync/atomic"
	"time"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
)

type streamWriter struct {
	ctx          context.Context
	stream       harnessexecutor.Stream
	chunks       chan<- harnessexecutor.LogChunk
	tail         *ringTail
	count        *atomic.Int64
	lastActivity *atomic.Int64
	now          func() time.Time
}

func (w *streamWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	at := w.now().UTC()
	copyChunk := append([]byte(nil), p...)
	w.count.Add(int64(len(copyChunk)))
	w.lastActivity.Store(at.UnixNano())
	w.tail.Write(copyChunk)
	select {
	case w.chunks <- harnessexecutor.LogChunk{At: at, Stream: w.stream, Data: copyChunk}:
	case <-w.ctx.Done():
		// Once a sink fails/cancels, continue draining the OS pipe while dropping
		// downstream chunks. This protects the child from deadlocking on output.
	}
	return len(p), nil
}
