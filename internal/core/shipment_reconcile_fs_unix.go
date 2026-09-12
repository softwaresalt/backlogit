//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package core

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// writeShipmentReconcileArchiveFileHandleRelative performs the actual
// handle-relative atomic write on Unix: it opens the (already
// symlink-resolved and containment-verified) archive directory no-follow,
// creates a uniquely-named temp file relative to that directory's file
// descriptor (unix.Openat), writes and fsyncs it, then atomically replaces
// the final target via unix.Renameat — bound to the SAME verified directory
// descriptor for both the temp file and the destination, never by
// re-walking a joined pathname a second time (167.006-T requirement:
// handle-relative, not by pathname).
func writeShipmentReconcileArchiveFileHandleRelative(archiveDir, fileName string, content []byte, seams shipmentReconcileFSSeams) error {
	dirFD, err := unix.Open(archiveDir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("%w: open archive directory %s: %w", blerrors.ErrWriteNotApplied, archiveDir, err)
	}
	dirFile := os.NewFile(uintptr(dirFD), archiveDir)
	defer dirFile.Close()

	tmpName := fmt.Sprintf(".%s.%d.tmp", fileName, os.Getpid())
	cleanupTmp := func() { _ = unix.Unlinkat(int(dirFile.Fd()), tmpName, 0) }

	tmpFD, err := unix.Openat(int(dirFile.Fd()), tmpName, unix.O_CREAT|unix.O_EXCL|unix.O_WRONLY|unix.O_NOFOLLOW, 0o644)
	if err != nil {
		return fmt.Errorf("%w: create temp archive file %s: %w", blerrors.ErrWriteNotApplied, tmpName, err)
	}
	tmpFile := os.NewFile(uintptr(tmpFD), filepath.Join(archiveDir, tmpName))

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

	if renameErr := unix.Renameat(int(dirFile.Fd()), tmpName, int(dirFile.Fd()), fileName); renameErr != nil {
		cleanupTmp()
		return fmt.Errorf("%w: rename archive file: %w", blerrors.ErrWriteNotApplied, renameErr)
	}

	// The rename has committed and the new content is visible. A directory
	// fsync failure now cannot un-apply the write, so it is indeterminate.
	if !seams.dirSyncEnabled {
		return nil
	}
	if syncErr := seams.syncDir(dirFile); syncErr != nil {
		return fmt.Errorf("%w: fsync archive directory after rename: %w", blerrors.ErrWriteIndeterminate, syncErr)
	}
	return nil
}
