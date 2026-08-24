package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var (
	ErrToolNotFound       = errors.New("harness mcp: tool not found in registry")
	ErrToolExpired        = errors.New("harness mcp: tool discovery TTL expired")
	ErrSchemaChanged      = errors.New("harness mcp: tool schema changed unexpectedly")
	ErrTransportBeforeSend = errors.New("harness mcp: transport failed before sending request")
	ErrTransportAfterSend  = errors.New("harness mcp: transport failed after sending request (uncertain effect)")
	ErrToolExecution      = errors.New("harness mcp: tool returned execution error")
)

type ToolRecord struct {
	ToolID           string            `json:"toolId"`
	ServerID         string            `json:"serverId"`
	ProtocolVersion  string            `json:"protocolVersion"`
	Capabilities     []string          `json:"capabilities"`
	InputSchema      string            `json:"inputSchema"`
	InputSchemaHash  string            `json:"inputSchemaHash"`
	OutputSchemaHash string            `json:"outputSchemaHash"`
	Risk             string            `json:"risk"`
	Available        bool              `json:"available"`
	LastDiscoveredAt time.Time         `json:"lastDiscoveredAt"`
	TTL              time.Duration     `json:"ttl"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

func ComputeSchemaHash(schema string) string {
	h := sha256.Sum256([]byte(schema))
	return hex.EncodeToString(h[:])
}

type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]ToolRecord
	now   func() time.Time
}

func NewToolRegistry(now func() time.Time) *ToolRegistry {
	if now == nil {
		now = time.Now
	}
	return &ToolRegistry{
		tools: make(map[string]ToolRecord),
		now:   now,
	}
}

func (r *ToolRegistry) Register(tool ToolRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tool.InputSchemaHash == "" && tool.InputSchema != "" {
		tool.InputSchemaHash = ComputeSchemaHash(tool.InputSchema)
	}
	if tool.LastDiscoveredAt.IsZero() {
		tool.LastDiscoveredAt = r.now().UTC()
	}
	if tool.TTL <= 0 {
		tool.TTL = 1 * time.Hour
	}
	tool.Available = true
	r.tools[tool.ToolID] = tool
}

func (r *ToolRegistry) Lookup(ctx context.Context, toolID string) (ToolRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[toolID]
	if !ok || !tool.Available {
		return ToolRecord{}, ErrToolNotFound
	}
	if tool.TTL > 0 && r.now().UTC().Sub(tool.LastDiscoveredAt) > tool.TTL {
		return ToolRecord{}, ErrToolExpired
	}
	return tool, nil
}

func (r *ToolRegistry) ListByCapability(capName string) []ToolRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []ToolRecord
	for _, tool := range r.tools {
		if !tool.Available {
			continue
		}
		for _, c := range tool.Capabilities {
			if c == capName {
				out = append(out, tool)
				break
			}
		}
	}
	return out
}

func (r *ToolRegistry) Invalidate(toolID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tool, ok := r.tools[toolID]; ok {
		tool.Available = false
		r.tools[toolID] = tool
	}
}
