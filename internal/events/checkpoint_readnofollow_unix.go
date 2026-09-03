//go:build !windows

package events

import (
	"fmt"
	"io"
	"os"
	"syscall"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

// readCheckpointFileNoFollow opens path for reading without following
// symlinks, using O_NOFOLLOW on Unix/Linux/macOS. The kernel rejects the
// open with ELOOP if path is a symlink, closing the TOCTOU race between
// rejectSymlinksInPathChain's Lstat-based pre-check and the actual read
// (153.001-T / S1 U1 adversarial-review finding F1).
//
// Platform note: O_NOFOLLOW is POSIX and available on Linux and macOS.
// The Windows counterpart (checkpoint_readnofollow_windows.go) uses a
// best-effort post-open Lstat check; that file documents the residual
// window. Both share the same function contract.
func readCheckpointFileNoFollow(path string) ([]byte, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno == syscall.ELOOP {
			return nil, fmt.Errorf("%w: target is a symlink", backlogiterrors.ErrCheckpointTargetUnsafe)
		}
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(fd), path)
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}
