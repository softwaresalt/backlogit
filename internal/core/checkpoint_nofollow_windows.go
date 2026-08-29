//go:build windows

package core

import (
	"fmt"
	"io"
	"os"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// readFileNoFollow opens path for reading without following symlinks.
// Windows does not expose O_NOFOLLOW via the standard library; this
// implementation opens the file and then performs a post-open Lstat check.
// The TOCTOU window between open and Lstat is narrow but non-zero; full
// closure on Windows would require using CreateFile with
// FILE_FLAG_OPEN_REPARSE_POINT, which is tracked as a follow-up.
//
// AR-P2-1 (148-F adversarial review finding): the plan required a documented
// Windows strategy. This provides best-effort protection: a symlink that
// exists at open time is detected and rejected; a symlink swapped in during
// the narrow open-to-Lstat window may not be caught.
func readFileNoFollow(path string) ([]byte, error) {
	// Open normally first; we will check for a symlink immediately after.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	// Post-open Lstat: verify the path has not become a symlink between the
	// open call above and this check. os.Lstat does not follow symlinks, so
	// a symlink at path is detected as ModeSymlink.
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat after open %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: target is a symlink", blerrors.ErrCheckpointTargetUnsafe)
	}

	return io.ReadAll(f)
}
