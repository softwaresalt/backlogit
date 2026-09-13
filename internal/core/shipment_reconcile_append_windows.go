//go:build windows

package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// shipmentReconcileAppendWindowsTOCTOUHook, when non-nil, is invoked exactly
// once per call, immediately after realLogsDir is snapshotted via
// filepath.EvalSymlinks and before the target file is opened. Production
// leaves this nil (a no-op); tests use it to deterministically simulate an
// attacker swapping an intermediate directory within logsDir for a reparse
// point in the narrow window between that snapshot and the open below,
// proving the post-open final-path re-validation actually rejects the
// resulting mismatch rather than existing as unexercised code.
var shipmentReconcileAppendWindowsTOCTOUHook func()

// appendShipmentReconcileEventHandleRelative performs the append on
// Windows. Windows has no POSIX openat-style directory-relative create
// without NT-native APIs this codebase does not otherwise depend on (see
// shipment_reconcile_lock_windows.go and shipment_reconcile_fs_windows.go
// for the same documented asymmetry), so this path validates the logs
// directory and the target file for reparse points, then opens the file
// directly (by path) with FILE_FLAG_OPEN_REPARSE_POINT so an existing
// reparse point at that path is opened as the reparse object itself rather
// than followed, appends eventBytes, and fsyncs.
//
// Copilot PR #440 review hardening: FILE_FLAG_OPEN_REPARSE_POINT alone only
// protects the FINAL path component (logPath itself) from being
// transparently followed; it does nothing to stop an INTERMEDIATE directory
// inside logsDir from being replaced by a reparse point between the
// isReparsePointPath(logsDir) pathname check above and the CreateFile call
// below. This is closed by (1) snapshotting logsDir's fully-resolved real
// path (filepath.EvalSymlinks, which walks every path segment, unlike
// Lstat's final-component-only check) immediately before use, and (2) after
// the open, re-deriving the ACTUAL handle's own resolved final path (the
// same handle-bound re-validation shipment_reconcile_evidence_windows.go's
// closure-evidence read already applies) and failing closed — before
// writing anything — unless that final path still lands directly inside the
// snapshot. Any swap occurring between the snapshot and the open is thereby
// caught after the fact instead of trusted blindly.
func appendShipmentReconcileEventHandleRelative(logsDir, fileName string, eventBytes []byte) (shipmentReconcileAppendResult, error) {
	if reparse, err := isReparsePointPath(logsDir); err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: stat logs directory %s: %w", blerrors.ErrWriteNotApplied, logsDir, err)
	} else if reparse {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: logs directory %s is a reparse point", blerrors.ErrValidation, logsDir)
	}

	realLogsDir, err := filepath.EvalSymlinks(logsDir)
	if err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: resolve logs directory %s: %w", blerrors.ErrWriteNotApplied, logsDir, err)
	}
	if shipmentReconcileAppendWindowsTOCTOUHook != nil {
		shipmentReconcileAppendWindowsTOCTOUHook()
	}

	logPath := filepath.Join(realLogsDir, fileName)
	if reparse, err := isReparsePointPath(logPath); err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: stat item log %s: %w", blerrors.ErrWriteNotApplied, logPath, err)
	} else if reparse {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: item log %s is a reparse point", blerrors.ErrValidation, logPath)
	}

	name, err := syscall.UTF16PtrFromString(logPath)
	if err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: resolve item log %s: %w", blerrors.ErrWriteNotApplied, logPath, err)
	}
	handle, err := syscall.CreateFile(
		name,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ,
		nil,
		syscall.OPEN_ALWAYS,
		syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: open item log %s: %w", blerrors.ErrWriteNotApplied, logPath, err)
	}
	file := os.NewFile(uintptr(handle), logPath)
	defer file.Close()

	// Post-open containment re-validation: reject if the opened object is
	// itself a reparse point (FILE_FLAG_OPEN_REPARSE_POINT opens it as the
	// reparse object rather than following it, so this positively detects
	// that case), and confirm the handle's OWN resolved final path still
	// lands directly inside realLogsDir. Both checks run, and eventBytes is
	// still unwritten, before either can be bypassed.
	var byHandleInfo syscall.ByHandleFileInformation
	if infoErr := syscall.GetFileInformationByHandle(handle, &byHandleInfo); infoErr != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: stat item log handle %s: %w", blerrors.ErrWriteNotApplied, logPath, infoErr)
	}
	if byHandleInfo.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: item log %s is a reparse point", blerrors.ErrValidation, logPath)
	}
	finalPath, err := shipmentReconcileWindowsFinalPath(windows.Handle(handle))
	if err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: resolve final path for item log %s: %w", blerrors.ErrWriteNotApplied, logPath, err)
	}
	if finalPath != realLogsDir && !pathContained(realLogsDir, finalPath) {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: item log %s resolves outside logs directory %s", blerrors.ErrValidation, finalPath, realLogsDir)
	}

	info, err := file.Stat()
	if err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: stat item log %s: %w", blerrors.ErrWriteNotApplied, logPath, err)
	}
	preAppendSize := info.Size()

	if preAppendSize > 0 {
		tail := make([]byte, 1)
		if _, err := file.ReadAt(tail, preAppendSize-1); err != nil {
			return shipmentReconcileAppendResult{}, fmt.Errorf("%w: read trailing byte of item log %s: %w", blerrors.ErrWriteIndeterminate, logPath, err)
		}
		if tail[0] != '\n' {
			return shipmentReconcileAppendResult{}, fmt.Errorf("%w: item log %s has a partial (non-newline-terminated) trailing line; refusing to concatenate", blerrors.ErrWriteIndeterminate, logPath)
		}
	}

	// Seek to end explicitly: CreateFile+OPEN_ALWAYS does not imply O_APPEND
	// semantics on Windows the way it does on POSIX.
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: seek to end of item log %s: %w", blerrors.ErrWriteNotApplied, logPath, err)
	}

	n, writeErr := file.Write(eventBytes)
	if writeErr != nil {
		if n == 0 {
			return shipmentReconcileAppendResult{}, fmt.Errorf("%w: write item log %s: %w", blerrors.ErrWriteNotApplied, logPath, writeErr)
		}
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: partial write to item log %s: %w", blerrors.ErrWriteIndeterminate, logPath, writeErr)
	}
	if n != len(eventBytes) {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: short write to item log %s: wrote %d of %d bytes", blerrors.ErrWriteIndeterminate, logPath, n, len(eventBytes))
	}
	if err := file.Sync(); err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: fsync item log %s: %w", blerrors.ErrWriteIndeterminate, logPath, err)
	}
	// Directory-handle fsync for a brand-new dirent is intentionally skipped
	// on Windows (no meaningful directory-handle flush), matching the
	// identical precedent already established in internal/atomicfile,
	// internal/core/durable_fs.go, and shipment_reconcile_fs_windows.go.

	// Re-read EXACTLY the bytes just written from the SAME still-open file
	// handle used for the append itself — never a fresh pathname open,
	// which would be a TOCTOU window in which logPath could be swapped for
	// a different (possibly outside-workspace) file between this append and
	// a later re-open (167.007-T hardening; shared portable-orchestration
	// contract with the Unix implementation, shipment_reconcile_append_unix.go).
	readBack := make([]byte, n)
	if _, err := file.ReadAt(readBack, preAppendSize); err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: re-read durably appended bytes from same handle for item log %s: %w", blerrors.ErrWriteIndeterminate, logPath, err)
	}

	return shipmentReconcileAppendResult{preAppendSize: preAppendSize, bytesWritten: n, readBack: readBack}, nil
}
