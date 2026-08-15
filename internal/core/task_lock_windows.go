//go:build windows

package core

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

const (
	taskLockSharingViolation = syscall.Errno(32)
	taskLockViolation        = syscall.Errno(33)
)

func openTaskLockHandle(lockPath string) (*os.File, bool, error) {
	name, err := syscall.UTF16PtrFromString(lockPath)
	if err != nil {
		return nil, false, fmt.Errorf("resolve task lock sidecar %s: %w", lockPath, err)
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
		if errors.Is(err, taskLockSharingViolation) || errors.Is(err, taskLockViolation) {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("open task lock sidecar %s: %w", lockPath, err)
	}
	return os.NewFile(uintptr(handle), lockPath), false, nil
}
