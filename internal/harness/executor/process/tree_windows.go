//go:build windows

package process

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsProcessTree struct {
	mu  sync.Mutex
	job windows.Handle
	pid uint32
}

func configureCommand(cmd *exec.Cmd) {
	// The child starts suspended so it cannot spawn an unowned grandchild in
	// the interval between CreateProcess and AssignProcessToJobObject. Once the
	// Job Object owns it, attachProcessTree resumes its primary thread.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_SUSPENDED}
}

func attachProcessTree(cmd *exec.Cmd) (processTree, error) {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return nil, fmt.Errorf("started process has no pid")
	}
	pid := uint32(cmd.Process.Pid)
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
		pid,
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
	if err := resumeSuspendedProcess(pid); err != nil {
		_ = windows.TerminateJobObject(job, 1)
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("resume job-owned process: %w", err)
	}
	return &windowsProcessTree{job: job, pid: pid}, nil
}

func resumeSuspendedProcess(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("snapshot threads: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	if err := windows.Thread32First(snapshot, &entry); err != nil {
		return fmt.Errorf("enumerate first thread: %w", err)
	}
	for {
		if entry.OwnerProcessID == pid {
			thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
			if err != nil {
				return fmt.Errorf("open primary thread %d: %w", entry.ThreadID, err)
			}
			_, resumeErr := windows.ResumeThread(thread)
			_ = windows.CloseHandle(thread)
			if resumeErr != nil {
				return fmt.Errorf("resume primary thread %d: %w", entry.ThreadID, resumeErr)
			}
			return nil
		}
		if err := windows.Thread32Next(snapshot, &entry); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				return fmt.Errorf("no thread found for suspended process %d", pid)
			}
			return fmt.Errorf("enumerate process threads: %w", err)
		}
	}
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
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.job == 0 {
		return nil
	}
	if err := windows.TerminateJobObject(t.job, 1); err != nil {
		return fmt.Errorf("terminate job object: %w", err)
	}
	return nil
}

func (t *windowsProcessTree) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.job == 0 {
		return nil
	}
	err := windows.CloseHandle(t.job)
	t.job = 0
	return err
}
