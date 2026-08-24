package cache

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/harness/artifact"
	"github.com/homiakus/agctl/internal/harness/artifact/cas"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

func setupTestCacheService(t *testing.T) (*Service, *artifact.Store, *sqlitestore.DB, harnessmodel.WorkflowRun) {
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

	now := time.Unix(160_000, 0).UTC()
	var run harnessmodel.WorkflowRun

	err = db.Update(ctx, func(tx harnessstore.Tx) error {
		def := harnessmodel.WorkflowDefinition{
			ID: "wfd_cache", Version: 1, Name: "cache-wf", CreatedAt: now, CompilerVersion: "test",
		}
		if err := tx.CreateWorkflowDefinition(ctx, def); err != nil {
			return err
		}
		run = harnessmodel.WorkflowRun{
			ID: "wfr_cache_1", DefinitionID: def.ID, DefinitionVersion: 1, State: harnessmodel.WorkflowRunning,
			CreatedAt: now, UpdatedAt: now,
		}
		return tx.CreateWorkflowRun(ctx, run)
	})
	if err != nil {
		t.Fatal(err)
	}

	artStore, err := artifact.NewStore(casStorage, db, artifact.Options{
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	cacheSvc, err := NewService(db, artStore, Options{
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	return cacheSvc, artStore, db, run
}

func TestCacheKeyDeterminismAndMissOnVariant(t *testing.T) {
	inputs1 := FingerprintInputs{
		NodeID:          "build_module",
		ExecutorKind:    "PROCESS",
		ExecutorVersion: "1.0.0",
		InputDigests:    map[string]string{"src": "sha256:1111111111111111111111111111111111111111111111111111111111111111"},
		ParamsDigest:    "sha256:2222222222222222222222222222222222222222222222222222222222222222",
		EnvFingerprint:  map[string]string{"GOOS": "windows", "GOARCH": "amd64"},
	}

	key1 := ComputeKey(inputs1)
	key1Repeat := ComputeKey(inputs1)
	if key1 != key1Repeat {
		t.Fatalf("cache key not deterministic: %s vs %s", key1, key1Repeat)
	}

	// Change input digest -> key MUST change
	inputsChangedInput := inputs1
	inputsChangedInput.InputDigests = map[string]string{"src": "sha256:9999999999999999999999999999999999999999999999999999999999999999"}
	if ComputeKey(inputsChangedInput) == key1 {
		t.Fatal("expected different key on changed input digest")
	}

	// Change tool version -> key MUST change
	inputsChangedTool := inputs1
	inputsChangedTool.ToolOrModelVersion = "v2.0"
	if ComputeKey(inputsChangedTool) == key1 {
		t.Fatal("expected different key on changed tool version")
	}

	// Change env -> key MUST change
	inputsChangedEnv := inputs1
	inputsChangedEnv.EnvFingerprint = map[string]string{"GOOS": "linux", "GOARCH": "amd64"}
	if ComputeKey(inputsChangedEnv) == key1 {
		t.Fatal("expected different key on changed environment")
	}
}

func TestCacheEligibilityRules(t *testing.T) {
	// 1. Side effect node -> NEVER eligible
	sideEffectSpec := harnessmodel.NodeSpec{
		ID:          "db_write",
		CachePolicy: harnessmodel.CacheGlobalContent,
		Determinism: harnessmodel.DeterminismSideEffectful,
	}
	eligible, _ := CheckEligibility(sideEffectSpec)
	if eligible {
		t.Fatal("side effect node must not be eligible for cache")
	}

	// 2. Disabled policy -> NOT eligible
	disabledSpec := harnessmodel.NodeSpec{
		ID:          "calc",
		CachePolicy: harnessmodel.CacheDisabled,
		Determinism: harnessmodel.DeterminismDeterministic,
	}
	eligible, _ = CheckEligibility(disabledSpec)
	if eligible {
		t.Fatal("disabled cache policy must not be eligible")
	}

	// 3. Non-deterministic with global content -> falls back to disabled
	nonDetGlobalSpec := harnessmodel.NodeSpec{
		ID:          "llm_prompt",
		CachePolicy: harnessmodel.CacheGlobalContent,
		Determinism: harnessmodel.DeterminismNonDeterministic,
	}
	eligible, _ = CheckEligibility(nonDetGlobalSpec)
	if eligible {
		t.Fatal("nondeterministic node cannot use global content cache")
	}

	// 4. Non-deterministic with run-local -> ELIGIBLE as RUN_LOCAL
	nonDetLocalSpec := harnessmodel.NodeSpec{
		ID:          "llm_prompt",
		CachePolicy: harnessmodel.CacheRunLocal,
		Determinism: harnessmodel.DeterminismNonDeterministic,
	}
	eligible, policy := CheckEligibility(nonDetLocalSpec)
	if !eligible || !policy.IsRunLocal() {
		t.Fatalf("expected eligible run_local, got eligible=%v policy=%s", eligible, policy)
	}

	// 5. Deterministic node -> ELIGIBLE for GLOBAL_CONTENT
	detSpec := harnessmodel.NodeSpec{
		ID:          "validator",
		CachePolicy: harnessmodel.CacheGlobalContent,
		Determinism: harnessmodel.DeterminismDeterministic,
	}
	eligible, policy = CheckEligibility(detSpec)
	if !eligible || !policy.IsGlobal() {
		t.Fatalf("expected eligible global_content, got eligible=%v policy=%s", eligible, policy)
	}
}

func TestCacheStoreLookupAndHit(t *testing.T) {
	ctx := context.Background()
	svc, artStore, _, run1 := setupTestCacheService(t)

	// Create artifact in CAS
	content := []byte("compiled binary or test result")
	digest, size, err := artStore.CAS().WriteBytes(ctx, content)
	if err != nil {
		t.Fatal(err)
	}

	key := "cache:sha256:aabbcc11223344"
	entry := harnessmodel.NodeCacheEntry{
		CacheKey:      key,
		WorkflowRunID: run1.ID,
		NodeRunID:     "nr_1",
		AttemptID:     "att_1",
		OutputArtifacts: []harnessmodel.OutputArtifactSummary{
			{
				ID:            "art_out",
				Name:          "output.bin",
				Type:          harnessmodel.ArtifactOutput,
				ContentDigest: digest,
				SizeBytes:     size,
			},
		},
		ResultPayload: `{"status":"ok"}`,
		CreatedAt:     time.Now().UTC(),
	}

	if err := svc.Put(ctx, entry); err != nil {
		t.Fatal(err)
	}

	// Global lookup from another workflow run -> HIT
	lookup, err := svc.Lookup(ctx, key, "wfr_cache_2", harnessmodel.CacheGlobalContent)
	if err != nil {
		t.Fatal(err)
	}
	if !lookup.Hit || lookup.Entry.HitCount != 1 {
		t.Fatalf("expected hit with hitCount 1, got %+v", lookup)
	}

	// Run-local lookup from another workflow run -> MISS (scope mismatch)
	lookupLocalOtherRun, err := svc.Lookup(ctx, key, "wfr_cache_2", harnessmodel.CacheRunLocal)
	if err != nil {
		t.Fatal(err)
	}
	if lookupLocalOtherRun.Hit || lookupLocalOtherRun.Reason != "run_local_scope_mismatch" {
		t.Fatalf("expected miss due to scope mismatch, got %+v", lookupLocalOtherRun)
	}

	// Run-local lookup from SAME workflow run -> HIT
	lookupLocalSameRun, err := svc.Lookup(ctx, key, run1.ID, harnessmodel.CacheRunLocal)
	if err != nil {
		t.Fatal(err)
	}
	if !lookupLocalSameRun.Hit {
		t.Fatalf("expected hit for same run, got %+v", lookupLocalSameRun)
	}
}

func TestDownstreamLineagePreservation(t *testing.T) {
	// If rerun of parent produces identical digest, downstream is valid
	if !IsDownstreamLineageValid("sha256:abcd", "sha256:abcd") {
		t.Fatal("expected downstream lineage valid when digests match")
	}

	// If parent produced different output, downstream is invalidated
	if IsDownstreamLineageValid("sha256:abcd", "sha256:ef01") {
		t.Fatal("expected downstream lineage invalid when digests differ")
	}
}
