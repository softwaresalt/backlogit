//go:build windows

package core

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

const shipmentOpsWindowsPointerSize = bits.UintSize / 8

func openShipmentOpsDirectory(realStorageRoot, opsRoot string) (*os.File, error) {
	handle, err := openShipmentOpsWindowsHandle(
		opsRoot,
		windows.GENERIC_READ|windows.FILE_TRAVERSE,
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

func readShipmentOperationJournalFile(dir *os.File, _ string, name string) ([]byte, error) {
	handle, err := openShipmentOpsWindowsChild(
		dir,
		name,
		windows.GENERIC_READ,
		windows.FILE_OPEN,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), name)
	defer func() { _ = file.Close() }()
	if err := validateShipmentOpsWindowsFileHandle(handle, name); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read shipment operation journal handle %s: %w", name, err)
	}
	return data, nil
}

func writeShipmentOperationJournalFile(dir *os.File, _ string, name string, data []byte) error {
	existing, err := openShipmentOpsWindowsChild(
		dir,
		name,
		windows.FILE_READ_ATTRIBUTES,
		windows.FILE_OPEN,
	)
	switch {
	case err == nil:
		if validateErr := validateShipmentOpsWindowsFileHandle(existing, name); validateErr != nil {
			_ = windows.CloseHandle(existing)
			return validateErr
		}
		if closeErr := windows.CloseHandle(existing); closeErr != nil {
			return fmt.Errorf("close existing shipment operation journal %s: %w", name, closeErr)
		}
	case shipmentOpsWindowsIsNotExist(err):
	default:
		return fmt.Errorf("inspect existing shipment operation journal %s: %w", name, err)
	}

	tempName, err := shipmentOperationJournalTempName(name)
	if err != nil {
		return fmt.Errorf("derive shipment operation journal temp file: %w", err)
	}
	tempHandle, err := createShipmentOpsWindowsTemp(dir, tempName)
	if err != nil {
		return err
	}
	temp := os.NewFile(uintptr(tempHandle), tempName)
	cleanup := true
	defer func() {
		_ = temp.Close()
		if cleanup {
			_ = removeShipmentOperationJournalTempFile(dir, "", tempName)
		}
	}()
	if err := validateShipmentOpsWindowsFileHandle(tempHandle, tempName); err != nil {
		return err
	}
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write shipment operation journal temp file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync shipment operation journal temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close shipment operation journal temp file: %w", err)
	}
	if err := renameShipmentOpsWindowsChild(dir, tempName, name); err != nil {
		return fmt.Errorf("replace shipment operation journal: %w", err)
	}
	cleanup = false
	return nil
}

func createShipmentOpsWindowsTemp(dir *os.File, tempName string) (windows.Handle, error) {
	handle, err := openShipmentOpsWindowsChild(
		dir,
		tempName,
		windows.GENERIC_WRITE|windows.FILE_READ_ATTRIBUTES,
		windows.FILE_CREATE,
	)
	if shipmentOpsWindowsIsExist(err) {
		if removeErr := removeShipmentOperationJournalTempFile(dir, "", tempName); removeErr != nil {
			return 0, fmt.Errorf("remove stale shipment operation journal temp file: %w", removeErr)
		}
		handle, err = openShipmentOpsWindowsChild(
			dir,
			tempName,
			windows.GENERIC_WRITE|windows.FILE_READ_ATTRIBUTES,
			windows.FILE_CREATE,
		)
	}
	if err != nil {
		return 0, fmt.Errorf("create shipment operation journal temp file: %w", err)
	}
	return handle, nil
}

func removeShipmentOperationJournalTempFile(dir *os.File, _ string, name string) error {
	if _, ok := shipmentOperationJournalTempTarget(name); !ok {
		return fmt.Errorf("shipment operation temp file %q is not writer-owned: %w",
			name, blerrors.ErrValidation)
	}
	handle, err := openShipmentOpsWindowsChild(
		dir,
		name,
		windows.DELETE|windows.FILE_READ_ATTRIBUTES,
		windows.FILE_OPEN,
	)
	if err != nil {
		return fmt.Errorf("open shipment operation temp file %s: %w", name, err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	if err := validateShipmentOpsWindowsFileHandle(handle, name); err != nil {
		return err
	}
	deleteFile := byte(1)
	if err := windows.SetFileInformationByHandle(
		handle,
		windows.FileDispositionInfo,
		&deleteFile,
		1,
	); err != nil {
		return fmt.Errorf("remove shipment operation temp file %s: %w", name, err)
	}
	if err := windows.CloseHandle(handle); err != nil {
		return fmt.Errorf("close removed shipment operation temp file %s: %w", name, err)
	}
	handle = windows.InvalidHandle
	return nil
}

func renameShipmentOpsWindowsChild(dir *os.File, oldName, newName string) error {
	targetName, writerOwned := shipmentOperationJournalTempTarget(oldName)
	if !writerOwned || targetName != newName {
		return fmt.Errorf("shipment operation temp file %q is not owned by writer for %q: %w",
			oldName, newName, blerrors.ErrValidation)
	}
	handle, err := openShipmentOpsWindowsChild(
		dir,
		oldName,
		windows.DELETE|windows.FILE_READ_ATTRIBUTES,
		windows.FILE_OPEN,
	)
	if err != nil {
		return fmt.Errorf("open shipment operation temp file for rename: %w", err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	if err := validateShipmentOpsWindowsFileHandle(handle, oldName); err != nil {
		return err
	}
	renameInfo, err := shipmentOpsWindowsRenameInfo(windows.Handle(dir.Fd()), newName)
	if err != nil {
		return err
	}
	var iosb windows.IO_STATUS_BLOCK
	if err := windows.NtSetInformationFile(
		handle,
		&iosb,
		&renameInfo[0],
		uint32(len(renameInfo)),
		windows.FileRenameInformation,
	); err != nil {
		err = shipmentOpsWindowsError(err)
		return fmt.Errorf("rename shipment operation temp file %s to %s: %w", oldName, newName, err)
	}
	return nil
}

func shipmentOpsWindowsRenameInfo(root windows.Handle, name string) ([]byte, error) {
	if err := validateShipmentOpsWindowsChildName(name); err != nil {
		return nil, err
	}
	encodedName, err := windows.UTF16FromString(name)
	if err != nil {
		return nil, fmt.Errorf("encode shipment operation rename target %s: %w", name, err)
	}
	encodedName = encodedName[:len(encodedName)-1]

	rootOffset := shipmentOpsWindowsPointerSize
	nameLengthOffset := rootOffset + shipmentOpsWindowsPointerSize
	nameOffset := nameLengthOffset + 4
	// FILE_RENAME_INFO has a one-WCHAR flexible-array placeholder followed
	// by native alignment padding. SetFileInformationByHandle validates the
	// full C structure size in addition to FileNameLength.
	buffer := make([]byte, nameOffset+4+len(encodedName)*2)
	buffer[0] = 1 // FILE_RENAME_INFO.ReplaceIfExists
	if shipmentOpsWindowsPointerSize == 8 {
		binary.LittleEndian.PutUint64(buffer[rootOffset:], uint64(root))
	} else {
		binary.LittleEndian.PutUint32(buffer[rootOffset:], uint32(root))
	}
	binary.LittleEndian.PutUint32(buffer[nameLengthOffset:], uint32(len(encodedName)*2))
	for index, codeUnit := range encodedName {
		binary.LittleEndian.PutUint16(buffer[nameOffset+index*2:], codeUnit)
	}
	return buffer, nil
}

func openShipmentOpsWindowsChild(
	dir *os.File,
	name string,
	access uint32,
	disposition uint32,
) (windows.Handle, error) {
	if dir == nil {
		return 0, fmt.Errorf("shipment operations directory handle is nil: %w", blerrors.ErrValidation)
	}
	if err := validateShipmentOpsWindowsChildName(name); err != nil {
		return 0, err
	}
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, fmt.Errorf("encode shipment operation child name %s: %w", name, err)
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(6 * shipmentOpsWindowsPointerSize),
		RootDirectory: windows.Handle(dir.Fd()),
		ObjectName:    objectName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}
	var (
		handle windows.Handle
		status windows.IO_STATUS_BLOCK
	)
	if err := windows.NtCreateFile(
		&handle,
		access|windows.SYNCHRONIZE,
		&attributes,
		&status,
		nil,
		windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		disposition,
		windows.FILE_NON_DIRECTORY_FILE|
			windows.FILE_SYNCHRONOUS_IO_NONALERT|
			windows.FILE_OPEN_REPARSE_POINT,
		0,
		0,
	); err != nil {
		return 0, fmt.Errorf("open shipment operation child %s relative to directory handle: %w", name, err)
	}
	return handle, nil
}

func validateShipmentOpsWindowsChildName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return fmt.Errorf("shipment operation child name %q is not a base filename: %w",
			name, blerrors.ErrValidation)
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

func validateShipmentOpsWindowsFileHandle(handle windows.Handle, name string) error {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return fmt.Errorf("stat shipment operation journal handle %s: %w", name, err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return fmt.Errorf("shipment operation journal %s is redirected or not regular: %w",
			name, blerrors.ErrValidation)
	}
	return nil
}

func shipmentOpsWindowsIsNotExist(err error) bool {
	return os.IsNotExist(shipmentOpsWindowsError(err))
}

func shipmentOpsWindowsIsExist(err error) bool {
	err = shipmentOpsWindowsError(err)
	return errors.Is(err, windows.ERROR_FILE_EXISTS) ||
		errors.Is(err, windows.ERROR_ALREADY_EXISTS)
}

func shipmentOpsWindowsError(err error) error {
	var status windows.NTStatus
	if errors.As(err, &status) {
		return status.Errno()
	}
	return err
}
