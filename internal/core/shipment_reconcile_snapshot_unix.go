//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package core

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// readShipmentReconcileArchiveSnapshotFile reads the pre-mutation archive
// file bytes TRUE directory-handle-relative on Unix (167.016-T hardening):
// it opens the archive directory no-follow, then opens the target file
// relative to that verified directory descriptor with O_NOFOLLOW, and reads
// from THAT single open handle. This replaces the prior os.Lstat-then-
// os.ReadFile pair — two separate pathname operations, hence a check/use
// race window in which the file could be replaced by a symlink between the
// two calls — with one no-follow, handle-bound open, mirroring
// writeShipmentReconcileArchiveFileHandleRelative
// (shipment_reconcile_fs_unix.go). A missing directory or missing file
// surfaces as an error satisfying errors.Is(err, os.ErrNotExist) (unix.Errno
// implements the same Is(os.ErrNotExist) contract os.PathError does), so the
// caller's existing not-exist handling is unaffected.
func readShipmentReconcileArchiveSnapshotFile(archiveDir, fileName string) ([]byte, error) {
	dirFD, err := unix.Open(archiveDir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open archive directory %s: %w", archiveDir, err)
	}
	dirFile := os.NewFile(uintptr(dirFD), archiveDir)
	defer dirFile.Close()

	fd, err := unix.Openat(int(dirFile.Fd()), fileName, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open archive file %s relative to %s: %w", fileName, archiveDir, err)
	}
	file := os.NewFile(uintptr(fd), fileName)
	defer file.Close()

	var stat unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil {
		return nil, fmt.Errorf("stat archive file %s: %w", fileName, err)
	}
	if stat.Mode&unix.S_IFMT == unix.S_IFDIR {
		return nil, fmt.Errorf("archive file %s is a directory", fileName)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, fmt.Errorf("archive file %s is not a regular file", fileName)
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read archive file %s: %w", fileName, err)
	}
	return content, nil
}
