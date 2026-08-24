package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

func seedArtifactRun(t *testing.T, db *DB, now time.Time) harnessmodel.NodeRun {
	t.Helper()
	seedRun(t, db, now)
	node := harnessmodel.NodeRun{
		ID:            "nr_art_test",
		WorkflowRunID: "wfr_test",
		NodeID:        "a",
		GraphRevision: 1,
		Generation:    1,
		State:         harnessmodel.NodeRunning,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	err := db.Update(context.Background(), func(tx harnessstore.Tx) error {
		if err := tx.CreateGraphRevision(context.Background(), harnessmodel.GraphRevision{
			WorkflowRunID: "wfr_test", Number: 1, CreatedAt: now, Reason: "artifact test fixture",
		}); err != nil {
			return err
		}
		if err := tx.CreateWorkflowProgress(context.Background(), harnessmodel.WorkflowProgress{
			WorkflowRunID: "wfr_test", TotalNodes: 1, UpdatedAt: now,
		}); err != nil {
			return err
		}
		return tx.CreateNodeRun(context.Background(), node)
	})
	if err != nil {
		t.Fatal(err)
	}
	return node
}

func TestArtifactStoreCRUDAndProvenance(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Unix(140_000, 0).UTC()
	node := seedArtifactRun(t, db, now)

	digestA := harnessmodel.ComputeSHA256Digest([]byte("content A"))
	art1 := harnessmodel.ArtifactMetadata{
		ID:                "art_1",
		WorkflowRunID:     node.WorkflowRunID,
		ProducerNodeRunID: node.ID,
		ContentDigest:     digestA,
		Type:              harnessmodel.ArtifactOutput,
		Name:              "output.json",
		URI:               "cas://" + digestA,
		SizeBytes:         1024,
		CreatedAt:         now,
		Metadata:          map[string]string{"format": "json"},
	}

	digestB := harnessmodel.ComputeSHA256Digest([]byte("content B"))
	art2 := harnessmodel.ArtifactMetadata{
		ID:                "art_2",
		WorkflowRunID:     node.WorkflowRunID,
		ProducerNodeRunID: node.ID,
		ContentDigest:     digestB,
		Type:              harnessmodel.ArtifactLog,
		Name:              "process.log",
		URI:               "cas://" + digestB,
		SizeBytes:         4096,
		CreatedAt:         now.Add(time.Second),
	}

	// Another artifact with SAME content digest (deduplication scenario)
	art3 := harnessmodel.ArtifactMetadata{
		ID:                "art_3",
		WorkflowRunID:     node.WorkflowRunID,
		ProducerNodeRunID: node.ID,
		ContentDigest:     digestA,
		Type:              harnessmodel.ArtifactFile,
		Name:              "copy_of_output.json",
		URI:               "cas://" + digestA,
		SizeBytes:         1024,
		CreatedAt:         now.Add(2 * time.Second),
	}

	err := db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CreateArtifact(ctx, art1); err != nil {
			return err
		}
		if err := tx.CreateArtifact(ctx, art2); err != nil {
			return err
		}
		if err := tx.CreateArtifact(ctx, art3); err != nil {
			return err
		}
		if err := tx.RecordProvenance(ctx, harnessmodel.ProvenanceEdge{
			ArtifactID: art1.ID, NodeRunID: node.ID, Relation: harnessmodel.ProvenanceProducedBy, CreatedAt: now,
		}); err != nil {
			return err
		}
		return tx.RecordProvenance(ctx, harnessmodel.ProvenanceEdge{
			ArtifactID: art2.ID, NodeRunID: node.ID, Relation: harnessmodel.ProvenanceProducedBy, CreatedAt: now,
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify reads
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		got1, err := r.GetArtifact(ctx, art1.ID)
		if err != nil {
			return err
		}
		if got1.Name != art1.Name || got1.ContentDigest != digestA || got1.Metadata["format"] != "json" {
			t.Fatalf("unexpected art1: %+v", got1)
		}

		byRun, err := r.ListArtifactsByRun(ctx, node.WorkflowRunID, 10)
		if err != nil {
			return err
		}
		if len(byRun) != 3 {
			t.Fatalf("expected 3 artifacts for run, got %d", len(byRun))
		}

		byDigest, err := r.ListArtifactsByDigest(ctx, digestA)
		if err != nil {
			return err
		}
		if len(byDigest) != 2 {
			t.Fatalf("expected 2 artifacts with digestA, got %d", len(byDigest))
		}

		prov, err := r.ListArtifactProvenance(ctx, node.ID)
		if err != nil {
			return err
		}
		if len(prov) != 2 {
			t.Fatalf("expected 2 provenance edges, got %d", len(prov))
		}

		allDigests, err := r.ListAllArtifactDigests(ctx)
		if err != nil {
			return err
		}
		if len(allDigests) != 2 { // digestA and digestB
			t.Fatalf("expected 2 distinct digests, got %d", len(allDigests))
		}
		if _, ok := allDigests[digestA]; !ok {
			t.Fatal("missing digestA in all digests")
		}
		if _, ok := allDigests[digestB]; !ok {
			t.Fatal("missing digestB in all digests")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestArtifactStoreSurvivesDatabaseReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	now := time.Unix(141_000, 0).UTC()

	first, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	node := seedArtifactRun(t, first, now)
	digest := harnessmodel.ComputeSHA256Digest([]byte("reopen test"))
	art := harnessmodel.ArtifactMetadata{
		ID:            "art_reopen",
		WorkflowRunID: node.WorkflowRunID,
		ContentDigest: digest,
		Type:          harnessmodel.ArtifactCustom,
		Name:          "reopen.bin",
		URI:           "cas://" + digest,
		SizeBytes:     11,
		CreatedAt:     now,
	}

	if err := first.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.CreateArtifact(ctx, art)
	}); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	if err := second.View(ctx, func(r harnessstore.Reader) error {
		got, err := r.GetArtifact(ctx, art.ID)
		if err != nil {
			return err
		}
		if got.ID != art.ID || got.ContentDigest != digest {
			t.Fatalf("unexpected reopened artifact: %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
