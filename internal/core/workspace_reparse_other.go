//go:build !windows

package core

import "os"

func isSymlinkOrReparse(info os.FileInfo, _ string) (bool, error) {
	return info.Mode()&os.ModeSymlink != 0, nil
}
