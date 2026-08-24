package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/harness/budget"
)

type mockModelClient struct {
	turns int
}

func (m *mockModelClient) Generate(ctx context.Context, prompt string, memory map[string]string) (string, int64, float64, error) {
	m.turns++
	return "I worked on the task", 50, 0.001, nil
}

type mockValidator struct {
	passOnTurn int
	current    int
}

func (v *mockValidator) Validate(ctx context.Context, state *AgentRunState) (bool, string, error) {
	v.current++
	if v.current >= v.passOnTurn {
		return true, "", nil
	}
	return false, "tests still failing", nil
}

func TestAgentExecutorMultiTurnWithValidationGate(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1000, 0)
	clock := func() time.Time { return now }

	exec := NewExecutor(Options{Now: clock})
	model := &mockModelClient{}
	val := &mockValidator{passOnTurn: 2}

	state, err := exec.Execute(ctx, ExecuteParams{
		RunID:         "ar_1",
		AgentID:       "coder",
		InitialPrompt: "Fix the bug",
		Limits: budget.Limits{
			MaxSteps: 5,
		},
		ModelClient: model,
		Validator:   val,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !state.IsComplete {
		t.Fatal("expected state to be complete after validator pass")
	}
	if len(state.Steps) != 2 || len(state.Checkpoints) != 2 {
		t.Fatalf("expected 2 steps and checkpoints, got %d / %d", len(state.Steps), len(state.Checkpoints))
	}
}

func TestAgentExecutorEnforcesStepBudget(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1000, 0)
	clock := func() time.Time { return now }

	exec := NewExecutor(Options{Now: clock})
	model := &mockModelClient{}
	val := &mockValidator{passOnTurn: 10} // never passes within 3 steps

	_, err := exec.Execute(ctx, ExecuteParams{
		RunID:         "ar_budget",
		AgentID:       "coder",
		InitialPrompt: "Infinite loop task",
		Limits: budget.Limits{
			MaxSteps: 3,
		},
		ModelClient: model,
		Validator:   val,
	})
	if err == nil || !errors.Is(err, budget.ErrStepBudgetExceeded) {
		t.Fatalf("expected ErrStepBudgetExceeded, got %v", err)
	}
}
