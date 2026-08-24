package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/harness/artifact"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type FingerprintInputs struct {
	NodeID              string            `json:"nodeId"`
	ExecutorKind        string            `json:"executorKind"`
	ExecutorVersion     string            `json:"executorVersion,omitempty"`
	InputDigests        map[string]string `json:"inputDigests,omitempty"`
	ParamsDigest        string            `json:"paramsDigest,omitempty"`
	EnvFingerprint      map[string]string `json:"envFingerprint,omitempty"`
	ToolOrModelVersion  string            `json:"toolOrModelVersion,omitempty"`
}

func ComputeKey(inputs FingerprintInputs) string {
	// Canonical sorted input digests
	sortedInputs := make(map[string]string)
	for k, v := range inputs.InputDigests {
		sortedInputs[k] = v
	}
	sortedEnv := make(map[string]string)
	for k, v := range inputs.EnvFingerprint {
		sortedEnv[k] = v
	}

	payload := map[string]any{
		"nodeId":             inputs.NodeID,
		"executorKind":       inputs.ExecutorKind,
		"executorVersion":    inputs.ExecutorVersion,
		"inputDigests":       sortedInputs,
		"paramsDigest":       inputs.ParamsDigest,
		"envFingerprint":     sortedEnv,
		"toolOrModelVersion": inputs.ToolOrModelVersion,
	}

	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return "cache:sha256:" + hex.EncodeToString(sum[:])
}

// CheckEligibility evaluates whether a node spec can use caching.
func CheckEligibility(spec harnessmodel.NodeSpec) (bool, harnessmodel.CachePolicy) {
	if spec.Determinism == harnessmodel.DeterminismSideEffectful {
		return false, harnessmodel.CacheDisabled
	}
	if !spec.CachePolicy.IsEnabled() {
		return false, harnessmodel.CacheDisabled
	}
	// Non-deterministic nodes cannot be cached globally across runs
	if spec.Determinism == harnessmodel.DeterminismNonDeterministic {
		if spec.CachePolicy.IsGlobal() {
			return false, harnessmodel.CacheDisabled
		}
		return true, harnessmodel.CacheRunLocal
	}
	return true, spec.CachePolicy
}

type Options struct {
	Now func() time.Time
}

type Service struct {
	store    harnessstore.Store
	artStore *artifact.Store
	now      func() time.Time
}

func NewService(store harnessstore.Store, artStore *artifact.Store, opts Options) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("harness store is required")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		store:    store,
		artStore: artStore,
		now:      now,
	}, nil
}

type LookupResult struct {
	Hit    bool
	Entry  harnessmodel.NodeCacheEntry
	Reason string
}

func (s *Service) Lookup(ctx context.Context, key string, currentRunID harnessmodel.WorkflowRunID, policy harnessmodel.CachePolicy) (LookupResult, error) {
	if strings.TrimSpace(key) == "" {
		return LookupResult{Hit: false, Reason: "empty_key"}, nil
	}
	if !policy.IsEnabled() {
		return LookupResult{Hit: false, Reason: "cache_disabled"}, nil
	}

	var entry harnessmodel.NodeCacheEntry
	err := s.store.View(ctx, func(r harnessstore.Reader) error {
		var readErr error
		entry, readErr = r.GetNodeCacheEntry(ctx, key)
		return readErr
	})
	if err != nil {
		if err == harnessstore.ErrNotFound {
			return LookupResult{Hit: false, Reason: "not_found"}, nil
		}
		return LookupResult{}, err
	}

	// If policy is RUN_LOCAL, must match current run ID
	if policy.IsRunLocal() && entry.WorkflowRunID != currentRunID {
		return LookupResult{Hit: false, Reason: "run_local_scope_mismatch"}, nil
	}

	// Verify all output artifacts exist in CAS
	if s.artStore != nil && len(entry.OutputArtifacts) > 0 {
		for _, out := range entry.OutputArtifacts {
			if out.ContentDigest != "" && !s.artStore.CAS().Exists(out.ContentDigest) {
				// Evict corrupted/missing cache entry
				_ = s.store.Update(ctx, func(tx harnessstore.Tx) error {
					return tx.DeleteNodeCacheEntry(ctx, key)
				})
				return LookupResult{Hit: false, Reason: "missing_artifact_in_cas"}, nil
			}
		}
	}

	// Record hit touch
	now := s.now().UTC()
	_ = s.store.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.TouchNodeCacheHit(ctx, key, now)
	})

	entry.LastHitAt = now
	entry.HitCount++
	return LookupResult{Hit: true, Entry: entry, Reason: "hit"}, nil
}

func (s *Service) Put(ctx context.Context, entry harnessmodel.NodeCacheEntry) error {
	if err := entry.Validate(); err != nil {
		return err
	}
	now := s.now().UTC()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	if entry.LastHitAt.IsZero() {
		entry.LastHitAt = now
	}
	return s.store.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.PutNodeCacheEntry(ctx, entry)
	})
}

func (s *Service) Invalidate(ctx context.Context, key string) error {
	return s.store.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.DeleteNodeCacheEntry(ctx, key)
	})
}

func (s *Service) Evict(ctx context.Context, olderThan time.Time) (int, error) {
	var count int
	err := s.store.Update(ctx, func(tx harnessstore.Tx) error {
		var evictErr error
		count, evictErr = tx.EvictNodeCacheEntries(ctx, olderThan)
		return evictErr
	})
	return count, err
}

// IsDownstreamLineageValid checks whether a parent rerun produced identical output
// so that downstream dependent nodes do NOT need recomputation.
func IsDownstreamLineageValid(oldOutputDigest, newOutputDigest string) bool {
	return oldOutputDigest != "" && oldOutputDigest == newOutputDigest
}

// ComputeParamsDigest helper to hash structured parameter maps
func ComputeParamsDigest(params map[string]any) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	data, _ := json.Marshal(params)
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
