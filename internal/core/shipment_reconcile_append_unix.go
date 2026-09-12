//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package core

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// appendShipmentReconcileEventHandleRelative performs the actual
// handle-relative, always-fsync append on Unix: it opens the logs directory
// no-follow, opens (creating if absent) the item's JSONL file relative to
// that verified directory descriptor, rejects an existing partial (non-
// newline-terminated) trailing line without writing anything, appends
// eventBytes, and fsyncs the file. A brand-new file's dirent is also made
// durable via a directory fsync.
func appendShipmentReconcileEventHandleRelative(logsDir, fileName string, eventBytes []byte) (shipmentReconcileAppendResult, error) {
	dirFD, err := unix.Open(logsDir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: open logs directory %s: %w", blerrors.ErrWriteNotApplied, logsDir, err)
	}
	dirFile := os.NewFile(uintptr(dirFD), logsDir)
	defer dirFile.Close()

	fd, err := unix.Openat(int(dirFile.Fd()), fileName, unix.O_CREAT|unix.O_WRONLY|unix.O_APPEND|unix.O_NOFOLLOW, 0o644)
	if err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: open item log %s relative to %s: %w", blerrors.ErrWriteNotApplied, fileName, logsDir, err)
	}
	file := os.NewFile(uintptr(fd), fileName)
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: stat item log %s: %w", blerrors.ErrWriteNotApplied, fileName, err)
	}
	preAppendSize := info.Size()
	isNewFile := preAppendSize == 0

	if preAppendSize > 0 {
		tail := make([]byte, 1)
		if _, err := file.ReadAt(tail, preAppendSize-1); err != nil {
			return shipmentReconcileAppendResult{}, fmt.Errorf("%w: read trailing byte of item log %s: %w", blerrors.ErrWriteIndeterminate, fileName, err)
		}
		if tail[0] != '\n' {
			return shipmentReconcileAppendResult{}, fmt.Errorf("%w: item log %s has a partial (non-newline-terminated) trailing line; refusing to concatenate", blerrors.ErrWriteIndeterminate, fileName)
		}
	}

	n, writeErr := file.Write(eventBytes)
	if writeErr != nil {
		if n == 0 {
			return shipmentReconcileAppendResult{}, fmt.Errorf("%w: write item log %s: %w", blerrors.ErrWriteNotApplied, fileName, writeErr)
		}
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: partial write to item log %s: %w", blerrors.ErrWriteIndeterminate, fileName, writeErr)
	}
	if n != len(eventBytes) {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: short write to item log %s: wrote %d of %d bytes", blerrors.ErrWriteIndeterminate, fileName, n, len(eventBytes))
	}
	if err := file.Sync(); err != nil {
		return shipmentReconcileAppendResult{}, fmt.Errorf("%w: fsync item log %s: %w", blerrors.ErrWriteIndeterminate, fileName, err)
	}
	if isNewFile {
		if err := dirFile.Sync(); err != nil {
			return shipmentReconcileAppendResult{}, fmt.Errorf("%w: fsync logs directory after creating %s: %w", blerrors.ErrWriteIndeterminate, fileName, err)
		}
	}

	return shipmentReconcileAppendResult{preAppendSize: preAppendSize, bytesWritten: n}, nil
}
