//go:build windows

package core

import (
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
// on Windows. Windows exposes no POSIX openat/renameat-style directory-
// relative create without depending on NT-native APIs this codebase does
// not otherwise use, so this path validates the archive directory and the
// destination target for reparse points (matching the same documented
// Windows/Unix asymmetry established in shipment_reconcile_lock_windows.go
// and internal/events/item_log_lock_windows.go), then uses a same-directory
// temp file plus os.Rename (MoveFileEx with MOVEFILE_REPLACE_EXISTING) for
// the atomic replace. seams.dirSyncEnabled defaults to false in production
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
func writeShipmentReconcileArchiveFileHandleRelative(archiveDir, fileName string, content []byte, seams shipmentReconcileFSSeams) error {
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

	tmpFile, err := os.CreateTemp(realArchiveDir, "."+fileName+"-*.tmp")
	if err != nil {
		return fmt.Errorf("%w: create temp archive file: %w", blerrors.ErrWriteNotApplied, err)
	}
	tmpPath := tmpFile.Name()
	cleanupTmp := func() { _ = os.Remove(tmpPath) }

	if _, writeErr := tmpFile.Write(content); writeErr != nil {
		_ = tmpFile.Close()
		cleanupTmp()
		return fmt.Errorf("%w: write temp archive file: %w", blerrors.ErrWriteNotApplied, writeErr)
	}
	if syncErr := seams.syncFile(tmpFile); syncErr != nil {
		_ = tmpFile.Close()
		cleanupTmp()
		return fmt.Errorf("%w: fsync temp archive file: %w", blerrors.ErrWriteNotApplied, syncErr)
	}
	if closeErr := tmpFile.Close(); closeErr != nil {
		cleanupTmp()
		return fmt.Errorf("%w: close temp archive file: %w", blerrors.ErrWriteNotApplied, closeErr)
	}

	if renameErr := os.Rename(tmpPath, finalPath); renameErr != nil {
		cleanupTmp()
		return fmt.Errorf("%w: rename archive file: %w", blerrors.ErrWriteNotApplied, renameErr)
	}

	// The rename has committed and the new content is visible. A failure
	// found from this point on cannot un-apply the write, so it is reported
	// as indeterminate rather than not-applied. Re-open the just-renamed
	// path and verify, from the ACTUAL opened handle's own resolved final
	// path (not a fresh pathname stat, which a mid-write reparse-point
	// substitution of an intermediate directory in archiveDir could already
	// have poisoned), that it still resolves directly inside realArchiveDir
	// — failing closed (reporting failure) rather than silently reporting
	// success for an archive file that may have landed outside the
	// workspace.
	if containErr := verifyShipmentReconcileArchiveFileContainment(finalPath, realArchiveDir); containErr != nil {
		return fmt.Errorf("%w: post-write containment check failed for archive file %s: %w", blerrors.ErrWriteIndeterminate, finalPath, containErr)
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
	defer dirFile.Close()
	if syncErr := seams.syncDir(dirFile); syncErr != nil {
		return fmt.Errorf("%w: fsync archive directory after rename: %w", blerrors.ErrWriteIndeterminate, syncErr)
	}
	return nil
}

// verifyShipmentReconcileArchiveFileContainment re-opens path (the
// just-renamed archive file) with FILE_FLAG_OPEN_REPARSE_POINT so a reparse
// point at path is opened as the reparse object itself rather than followed,
// rejects it if it is one, and then re-derives the ACTUAL handle's own
// resolved final path via GetFinalPathNameByHandle — mirroring
// shipment_reconcile_evidence_windows.go's closure-evidence read — and
// requires that resolved path to still land directly inside realDir.
func verifyShipmentReconcileArchiveFileContainment(path, realDir string) error {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("encode archive file path %s: %w", path, err)
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return fmt.Errorf("open archive file %s: %w", path, err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()

	var info windows.ByHandleFileInformation
	if infoErr := windows.GetFileInformationByHandle(handle, &info); infoErr != nil {
		return fmt.Errorf("stat archive file %s: %w", path, infoErr)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("%w: archive file %s is a reparse point", blerrors.ErrValidation, path)
	}
	finalPath, err := shipmentReconcileWindowsFinalPath(handle)
	if err != nil {
		return fmt.Errorf("resolve final path for archive file %s: %w", path, err)
	}
	if finalPath != realDir && !pathContained(realDir, finalPath) {
		return fmt.Errorf("%w: archive file %s resolves outside archive directory %s", blerrors.ErrValidation, finalPath, realDir)
	}
	return nil
}
