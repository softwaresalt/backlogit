//go:build windows

package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

const (
	shipmentReconcileLockSharingViolation = syscall.Errno(32)
	shipmentReconcileLockViolation        = syscall.Errno(33)
)

// openShipmentReconcileLockHandleRelative opens the item-log lock (C)
// sidecar on Windows with the strongest practical hardening available
// without depending on NT-native APIs this codebase does not otherwise use:
// it first opens and validates the namespace directory itself (rejecting a
// reparse point there via a post-open attribute check, matching
// internal/events/item_log_lock_windows.go's own documented best-effort
// pattern), then opens the sidecar file with
// FILE_FLAG_OPEN_REPARSE_POINT so a reparse point at the sidecar's own name
// is opened as the reparse object itself rather than transparently
// followed, and is rejected by the same post-open attribute check.
//
// Windows has no POSIX openat/renameat-style directory-relative file
// create in the standard syscall surface, so this is not a TRUE
// handle-relative open the way the Unix implementation is (167.011-T,
// parity note, mirrors the same documented Windows/Unix asymmetry already
// established in internal/events/checkpoint_readnofollow_windows.go and
// internal/events/item_log_lock_windows.go). Both platforms still target
// the identical sidecar path, so the shared OS-level advisory lock (via
// CreateFile's own sharing-violation semantics) still mutually excludes
// callers regardless of which platform's open path is used.
func openShipmentReconcileLockHandleRelative(namespaceDir, fileName string) (*os.File, bool, error) {
	if reparse, err := isReparsePointPath(namespaceDir); err != nil {
		return nil, false, fmt.Errorf("stat item log lock namespace directory %s: %w", namespaceDir, err)
	} else if reparse {
		return nil, false, fmt.Errorf("%w: item log lock namespace directory %s is a reparse point", blerrors.ErrValidation, namespaceDir)
	}

	lockPath := filepath.Join(namespaceDir, fileName)
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
		if errors.Is(err, shipmentReconcileLockSharingViolation) || errors.Is(err, shipmentReconcileLockViolation) {
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

// isReparsePointPath reports whether path is (or resolves through) a
// reparse point, using Lstat's ModeSymlink plus a FILE_ATTRIBUTE_REPARSE_POINT
// check — the same detection internal/events/checkpoint_readnofollow_windows.go
// already establishes. A non-existent path is not a reparse point.
func isReparsePointPath(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return true, nil
	}
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false, nil
	}
	attrs, err := syscall.GetFileAttributes(pathPtr)
	if err != nil {
		return false, nil
	}
	return attrs&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}
