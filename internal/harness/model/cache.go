package model

import (
	"fmt"
	"strings"
	"time"
)

type OutputArtifactSummary struct {
	ID            ArtifactID   `json:"id"`
	Name          string       `json:"name"`
	Type          ArtifactType `json:"type"`
	ContentDigest string       `json:"contentDigest"`
	SizeBytes     int64        `json:"sizeBytes"`
}

type NodeCacheEntry struct {
	CacheKey        string                  `json:"cacheKey"`
	WorkflowRunID   WorkflowRunID           `json:"workflowRunId"`
	NodeRunID       NodeRunID               `json:"nodeRunId"`
	AttemptID       AttemptID               `json:"attemptId"`
	OutputArtifacts []OutputArtifactSummary `json:"outputArtifacts"`
	ResultPayload   string                  `json:"resultPayload,omitempty"`
	CreatedAt       time.Time               `json:"createdAt"`
	LastHitAt       time.Time               `json:"lastHitAt"`
	HitCount        int                     `json:"hitCount"`
}

func (e NodeCacheEntry) Validate() error {
	if strings.TrimSpace(e.CacheKey) == "" {
		return fmt.Errorf("cache key is required")
	}
	if e.WorkflowRunID == "" || e.NodeRunID == "" {
		return fmt.Errorf("workflow run id and node run id are required")
	}
	if e.CreatedAt.IsZero() {
		return fmt.Errorf("cache entry created_at is required")
	}
	if e.HitCount < 0 {
		return fmt.Errorf("hit count cannot be negative")
	}
	return nil
}
