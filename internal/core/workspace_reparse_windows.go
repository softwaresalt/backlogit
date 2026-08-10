//go:build windows

package core

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func isSymlinkOrReparse(info os.FileInfo, path string) (bool, error) {
	if info.Mode()&os.ModeSymlink != 0 {
		return true, nil
	}

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, fmt.Errorf("encode path %s: %w", path, err)
	}

	attributes, err := windows.GetFileAttributes(pathPtr)
	if err != nil {
		return false, fmt.Errorf("get file attributes %s: %w", path, err)
	}

	return attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}
