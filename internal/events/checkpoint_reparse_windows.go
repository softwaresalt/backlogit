//go:build windows

package events

import (
	"golang.org/x/sys/windows"
)

// isSymlinkOrReparseWindows reports whether path has FILE_ATTRIBUTE_REPARSE_POINT
// set (covering junctions and other reparse points that Go 1.24 reports as
// ModeIrregular rather than ModeSymlink) or ModeSymlink. Called from
// rejectSymlinksInPathChain to close the reparse-point gap on Windows
// (Copilot 6th-review PRRT_kwDORzozKM6fB9zl).
func isPathSymlinkOrReparse(path string) bool {
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
