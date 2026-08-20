package process

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

var errStartTimeout = errors.New("process executor: start timeout")

// start keeps Cmd.Start synchronous. The OS process creation call is expected to
// be short; after it returns we enforce the configured start budget and tear
// down the just-created process tree before returning a timeout. This avoids a
// dangerous late-start goroutine that could outlive closed log channels.
func (e *Executor) start(ctx context.Context, cmd *exec.Cmd, timeout time.Duration) (time.Time, processTree, error) {
	if err := ctx.Err(); err != nil {
		return time.Time{}, nil, err
	}
	realStart := time.Now()
	if err := cmd.Start(); err != nil {
		return time.Time{}, nil, err
	}
	startedAt := e.now().UTC()
	tree, err := attachProcessTree(cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return time.Time{}, nil, fmt.Errorf("attach process tree: %w", err)
	}
	if err := ctx.Err(); err != nil {
		_ = tree.HardKill()
		_ = cmd.Wait()
		_ = tree.Close()
		return time.Time{}, nil, err
	}
	if timeout > 0 && time.Since(realStart) > timeout {
		_ = tree.HardKill()
		_ = cmd.Wait()
		_ = tree.Close()
		return time.Time{}, nil, fmt.Errorf("%w after %s", errStartTimeout, timeout)
	}
	return startedAt, tree, nil
}
