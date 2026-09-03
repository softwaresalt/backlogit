//go:build windows

package events

import (
	"fmt"
	"io"
	"os"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

// readCheckpointFileNoFollow opens path for reading without following
// symlinks, with best-effort protection on Windows. Windows does not expose
// O_NOFOLLOW via the standard library; this implementation opens the file
// and then performs a post-open Lstat check. The TOCTOU window between open
// and Lstat is narrow but non-zero; full closure on Windows would require
// using CreateFile with FILE_FLAG_OPEN_REPARSE_POINT, tracked as future work.
//
// This provides best-effort protection: a symlink that exists at open time
// is detected and the read rejected; a symlink swapped in during the narrow
// open-to-Lstat window may not be caught.
//
// Mirrors the same AR-P2-1 note as internal/core/checkpoint_nofollow_windows.go
// (153.001-T / S1 U1 adversarial-review finding F1 remediation).
func readCheckpointFileNoFollow(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	// Post-open Lstat: check that path has not become (or always was) a symlink.
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat after open %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: target is a symlink", backlogiterrors.ErrCheckpointTargetUnsafe)
	}

	return io.ReadAll(f)
}
