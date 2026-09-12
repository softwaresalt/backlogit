//go:build windows

package events

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

const (
	itemLogLockSharingViolation = syscall.Errno(32)
	itemLogLockViolation        = syscall.Errno(33)
)

// openItemLogLockHandle opens (creating if absent) the item-log lock (C)
// sidecar with FILE_FLAG_OPEN_REPARSE_POINT so a reparse point (symlink or
// junction) at lockPath is opened as the reparse point object itself rather
// than transparently followed to its target, then rejects the handle with a
// post-open attribute check — the same best-effort no-follow pattern already
// established for checkpoint reads (checkpoint_readnofollow_windows.go),
// applied here to lock-file opens (167.017-T, requirement (b): handle-bound,
// reparse/no-follow-safe semantics).
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
		syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		if errors.Is(err, itemLogLockSharingViolation) || errors.Is(err, itemLogLockViolation) {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("open item log lock sidecar %s: %w", lockPath, err)
	}
	var info syscall.ByHandleFileInformation
	if infoErr := syscall.GetFileInformationByHandle(handle, &info); infoErr != nil {
		_ = syscall.CloseHandle(handle)
		return nil, false, fmt.Errorf("stat item log lock sidecar %s: %w", lockPath, infoErr)
	}
	if info.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = syscall.CloseHandle(handle)
		return nil, false, fmt.Errorf("%w: item log lock sidecar %s is a reparse point", blerrors.ErrValidation, lockPath)
	}
	return os.NewFile(uintptr(handle), lockPath), false, nil
}
