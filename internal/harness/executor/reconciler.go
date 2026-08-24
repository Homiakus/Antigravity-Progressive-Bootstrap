package executor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FilesystemReconciler inspects local filesystem operations to verify whether
// a file was written, created, or deleted.
type FilesystemReconciler struct {
	BaseDir string
}

func (r *FilesystemReconciler) ReconcileEffect(ctx context.Context, req EffectReconcileRequest) (EffectReconcileResult, error) {
	if err := req.Validate(); err != nil {
		return EffectReconcileResult{}, err
	}

	var payload struct {
		Path         string `json:"path"`
		ExpectedHash string `json:"expected_sha256,omitempty"`
		ExpectedSize int64  `json:"expected_size,omitempty"`
	}

	targetPath := req.ProviderRef
	if targetPath == "" && len(req.SemanticInputDigest) > 0 {
		// If ProviderRef not set, attempt to extract path from operation details
		targetPath = req.Operation
	}

	// If semantic payload is json-encoded path or params
	if strings.HasPrefix(targetPath, "path:") {
		targetPath = strings.TrimPrefix(targetPath, "path:")
	}

	if targetPath == "" {
		return EffectReconcileResult{
			Status:       EffectReconcileUnknown,
			ErrorClass:   "MISSING_PATH",
			ErrorMessage: "filesystem reconciliation requires target path in providerRef or operation",
		}, nil
	}

	fullPath := targetPath
	if r.BaseDir != "" && !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(r.BaseDir, fullPath)
	}

	switch req.Operation {
	case "delete", "delete_file", "remove":
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return EffectReconcileResult{
				Status:       EffectReconcileConfirmed,
				ProviderRef:  "path:" + targetPath,
				ResultDigest: "absent_confirmed",
			}, nil
		}
		return EffectReconcileResult{
			Status: EffectReconcileAbsent,
		}, nil

	case "write", "write_file", "create", "create_file":
		f, err := os.Open(fullPath)
		if os.IsNotExist(err) {
			return EffectReconcileResult{
				Status: EffectReconcileAbsent,
			}, nil
		}
		if err != nil {
			return EffectReconcileResult{
				Status:       EffectReconcileUnknown,
				ErrorClass:   "FS_ERROR",
				ErrorMessage: err.Error(),
			}, nil
		}
		defer f.Close()

		h := sha256.New()
		size, err := io.Copy(h, f)
		if err != nil {
			return EffectReconcileResult{
				Status:       EffectReconcileUnknown,
				ErrorClass:   "READ_ERROR",
				ErrorMessage: err.Error(),
			}, nil
		}
		actualDigest := "sha256:" + hex.EncodeToString(h.Sum(nil))

		if payload.ExpectedHash != "" && payload.ExpectedHash != actualDigest {
			return EffectReconcileResult{
				Status:       EffectReconcileFailed,
				ErrorClass:   "CONTENT_HASH_MISMATCH",
				ErrorMessage: fmt.Sprintf("expected hash %s, got %s", payload.ExpectedHash, actualDigest),
				ProviderRef:  "path:" + targetPath,
				ResultDigest: actualDigest,
			}, nil
		}
		if payload.ExpectedSize > 0 && payload.ExpectedSize != size {
			return EffectReconcileResult{
				Status:       EffectReconcileFailed,
				ErrorClass:   "CONTENT_SIZE_MISMATCH",
				ErrorMessage: fmt.Sprintf("expected size %d, got %d", payload.ExpectedSize, size),
				ProviderRef:  "path:" + targetPath,
				ResultDigest: actualDigest,
			}, nil
		}

		return EffectReconcileResult{
			Status:       EffectReconcileConfirmed,
			ProviderRef:  "path:" + targetPath,
			ResultDigest: actualDigest,
		}, nil

	default:
		// Generic file existence check
		if fi, err := os.Stat(fullPath); err == nil && !fi.IsDir() {
			return EffectReconcileResult{
				Status:      EffectReconcileConfirmed,
				ProviderRef: "path:" + targetPath,
			}, nil
		} else if os.IsNotExist(err) {
			return EffectReconcileResult{
				Status: EffectReconcileAbsent,
			}, nil
		}
		return EffectReconcileResult{
			Status:       EffectReconcileUnknown,
			ErrorClass:   "UNSUPPORTED_FS_OP",
			ErrorMessage: fmt.Sprintf("unsupported fs operation %q", req.Operation),
		}, nil
	}
}

// GitQueryFunc allows mocking or customizing git queries.
type GitQueryFunc func(ctx context.Context, req EffectReconcileRequest) (EffectReconcileResult, error)

// GitReconciler reconciles git commits, branches, tags.
type GitReconciler struct {
	RepoDir   string
	QueryFunc GitQueryFunc
}

func (r *GitReconciler) ReconcileEffect(ctx context.Context, req EffectReconcileRequest) (EffectReconcileResult, error) {
	if err := req.Validate(); err != nil {
		return EffectReconcileResult{}, err
	}
	if r.QueryFunc != nil {
		return r.QueryFunc(ctx, req)
	}

	// Default git query behavior by provider ref / idempotency key
	if req.ProviderRef != "" {
		return EffectReconcileResult{
			Status:       EffectReconcileConfirmed,
			ProviderRef:  req.ProviderRef,
			ResultDigest: req.SemanticInputDigest,
		}, nil
	}

	return EffectReconcileResult{
		Status:       EffectReconcileUnknown,
		ErrorClass:   "GIT_RECONCILE_UNAVAILABLE",
		ErrorMessage: "git repository query function not configured",
	}, nil
}

// GitHubQueryFunc allows querying or mocking GitHub API queries.
type GitHubQueryFunc func(ctx context.Context, req EffectReconcileRequest) (EffectReconcileResult, error)

// GitHubReconciler reconciles GitHub API operations (issues, PRs, comments).
type GitHubReconciler struct {
	QueryFunc GitHubQueryFunc
}

func (r *GitHubReconciler) ReconcileEffect(ctx context.Context, req EffectReconcileRequest) (EffectReconcileResult, error) {
	if err := req.Validate(); err != nil {
		return EffectReconcileResult{}, err
	}
	if r.QueryFunc != nil {
		return r.QueryFunc(ctx, req)
	}

	if req.ProviderRef != "" {
		return EffectReconcileResult{
			Status:       EffectReconcileConfirmed,
			ProviderRef:  req.ProviderRef,
			ResultDigest: req.SemanticInputDigest,
		}, nil
	}

	return EffectReconcileResult{
		Status:       EffectReconcileUnknown,
		ErrorClass:   "GITHUB_RECONCILE_UNAVAILABLE",
		ErrorMessage: "github api query function not configured",
	}, nil
}

// MCPQueryFunc allows querying or mocking MCP tool execution queries.
type MCPQueryFunc func(ctx context.Context, req EffectReconcileRequest) (EffectReconcileResult, error)

// MCPReconciler reconciles Model Context Protocol tool invocations.
type MCPReconciler struct {
	QueryFunc MCPQueryFunc
}

func (r *MCPReconciler) ReconcileEffect(ctx context.Context, req EffectReconcileRequest) (EffectReconcileResult, error) {
	if err := req.Validate(); err != nil {
		return EffectReconcileResult{}, err
	}
	if r.QueryFunc != nil {
		return r.QueryFunc(ctx, req)
	}

	if req.ProviderRef != "" {
		return EffectReconcileResult{
			Status:       EffectReconcileConfirmed,
			ProviderRef:  req.ProviderRef,
			ResultDigest: req.SemanticInputDigest,
		}, nil
	}

	// By default, arbitrary MCP tools without query evidence return UNKNOWN
	// to prevent blind retry of dangerous / non-idempotent tool calls.
	return EffectReconcileResult{
		Status:       EffectReconcileUnknown,
		ErrorClass:   "MCP_TOOL_QUERY_UNSUPPORTED",
		ErrorMessage: fmt.Sprintf("mcp tool %s/%s does not support reconciliation query", req.OperationNamespace, req.Operation),
	}, nil
}

// CompositeReconciler routes reconciliation requests to namespace-specific reconcilers.
type CompositeReconciler struct {
	mu          sync.RWMutex
	reconcilers map[string]EffectReconciler
	fallback    EffectReconciler
}

func NewCompositeReconciler() *CompositeReconciler {
	c := &CompositeReconciler{
		reconcilers: make(map[string]EffectReconciler),
	}
	c.Register("fs", &FilesystemReconciler{})
	c.Register("filesystem", &FilesystemReconciler{})
	c.Register("git", &GitReconciler{})
	c.Register("github", &GitHubReconciler{})
	c.Register("mcp", &MCPReconciler{})
	c.Register("mcp_tool", &MCPReconciler{})
	return c
}

func (c *CompositeReconciler) Register(namespace string, reconciler EffectReconciler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reconcilers[strings.ToLower(strings.TrimSpace(namespace))] = reconciler
}

func (c *CompositeReconciler) SetFallback(fallback EffectReconciler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fallback = fallback
}

func (c *CompositeReconciler) ReconcileEffect(ctx context.Context, req EffectReconcileRequest) (EffectReconcileResult, error) {
	if err := req.Validate(); err != nil {
		return EffectReconcileResult{}, err
	}
	ns := strings.ToLower(strings.TrimSpace(req.OperationNamespace))
	c.mu.RLock()
	r, ok := c.reconcilers[ns]
	fb := c.fallback
	c.mu.RUnlock()

	if ok && r != nil {
		return r.ReconcileEffect(ctx, req)
	}
	if fb != nil {
		return fb.ReconcileEffect(ctx, req)
	}

	return EffectReconcileResult{
		Status:       EffectReconcileUnknown,
		ErrorClass:   "UNKNOWN_PROVIDER",
		ErrorMessage: fmt.Sprintf("no reconciler registered for namespace %q", req.OperationNamespace),
	}, nil
}

var _ = json.Marshal
