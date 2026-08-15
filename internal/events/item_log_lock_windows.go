//go:build windows

package events

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

const (
	itemLogLockSharingViolation = syscall.Errno(32)
	itemLogLockViolation        = syscall.Errno(33)
)

func openItemLogLockHandle(lockPath string) (*os.File, bool, error) {
	name, err := syscall.UTF16PtrFromString(lockPath)
	if err != nil {
		return nil, false, fmt.Errorf("resolve item log lock sidecar %s: %w", lockPath, err)
	}
	handle, err := syscall.CreateFile(
		name,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0,
		nil,
		syscall.OPEN_ALWAYS,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		if errors.Is(err, itemLogLockSharingViolation) || errors.Is(err, itemLogLockViolation) {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("open item log lock sidecar %s: %w", lockPath, err)
	}
	return os.NewFile(uintptr(handle), lockPath), false, nil
}
