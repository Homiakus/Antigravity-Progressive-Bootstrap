package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

const artifactSelect = `
SELECT artifact_id, workflow_run_id, producer_node_run_id, producer_attempt_id,
       content_digest, artifact_type, name, uri, size_bytes, created_at, metadata_json
FROM artifacts`

func (t *transaction) CreateArtifact(ctx context.Context, art harnessmodel.ArtifactMetadata) error {
	if err := art.Validate(); err != nil {
		return err
	}
	metaJSON, err := json.Marshal(art.Metadata)
	if err != nil {
		return fmt.Errorf("marshal artifact metadata: %w", err)
	}

	_, err = t.tx.ExecContext(ctx, `
INSERT INTO artifacts(
    artifact_id, workflow_run_id, producer_node_run_id, producer_attempt_id,
    content_digest, artifact_type, name, uri, size_bytes, created_at, metadata_json
) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		string(art.ID), string(art.WorkflowRunID), string(art.ProducerNodeRunID), string(art.ProducerAttemptID),
		art.ContentDigest, string(art.Type), art.Name, art.URI, art.SizeBytes, formatTime(art.CreatedAt), string(metaJSON))
	if err != nil {
		return fmt.Errorf("create artifact %s: %w", art.ID, err)
	}
	return nil
}

func (t *transaction) GetArtifact(ctx context.Context, id harnessmodel.ArtifactID) (harnessmodel.ArtifactMetadata, error) {
	if id == "" {
		return harnessmodel.ArtifactMetadata{}, fmt.Errorf("artifact id is required")
	}
	return scanArtifact(t.tx.QueryRowContext(ctx, artifactSelect+` WHERE artifact_id=?`, string(id)))
}

func (t *transaction) ListArtifactsByRun(ctx context.Context, runID harnessmodel.WorkflowRunID, limit int) ([]harnessmodel.ArtifactMetadata, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	query := artifactSelect + ` WHERE workflow_run_id=? ORDER BY created_at, artifact_id LIMIT ?`
	rows, err := t.tx.QueryContext(ctx, query, string(runID), limit)
	if err != nil {
		return nil, fmt.Errorf("list artifacts by run %s: %w", runID, err)
	}
	defer rows.Close()

	var out []harnessmodel.ArtifactMetadata
	for rows.Next() {
		art, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, art)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate artifacts: %w", err)
	}
	return out, nil
}

func (t *transaction) ListArtifactsByDigest(ctx context.Context, digest string) ([]harnessmodel.ArtifactMetadata, error) {
	if strings.TrimSpace(digest) == "" {
		return nil, fmt.Errorf("content digest is required")
	}
	rows, err := t.tx.QueryContext(ctx, artifactSelect+` WHERE content_digest=? ORDER BY created_at, artifact_id`, digest)
	if err != nil {
		return nil, fmt.Errorf("list artifacts by digest %s: %w", digest, err)
	}
	defer rows.Close()

	var out []harnessmodel.ArtifactMetadata
	for rows.Next() {
		art, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, art)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate artifacts by digest: %w", err)
	}
	return out, nil
}

func (t *transaction) DeleteArtifact(ctx context.Context, id harnessmodel.ArtifactID) error {
	if id == "" {
		return fmt.Errorf("artifact id is required")
	}
	res, err := t.tx.ExecContext(ctx, `DELETE FROM artifacts WHERE artifact_id=?`, string(id))
	if err != nil {
		return fmt.Errorf("delete artifact %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return harnessstore.ErrNotFound
	}
	return nil
}

func (t *transaction) RecordProvenance(ctx context.Context, edge harnessmodel.ProvenanceEdge) error {
	if err := edge.Validate(); err != nil {
		return err
	}
	_, err := t.tx.ExecContext(ctx, `
INSERT INTO artifact_provenance(artifact_id, node_run_id, relation, created_at)
VALUES(?,?,?,?) ON CONFLICT(artifact_id, node_run_id, relation) DO NOTHING`,
		string(edge.ArtifactID), string(edge.NodeRunID), string(edge.Relation), formatTime(edge.CreatedAt))
	if err != nil {
		return fmt.Errorf("record provenance edge: %w", err)
	}
	return nil
}

func (t *transaction) ListArtifactProvenance(ctx context.Context, nodeRunID harnessmodel.NodeRunID) ([]harnessmodel.ProvenanceEdge, error) {
	if nodeRunID == "" {
		return nil, fmt.Errorf("node run id is required")
	}
	rows, err := t.tx.QueryContext(ctx, `
SELECT artifact_id, node_run_id, relation, created_at
FROM artifact_provenance
WHERE node_run_id=?
ORDER BY created_at, artifact_id`, string(nodeRunID))
	if err != nil {
		return nil, fmt.Errorf("list artifact provenance: %w", err)
	}
	defer rows.Close()

	var out []harnessmodel.ProvenanceEdge
	for rows.Next() {
		var artID, nrID, rel, createdAt string
		if err := rows.Scan(&artID, &nrID, &rel, &createdAt); err != nil {
			return nil, err
		}
		tVal, err := parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		out = append(out, harnessmodel.ProvenanceEdge{
			ArtifactID: harnessmodel.ArtifactID(artID),
			NodeRunID:  harnessmodel.NodeRunID(nrID),
			Relation:   harnessmodel.ProvenanceRelation(rel),
			CreatedAt:  tVal,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate artifact provenance: %w", err)
	}
	return out, nil
}

func (t *transaction) ListAllArtifactDigests(ctx context.Context) (map[string]struct{}, error) {
	rows, err := t.tx.QueryContext(ctx, `SELECT DISTINCT content_digest FROM artifacts`)
	if err != nil {
		return nil, fmt.Errorf("list all artifact digests: %w", err)
	}
	defer rows.Close()

	out := make(map[string]struct{})
	for rows.Next() {
		var digest string
		if err := rows.Scan(&digest); err != nil {
			return nil, err
		}
		out[digest] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate artifact digests: %w", err)
	}
	return out, nil
}

func scanArtifact(row interface{ Scan(...any) error }) (harnessmodel.ArtifactMetadata, error) {
	var id, wfrID, producerNodeID, producerAttemptID, digest, artType, name, uri, createdAt, metaJSON string
	var sizeBytes int64
	if err := row.Scan(&id, &wfrID, &producerNodeID, &producerAttemptID, &digest, &artType, &name, &uri, &sizeBytes, &createdAt, &metaJSON); err != nil {
		return harnessmodel.ArtifactMetadata{}, mapNotFound(err)
	}
	tVal, err := parseTime(createdAt)
	if err != nil {
		return harnessmodel.ArtifactMetadata{}, fmt.Errorf("parse artifact created_at: %w", err)
	}
	var meta map[string]string
	if metaJSON != "" && metaJSON != "{}" {
		_ = json.Unmarshal([]byte(metaJSON), &meta)
	}
	return harnessmodel.ArtifactMetadata{
		ID:                harnessmodel.ArtifactID(id),
		WorkflowRunID:     harnessmodel.WorkflowRunID(wfrID),
		ProducerNodeRunID: harnessmodel.NodeRunID(producerNodeID),
		ProducerAttemptID: harnessmodel.AttemptID(producerAttemptID),
		ContentDigest:     digest,
		Type:              harnessmodel.ArtifactType(artType),
		Name:              name,
		URI:               uri,
		SizeBytes:         sizeBytes,
		CreatedAt:         tVal,
		Metadata:          meta,
	}, nil
}
