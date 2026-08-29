//go:build !windows

package core

import (
	"fmt"
	"io"
	"os"
	"syscall"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// readFileNoFollow opens path for reading without following symlinks.
// On Unix systems, O_NOFOLLOW causes the open to fail with ELOOP if
// path is a symlink, closing the TOCTOU race between ResolveDispositionTarget's
// Lstat check and the actual read. Returns ErrCheckpointTargetUnsafe on
// symlink detection.
//
// AR-P2-1 (platform portability): O_NOFOLLOW is POSIX and available on
// Linux and macOS (supported by golang.org/x/sys/unix). On Windows, a
// separate implementation uses FILE_FLAG_OPEN_REPARSE_POINT equivalent
// (see checkpoint_nofollow_windows.go). Both files share this contract.
func readFileNoFollow(path string) ([]byte, error) {
	// O_NOFOLLOW: if the final component of path is a symbolic link, open
	// fails with ELOOP rather than following the link. This is the kernel-level
	// protection against a symlink being swapped in between our Lstat check
	// and this open.
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		if isELOOP(err) {
			return nil, fmt.Errorf("%w: target is a symlink", blerrors.ErrCheckpointTargetUnsafe)
		}
		// Convert to *os.PathError for callers using os.IsNotExist etc.
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(fd), path)
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

// isELOOP reports whether err is ELOOP (too many symbolic links), which is
// what the kernel sets when O_NOFOLLOW encounters a symlink as the last path
// component.
func isELOOP(err error) bool {
	if errno, ok := err.(syscall.Errno); ok {
		return errno == syscall.ELOOP
	}
	return false
}
