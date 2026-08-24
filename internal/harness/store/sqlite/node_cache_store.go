package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

const nodeCacheSelect = `
SELECT cache_key, workflow_run_id, node_run_id, attempt_id,
       output_artifacts_json, result_payload_json, created_at, last_hit_at, hit_count
FROM node_cache_entries`

func (t *transaction) PutNodeCacheEntry(ctx context.Context, entry harnessmodel.NodeCacheEntry) error {
	if err := entry.Validate(); err != nil {
		return err
	}
	artifactsJSON, err := json.Marshal(entry.OutputArtifacts)
	if err != nil {
		return fmt.Errorf("marshal output artifacts: %w", err)
	}

	lastHit := entry.LastHitAt
	if lastHit.IsZero() {
		lastHit = entry.CreatedAt
	}

	_, err = t.tx.ExecContext(ctx, `
INSERT INTO node_cache_entries(
    cache_key, workflow_run_id, node_run_id, attempt_id,
    output_artifacts_json, result_payload_json, created_at, last_hit_at, hit_count
) VALUES(?,?,?,?,?,?,?,?,?)
ON CONFLICT(cache_key) DO UPDATE SET
    workflow_run_id=excluded.workflow_run_id,
    node_run_id=excluded.node_run_id,
    attempt_id=excluded.attempt_id,
    output_artifacts_json=excluded.output_artifacts_json,
    result_payload_json=excluded.result_payload_json,
    created_at=excluded.created_at,
    last_hit_at=excluded.last_hit_at,
    hit_count=excluded.hit_count`,
		entry.CacheKey, string(entry.WorkflowRunID), string(entry.NodeRunID), string(entry.AttemptID),
		string(artifactsJSON), entry.ResultPayload, formatTime(entry.CreatedAt), formatTime(lastHit), entry.HitCount)
	if err != nil {
		return fmt.Errorf("put node cache entry %s: %w", entry.CacheKey, err)
	}
	return nil
}

func (t *transaction) TouchNodeCacheHit(ctx context.Context, key string, now time.Time) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("cache key is required")
	}
	res, err := t.tx.ExecContext(ctx, `
UPDATE node_cache_entries
SET last_hit_at=?, hit_count=hit_count+1
WHERE cache_key=?`, formatTime(now), key)
	if err != nil {
		return fmt.Errorf("touch node cache hit %s: %w", key, err)
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

func (t *transaction) GetNodeCacheEntry(ctx context.Context, key string) (harnessmodel.NodeCacheEntry, error) {
	if strings.TrimSpace(key) == "" {
		return harnessmodel.NodeCacheEntry{}, fmt.Errorf("cache key is required")
	}
	return scanNodeCache(t.tx.QueryRowContext(ctx, nodeCacheSelect+` WHERE cache_key=?`, key))
}

func (t *transaction) ListNodeCacheEntriesByRun(ctx context.Context, runID harnessmodel.WorkflowRunID) ([]harnessmodel.NodeCacheEntry, error) {
	if runID == "" {
		return nil, fmt.Errorf("workflow run id is required")
	}
	rows, err := t.tx.QueryContext(ctx, nodeCacheSelect+` WHERE workflow_run_id=? ORDER BY created_at`, string(runID))
	if err != nil {
		return nil, fmt.Errorf("list node cache entries by run %s: %w", runID, err)
	}
	defer rows.Close()

	var out []harnessmodel.NodeCacheEntry
	for rows.Next() {
		entry, err := scanNodeCache(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate node cache entries: %w", err)
	}
	return out, nil
}

func (t *transaction) DeleteNodeCacheEntry(ctx context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("cache key is required")
	}
	res, err := t.tx.ExecContext(ctx, `DELETE FROM node_cache_entries WHERE cache_key=?`, key)
	if err != nil {
		return fmt.Errorf("delete node cache entry %s: %w", key, err)
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

func (t *transaction) EvictNodeCacheEntries(ctx context.Context, olderThan time.Time) (int, error) {
	res, err := t.tx.ExecContext(ctx, `DELETE FROM node_cache_entries WHERE last_hit_at < ?`, formatTime(olderThan))
	if err != nil {
		return 0, fmt.Errorf("evict node cache entries: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func scanNodeCache(row interface{ Scan(...any) error }) (harnessmodel.NodeCacheEntry, error) {
	var key, wfrID, nrID, attID, artifactsJSON, payload, createdAt, lastHitAt string
	var hitCount int
	if err := row.Scan(&key, &wfrID, &nrID, &attID, &artifactsJSON, &payload, &createdAt, &lastHitAt, &hitCount); err != nil {
		return harnessmodel.NodeCacheEntry{}, mapNotFound(err)
	}
	cTime, err := parseTime(createdAt)
	if err != nil {
		return harnessmodel.NodeCacheEntry{}, fmt.Errorf("parse cache created_at: %w", err)
	}
	hTime, err := parseTime(lastHitAt)
	if err != nil {
		return harnessmodel.NodeCacheEntry{}, fmt.Errorf("parse cache last_hit_at: %w", err)
	}

	var artifacts []harnessmodel.OutputArtifactSummary
	if artifactsJSON != "" && artifactsJSON != "[]" {
		_ = json.Unmarshal([]byte(artifactsJSON), &artifacts)
	}

	return harnessmodel.NodeCacheEntry{
		CacheKey:        key,
		WorkflowRunID:   harnessmodel.WorkflowRunID(wfrID),
		NodeRunID:       harnessmodel.NodeRunID(nrID),
		AttemptID:       harnessmodel.AttemptID(attID),
		OutputArtifacts: artifacts,
		ResultPayload:   payload,
		CreatedAt:       cTime,
		LastHitAt:       hTime,
		HitCount:        hitCount,
	}, nil
}
