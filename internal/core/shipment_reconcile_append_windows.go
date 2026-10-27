//go:build windows

package core

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

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
// Windows. After the logs directory is validated for reparse points it is
// pinned by handle (shipment_reconcile_dirpin_windows.go) and the item log is
// opened with NtCreateFile relative to that handle, so an existing reparse
// point at that name is opened as the reparse object itself rather than
// followed, and no empty file can be created at a swapped-in location.
// eventBytes is then appended and fsynced.
//
// Copilot PR #440 review hardening and 201.006-T: FILE_FLAG_OPEN_REPARSE_POINT
// alone only protects the FINAL path component. The directory is therefore
// pinned by handle on the caller's trusted, already-resolved logsDir, which is
// deliberately NOT re-resolved with EvalSymlinks here (a directory symlink
// swapped in before a re-resolution would make the pin bind the outside
// directory); the pin's handle-derived final path must equal logsDir, so a swap
// of the directory or an ancestor is rejected before anything is created. After
// the open, the file handle's own final path is re-checked against logsDir
// before anything is written.
func appendShipmentReconcileEventHandleRelative(logsDir, fileName string, eventBytes []byte) (_ shipmentReconcileAppendResult, err error) {
	if nameErr := validateShipmentReconcileFileName(fileName); nameErr != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: %w", blerrors.ErrWriteNotApplied, nameErr)
	}
	if reparse, err := isReparsePointPath(logsDir); err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: stat logs directory %s: %w", blerrors.ErrWriteNotApplied, logsDir, err)
	} else if reparse {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: logs directory %s is a reparse point", blerrors.ErrValidation, logsDir)
	}
	// logsDir is the caller's trusted destination and is not re-resolved: see
	// writeShipmentReconcileArchiveFileHandleRelative.
	if shipmentReconcileAppendWindowsTOCTOUHook != nil {
		shipmentReconcileAppendWindowsTOCTOUHook()
	}
	realLogsDir := filepath.Clean(logsDir)

	logPath := filepath.Join(realLogsDir, fileName)
	if reparse, err := isReparsePointPath(logPath); err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: stat item log %s: %w", blerrors.ErrWriteNotApplied, logPath, err)
	} else if reparse {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: item log %s is a reparse point", blerrors.ErrValidation, logPath)
	}

	pin, err := pinShipmentReconcileDir(realLogsDir)
	if err != nil {
		return shipmentReconcileAppendResult{}, err
	}
	defer func() {
		if cerr := pin.close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("%w: close pinned logs directory: %w", blerrors.ErrWriteIndeterminate, cerr))
		}
	}()
	file, err := pin.createFile(fileName, windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE, windows.FILE_OPEN_IF)
	if err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: open item log %s: %w", blerrors.ErrWriteNotApplied, logPath, err)
	}
	handle := windows.Handle(file.Fd())
	defer func() {
		if cerr := file.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("%w: close item log %s: %w", blerrors.ErrWriteIndeterminate, logPath, cerr))
		}
	}()

	// Post-open containment re-validation: reject if the opened object is
	// itself a reparse point (the create opens it as the reparse object
	// rather than following it, so this positively detects that case), and
	// confirm the handle's OWN resolved final path still lands directly
	// inside realLogsDir. Both checks run, and eventBytes is still
	// unwritten, before either can be bypassed.
	var byHandleInfo windows.ByHandleFileInformation
	if infoErr := windows.GetFileInformationByHandle(handle, &byHandleInfo); infoErr != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: stat item log handle %s: %w", blerrors.ErrWriteNotApplied, logPath, infoErr)
	}
	if byHandleInfo.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: item log %s is a reparse point", blerrors.ErrValidation, logPath)
	}
	finalPath, err := shipmentReconcileWindowsFinalPath(handle)
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
