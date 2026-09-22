//go:build windows

package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func openShipmentOpsDirectory(realStorageRoot, opsRoot string) (*os.File, error) {
	handle, err := openShipmentOpsWindowsHandle(
		opsRoot,
		windows.GENERIC_READ,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
	)
	if err != nil {
		return nil, fmt.Errorf("open shipment operations directory: %w", err)
	}
	file := os.NewFile(uintptr(handle), opsRoot)
	valid := false
	defer func() {
		if !valid {
			_ = file.Close()
		}
	}()

	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return nil, fmt.Errorf("stat shipment operations directory handle: %w", err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return nil, fmt.Errorf("shipment operations directory handle is redirected or not a directory: %w",
			blerrors.ErrValidation)
	}
	finalPath, err := shipmentReconcileWindowsFinalPath(handle)
	if err != nil {
		return nil, fmt.Errorf("resolve shipment operations directory handle: %w", err)
	}
	if filepath.Clean(finalPath) != filepath.Clean(opsRoot) ||
		!pathContained(realStorageRoot, finalPath) {
		return nil, fmt.Errorf("shipment operations directory handle resolves outside storage root: %w",
			blerrors.ErrValidation)
	}
	valid = true
	return file, nil
}

func readShipmentOperationJournalFile(_ *os.File, opsRoot, name string) ([]byte, error) {
	path := filepath.Join(opsRoot, name)
	handle, err := openShipmentOpsWindowsHandle(
		path,
		windows.GENERIC_READ,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), path)
	defer file.Close()
	if err := validateShipmentOpsWindowsFileHandle(handle, opsRoot, path); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read journal handle %s: %w", path, err)
	}
	return data, nil
}

func writeShipmentOperationJournalFile(_ *os.File, opsRoot, name string, data []byte) error {
	targetPath := filepath.Join(opsRoot, name)
	if _, err := os.Lstat(targetPath); err == nil {
		handle, openErr := openShipmentOpsWindowsHandle(
			targetPath,
			windows.GENERIC_READ,
			windows.OPEN_EXISTING,
			windows.FILE_FLAG_OPEN_REPARSE_POINT,
		)
		if openErr != nil {
			return fmt.Errorf("open existing shipment operation journal: %w", openErr)
		}
		if validateErr := validateShipmentOpsWindowsFileHandle(handle, opsRoot, targetPath); validateErr != nil {
			_ = windows.CloseHandle(handle)
			return validateErr
		}
		_ = windows.CloseHandle(handle)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect existing shipment operation journal: %w", err)
	}

	temp, err := os.CreateTemp(opsRoot, ".shipment-operation-*.tmp")
	if err != nil {
		return fmt.Errorf("create shipment operation journal temp file: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := validateShipmentOpsWindowsFileHandle(windows.Handle(temp.Fd()), opsRoot, tempPath); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write shipment operation journal temp file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync shipment operation journal temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close shipment operation journal temp file: %w", err)
	}

	from, err := windows.UTF16PtrFromString(tempPath)
	if err != nil {
		return fmt.Errorf("encode shipment operation temp path: %w", err)
	}
	to, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return fmt.Errorf("encode shipment operation target path: %w", err)
	}
	if err := windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH); err != nil {
		return fmt.Errorf("replace shipment operation journal: %w", err)
	}
	return nil
}

func openShipmentOpsWindowsHandle(path string, access, disposition, flags uint32) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("encode path %s: %w", path, err)
	}
	handle, err := windows.CreateFile(
		name,
		access,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		disposition,
		windows.FILE_ATTRIBUTE_NORMAL|flags,
		0,
	)
	if err != nil {
		return 0, err
	}
	return handle, nil
}

func validateShipmentOpsWindowsFileHandle(handle windows.Handle, opsRoot, path string) error {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return fmt.Errorf("stat shipment operation journal handle %s: %w", path, err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return fmt.Errorf("shipment operation journal %s is redirected or not regular: %w",
			path, blerrors.ErrValidation)
	}
	finalPath, err := shipmentReconcileWindowsFinalPath(handle)
	if err != nil {
		return fmt.Errorf("resolve shipment operation journal handle %s: %w", path, err)
	}
	if filepath.Dir(finalPath) != filepath.Clean(opsRoot) {
		return fmt.Errorf("shipment operation journal %s resolves outside operations directory: %w",
			finalPath, blerrors.ErrValidation)
	}
	return nil
}
