//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package events

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func openItemLogLockHandle(lockPath string) (*os.File, bool, error) {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, fmt.Errorf("open item log lock sidecar %s: %w", lockPath, err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		closeErr := file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, true, nil
		}
		if closeErr != nil {
			return nil, false, errors.Join(err, closeErr)
		}
		return nil, false, fmt.Errorf("lock item log sidecar %s: %w", lockPath, err)
	}
	return file, false, nil
}
