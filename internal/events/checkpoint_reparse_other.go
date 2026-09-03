//go:build !windows

package events

import "os"

// isPathSymlinkOrReparse reports whether path is a symlink (or, on Windows,
// a non-symlink reparse point such as a junction). On non-Windows platforms,
// only ModeSymlink is checked — reparse points are a Windows concept.
func isPathSymlinkOrReparse(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}
