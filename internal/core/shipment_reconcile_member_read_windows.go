//go:build windows

package core

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// readShipmentReconcileMemberFile opens a manifest member file once, verifies
// on that same handle that its final path lies inside storageRoot (via
// GetFinalPathNameByHandle, so a reparse point at ANY path component is
// caught, not only the leaf), and returns the bytes read from that handle.
func readShipmentReconcileMemberFile(storageRoot, path string) (content []byte, retErr error) {
	rootAbs, err := filepath.Abs(storageRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve storage root %s: %w", storageRoot, err)
	}
	realRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, fmt.Errorf("resolve storage root real path %s: %w", rootAbs, err)
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve member path %s: %w", path, err)
	}
	name, err := windows.UTF16PtrFromString(pathAbs)
	if err != nil {
		return nil, fmt.Errorf("encode member path %s: %w", pathAbs, err)
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
		return nil, fmt.Errorf("open member file %s: %w", pathAbs, err)
	}
	file := os.NewFile(uintptr(handle), pathAbs)
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close member file %s: %w", pathAbs, closeErr))
		}
	}()

	var info windows.ByHandleFileInformation
	if infoErr := windows.GetFileInformationByHandle(handle, &info); infoErr != nil {
		return nil, fmt.Errorf("stat member file %s: %w", pathAbs, infoErr)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, fmt.Errorf("%w: member file %s is a reparse point", blerrors.ErrValidation, pathAbs)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return nil, fmt.Errorf("%w: member file %s is a directory", blerrors.ErrValidation, pathAbs)
	}
	finalPath, err := shipmentReconcileWindowsFinalPath(handle)
	if err != nil {
		return nil, fmt.Errorf("resolve final path for member file %s: %w", pathAbs, err)
	}
	if finalPath != realRoot && !pathContained(realRoot, finalPath) {
		return nil, fmt.Errorf("%w: member file %s resolves outside storage root %s", blerrors.ErrValidation, finalPath, realRoot)
	}
	content, err = io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read member file %s: %w", pathAbs, err)
	}
	return content, nil
}
