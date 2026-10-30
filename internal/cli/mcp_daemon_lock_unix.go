//go:build !windows

package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// tryLockFile takes an exclusive non-blocking lock on f and reports whether it
// was acquired.
func tryLockFile(f *os.File) (bool, error) {
	for {
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		switch {
		case err == nil:
			return true, nil
		case errors.Is(err, unix.EINTR):
			continue
		case errors.Is(err, unix.EWOULDBLOCK):
			return false, nil
		default:
			return false, fmt.Errorf("flock: %w", err)
		}
	}
}

// unlockFile releases a lock taken by tryLockFile.
func unlockFile(f *os.File) error {
	if err := unix.Flock(int(f.Fd()), unix.LOCK_UN); err != nil {
		return fmt.Errorf("unlock file: %w", err)
	}
	return nil
}

// processStartTime returns a start-time token for pid, read from /proc where
// available and otherwise 1 so the sidecar value stays non-zero.
func processStartTime(pid int) (int64, error) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 1, nil
	}
	s := string(data)
	end := strings.LastIndexByte(s, ')')
	if end < 0 {
		return 1, nil
	}
	fields := strings.Fields(s[end+1:])
	// starttime is field 22 overall; fields[0] is field 3.
	if len(fields) < 20 {
		return 1, nil
	}
	v, err := strconv.ParseInt(fields[19], 10, 64)
	if err != nil || v == 0 {
		return 1, nil
	}
	return v, nil
}
