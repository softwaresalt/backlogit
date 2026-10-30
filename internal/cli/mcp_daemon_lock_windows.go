//go:build windows

package cli

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
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
