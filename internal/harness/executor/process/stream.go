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
	activityBase time.Time
	now          func() time.Time
}

func (w *streamWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	at := w.now().UTC()
	copyChunk := append([]byte(nil), p...)
	w.count.Add(int64(len(copyChunk)))
	w.lastActivity.Store(time.Since(w.activityBase).Nanoseconds())
	w.tail.Write(copyChunk)
	if w.ctx.Err() != nil {
		// The sink has already failed/cancelled. Keep accepting child output so
		// os/exec can drain its pipe, but do not enqueue any more downstream data.
		return len(p), nil
	}
	select {
	case w.chunks <- harnessexecutor.LogChunk{At: at, Stream: w.stream, Data: copyChunk}:
	case <-w.ctx.Done():
		// Sink failure raced this send. Dropping the chunk is preferable to
		// blocking the child process on an unavailable log consumer.
	}
	return len(p), nil
}
