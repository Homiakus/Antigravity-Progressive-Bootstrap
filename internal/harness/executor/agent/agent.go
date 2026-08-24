package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/homiakus/agctl/internal/harness/budget"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

var (
	ErrValidationFailed = errors.New("agent execution: external validation gate failed")
	ErrExecutionAborted = errors.New("agent execution: aborted by budget or error")
)

type AgentRunID string

type AgentStep struct {
	Index        int       `json:"index"`
	Timestamp    time.Time `json:"timestamp"`
	ModelPrompt  string    `json:"modelPrompt,omitempty"`
	ModelOutput  string    `json:"modelOutput,omitempty"`
	ToolName     string    `json:"toolName,omitempty"`
	ToolInput    string    `json:"toolInput,omitempty"`
	ToolOutput   string    `json:"toolOutput,omitempty"`
	TokensUsed   int64     `json:"tokensUsed,omitempty"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
}

type Checkpoint struct {
	StepIndex    int               `json:"stepIndex"`
	Timestamp    time.Time         `json:"timestamp"`
	MemoryState  map[string]string `json:"memoryState,omitempty"`
	ArtifactRefs []string          `json:"artifactRefs,omitempty"`
	BudgetUsage  budget.Usage      `json:"budgetUsage"`
}

type AgentRunState struct {
	mu           sync.RWMutex
	ID           AgentRunID        `json:"id"`
	AttemptID    harnessmodel.AttemptID `json:"attemptId"`
	AgentID      string            `json:"agentId"`
	Steps        []AgentStep       `json:"steps"`
	Checkpoints  []Checkpoint      `json:"checkpoints"`
	Memory       map[string]string `json:"memory"`
	ArtifactRefs []string          `json:"artifactRefs"`
	IsComplete   bool              `json:"isComplete"`
}

type ModelClient interface {
	Generate(ctx context.Context, prompt string, memory map[string]string) (output string, tokens int64, costUSD float64, err error)
}

type ToolDispatcher interface {
	Execute(ctx context.Context, toolName string, toolInput string) (toolOutput string, err error)
}

type Validator interface {
	Validate(ctx context.Context, state *AgentRunState) (valid bool, reason string, err error)
}

type Options struct {
	Now func() time.Time
}

type Executor struct {
	now func() time.Time
}

func NewExecutor(opts Options) *Executor {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Executor{now: now}
}

type ExecuteParams struct {
	RunID          AgentRunID
	AttemptID      harnessmodel.AttemptID
	AgentID        string
	InitialPrompt  string
	Limits         budget.Limits
	ModelClient    ModelClient
	ToolDispatcher ToolDispatcher
	Validator      Validator
}

func (e *Executor) Execute(ctx context.Context, params ExecuteParams) (*AgentRunState, error) {
	if params.RunID == "" {
		return nil, fmt.Errorf("agent run id is required")
	}
	if params.ModelClient == nil {
		return nil, fmt.Errorf("model client is required")
	}

	tracker := budget.NewTracker(params.Limits, e.now)
	state := &AgentRunState{
		ID:        params.RunID,
		AttemptID: params.AttemptID,
		AgentID:   params.AgentID,
		Memory:    make(map[string]string),
	}

	prompt := params.InitialPrompt
	for {
		// 1. Budget step check
		if err := tracker.RecordStep(); err != nil {
			return state, fmt.Errorf("step budget limit: %w", err)
		}

		// 2. Model call
		modelOut, tokens, cost, err := params.ModelClient.Generate(ctx, prompt, state.Memory)
		if err != nil {
			_ = tracker.RecordFailure()
			return state, fmt.Errorf("model call failed: %w", err)
		}
		if err := tracker.RecordModelCall(tokens, cost); err != nil {
			return state, fmt.Errorf("model budget limit: %w", err)
		}

		step := AgentStep{
			Index:       len(state.Steps) + 1,
			Timestamp:   e.now().UTC(),
			ModelPrompt: prompt,
			ModelOutput: modelOut,
			TokensUsed:  tokens,
		}

		// Checkpoint after model response
		state.mu.Lock()
		state.Steps = append(state.Steps, step)
		state.Checkpoints = append(state.Checkpoints, Checkpoint{
			StepIndex:   step.Index,
			Timestamp:   e.now().UTC(),
			MemoryState: copyMap(state.Memory),
			BudgetUsage: tracker.GetUsage(),
		})
		state.mu.Unlock()

		// 3. External validation gate check
		if params.Validator != nil {
			valid, reason, vErr := params.Validator.Validate(ctx, state)
			if vErr != nil {
				return state, fmt.Errorf("validator error: %w", vErr)
			}
			if valid {
				state.mu.Lock()
				state.IsComplete = true
				state.mu.Unlock()
				return state, nil
			}
			// If not yet valid, provide reason back into prompt for next turn
			prompt = fmt.Sprintf("Previous attempt did not satisfy completion criteria: %s. Please address this.", reason)
			_ = tracker.RecordFailure()
			continue
		}

		// If no validator specified, single turn completes
		state.mu.Lock()
		state.IsComplete = true
		state.mu.Unlock()
		return state, nil
	}
}

func copyMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
