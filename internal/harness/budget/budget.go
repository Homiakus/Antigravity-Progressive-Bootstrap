package budget

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrStepBudgetExceeded      = errors.New("harness budget: max steps exceeded")
	ErrModelCallBudgetExceeded = errors.New("harness budget: max model calls exceeded")
	ErrToolCallBudgetExceeded  = errors.New("harness budget: max tool calls exceeded")
	ErrTokenBudgetExceeded     = errors.New("harness budget: max tokens exceeded")
	ErrCostBudgetExceeded      = errors.New("harness budget: max cost exceeded")
	ErrDeadlineExceeded        = errors.New("harness budget: wall-clock deadline exceeded")
	ErrFailureBudgetExceeded   = errors.New("harness budget: failure budget exceeded")
)

type Limits struct {
	MaxSteps      int           `json:"maxSteps,omitempty"`
	MaxModelCalls int           `json:"maxModelCalls,omitempty"`
	MaxToolCalls  int           `json:"maxToolCalls,omitempty"`
	MaxTokens     int64         `json:"maxTokens,omitempty"`
	MaxCostUSD    float64       `json:"maxCostUSD,omitempty"`
	Timeout       time.Duration `json:"timeout,omitempty"`
	FailureBudget int           `json:"failureBudget,omitempty"`
}

type Usage struct {
	Steps      int       `json:"steps"`
	ModelCalls int       `json:"modelCalls"`
	ToolCalls  int       `json:"toolCalls"`
	Tokens     int64     `json:"tokens"`
	CostUSD    float64   `json:"costUSD"`
	Failures   int       `json:"failures"`
	StartedAt  time.Time `json:"startedAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type Tracker struct {
	mu     sync.RWMutex
	limits Limits
	usage  Usage
	now    func() time.Time
}

func NewTracker(limits Limits, now func() time.Time) *Tracker {
	if now == nil {
		now = time.Now
	}
	startTime := now().UTC()
	return &Tracker{
		limits: limits,
		usage: Usage{
			StartedAt: startTime,
			UpdatedAt: startTime,
		},
		now: now,
	}
}

func (t *Tracker) RecordStep() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.checkDeadlineLocked(); err != nil {
		return err
	}
	if t.limits.MaxSteps > 0 && t.usage.Steps >= t.limits.MaxSteps {
		return fmt.Errorf("%w (%d >= %d)", ErrStepBudgetExceeded, t.usage.Steps+1, t.limits.MaxSteps)
	}
	t.usage.Steps++
	t.usage.UpdatedAt = t.now().UTC()
	return nil
}

func (t *Tracker) RecordModelCall(tokens int64, costUSD float64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.checkDeadlineLocked(); err != nil {
		return err
	}
	if t.limits.MaxModelCalls > 0 && t.usage.ModelCalls >= t.limits.MaxModelCalls {
		return fmt.Errorf("%w (%d >= %d)", ErrModelCallBudgetExceeded, t.usage.ModelCalls+1, t.limits.MaxModelCalls)
	}
	if t.limits.MaxTokens > 0 && t.usage.Tokens+tokens > t.limits.MaxTokens {
		return fmt.Errorf("%w (%d > %d)", ErrTokenBudgetExceeded, t.usage.Tokens+tokens, t.limits.MaxTokens)
	}
	if t.limits.MaxCostUSD > 0 && t.usage.CostUSD+costUSD > t.limits.MaxCostUSD {
		return fmt.Errorf("%w (%.4f > %.4f)", ErrCostBudgetExceeded, t.usage.CostUSD+costUSD, t.limits.MaxCostUSD)
	}

	t.usage.ModelCalls++
	t.usage.Tokens += tokens
	t.usage.CostUSD += costUSD
	t.usage.UpdatedAt = t.now().UTC()
	return nil
}

func (t *Tracker) RecordToolCall() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.checkDeadlineLocked(); err != nil {
		return err
	}
	if t.limits.MaxToolCalls > 0 && t.usage.ToolCalls >= t.limits.MaxToolCalls {
		return fmt.Errorf("%w (%d >= %d)", ErrToolCallBudgetExceeded, t.usage.ToolCalls+1, t.limits.MaxToolCalls)
	}
	t.usage.ToolCalls++
	t.usage.UpdatedAt = t.now().UTC()
	return nil
}

func (t *Tracker) RecordFailure() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.usage.Failures++
	t.usage.UpdatedAt = t.now().UTC()
	if t.limits.FailureBudget > 0 && t.usage.Failures > t.limits.FailureBudget {
		return fmt.Errorf("%w (%d > %d)", ErrFailureBudgetExceeded, t.usage.Failures, t.limits.FailureBudget)
	}
	return nil
}

func (t *Tracker) Check() error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.checkDeadlineLocked()
}

func (t *Tracker) checkDeadlineLocked() error {
	if t.limits.Timeout > 0 {
		elapsed := t.now().UTC().Sub(t.usage.StartedAt)
		if elapsed > t.limits.Timeout {
			return fmt.Errorf("%w (%s > %s)", ErrDeadlineExceeded, elapsed, t.limits.Timeout)
		}
	}
	return nil
}

func (t *Tracker) GetUsage() Usage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.usage
}
