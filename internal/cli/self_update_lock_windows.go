//go:build windows

package cli

import (
	"errors"
	"os"
	"syscall"
)

const (
	selfUpdateSharingViolation = syscall.Errno(32)
	selfUpdateLockViolation    = syscall.Errno(33)
)

func openSelfUpdateLockFile(lockPath string) (*os.File, bool, error) {
	name, err := syscall.UTF16PtrFromString(lockPath)
	if err != nil {
		return nil, false, err
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
		if errors.Is(err, selfUpdateSharingViolation) || errors.Is(err, selfUpdateLockViolation) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return os.NewFile(uintptr(handle), lockPath), true, nil
}
