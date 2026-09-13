//go:build windows

package events

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

const (
	itemLogLockSharingViolation = syscall.Errno(32)
	itemLogLockViolation        = syscall.Errno(33)
)

// itemLogLockNamespaceIsReparsePoint reports whether path — the item-log
// lock (C) namespace directory — is ITSELF a reparse point (Lstat's
// ModeSymlink first, then a FILE_ATTRIBUTE_REPARSE_POINT fallback via the
// existing isReparsePoint helper already established in
// checkpoint_readnofollow_windows.go), mirroring
// internal/core/shipment_reconcile_lock_windows.go's isReparsePointPath
// check byte-for-byte.
//
// Scope correction (Copilot PR #440 review, finding 2): this checks ONLY
// path's own final path component — it does NOT walk up path's ancestors,
// despite the prior wording here suggesting a fuller "resolves through"
// check. An ancestor directory of the namespace directory (e.g. the
// caller-supplied locks root itself, or any intermediate segment) that is
// swapped for a reparse point is NOT detected by this function; a
// symlink/junction planted at one of those ancestors could still silently
// redirect the open. This mirrors the same documented, accepted scope
// bound events.ItemLogLockPath's own doc comment already calls out for the
// caller-supplied locksRoot (it does not defend against an adversarial
// rename/symlink-swap of locksRoot itself either) — a repository-wide
// threat-model boundary, not a gap specific to this one check. A
// non-existent path is not a reparse point.
func itemLogLockNamespaceIsReparsePoint(path string) (bool, error) {
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
	return isReparsePoint(path), nil
}

// openItemLogLockHandle opens (creating if absent) the item-log lock (C)
// sidecar with FILE_FLAG_OPEN_REPARSE_POINT so a reparse point (symlink or
// junction) at lockPath is opened as the reparse point object itself rather
// than transparently followed to its target, then rejects the handle with a
// post-open attribute check — the same best-effort no-follow pattern already
// established for checkpoint reads (checkpoint_readnofollow_windows.go),
// applied here to lock-file opens (167.017-T, requirement (b): handle-bound,
// reparse/no-follow-safe semantics).
//
// Parity note (Copilot PR #440 review): this now ALSO validates the
// namespace (parent) directory for a reparse point immediately before the
// open, achieving parity with
// internal/core/shipment_reconcile_lock_windows.go's
// openShipmentReconcileLockHandleRelative — the two are meant to be the SAME
// underlying lock (167.017-T), so both sides must apply the identical
// best-effort hardening or a swapped namespace directory could silently
// redirect only one of the two acquirers, defeating their mutual exclusion.
// A true handle-relative open (directory opened and validated first, sidecar
// opened relative to that verified handle) requires NT-native APIs this
// codebase does not otherwise depend on — the same documented Windows/Unix
// asymmetry shipment_reconcile_lock_windows.go already carries.
func openItemLogLockHandle(lockPath string) (*os.File, bool, error) {
	namespaceDir := filepath.Dir(lockPath)
	if reparse, err := itemLogLockNamespaceIsReparsePoint(namespaceDir); err != nil {
		return nil, false, fmt.Errorf("stat item log lock namespace directory %s: %w", namespaceDir, err)
	} else if reparse {
		return nil, false, fmt.Errorf("%w: item log lock namespace directory %s is a reparse point", blerrors.ErrValidation, namespaceDir)
	}

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
