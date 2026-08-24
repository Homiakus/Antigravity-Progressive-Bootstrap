package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestFilesystemReconcilerWriteAndAbsent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	rec := &FilesystemReconciler{BaseDir: dir}

	fileName := "test.txt"
	fullPath := filepath.Join(dir, fileName)
	content := []byte("hello world")

	req := EffectReconcileRequest{
		EffectIntentID:      "eff_fs_1",
		WorkflowRunID:       "wfr_1",
		NodeRunID:           "nr_1",
		OperationNamespace:  "fs",
		Operation:           "write_file",
		Class:               harnessmodel.EffectQueryable,
		IdempotencyKey:      "effk_v1_dummy",
		SemanticInputDigest: "sha256:dummy",
		ProviderRef:         "path:" + fileName,
	}

	// Before file exists: status should be ABSENT
	res, err := rec.ReconcileEffect(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != EffectReconcileAbsent {
		t.Fatalf("expected ABSENT before file creation, got %s", res.Status)
	}

	// Create file
	if err := os.WriteFile(fullPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	// After file exists: status should be CONFIRMED
	res, err = rec.ReconcileEffect(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != EffectReconcileConfirmed || res.ResultDigest == "" {
		t.Fatalf("expected CONFIRMED after file creation, got %+v", res)
	}
}

func TestFilesystemReconcilerDelete(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	rec := &FilesystemReconciler{BaseDir: dir}

	fileName := "deleted.txt"
	fullPath := filepath.Join(dir, fileName)
	if err := os.WriteFile(fullPath, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := EffectReconcileRequest{
		EffectIntentID:      "eff_fs_del",
		WorkflowRunID:       "wfr_1",
		NodeRunID:           "nr_1",
		OperationNamespace:  "fs",
		Operation:           "delete_file",
		Class:               harnessmodel.EffectQueryable,
		IdempotencyKey:      "effk_v1_del",
		SemanticInputDigest: "sha256:dummy",
		ProviderRef:         "path:" + fileName,
	}

	// File still exists -> delete not completed (ABSENT)
	res, err := rec.ReconcileEffect(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != EffectReconcileAbsent {
		t.Fatalf("expected ABSENT while file still exists, got %s", res.Status)
	}

	// Remove file
	if err := os.Remove(fullPath); err != nil {
		t.Fatal(err)
	}

	// File gone -> delete CONFIRMED
	res, err = rec.ReconcileEffect(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != EffectReconcileConfirmed {
		t.Fatalf("expected CONFIRMED after removal, got %s", res.Status)
	}
}

func TestCompositeReconcilerRoutesToProviders(t *testing.T) {
	ctx := context.Background()
	comp := NewCompositeReconciler()

	gitCalled := false
	comp.Register("git", &GitReconciler{
		QueryFunc: func(ctx context.Context, req EffectReconcileRequest) (EffectReconcileResult, error) {
			gitCalled = true
			return EffectReconcileResult{Status: EffectReconcileConfirmed, ProviderRef: "commit:abcdef"}, nil
		},
	})

	ghCalled := false
	comp.Register("github", &GitHubReconciler{
		QueryFunc: func(ctx context.Context, req EffectReconcileRequest) (EffectReconcileResult, error) {
			ghCalled = true
			return EffectReconcileResult{Status: EffectReconcileAbsent}, nil
		},
	})

	// Test git routing
	gitReq := EffectReconcileRequest{
		EffectIntentID:      "eff_git",
		WorkflowRunID:       "wfr_1",
		NodeRunID:           "nr_1",
		OperationNamespace:  "git",
		Operation:           "commit",
		Class:               harnessmodel.EffectQueryable,
		IdempotencyKey:      "effk_v1_git",
		SemanticInputDigest: "sha256:git",
	}
	res, err := comp.ReconcileEffect(ctx, gitReq)
	if err != nil {
		t.Fatal(err)
	}
	if !gitCalled || res.Status != EffectReconcileConfirmed {
		t.Fatalf("git reconciler not called or wrong status: %+v", res)
	}

	// Test github routing
	ghReq := EffectReconcileRequest{
		EffectIntentID:      "eff_gh",
		WorkflowRunID:       "wfr_1",
		NodeRunID:           "nr_1",
		OperationNamespace:  "github",
		Operation:           "create_issue",
		Class:               harnessmodel.EffectQueryable,
		IdempotencyKey:      "effk_v1_gh",
		SemanticInputDigest: "sha256:gh",
	}
	res, err = comp.ReconcileEffect(ctx, ghReq)
	if err != nil {
		t.Fatal(err)
	}
	if !ghCalled || res.Status != EffectReconcileAbsent {
		t.Fatalf("github reconciler not called or wrong status: %+v", res)
	}

	// Test unknown MCP tool -> returns UNKNOWN
	mcpReq := EffectReconcileRequest{
		EffectIntentID:      "eff_mcp",
		WorkflowRunID:       "wfr_1",
		NodeRunID:           "nr_1",
		OperationNamespace:  "mcp",
		Operation:           "database_drop",
		Class:               harnessmodel.EffectNonIdempotentUnknown,
		IdempotencyKey:      "effk_v1_mcp",
		SemanticInputDigest: "sha256:mcp",
	}
	res, err = comp.ReconcileEffect(ctx, mcpReq)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != EffectReconcileUnknown {
		t.Fatalf("mcp reconciler without query support should return UNKNOWN, got %s", res.Status)
	}

	// Test unregistered namespace -> returns UNKNOWN
	unknownReq := EffectReconcileRequest{
		EffectIntentID:      "eff_unk",
		WorkflowRunID:       "wfr_1",
		NodeRunID:           "nr_1",
		OperationNamespace:  "stripe_payments",
		Operation:           "charge",
		Class:               harnessmodel.EffectNonIdempotentUnknown,
		IdempotencyKey:      "effk_v1_unk",
		SemanticInputDigest: "sha256:unk",
	}
	res, err = comp.ReconcileEffect(ctx, unknownReq)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != EffectReconcileUnknown || res.ErrorClass != "UNKNOWN_PROVIDER" {
		t.Fatalf("unregistered provider should return UNKNOWN, got %+v", res)
	}
}
