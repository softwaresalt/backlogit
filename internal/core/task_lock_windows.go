//go:build windows

package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	taskLockSharingViolation = syscall.Errno(32)
	taskLockViolation        = syscall.Errno(33)
)

func openTaskLockHandle(lockPath string) (*os.File, bool, error) {
	return openExclusiveLockHandle(lockPath, syscall.GENERIC_READ|syscall.GENERIC_WRITE)
}

func openStashLockHandle(lockPath string) (*stashLockFile, bool, error) {
	// Expand 8.3 components without resolving link targets. EvalSymlinks here
	// would bless an ancestor redirected between validation and pinning.
	// The pin must still compare the opened final path with this pathname.
	dir, err := windows.UTF16PtrFromString(filepath.Dir(lockPath))
	if err != nil {
		return nil, false, fmt.Errorf("encode stash sidecar directory: %w", err)
	}
	size, err := windows.GetLongPathName(dir, nil, 0)
	if err != nil {
		return nil, false, fmt.Errorf("size long stash sidecar directory: %w", err)
	}
	if size == 0 {
		return nil, false, fmt.Errorf("empty long stash sidecar directory: %w", os.ErrPermission)
	}
	buffer := make([]uint16, size)
	length, err := windows.GetLongPathName(dir, &buffer[0], size)
	if err != nil {
		return nil, false, fmt.Errorf("expand stash sidecar directory: %w", err)
	}
	if length == 0 || length >= size {
		return nil, false, fmt.Errorf("stash sidecar directory changed during expansion: %w", os.ErrPermission)
	}
	lockPath = filepath.Join(windows.UTF16ToString(buffer), filepath.Base(lockPath))
	pin, err := pinShipmentReconcileDir(filepath.Dir(lockPath))
	if err != nil {
		return nil, false, fmt.Errorf("pin stash sidecar directory: %w", err)
	}
	file, err := pin.createFile(filepath.Base(lockPath),
		windows.GENERIC_READ|windows.GENERIC_WRITE|windows.DELETE, windows.FILE_OPEN_IF)
	closeErr := pin.close()
	if err != nil {
		if closeErr == nil && (errors.Is(err, windows.STATUS_SHARING_VIOLATION) ||
			errors.Is(err, windows.STATUS_FILE_LOCK_CONFLICT)) {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("open pinned stash sidecar: %w", errors.Join(err, closeErr))
	}
	owned := &stashLockFile{File: file, basename: filepath.Base(lockPath)}
	fail := func(err error) (*stashLockFile, bool, error) {
		return nil, false, errors.Join(err, owned.Close())
	}
	if closeErr != nil {
		return fail(fmt.Errorf("close stash directory pin: %w", closeErr))
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &info); err != nil {
		return fail(fmt.Errorf("stat stash sidecar handle: %w", err))
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 ||
		info.NumberOfLinks != 1 {
		return fail(fmt.Errorf("linked or nonregular stash sidecar handle: %w", os.ErrPermission))
	}
	finalPath, err := shipmentReconcileWindowsFinalPath(windows.Handle(file.Fd()))
	if err != nil {
		return fail(fmt.Errorf("resolve stash sidecar handle: %w", err))
	}
	if !strings.EqualFold(finalPath, filepath.Clean(lockPath)) {
		return fail(fmt.Errorf("stash sidecar handle escaped destination: %w", os.ErrPermission))
	}
	return owned, false, nil
}

func openExclusiveLockHandle(lockPath string, access uint32) (*os.File, bool, error) {
	name, err := syscall.UTF16PtrFromString(lockPath)
	if err != nil {
		return nil, false, fmt.Errorf("resolve task lock sidecar %s: %w", lockPath, err)
	}

	handle, err := syscall.CreateFile(
		name,
		access,
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

// releaseStashSidecarFile deletes the owned file on close, not a successor.
func releaseStashSidecarFile(file *stashLockFile) error {
	deleteFile := byte(1)
	err := windows.SetFileInformationByHandle(windows.Handle(file.Fd()),
		windows.FileDispositionInfo, &deleteFile, 1)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return fmt.Errorf("release stash sidecar: %w", errors.Join(err, closeErr))
	}
	return nil
}
