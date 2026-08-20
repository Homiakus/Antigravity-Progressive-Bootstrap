package process

import "os/exec"

type processTree interface {
	SoftCancel() error
	GracefulTerminate() error
	HardKill() error
	Close() error
}

// configureCommand applies OS-specific creation flags before Cmd.Start.
func configureCommand(cmd *exec.Cmd)

// attachProcessTree establishes ownership of the started process tree.
func attachProcessTree(cmd *exec.Cmd) (processTree, error)
