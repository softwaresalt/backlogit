//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// openShipmentReconcileLockHandleRelative opens the item-log lock (C)
// sidecar TRUE directory-handle-relative on Unix: it opens and validates the
// namespace directory itself (rejecting a symlink there via O_NOFOLLOW), then
// opens the sidecar file via unix.Openat bound to that verified directory
// file descriptor rather than by re-walking a joined pathname a second time
// (167.011-T requirement: handle-bound, not by pathname). A symlink planted
// at the sidecar's own name is rejected the same way via O_NOFOLLOW on the
// openat call itself.
func openShipmentReconcileLockHandleRelative(namespaceDir, fileName string) (*os.File, bool, error) {
	dirFD, err := unix.Open(namespaceDir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, false, fmt.Errorf("open item log lock namespace directory %s: %w", namespaceDir, err)
	}
	dirFile := os.NewFile(uintptr(dirFD), namespaceDir)
	defer dirFile.Close()

	fd, err := unix.Openat(int(dirFile.Fd()), fileName, unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW, 0o644)
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return nil, false, fmt.Errorf("%w: item log lock sidecar %s is a symlink", blerrors.ErrValidation, fileName)
		}
		return nil, false, fmt.Errorf("open item log lock sidecar %s relative to %s: %w", fileName, namespaceDir, err)
	}
	file := os.NewFile(uintptr(fd), filepath.Join(namespaceDir, fileName))

	var stat unix.Stat_t
	if statErr := unix.Fstat(int(file.Fd()), &stat); statErr != nil {
		closeErr := file.Close()
		if closeErr != nil {
			return nil, false, errors.Join(statErr, closeErr)
		}
		return nil, false, fmt.Errorf("stat item log lock sidecar %s: %w", fileName, statErr)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		closeErr := file.Close()
		if closeErr != nil {
			return nil, false, closeErr
		}
		return nil, false, fmt.Errorf("%w: item log lock sidecar %s is not a regular file", blerrors.ErrValidation, fileName)
	}

	if flockErr := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); flockErr != nil {
		closeErr := file.Close()
		if errors.Is(flockErr, unix.EWOULDBLOCK) || errors.Is(flockErr, unix.EAGAIN) {
			return nil, true, nil
		}
		if closeErr != nil {
			return nil, false, errors.Join(flockErr, closeErr)
		}
		return nil, false, fmt.Errorf("lock item log lock sidecar %s: %w", fileName, flockErr)
	}
	return file, false, nil
}
