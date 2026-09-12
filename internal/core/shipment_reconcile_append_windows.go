//go:build windows

package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// appendShipmentReconcileEventHandleRelative performs the append on
// Windows. Windows has no POSIX openat-style directory-relative create
// without NT-native APIs this codebase does not otherwise depend on (see
// shipment_reconcile_lock_windows.go and shipment_reconcile_fs_windows.go
// for the same documented asymmetry), so this path validates the logs
// directory and the target file for reparse points, then opens the file
// directly (by path) with FILE_FLAG_OPEN_REPARSE_POINT so an existing
// reparse point at that path is opened as the reparse object itself rather
// than followed, appends eventBytes, and fsyncs.
func appendShipmentReconcileEventHandleRelative(logsDir, fileName string, eventBytes []byte) (shipmentReconcileAppendResult, error) {
	if reparse, err := isReparsePointPath(logsDir); err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: stat logs directory %s: %w", blerrors.ErrWriteNotApplied, logsDir, err)
	} else if reparse {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: logs directory %s is a reparse point", blerrors.ErrValidation, logsDir)
	}

	logPath := filepath.Join(logsDir, fileName)
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

	return shipmentReconcileAppendResult{preAppendSize: preAppendSize, bytesWritten: n}, nil
}
