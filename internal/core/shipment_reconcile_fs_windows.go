//go:build windows

package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// shipmentReconcileFSWindowsTOCTOUHook, when non-nil, is invoked exactly
// once per call, immediately after realArchiveDir is snapshotted via
// filepath.EvalSymlinks and before the temp file is created. Production
// leaves this nil (a no-op); tests use it to deterministically simulate an
// attacker swapping an intermediate directory within archiveDir for a
// reparse point in the narrow window between that snapshot and the
// create-temp/write/rename sequence below, proving the post-rename
// final-path re-validation actually rejects the resulting mismatch rather
// than existing as unexercised code.
var shipmentReconcileFSWindowsTOCTOUHook func()

// writeShipmentReconcileArchiveFileHandleRelative performs the atomic write
// on Windows. After the archive directory is validated for reparse points
// it is pinned by handle, and the same-directory temp file is created with
// NtCreateFile and renamed with FileRenameInformation, both relative to that
// pinned handle (see shipment_reconcile_dirpin_windows.go), for the atomic
// replace. seams.dirSyncEnabled defaults to false in production
// (Windows has no directory-handle flush; FlushFileBuffers on a directory
// handle is not meaningful there), matching the identical gate already
// established in internal/atomicfile and internal/core/durable_fs.go; tests
// may still force seams.dirSyncEnabled/syncDir to exercise the
// classification logic deterministically without depending on real Windows
// directory-fsync behavior.
//
// Copilot PR #440 review hardening and 201.006-T: pathname-based re-walks of
// archiveDir (os.CreateTemp, os.Rename) left a window in which a directory
// could be swapped for a link. That is closed by binding every create and
// rename to a pinned directory handle (pinShipmentReconcileDir) opened on the
// caller's trusted, already-resolved archiveDir. archiveDir is NOT re-resolved
// with EvalSymlinks inside this function: a directory symlink swapped in
// between the reparse pre-check and a re-resolution would make the resolved
// path (and therefore the pin and its verification) point outside the
// workspace. Instead the pin's handle-derived final path must equal the
// trusted path, so a swap of the directory or any ancestor is rejected with
// nothing written. After the rename, the renamed file's own handle-derived
// final path is re-checked as defense in depth (indeterminate on failure,
// since the write already committed).
//
// Residual risk: the pin (no FILE_SHARE_DELETE) refuses rename or deletion of
// the pinned directory while held, but a swap that completes before the pin
// is only detected, not prevented; the pre-existing leaf reparse-point
// pre-check and the shared-directory semantics are described in
// docs/decisions/2026-10-08-cy-c-windows-relative-create-spike.md.
func writeShipmentReconcileArchiveFileHandleRelative(archiveDir, fileName string, content []byte, seams shipmentReconcileFSSeams) (err error) {
	if nameErr := validateShipmentReconcileFileName(fileName); nameErr != nil {
		return fmt.Errorf("%w: %w", blerrors.ErrWriteNotApplied, nameErr)
	}
	if reparse, err := isReparsePointPath(archiveDir); err != nil {
		return fmt.Errorf("%w: stat archive directory %s: %w", blerrors.ErrWriteNotApplied, archiveDir, err)
	} else if reparse {
		return fmt.Errorf("%w: archive directory %s is a reparse point", blerrors.ErrValidation, archiveDir)
	}
	// archiveDir is the caller's already-resolved, containment-checked
	// canonical destination. It is deliberately NOT re-resolved here: a
	// symlink swapped in after the pre-check would be followed by
	// EvalSymlinks and the pin would then bind (and verify) the OUTSIDE
	// directory. The pin below is opened on the trusted path itself and its
	// handle-derived final path must still equal it, so any swap of the
	// directory or of an ancestor is rejected before anything is created.
	if shipmentReconcileFSWindowsTOCTOUHook != nil {
		shipmentReconcileFSWindowsTOCTOUHook()
	}
	realArchiveDir := filepath.Clean(archiveDir)

	finalPath := filepath.Join(realArchiveDir, fileName)
	if reparse, err := isReparsePointPath(finalPath); err != nil {
		return fmt.Errorf("%w: stat archive file %s: %w", blerrors.ErrWriteNotApplied, finalPath, err)
	} else if reparse {
		return fmt.Errorf("%w: archive file %s is a reparse point", blerrors.ErrValidation, finalPath)
	}

	pin, err := pinShipmentReconcileDir(realArchiveDir)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := pin.close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("%w: close pinned archive directory: %w", blerrors.ErrWriteIndeterminate, cerr))
		}
	}()

	tmpName, err := shipmentReconcileTempName(fileName)
	if err != nil {
		return fmt.Errorf("%w: %w", blerrors.ErrWriteNotApplied, err)
	}
	tmpFile, err := pin.createFile(tmpName, windows.FILE_GENERIC_WRITE|windows.DELETE, windows.FILE_CREATE)
	if err != nil {
		return fmt.Errorf("%w: create temp archive file: %w", blerrors.ErrWriteNotApplied, err)
	}
	cleanupTmp := func(cause error) error {
		if delErr := pin.deleteFile(tmpFile); delErr != nil {
			cause = errors.Join(cause, delErr)
		}
		if closeErr := tmpFile.Close(); closeErr != nil {
			cause = errors.Join(cause, fmt.Errorf("close temp archive file: %w", closeErr))
		}
		return cause
	}

	if _, writeErr := tmpFile.Write(content); writeErr != nil {
		return cleanupTmp(fmt.Errorf("%w: write temp archive file: %w", blerrors.ErrWriteNotApplied, writeErr))
	}
	if syncErr := seams.syncFile(tmpFile); syncErr != nil {
		return cleanupTmp(fmt.Errorf("%w: fsync temp archive file: %w", blerrors.ErrWriteNotApplied, syncErr))
	}

	if renameErr := pin.renameFile(tmpFile, fileName); renameErr != nil {
		return cleanupTmp(fmt.Errorf("%w: rename archive file: %w", blerrors.ErrWriteNotApplied, renameErr))
	}

	// The rename has committed and the new content is visible. A failure
	// found from this point on cannot un-apply the write, so it is reported
	// as indeterminate rather than not-applied. The create and rename were
	// bound to the pinned directory handle, so the file cannot have landed
	// elsewhere; re-derive the renamed file's own resolved final path from
	// the still-open handle (not a fresh pathname open) as defense in depth.
	if containErr := verifyShipmentReconcileRenamedFileContainment(tmpFile, realArchiveDir); containErr != nil {
		closeErr := tmpFile.Close()
		return errors.Join(fmt.Errorf("%w: post-write containment check failed for archive file %s: %w", blerrors.ErrWriteIndeterminate, finalPath, containErr), closeErr)
	}
	if closeErr := tmpFile.Close(); closeErr != nil {
		return fmt.Errorf("%w: close archive file %s: %w", blerrors.ErrWriteIndeterminate, finalPath, closeErr)
	}

	// A directory fsync failure now cannot un-apply the write, so it is
	// indeterminate. Production leaves seams.dirSyncEnabled false on real
	// Windows (no meaningful directory-handle flush); this only actually
	// executes when a test forces it on.
	if !seams.dirSyncEnabled {
		return nil
	}
	dirFile, openErr := os.Open(realArchiveDir)
	if openErr != nil {
		return fmt.Errorf("%w: open archive directory for post-rename fsync: %w", blerrors.ErrWriteIndeterminate, openErr)
	}
	defer func() {
		if cerr := dirFile.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("%w: close archive directory after fsync: %w", blerrors.ErrWriteIndeterminate, cerr))
		}
	}()
	if syncErr := seams.syncDir(dirFile); syncErr != nil {
		return fmt.Errorf("%w: fsync archive directory after rename: %w", blerrors.ErrWriteIndeterminate, syncErr)
	}
	return nil
}

// verifyShipmentReconcileRenamedFileContainment re-derives the ACTUAL open
// handle's own resolved final path via GetFinalPathNameByHandle and requires
// it to be inside realDir.
func verifyShipmentReconcileRenamedFileContainment(file *os.File, realDir string) error {
	finalPath, err := shipmentReconcileWindowsFinalPath(windows.Handle(file.Fd()))
	if err != nil {
		return fmt.Errorf("resolve final path for archive file: %w", err)
	}
	if finalPath != realDir && !pathContained(realDir, finalPath) {
		return fmt.Errorf("%w: archive file %s resolves outside archive directory %s", blerrors.ErrValidation, finalPath, realDir)
	}
	return nil
}
