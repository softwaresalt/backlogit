//go:build windows

package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"golang.org/x/sys/windows"
)

func snapshotFinalPath(handle windows.Handle) (string, error) {
	buffer := make([]uint16, 32768)
	n, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
	if err != nil {
		return "", fmt.Errorf("resolve snapshot handle: %w", err)
	}
	if n == 0 || n >= uint32(len(buffer)) {
		return "", fmt.Errorf("invalid snapshot handle path: %w", os.ErrPermission)
	}
	return normalizeSnapshotWindowsPath(windows.UTF16ToString(buffer[:n])), nil
}

func normalizeSnapshotWindowsPath(path string) string {
	if len(path) >= len(`\\?\UNC\`) && strings.EqualFold(path[:len(`\\?\UNC\`)], `\\?\UNC\`) {
		return `\\` + path[len(`\\?\UNC\`):]
	}
	return strings.TrimPrefix(path, `\\?\`)
}

func pinSnapshotDirectory(dir string) (*os.File, string, error) {
	name, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return nil, "", fmt.Errorf("encode snapshot directory: %w", err)
	}
	buffer := make([]uint16, 32768)
	n, err := windows.GetLongPathName(name, &buffer[0], uint32(len(buffer)))
	if err != nil {
		return nil, "", fmt.Errorf("expand snapshot directory spelling: %w", err)
	}
	if n == 0 || n >= uint32(len(buffer)) {
		return nil, "", fmt.Errorf("invalid snapshot directory spelling: %w", os.ErrPermission)
	}
	dir = normalizeSnapshotWindowsPath(windows.UTF16ToString(buffer[:n]))
	name, err = windows.UTF16PtrFromString(dir)
	if err != nil {
		return nil, "", fmt.Errorf("encode canonical snapshot directory: %w", err)
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE|0x20,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open snapshot directory: %w", err)
	}
	parent := os.NewFile(uintptr(handle), dir)
	var info windows.ByHandleFileInformation
	statErr := windows.GetFileInformationByHandle(handle, &info)
	final, pathErr := snapshotFinalPath(handle)
	if statErr != nil || pathErr != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || !strings.EqualFold(final, dir) {
		return nil, "", fmt.Errorf("redirected snapshot directory handle: %w",
			errors.Join(os.ErrPermission, statErr, pathErr, parent.Close()))
	}
	return parent, dir, nil
}

func (s *snapshotSource) open(name string) (*os.File, error) {
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, fmt.Errorf("encode snapshot leaf: %w", err)
	}
	attrs := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(s.parent.Fd()),
		ObjectName: objectName, Attributes: windows.OBJ_CASE_INSENSITIVE}
	attrs.Length = uint32(reflect.TypeOf(attrs).Size())
	var iosb windows.IO_STATUS_BLOCK
	var handle windows.Handle
	err = windows.NtCreateFile(&handle, windows.GENERIC_READ|windows.SYNCHRONIZE,
		&attrs, &iosb, nil, windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN, windows.FILE_NON_DIRECTORY_FILE|windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_OPEN_REPARSE_POINT, 0, 0)
	if err != nil {
		if errors.Is(err, windows.STATUS_OBJECT_NAME_NOT_FOUND) {
			return nil, fmt.Errorf("open snapshot leaf: %w", os.ErrNotExist)
		}
		return nil, fmt.Errorf("open snapshot leaf: %w", err)
	}
	return os.NewFile(uintptr(handle), filepath.Join(s.path, name)), nil
}

func (s *snapshotSource) check(file *os.File, name string) error {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &info); err != nil {
		return fmt.Errorf("stat snapshot handle: %w", err)
	}
	final, err := snapshotFinalPath(windows.Handle(file.Fd()))
	if err != nil {
		return err
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 ||
		info.NumberOfLinks != 1 || !strings.EqualFold(final, filepath.Join(s.path, name)) {
		return fmt.Errorf("linked or redirected snapshot handle: %w", os.ErrPermission)
	}
	current, err := os.Lstat(filepath.Join(s.path, name))
	if err != nil {
		return fmt.Errorf("inspect opened snapshot basename: %w", err)
	}
	owned, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect opened snapshot identity: %w", err)
	}
	if !os.SameFile(owned, current) || current.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("replaced snapshot basename: %w", os.ErrPermission)
	}
	return nil
}
