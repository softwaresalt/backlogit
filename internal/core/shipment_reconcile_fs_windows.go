//go:build windows

package core

import (
	"fmt"
	"os"
	"path/filepath"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

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
func writeShipmentReconcileArchiveFileHandleRelative(archiveDir, fileName string, content []byte, seams shipmentReconcileFSSeams) error {
	if reparse, err := isReparsePointPath(archiveDir); err != nil {
		return fmt.Errorf("%w: stat archive directory %s: %w", blerrors.ErrWriteNotApplied, archiveDir, err)
	} else if reparse {
		return fmt.Errorf("%w: archive directory %s is a reparse point", blerrors.ErrValidation, archiveDir)
	}

	finalPath := filepath.Join(archiveDir, fileName)
	if reparse, err := isReparsePointPath(finalPath); err != nil {
		return fmt.Errorf("%w: stat archive file %s: %w", blerrors.ErrWriteNotApplied, finalPath, err)
	} else if reparse {
		return fmt.Errorf("%w: archive file %s is a reparse point", blerrors.ErrValidation, finalPath)
	}

	tmpFile, err := os.CreateTemp(archiveDir, "."+fileName+"-*.tmp")
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

	// The rename has committed and the new content is visible. A directory
	// fsync failure now cannot un-apply the write, so it is indeterminate.
	// Production leaves seams.dirSyncEnabled false on real Windows (no
	// meaningful directory-handle flush); this only actually executes when a
	// test forces it on.
	if !seams.dirSyncEnabled {
		return nil
	}
	dirFile, openErr := os.Open(archiveDir)
	if openErr != nil {
		return fmt.Errorf("%w: open archive directory for post-rename fsync: %w", blerrors.ErrWriteIndeterminate, openErr)
	}
	defer dirFile.Close()
	if syncErr := seams.syncDir(dirFile); syncErr != nil {
		return fmt.Errorf("%w: fsync archive directory after rename: %w", blerrors.ErrWriteIndeterminate, syncErr)
	}
	return nil
}
