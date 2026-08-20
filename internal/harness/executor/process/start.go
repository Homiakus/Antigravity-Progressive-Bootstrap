package process

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

var errStartTimeout = errors.New("process executor: start timeout")

type startOutcome struct {
	startedAt time.Time
	tree      processTree
	err       error
}

func (e *Executor) start(ctx context.Context, cmd *exec.Cmd, timeout time.Duration) (time.Time, processTree, error) {
	outcomes := make(chan startOutcome, 1)
	go func() {
		if err := cmd.Start(); err != nil {
			outcomes <- startOutcome{err: err}
			return
		}
		startedAt := e.now().UTC()
		tree, err := attachProcessTree(cmd)
		if err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			outcomes <- startOutcome{startedAt: startedAt, err: fmt.Errorf("attach process tree: %w", err)}
			return
		}
		outcomes <- startOutcome{startedAt: startedAt, tree: tree}
	}()

	var timer *time.Timer
	if timeout > 0 {
		timer = time.NewTimer(timeout)
		defer timer.Stop()
	}
	select {
	case outcome := <-outcomes:
		return outcome.startedAt, outcome.tree, outcome.err
	case <-ctx.Done():
		cleanupLateStart(cmd, outcomes)
		return time.Time{}, nil, ctx.Err()
	case <-timerChan(timer):
		cleanupLateStart(cmd, outcomes)
		return time.Time{}, nil, fmt.Errorf("%w after %s", errStartTimeout, timeout)
	}
}

func cleanupLateStart(cmd *exec.Cmd, outcomes <-chan startOutcome) {
	go func() {
		outcome := <-outcomes
		if outcome.err != nil || outcome.tree == nil {
			return
		}
		_ = outcome.tree.HardKill()
		_ = cmd.Wait()
		_ = outcome.tree.Close()
	}()
}
