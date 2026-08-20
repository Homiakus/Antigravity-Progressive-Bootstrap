//go:build !windows

package process

import (
	"fmt"
	"os/exec"
	"syscall"
)

type unixProcessTree struct {
	pid int
}

func configureCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func attachProcessTree(cmd *exec.Cmd) (processTree, error) {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return nil, fmt.Errorf("started process has no pid")
	}
	return &unixProcessTree{pid: cmd.Process.Pid}, nil
}

func (t *unixProcessTree) SoftCancel() error {
	return signalProcessGroup(t.pid, syscall.SIGINT)
}

func (t *unixProcessTree) GracefulTerminate() error {
	return signalProcessGroup(t.pid, syscall.SIGTERM)
}

func (t *unixProcessTree) HardKill() error {
	return signalProcessGroup(t.pid, syscall.SIGKILL)
}

func (t *unixProcessTree) Close() error { return nil }

func signalProcessGroup(pid int, signal syscall.Signal) error {
	if pid <= 0 {
		return fmt.Errorf("invalid process group pid %d", pid)
	}
	if err := syscall.Kill(-pid, signal); err != nil && err != syscall.ESRCH {
		return err
	}
	return nil
}
