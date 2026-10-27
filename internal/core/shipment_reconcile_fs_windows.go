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
// Copilot PR #440 review hardening: the pathname-based archiveDir/finalPath
// reparse checks below only see the state of the filesystem at check time;
// os.CreateTemp and os.Rename are themselves plain pathname operations that
// re-walk archiveDir a second (and third) time afterward, leaving a window
// in which an INTERMEDIATE directory inside archiveDir could be swapped for
// a reparse point between the check and those calls, redirecting the
// governed archive write outside the workspace. This is closed by (1)
// snapshotting archiveDir's fully-resolved real path (filepath.EvalSymlinks,
// which walks every path segment) immediately before use, and (2) after the
// rename commits, re-opening finalPath and re-deriving the ACTUAL handle's
// own resolved final path (the same handle-bound re-validation
// shipment_reconcile_evidence_windows.go's closure-evidence read already
// applies) — failing the call (as indeterminate, since the write already
// committed and cannot be un-applied) unless that final path still lands
// directly inside the snapshot.
//
// Residual risk (201.006-T, replaces the pre-CY-C characterization that
// payload bytes could leak to an outside location): the directory is
// pinned by handle (pinShipmentReconcileDir) after the EvalSymlinks snapshot
// and verified on that handle before anything is created; the temp file is
// created with NtCreateFile relative to the pinned handle, and the rename uses
// FileRenameInformation with the same RootDirectory handle, so no create,
// write, or rename re-walks archiveDir by path. A swap before the pin is
// rejected by the handle check with nothing written; a swap after the pin
// cannot redirect the write, and the pin (no FILE_SHARE_DELETE) refuses
// rename or deletion of the pinned directory itself while the handle is open
// (Windows normally refuses renaming its ancestors too; this is not relied on).
// The remaining exposure is the pre-existing leaf reparse-point pre-check and
// the shared-directory semantics described in
// docs/decisions/2026-10-08-cy-c-windows-relative-create-spike.md.
func writeShipmentReconcileArchiveFileHandleRelative(archiveDir, fileName string, content []byte, seams shipmentReconcileFSSeams) (err error) {
	if reparse, err := isReparsePointPath(archiveDir); err != nil {
		return fmt.Errorf("%w: stat archive directory %s: %w", blerrors.ErrWriteNotApplied, archiveDir, err)
	} else if reparse {
		return fmt.Errorf("%w: archive directory %s is a reparse point", blerrors.ErrValidation, archiveDir)
	}

	realArchiveDir, err := filepath.EvalSymlinks(archiveDir)
	if err != nil {
		return fmt.Errorf("%w: resolve archive directory %s: %w", blerrors.ErrWriteNotApplied, archiveDir, err)
	}
	if shipmentReconcileFSWindowsTOCTOUHook != nil {
		shipmentReconcileFSWindowsTOCTOUHook()
	}

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
	cleanupTmp := func() {
		pin.deleteFile(tmpFile)
		_ = tmpFile.Close()
	}

	if _, writeErr := tmpFile.Write(content); writeErr != nil {
		cleanupTmp()
		return fmt.Errorf("%w: write temp archive file: %w", blerrors.ErrWriteNotApplied, writeErr)
	}
	if syncErr := seams.syncFile(tmpFile); syncErr != nil {
		cleanupTmp()
		return fmt.Errorf("%w: fsync temp archive file: %w", blerrors.ErrWriteNotApplied, syncErr)
	}

	if renameErr := pin.renameFile(tmpFile, fileName); renameErr != nil {
		cleanupTmp()
		return fmt.Errorf("%w: rename archive file: %w", blerrors.ErrWriteNotApplied, renameErr)
	}

	// The rename has committed and the new content is visible. A failure
	// found from this point on cannot un-apply the write, so it is reported
	// as indeterminate rather than not-applied. The create and rename were
	// bound to the pinned directory handle, so the file cannot have landed
	// elsewhere; re-derive the renamed file's own resolved final path from
	// the still-open handle (not a fresh pathname open) as defense in depth.
	if containErr := verifyShipmentReconcileRenamedFileContainment(tmpFile, finalPath, realArchiveDir); containErr != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("%w: post-write containment check failed for archive file %s: %w", blerrors.ErrWriteIndeterminate, finalPath, containErr)
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
// it to be exactly wantPath inside realDir.
func verifyShipmentReconcileRenamedFileContainment(file *os.File, wantPath, realDir string) error {
	finalPath, err := shipmentReconcileWindowsFinalPath(windows.Handle(file.Fd()))
	if err != nil {
		return fmt.Errorf("resolve final path for archive file %s: %w", wantPath, err)
	}
	if finalPath != realDir && !pathContained(realDir, finalPath) {
		return fmt.Errorf("%w: archive file %s resolves outside archive directory %s", blerrors.ErrValidation, finalPath, realDir)
	}
	return nil
}
