package artifact

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

type LogSinkOptions struct {
	MaxTailBytes int
}

type ArtifactLogSink struct {
	store           *Store
	runID           harnessmodel.WorkflowRunID
	nodeRunID       harnessmodel.NodeRunID
	attemptID       harnessmodel.AttemptID
	name            string
	maxTailBytes    int
	mu              sync.Mutex
	buf             bytes.Buffer
	tailBuf         []byte
	createdArtifact *harnessmodel.ArtifactMetadata
}

func NewArtifactLogSink(store *Store, runID harnessmodel.WorkflowRunID, nodeRunID harnessmodel.NodeRunID, attemptID harnessmodel.AttemptID, name string, opts LogSinkOptions) *ArtifactLogSink {
	maxTail := opts.MaxTailBytes
	if maxTail <= 0 {
		maxTail = 8 * 1024 // 8 KB default bounded tail
	}
	return &ArtifactLogSink{
		store:        store,
		runID:        runID,
		nodeRunID:    nodeRunID,
		attemptID:    attemptID,
		name:         name,
		maxTailBytes: maxTail,
	}
}

func (s *ArtifactLogSink) WriteChunk(ctx context.Context, chunk harnessexecutor.LogChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.Write(chunk.Data)
	s.buf.WriteByte('\n')

	// Update bounded tail
	s.tailBuf = append(s.tailBuf, chunk.Data...)
	s.tailBuf = append(s.tailBuf, '\n')
	if len(s.tailBuf) > s.maxTailBytes {
		s.tailBuf = s.tailBuf[len(s.tailBuf)-s.maxTailBytes:]
	}
	return nil
}

func (s *ArtifactLogSink) Flush() error {
	return nil
}

func (s *ArtifactLogSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createdArtifact != nil {
		return nil
	}
	art, err := s.store.Put(context.Background(), PutParams{
		WorkflowRunID:     s.runID,
		ProducerNodeRunID: s.nodeRunID,
		ProducerAttemptID: s.attemptID,
		Type:              harnessmodel.ArtifactLog,
		Name:              s.name,
	}, bytes.NewReader(s.buf.Bytes()))
	if err != nil {
		return fmt.Errorf("commit log artifact: %w", err)
	}
	s.createdArtifact = &art
	return nil
}

func (s *ArtifactLogSink) TailSummary() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return string(s.tailBuf)
}

func (s *ArtifactLogSink) Artifact() *harnessmodel.ArtifactMetadata {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createdArtifact
}
