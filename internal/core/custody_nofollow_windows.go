//go:build windows

package core

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// openCustodyNoFollow opens path read-only with FILE_FLAG_OPEN_REPARSE_POINT,
// so a symlink or junction at path is opened as the reparse point itself
// rather than followed. The opened handle is rejected when it carries the
// reparse-point attribute.
func openCustodyNoFollow(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("encode path %s: %w", path, err)
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("open %s without following reparse points: %w", path, err)
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		_ = windows.CloseHandle(handle)
		return nil, fmt.Errorf("stat handle for %s: %w", path, err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(handle)
		return nil, fmt.Errorf("%s is a symlink or reparse point", path)
	}
	return os.NewFile(uintptr(handle), path), nil
}
