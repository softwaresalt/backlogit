//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package events

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// openItemLogLockHandle opens (creating if absent) the item-log lock (C)
// sidecar with O_NOFOLLOW so the kernel rejects the open with ELOOP if
// lockPath is (or resolves through) a symlink, mirroring the same no-follow
// pattern already established for checkpoint reads
// (checkpoint_readnofollow_unix.go) and applying it to lock-file opens
// (167.017-T, requirement (b): handle-bound, reparse/no-follow-safe
// semantics). O_NOFOLLOW only rejects an existing symlink target; creating a
// new regular file when lockPath does not yet exist is unaffected.
func openItemLogLockHandle(lockPath string) (*os.File, bool, error) {
	fd, err := unix.Open(lockPath, unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW, 0o644)
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return nil, false, fmt.Errorf("%w: item log lock sidecar %s is a symlink", blerrors.ErrValidation, lockPath)
		}
		return nil, false, fmt.Errorf("open item log lock sidecar %s: %w", lockPath, err)
	}
	file := os.NewFile(uintptr(fd), lockPath)
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
