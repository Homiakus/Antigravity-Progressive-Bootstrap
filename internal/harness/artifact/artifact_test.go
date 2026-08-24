package artifact

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/harness/artifact/cas"
	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

func setupTestArtifactStore(t *testing.T) (*Store, *sqlitestore.DB, harnessmodel.NodeRun, harnessmodel.NodeRun) {
	t.Helper()
	ctx := context.Background()
	tempDir := t.TempDir()

	db, err := sqlitestore.Open(ctx, filepath.Join(tempDir, "state.db"), sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	casStorage, err := cas.New(filepath.Join(tempDir, "cas"))
	if err != nil {
		t.Fatal(err)
	}

	now := time.Unix(150_000, 0).UTC()
	var producerNode, consumerNode harnessmodel.NodeRun

	err = db.Update(ctx, func(tx harnessstore.Tx) error {
		def := harnessmodel.WorkflowDefinition{
			ID: "wfd_art", Version: 1, Name: "art-wf", CreatedAt: now, CompilerVersion: "test",
			Nodes: []harnessmodel.NodeSpec{
				{ID: "p", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess},
				{ID: "c", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess},
			},
		}
		if err := tx.CreateWorkflowDefinition(ctx, def); err != nil {
			return err
		}
		run := harnessmodel.WorkflowRun{
			ID: "wfr_art", DefinitionID: def.ID, DefinitionVersion: 1, State: harnessmodel.WorkflowRunning,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.CreateWorkflowRun(ctx, run); err != nil {
			return err
		}
		if err := tx.CreateGraphRevision(ctx, harnessmodel.GraphRevision{
			WorkflowRunID: run.ID, Number: 1, CreatedAt: now, Reason: "test",
		}); err != nil {
			return err
		}
		if err := tx.CreateWorkflowProgress(ctx, harnessmodel.WorkflowProgress{
			WorkflowRunID: run.ID, TotalNodes: 2, UpdatedAt: now,
		}); err != nil {
			return err
		}
		producerNode = harnessmodel.NodeRun{
			ID: "nr_producer", WorkflowRunID: run.ID,
			NodeID: "p", GraphRevision: 1, Generation: 1, State: harnessmodel.NodeRunning, CreatedAt: now, UpdatedAt: now,
		}
		consumerNode = harnessmodel.NodeRun{
			ID: "nr_consumer", WorkflowRunID: run.ID,
			NodeID: "c", GraphRevision: 1, Generation: 1, State: harnessmodel.NodeRunning, CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.CreateNodeRun(ctx, producerNode); err != nil {
			return err
		}
		return tx.CreateNodeRun(ctx, consumerNode)
	})
	if err != nil {
		t.Fatal(err)
	}

	artStore, err := NewStore(casStorage, db, Options{
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	return artStore, db, producerNode, consumerNode
}

func TestArtifactStoreLifecycleAndProvenance(t *testing.T) {
	ctx := context.Background()
	store, _, producer, consumer := setupTestArtifactStore(t)

	content := []byte(`{"test_result":"passed","duration_ms":150}`)
	meta, err := store.PutBytes(ctx, PutParams{
		WorkflowRunID:     producer.WorkflowRunID,
		ProducerNodeRunID: producer.ID,
		Type:              harnessmodel.ArtifactJSON,
		Name:              "test_report.json",
		Metadata:          map[string]string{"env": "ci"},
	}, content)
	if err != nil {
		t.Fatal(err)
	}

	if meta.ID == "" || meta.ContentDigest == "" || meta.SizeBytes != int64(len(content)) {
		t.Fatalf("unexpected created artifact: %+v", meta)
	}

	// Verify reading content back
	gotMeta, gotBytes, err := store.GetContent(ctx, meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotMeta.Name != "test_report.json" || string(gotBytes) != string(content) {
		t.Fatalf("content mismatch: %s vs %s", string(gotBytes), string(content))
	}

	// Consumer node consumes this artifact
	if err := store.RecordConsumed(ctx, meta.ID, consumer.ID); err != nil {
		t.Fatal(err)
	}

	// Query provenance for producer
	pArtifacts, err := store.Provenance().GetNodeArtifacts(ctx, producer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pArtifacts.Produced) != 1 || pArtifacts.Produced[0].ID != meta.ID {
		t.Fatalf("unexpected produced artifacts: %+v", pArtifacts)
	}
	if len(pArtifacts.Consumed) != 0 {
		t.Fatalf("producer should have 0 consumed artifacts, got %d", len(pArtifacts.Consumed))
	}

	// Query provenance for consumer
	cArtifacts, err := store.Provenance().GetNodeArtifacts(ctx, consumer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cArtifacts.Consumed) != 1 || cArtifacts.Consumed[0].ID != meta.ID {
		t.Fatalf("unexpected consumed artifacts: %+v", cArtifacts)
	}
}

func TestArtifactStoreEndToEndGC(t *testing.T) {
	ctx := context.Background()
	store, db, producer, _ := setupTestArtifactStore(t)

	// Put artifact 1 (will be retained)
	art1, err := store.PutBytes(ctx, PutParams{
		WorkflowRunID:     producer.WorkflowRunID,
		ProducerNodeRunID: producer.ID,
		Type:              harnessmodel.ArtifactOutput,
		Name:              "retained.txt",
	}, []byte("retained content"))
	if err != nil {
		t.Fatal(err)
	}

	// Put artifact 2 (will be deleted from DB to simulate orphan)
	art2, err := store.PutBytes(ctx, PutParams{
		WorkflowRunID:     producer.WorkflowRunID,
		ProducerNodeRunID: producer.ID,
		Type:              harnessmodel.ArtifactOutput,
		Name:              "orphan.txt",
	}, []byte("orphan content to be collected"))
	if err != nil {
		t.Fatal(err)
	}

	// Delete art2 record from DB
	err = db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.DeleteArtifact(ctx, art2.ID)
	})
	if err != nil {
		t.Fatal(err)
	}

	// Run GC with zero grace period
	reclaimed, count, err := store.GC(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || reclaimed == 0 {
		t.Fatalf("expected 1 orphan collected, got count=%d bytes=%d", count, reclaimed)
	}

	// art1 content must still exist in CAS
	if !store.CAS().Exists(art1.ContentDigest) {
		t.Fatal("retained artifact was deleted from CAS")
	}
	// art2 content must no longer exist in CAS
	if store.CAS().Exists(art2.ContentDigest) {
		t.Fatal("orphan artifact still exists in CAS")
	}
}

func TestArtifactLogSinkStreamingAndTail(t *testing.T) {
	ctx := context.Background()
	store, _, producer, _ := setupTestArtifactStore(t)

	sink := NewArtifactLogSink(store, producer.WorkflowRunID, producer.ID, "att_1", "process.log", LogSinkOptions{
		MaxTailBytes: 30,
	})

	// Stream some chunks
	if err := sink.WriteChunk(ctx, harnessexecutor.LogChunk{At: time.Now(), Stream: harnessexecutor.StreamStdout, Data: []byte("line 1: starting worker")}); err != nil {
		t.Fatal(err)
	}
	if err := sink.WriteChunk(ctx, harnessexecutor.LogChunk{At: time.Now(), Stream: harnessexecutor.StreamStdout, Data: []byte("line 2: long output message")}); err != nil {
		t.Fatal(err)
	}
	if err := sink.WriteChunk(ctx, harnessexecutor.LogChunk{At: time.Now(), Stream: harnessexecutor.StreamStdout, Data: []byte("line 3: done")}); err != nil {
		t.Fatal(err)
	}

	// Close sink to commit artifact
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}

	art := sink.Artifact()
	if art == nil {
		t.Fatal("expected artifact to be created")
	}
	if art.Type != harnessmodel.ArtifactLog || art.Name != "process.log" {
		t.Fatalf("unexpected log artifact metadata: %+v", art)
	}

	// Verify content from store
	_, data, err := store.GetContent(ctx, art.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("log content was empty")
	}

	// Bounded tail must be <= 30 bytes
	tail := sink.TailSummary()
	if len(tail) > 30 {
		t.Fatalf("tail summary exceeds 30 bytes: %d (%q)", len(tail), tail)
	}
}
