package mcp

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockCaller struct {
	response string
	err      error
}

func (m *mockCaller) Call(ctx context.Context, serverID, toolName, arguments string) (string, error) {
	return m.response, m.err
}

func TestToolRegistryLookupAndCapabilityFiltering(t *testing.T) {
	now := time.Unix(1000, 0)
	clock := func() time.Time { return now }

	reg := NewToolRegistry(clock)
	reg.Register(ToolRecord{
		ToolID:       "fetch_data",
		ServerID:     "http_server",
		Capabilities: []string{"network", "read"},
		InputSchema:  `{"type":"object","properties":{"url":{"type":"string"}}}`,
		TTL:          10 * time.Minute,
	})
	reg.Register(ToolRecord{
		ToolID:       "write_file",
		ServerID:     "fs_server",
		Capabilities: []string{"filesystem", "write"},
		InputSchema:  `{"type":"object","properties":{"path":{"type":"string"}}}`,
		TTL:          10 * time.Minute,
	})

	ctx := context.Background()

	// 1. Lookup fetch_data
	tool, err := reg.Lookup(ctx, "fetch_data")
	if err != nil {
		t.Fatal(err)
	}
	if tool.InputSchemaHash == "" {
		t.Fatal("expected non-empty input schema hash")
	}

	// 2. Capability filter
	netTools := reg.ListByCapability("network")
	if len(netTools) != 1 || netTools[0].ToolID != "fetch_data" {
		t.Fatalf("expected 1 network tool, got %v", netTools)
	}

	// 3. TTL expiry
	now = now.Add(15 * time.Minute)
	_, err = reg.Lookup(ctx, "fetch_data")
	if err == nil || !errors.Is(err, ErrToolExpired) {
		t.Fatalf("expected ErrToolExpired, got %v", err)
	}
}

func TestToolExecutorSchemaValidationAndExecution(t *testing.T) {
	now := time.Unix(1000, 0)
	clock := func() time.Time { return now }

	reg := NewToolRegistry(clock)
	schema := `{"type":"object"}`
	hash := ComputeSchemaHash(schema)

	reg.Register(ToolRecord{
		ToolID:          "query_db",
		ServerID:        "db_server",
		InputSchema:     schema,
		InputSchemaHash: hash,
		TTL:             10 * time.Minute,
	})

	caller := &mockCaller{response: `{"status":"ok","rows":5}`}
	exec := NewExecutor(reg, caller, clock)
	ctx := context.Background()

	// 1. Success execution with correct expected schema hash
	res, err := exec.Execute(ctx, ToolCallParams{
		ToolID:             "query_db",
		Arguments:          `{"query":"SELECT 1"}`,
		ExpectedSchemaHash: hash,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != `{"status":"ok","rows":5}` {
		t.Fatalf("unexpected output: %s", res.Output)
	}

	// 2. Schema changed error
	_, err = exec.Execute(ctx, ToolCallParams{
		ToolID:             "query_db",
		Arguments:          `{"query":"SELECT 1"}`,
		ExpectedSchemaHash: "different_hash",
	})
	if err == nil || !errors.Is(err, ErrSchemaChanged) {
		t.Fatalf("expected ErrSchemaChanged, got %v", err)
	}
}
