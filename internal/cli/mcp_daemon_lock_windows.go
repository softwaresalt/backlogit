//go:build windows

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// tryLockFile takes an exclusive non-blocking lock on f and reports whether it
// was acquired.
func tryLockFile(f *os.File) (bool, error) {
	ol := new(windows.Overlapped)
	err := windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
	if err == nil {
		return true, nil
	}
	if err == windows.ERROR_LOCK_VIOLATION || err == windows.ERROR_IO_PENDING {
		return false, nil
	}
	return false, fmt.Errorf("lock file: %w", err)
}

// unlockFile releases a lock taken by tryLockFile.
func unlockFile(f *os.File) error {
	ol := new(windows.Overlapped)
	if err := windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol); err != nil {
		return fmt.Errorf("unlock file: %w", err)
	}
	return nil
}

// processStartTime returns the OS creation time of pid in Unix nanoseconds.
func processStartTime(pid int) (int64, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0, fmt.Errorf("open process %d: %w", pid, err)
	}
	defer func() { _ = windows.CloseHandle(h) }()
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return 0, fmt.Errorf("get process times: %w", err)
	}
	return time.Unix(0, creation.Nanoseconds()).UnixNano(), nil
}

// terminateVerifiedProcess verifies the process identity and terminates it
// through one retained handle, so a recycled PID is never signaled.
func terminateVerifiedProcess(pid int, startTime int64, exePath string, reverify func() error) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		if err == windows.ERROR_INVALID_PARAMETER {
			return nil
		}
		return fmt.Errorf("open process %d: %w", pid, err)
	}
	defer func() { _ = windows.CloseHandle(h) }()

	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return fmt.Errorf("get process times: %w", err)
	}
	if got := time.Unix(0, creation.Nanoseconds()).UnixNano(); got != startTime {
		return fmt.Errorf("%w: process %d start time does not match daemon metadata", corerrors.ErrValidation, pid)
	}
	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return fmt.Errorf("query image path: %w", err)
	}
	image := windows.UTF16ToString(buf[:size])
	if !strings.EqualFold(filepath.Clean(image), filepath.Clean(exePath)) {
		return fmt.Errorf("%w: process %d image path does not match daemon metadata", corerrors.ErrValidation, pid)
	}
	if err := reverify(); err != nil {
		return err
	}
	if err := windows.TerminateProcess(h, 1); err != nil {
		if ev, werr := windows.WaitForSingleObject(h, 0); werr == nil && ev == windows.WAIT_OBJECT_0 {
			return nil
		}
		return fmt.Errorf("terminate process %d: %w", pid, err)
	}
	return nil
}
