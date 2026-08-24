package mcp

import (
	"context"
	"fmt"
	"time"
)

type Caller interface {
	Call(ctx context.Context, serverID, toolName, arguments string) (result string, err error)
}

type Executor struct {
	registry *ToolRegistry
	caller   Caller
	now      func() time.Time
}

func NewExecutor(registry *ToolRegistry, caller Caller, now func() time.Time) *Executor {
	if now == nil {
		now = time.Now
	}
	return &Executor{
		registry: registry,
		caller:   caller,
		now:      now,
	}
}

type ToolCallParams struct {
	ToolID             string
	Arguments          string
	ExpectedSchemaHash string
}

type ToolCallResult struct {
	Output     string
	Duration   time.Duration
	SchemaHash string
}

func (e *Executor) Execute(ctx context.Context, params ToolCallParams) (ToolCallResult, error) {
	if params.ToolID == "" {
		return ToolCallResult{}, fmt.Errorf("tool id is required")
	}

	tool, err := e.registry.Lookup(ctx, params.ToolID)
	if err != nil {
		return ToolCallResult{}, err
	}

	if params.ExpectedSchemaHash != "" && tool.InputSchemaHash != "" && tool.InputSchemaHash != params.ExpectedSchemaHash {
		return ToolCallResult{}, fmt.Errorf("%w: expected %s, got %s", ErrSchemaChanged, params.ExpectedSchemaHash, tool.InputSchemaHash)
	}

	start := e.now().UTC()
	out, err := e.caller.Call(ctx, tool.ServerID, tool.ToolID, params.Arguments)
	duration := e.now().UTC().Sub(start)

	if err != nil {
		return ToolCallResult{Duration: duration}, fmt.Errorf("%w: %v", ErrToolExecution, err)
	}

	return ToolCallResult{
		Output:     out,
		Duration:   duration,
		SchemaHash: tool.InputSchemaHash,
	}, nil
}
