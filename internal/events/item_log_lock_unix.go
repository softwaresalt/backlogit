//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package events

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// openItemLogLockHandle opens (creating if absent) the item-log lock (C)
// sidecar TRUE directory-handle-relative on Unix: it opens and validates the
// "itemlog" namespace directory itself (rejecting a symlink there via
// O_NOFOLLOW), then opens the sidecar file via unix.Openat bound to that
// verified directory file descriptor rather than by re-opening a freshly
// joined pathname (167.017-T requirement (b): handle-bound, not by
// pathname). This mirrors openShipmentReconcileLockHandleRelative
// (shipment_reconcile_lock_unix.go) exactly, targeting the identical
// namespace-directory + sidecar-filename resource, so this primitive and
// the reconcile transaction's own lock genuinely contend on the same
// verified inode rather than one being handle-bound while the other is
// re-derived from a (possibly since-swapped) ancestor pathname. A symlink
// planted at the sidecar's own name is rejected the same way via O_NOFOLLOW
// on the openat call itself.
func openItemLogLockHandle(lockPath string) (*os.File, bool, error) {
	namespaceDir := filepath.Dir(lockPath)
	fileName := filepath.Base(lockPath)

	dirFD, err := unix.Open(namespaceDir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, false, fmt.Errorf("open item log lock namespace directory %s: %w", namespaceDir, err)
	}
	dirFile := os.NewFile(uintptr(dirFD), namespaceDir)
	defer dirFile.Close()

	fd, err := unix.Openat(int(dirFile.Fd()), fileName, unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW, 0o644)
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return nil, false, fmt.Errorf("%w: item log lock sidecar %s is a symlink", blerrors.ErrValidation, lockPath)
		}
		return nil, false, fmt.Errorf("open item log lock sidecar %s relative to %s: %w", fileName, namespaceDir, err)
	}
	file := os.NewFile(uintptr(fd), lockPath)

	var stat unix.Stat_t
	if statErr := unix.Fstat(int(file.Fd()), &stat); statErr != nil {
		closeErr := file.Close()
		if closeErr != nil {
			return nil, false, errors.Join(statErr, closeErr)
		}
		return nil, false, fmt.Errorf("stat item log lock sidecar %s: %w", lockPath, statErr)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		closeErr := file.Close()
		if closeErr != nil {
			return nil, false, closeErr
		}
		return nil, false, fmt.Errorf("%w: item log lock sidecar %s is not a regular file", blerrors.ErrValidation, lockPath)
	}

	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		closeErr := file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, true, nil
		}
		if closeErr != nil {
			return nil, false, errors.Join(err, closeErr)
		}
		return nil, false, fmt.Errorf("lock item log sidecar %s: %w", lockPath, err)
	}
	return file, false, nil
}
