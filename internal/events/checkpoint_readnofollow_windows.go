//go:build windows

package events

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/windows"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

// readCheckpointFileNoFollow opens path for reading without following
// symlinks or reparse points, with best-effort protection on Windows.
// Windows does not expose O_NOFOLLOW via the standard library; this
// implementation opens the file and then performs a post-open check for both
// ModeSymlink (classic symlinks) and FILE_ATTRIBUTE_REPARSE_POINT (junctions
// and other reparse points, which Go 1.24 reports as ModeIrregular rather
// than ModeSymlink). The TOCTOU window between open and the attribute check
// is narrow but non-zero; full closure would require CreateFile with
// FILE_FLAG_OPEN_REPARSE_POINT, tracked as future work.
//
// Mirrors the same AR-P2-1 note as internal/core/checkpoint_nofollow_windows.go
// (153.001-T / S1 U1 adversarial-review finding F1 + Copilot 6th-review
// PRRT_kwDORzozKM6fB90S remediation).
func readCheckpointFileNoFollow(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	// Post-open check: detect symlinks (ModeSymlink) and non-symlink reparse
	// points such as junctions (FILE_ATTRIBUTE_REPARSE_POINT, reported as
	// ModeIrregular on Go 1.24 for Windows).
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat after open %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: target is a symlink", backlogiterrors.ErrCheckpointTargetUnsafe)
	}
	// Check FILE_ATTRIBUTE_REPARSE_POINT for junctions and other reparse points.
	if isReparsePoint(path) {
		return nil, fmt.Errorf("%w: target is a reparse point", backlogiterrors.ErrCheckpointTargetUnsafe)
	}

	return io.ReadAll(f)
}

// isReparsePoint reports whether path has FILE_ATTRIBUTE_REPARSE_POINT set.
// Returns false on any error (fail-open for the reparse check; the caller's
// Lstat-based ModeSymlink check remains the primary guard).
func isReparsePoint(path string) bool {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attrs, err := windows.GetFileAttributes(pathPtr)
	if err != nil {
		return false
	}
	return attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
