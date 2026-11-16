//go:build windows

package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func verifyLocksDirHandle(dirFile *os.File, expectedDirAbs string) error {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(dirFile.Fd()), &info); err != nil {
		return fmt.Errorf("inspect pinned locks directory handle: %w", err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("locks directory is redirected or not a directory: %w", blerrors.ErrValidation)
	}
	finalPath, err := shipmentReconcileWindowsFinalPath(windows.Handle(dirFile.Fd()))
	if err != nil {
		return fmt.Errorf("resolve pinned locks directory path: %w", err)
	}
	canonExpected := filepath.Clean(expectedDirAbs)
	dirPtr, err := windows.UTF16PtrFromString(canonExpected)
	if err != nil {
		return fmt.Errorf("encode expected locks directory %q: %w", canonExpected, err)
	}
	expectedHandle, err := windows.CreateFile(
		dirPtr,
		windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return fmt.Errorf("open expected locks directory %q: %w", canonExpected, err)
	}
	defer func() {
		// Verification handle; close immediately.
		_ = windows.CloseHandle(expectedHandle)
	}()

	var expectedInfo windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(expectedHandle, &expectedInfo); err != nil {
		return fmt.Errorf("inspect expected locks directory %q: %w", canonExpected, err)
	}
	if expectedInfo.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		expectedInfo.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("locks directory %q is redirected or not a directory: %w", canonExpected, blerrors.ErrValidation)
	}
	expectedFinalPath, err := shipmentReconcileWindowsFinalPath(expectedHandle)
	if err != nil {
		return fmt.Errorf("resolve expected locks directory %q: %w", canonExpected, err)
	}
	if expectedInfo.VolumeSerialNumber != info.VolumeSerialNumber ||
		expectedInfo.FileIndexHigh != info.FileIndexHigh ||
		expectedInfo.FileIndexLow != info.FileIndexLow ||
		!strings.EqualFold(filepath.Clean(finalPath), filepath.Clean(expectedFinalPath)) {
		return fmt.Errorf("locks directory %q redirected to %q: %w", expectedFinalPath, finalPath, blerrors.ErrValidation)
	}
	return nil
}

func verifyLocksLeafHandle(f *os.File) error {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info); err != nil {
		return fmt.Errorf("inspect locks leaf handle: %w", err)
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		return fmt.Errorf("locks leaf is a reparse point or directory: %w", blerrors.ErrValidation)
	}
	return nil
}

func tryLockAllocatorFileHandle(f *os.File) (bool, error) {
	var ov windows.Overlapped
	err := windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&ov,
	)
	if err == nil {
		return false, nil
	}
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return true, nil
	}
	return false, fmt.Errorf("lock id allocator sidecar: %w", err)
}

func unlockAllocatorFileHandle(f *os.File) error {
	var ov windows.Overlapped
	if err := windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ov); err != nil {
		return fmt.Errorf("unlock id allocator sidecar: %w", err)
	}
	return nil
}
