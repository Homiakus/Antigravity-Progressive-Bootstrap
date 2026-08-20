//go:build windows

package process

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsProcessTree struct {
	job windows.Handle
	pid uint32
}

func configureCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}

func attachProcessTree(cmd *exec.Cmd) (processTree, error) {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return nil, fmt.Errorf("started process has no pid")
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("configure job object: %w", err)
	}
	processHandle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		uint32(cmd.Process.Pid),
	)
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("open process for job assignment: %w", err)
	}
	assignErr := windows.AssignProcessToJobObject(job, processHandle)
	_ = windows.CloseHandle(processHandle)
	if assignErr != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("assign process to job object: %w", assignErr)
	}
	return &windowsProcessTree{job: job, pid: uint32(cmd.Process.Pid)}, nil
}

func (t *windowsProcessTree) SoftCancel() error {
	if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, t.pid); err != nil {
		return fmt.Errorf("send CTRL_BREAK to process group %d: %w", t.pid, err)
	}
	return nil
}

func (t *windowsProcessTree) GracefulTerminate() error {
	return t.SoftCancel()
}

func (t *windowsProcessTree) HardKill() error {
	if t.job == 0 {
		return nil
	}
	if err := windows.TerminateJobObject(t.job, 1); err != nil {
		return fmt.Errorf("terminate job object: %w", err)
	}
	return nil
}

func (t *windowsProcessTree) Close() error {
	if t.job == 0 {
		return nil
	}
	err := windows.CloseHandle(t.job)
	t.job = 0
	return err
}
