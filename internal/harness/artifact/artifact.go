package artifact

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/harness/artifact/cas"
	"github.com/homiakus/agctl/internal/harness/artifact/provenance"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type Options struct {
	IDs harnessmodel.IDGenerator
	Now func() time.Time
}

type Store struct {
	cas        *cas.CAS
	db         harnessstore.Store
	provenance *provenance.Graph
	ids        harnessmodel.IDGenerator
	now        func() time.Time
}

func NewStore(casStorage *cas.CAS, db harnessstore.Store, opts Options) (*Store, error) {
	if casStorage == nil {
		return nil, fmt.Errorf("cas storage is required")
	}
	if db == nil {
		return nil, fmt.Errorf("harness store is required")
	}
	graph, err := provenance.NewGraph(db)
	if err != nil {
		return nil, err
	}
	ids := opts.IDs
	if ids == nil {
		g := harnessmodel.NewIDGenerator()
		ids = g
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Store{
		cas:        casStorage,
		db:         db,
		provenance: graph,
		ids:        ids,
		now:        now,
	}, nil
}

func (s *Store) CAS() *cas.CAS {
	return s.cas
}

func (s *Store) Provenance() *provenance.Graph {
	return s.provenance
}

type PutParams struct {
	WorkflowRunID     harnessmodel.WorkflowRunID
	ProducerNodeRunID harnessmodel.NodeRunID
	ProducerAttemptID harnessmodel.AttemptID
	Type              harnessmodel.ArtifactType
	Name              string
	Metadata          map[string]string
}

func (s *Store) Put(ctx context.Context, params PutParams, r io.Reader) (harnessmodel.ArtifactMetadata, error) {
	if params.WorkflowRunID == "" {
		return harnessmodel.ArtifactMetadata{}, fmt.Errorf("workflow run id is required")
	}
	if strings.TrimSpace(params.Name) == "" {
		return harnessmodel.ArtifactMetadata{}, fmt.Errorf("artifact name is required")
	}
	if !params.Type.Valid() {
		return harnessmodel.ArtifactMetadata{}, fmt.Errorf("invalid artifact type %q", params.Type)
	}

	digest, size, err := s.cas.Write(ctx, r)
	if err != nil {
		return harnessmodel.ArtifactMetadata{}, fmt.Errorf("write to cas: %w", err)
	}

	now := s.now().UTC()
	rawID, err := s.ids.New(harnessmodel.IDArtifact)
	if err != nil {
		return harnessmodel.ArtifactMetadata{}, err
	}

	art := harnessmodel.ArtifactMetadata{
		ID:                harnessmodel.ArtifactID(rawID),
		WorkflowRunID:     params.WorkflowRunID,
		ProducerNodeRunID: params.ProducerNodeRunID,
		ProducerAttemptID: params.ProducerAttemptID,
		ContentDigest:     digest,
		Type:              params.Type,
		Name:              params.Name,
		URI:               "cas://" + digest,
		SizeBytes:         size,
		CreatedAt:         now,
		Metadata:          params.Metadata,
	}

	err = s.db.Update(ctx, func(tx harnessstore.Tx) error {
		if err := tx.CreateArtifact(ctx, art); err != nil {
			return err
		}
		if params.ProducerNodeRunID != "" {
			if err := tx.RecordProvenance(ctx, harnessmodel.ProvenanceEdge{
				ArtifactID: art.ID,
				NodeRunID:  params.ProducerNodeRunID,
				Relation:   harnessmodel.ProvenanceProducedBy,
				CreatedAt:  now,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return harnessmodel.ArtifactMetadata{}, fmt.Errorf("commit artifact metadata: %w", err)
	}
	return art, nil
}

func (s *Store) PutBytes(ctx context.Context, params PutParams, data []byte) (harnessmodel.ArtifactMetadata, error) {
	return s.Put(ctx, params, strings.NewReader(string(data)))
}

func (s *Store) Get(ctx context.Context, id harnessmodel.ArtifactID) (harnessmodel.ArtifactMetadata, io.ReadCloser, error) {
	var meta harnessmodel.ArtifactMetadata
	err := s.db.View(ctx, func(r harnessstore.Reader) error {
		var readErr error
		meta, readErr = r.GetArtifact(ctx, id)
		return readErr
	})
	if err != nil {
		return harnessmodel.ArtifactMetadata{}, nil, err
	}

	rc, _, err := s.cas.Open(meta.ContentDigest)
	if err != nil {
		return meta, nil, fmt.Errorf("open cas file: %w", err)
	}
	return meta, rc, nil
}

func (s *Store) GetContent(ctx context.Context, id harnessmodel.ArtifactID) (harnessmodel.ArtifactMetadata, []byte, error) {
	meta, rc, err := s.Get(ctx, id)
	if err != nil {
		return harnessmodel.ArtifactMetadata{}, nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return meta, nil, fmt.Errorf("read artifact content: %w", err)
	}
	return meta, data, nil
}

func (s *Store) ListByRun(ctx context.Context, runID harnessmodel.WorkflowRunID, limit int) ([]harnessmodel.ArtifactMetadata, error) {
	var list []harnessmodel.ArtifactMetadata
	err := s.db.View(ctx, func(r harnessstore.Reader) error {
		var readErr error
		list, readErr = r.ListArtifactsByRun(ctx, runID, limit)
		return readErr
	})
	return list, err
}

func (s *Store) RecordConsumed(ctx context.Context, artifactID harnessmodel.ArtifactID, nodeRunID harnessmodel.NodeRunID) error {
	now := s.now().UTC()
	return s.db.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.RecordProvenance(ctx, harnessmodel.ProvenanceEdge{
			ArtifactID: artifactID,
			NodeRunID:  nodeRunID,
			Relation:   harnessmodel.ProvenanceConsumedBy,
			CreatedAt:  now,
		})
	})
}

func (s *Store) GC(ctx context.Context, gracePeriod time.Duration) (reclaimedBytes int64, removedCount int, err error) {
	var reachable map[string]struct{}
	err = s.db.View(ctx, func(r harnessstore.Reader) error {
		var readErr error
		reachable, readErr = r.ListAllArtifactDigests(ctx)
		return readErr
	})
	if err != nil {
		return 0, 0, fmt.Errorf("list reachable digests: %w", err)
	}

	return s.cas.GC(ctx, reachable, gracePeriod)
}
