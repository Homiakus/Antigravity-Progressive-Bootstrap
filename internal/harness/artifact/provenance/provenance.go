package provenance

import (
	"context"
	"fmt"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

// Graph provides lineage and provenance queries across artifacts and node runs.
type Graph struct {
	store harnessstore.Store
}

func NewGraph(store harnessstore.Store) (*Graph, error) {
	if store == nil {
		return nil, fmt.Errorf("harness store is required")
	}
	return &Graph{store: store}, nil
}

type NodeArtifacts struct {
	NodeRunID harnessmodel.NodeRunID          `json:"nodeRunId"`
	Produced  []harnessmodel.ArtifactMetadata `json:"produced"`
	Consumed  []harnessmodel.ArtifactMetadata `json:"consumed"`
}

func (g *Graph) GetNodeArtifacts(ctx context.Context, nodeRunID harnessmodel.NodeRunID) (NodeArtifacts, error) {
	if nodeRunID == "" {
		return NodeArtifacts{}, fmt.Errorf("node run id is required")
	}
	var res NodeArtifacts
	res.NodeRunID = nodeRunID

	err := g.store.View(ctx, func(r harnessstore.Reader) error {
		edges, err := r.ListArtifactProvenance(ctx, nodeRunID)
		if err != nil {
			return err
		}
		for _, edge := range edges {
			art, err := r.GetArtifact(ctx, edge.ArtifactID)
			if err != nil {
				return err
			}
			if edge.Relation == harnessmodel.ProvenanceProducedBy {
				res.Produced = append(res.Produced, art)
			} else if edge.Relation == harnessmodel.ProvenanceConsumedBy {
				res.Consumed = append(res.Consumed, art)
			}
		}
		return nil
	})
	return res, err
}
