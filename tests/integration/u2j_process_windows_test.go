package integration_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type u2jWindowsLease struct {
	mu        sync.Mutex
	job       windows.Handle
	process   windows.Handle
	pid       uint32
	cancelled bool
	closed    bool
	cancelErr error
	proof     string
	closeErr  error
}

func u2jNewProcessLease(cmd *exec.Cmd) (u2jProcessLease, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("u2j create owned job: %w", err)
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	// These test-only pinned pointers marshal fixed Windows ABI structures through
	// the SDK's uintptr parameters; neither synchronous API retains a Go pointer.
	var pinned runtime.Pinner
	pinned.Pin(&limits)
	defer pinned.Unpin()
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)))
	if err != nil {
		return nil, errors.Join(fmt.Errorf("u2j configure owned job: %w", err), windows.CloseHandle(job))
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
	return &u2jWindowsLease{job: job}, nil
}

func (l *u2jWindowsLease) start(cmd *exec.Cmd) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := cmd.Start(); err != nil {
		return err
	}
	l.pid = uint32(cmd.Process.Pid)
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|
		windows.SYNCHRONIZE, false, l.pid)
	if err != nil {
		return errors.Join(fmt.Errorf("u2j open suspended parent %d: %w", l.pid, err), cmd.Process.Kill())
	}
	l.process = process
	if err := windows.AssignProcessToJobObject(l.job, process); err != nil {
		// No user code has run, so this exact suspended parent cannot have children.
		return errors.Join(fmt.Errorf("u2j assign suspended parent %d: %w", l.pid, err), cmd.Process.Kill())
	}
	if err := u2jResumeOwnedThread(l.pid); err != nil {
		return err
	}
	return nil
}

func u2jResumeOwnedThread(pid uint32) (resultErr error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("u2j snapshot suspended parent threads: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, windows.CloseHandle(snapshot)) }()
	entry := windows.ThreadEntry32{}
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, openErr := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if openErr != nil {
			return fmt.Errorf("u2j open owned primary thread: %w", openErr)
		}
		previous, resumeErr := windows.ResumeThread(thread)
		closeErr := windows.CloseHandle(thread)
		if resumeErr != nil {
			return errors.Join(fmt.Errorf("u2j resume owned primary thread: %w", resumeErr), closeErr)
		}
		if previous != 1 {
			return errors.Join(fmt.Errorf("u2j unexpected primary thread suspension count: %d", previous), closeErr)
		}
		return closeErr
	}
	return fmt.Errorf("u2j locate owned primary thread for %d: %w", pid, err)
}

func (l *u2jWindowsLease) waitParent() error {
	state, err := windows.WaitForSingleObject(l.process, windows.INFINITE)
	if err != nil {
		return fmt.Errorf("u2j wait owned parent %d: %w", l.pid, err)
	}
	if state != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("u2j owned parent wait state: %d", state)
	}
	return nil
}

func (l *u2jWindowsLease) cancel() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return os.ErrProcessDone
	}
	return l.cancelLocked()
}

func (l *u2jWindowsLease) cancelLocked() error {
	if !l.cancelled {
		l.cancelled = true
		if err := windows.TerminateJobObject(l.job, 1); err != nil {
			l.cancelErr = fmt.Errorf("u2j terminate owned job for %d: %w", l.pid, err)
		}
	}
	return l.cancelErr
}

// u2jJobAccounting matches JOBOBJECT_BASIC_ACCOUNTING_INFORMATION's fixed ABI.
type u2jJobAccounting struct {
	UserTime       int64
	KernelTime     int64
	PeriodUser     int64
	PeriodKernel   int64
	PageFaults     uint32
	TotalProcesses uint32
	Active         uint32
	Terminated     uint32
}

func (l *u2jWindowsLease) close() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return l.proof, l.closeErr
	}
	l.closed = true
	terminateErr := l.cancelLocked()
	deadline := time.Now().Add(500 * time.Millisecond)
	var accounting u2jJobAccounting
	var pinned runtime.Pinner
	pinned.Pin(&accounting)
	defer pinned.Unpin()
	var queryErr error
	for {
		var length uint32
		queryErr = windows.QueryInformationJobObject(l.job, windows.JobObjectBasicAccountingInformation,
			uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), &length)
		if queryErr != nil {
			queryErr = fmt.Errorf("u2j query owned job disposal: %w", queryErr)
			break
		}
		if length != uint32(unsafe.Sizeof(accounting)) {
			queryErr = fmt.Errorf("u2j owned job accounting length: %d", length)
			break
		}
		if accounting.Active == 0 {
			break
		}
		if !time.Now().Before(deadline) {
			queryErr = fmt.Errorf("u2j owned job disposal deadline: %d active processes", accounting.Active)
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	jobErr := windows.CloseHandle(l.job)
	var processErr error
	if l.process != 0 {
		processErr = windows.CloseHandle(l.process)
	}
	l.proof = fmt.Sprintf("job_total=%d; job_active=%d; job_terminated=%d; job_closed=%t",
		accounting.TotalProcesses, accounting.Active, accounting.Terminated, jobErr == nil)
	l.closeErr = errors.Join(terminateErr, queryErr, jobErr, processErr)
	return l.proof, l.closeErr
}
