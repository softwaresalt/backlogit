//go:build windows

package cli

import (
	"fmt"
	"os"
	"syscall"
)

func holdAdvisoryLock(path string) (func(), error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("lock test sidecar: %w", err)
	}
	file := os.NewFile(uintptr(handle), path)
	return func() { _ = file.Close() }, nil
}
