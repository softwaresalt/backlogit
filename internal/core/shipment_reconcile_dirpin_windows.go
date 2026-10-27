//go:build windows

package core

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"golang.org/x/sys/windows"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// shipmentReconcilePinAccess is FILE_ADD_FILE (0x2) | FILE_TRAVERSE (0x20) |
// FILE_READ_ATTRIBUTES (0x80) | SYNCHRONIZE; golang.org/x/sys/windows does not
// export the directory-specific names.
const shipmentReconcilePinAccess = 0x2 | 0x20 | 0x80 | windows.SYNCHRONIZE

// shipmentReconcilePinnedDir is a directory opened once and verified, to which
// every later create and rename of a governed reconcile write is bound by
// handle (NtCreateFile / FileRenameInformation with RootDirectory) instead of
// by path. It is opened without FILE_SHARE_DELETE, so the directory object
// cannot be renamed, replaced, or deleted while the pin is held, and a path
// swap after the pin cannot redirect any create or rename. 201.004-T spike
// (docs/decisions/2026-10-08-cy-c-windows-relative-create-spike.md).
type shipmentReconcilePinnedDir struct {
	handle  windows.Handle
	realDir string
}

// pinShipmentReconcileDir opens dir and verifies on the opened handle that it
// is a real, non-reparse directory whose final path still equals dir. dir must
// be the caller's already-resolved canonical destination (never re-resolved
// here), so a swap of the directory or any ancestor at any point before this
// open is rejected before anything is created.
func pinShipmentReconcileDir(dir string) (_ *shipmentReconcilePinnedDir, err error) {
	name, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return nil, fmt.Errorf("%w: encode directory %s: %w", blerrors.ErrWriteNotApplied, dir, err)
	}
	// Minimal rights: add files, traverse, and read attributes. No generic
	// read/write, so the pin cannot enumerate or delete directory entries.
	handle, err := windows.CreateFile(
		name,
		shipmentReconcilePinAccess,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: open directory %s: %w", blerrors.ErrWriteNotApplied, dir, err)
	}
	pin := &shipmentReconcilePinnedDir{handle: handle, realDir: dir}
	defer func() {
		if err != nil {
			if closeErr := pin.close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("close directory %s: %w", dir, closeErr))
			}
		}
	}()

	var info windows.ByHandleFileInformation
	if infoErr := windows.GetFileInformationByHandle(handle, &info); infoErr != nil {
		return nil, fmt.Errorf("%w: stat directory %s: %w", blerrors.ErrWriteNotApplied, dir, infoErr)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, fmt.Errorf("%w: directory %s is a reparse point", blerrors.ErrValidation, dir)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return nil, fmt.Errorf("%w: %s is not a directory", blerrors.ErrValidation, dir)
	}
	finalPath, err := shipmentReconcileWindowsFinalPath(handle)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve final path for directory %s: %w", blerrors.ErrWriteNotApplied, dir, err)
	}
	if !strings.EqualFold(finalPath, filepath.Clean(dir)) {
		return nil, fmt.Errorf("%w: directory %s resolves to %s after open", blerrors.ErrValidation, dir, finalPath)
	}
	return pin, nil
}

func (p *shipmentReconcilePinnedDir) close() error {
	return windows.CloseHandle(p.handle)
}

// createFile creates or opens the single path component name directly under
// the pinned directory. Reparse points at name are opened as the reparse
// object rather than followed.
func (p *shipmentReconcilePinnedDir) createFile(name string, access, disposition uint32) (*os.File, error) {
	if err := validateShipmentReconcileFileName(name); err != nil {
		return nil, err
	}
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, fmt.Errorf("encode file name %q: %w", name, err)
	}
	attrs := &windows.OBJECT_ATTRIBUTES{
		RootDirectory: p.handle,
		ObjectName:    objectName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}
	// OBJECT_ATTRIBUTES.Length is the struct size; reflect avoids unsafe.Sizeof.
	attrs.Length = uint32(reflect.TypeOf(*attrs).Size())
	var iosb windows.IO_STATUS_BLOCK
	var handle windows.Handle
	err = windows.NtCreateFile(
		&handle,
		access|windows.SYNCHRONIZE,
		attrs,
		&iosb,
		nil,
		windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ,
		disposition,
		windows.FILE_NON_DIRECTORY_FILE|windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_OPEN_REPARSE_POINT,
		0,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("create %s in pinned directory: %w", name, err)
	}
	return os.NewFile(uintptr(handle), filepath.Join(p.realDir, name)), nil
}

// renameFile renames the open file to name, a single path component under the
// pinned directory, replacing any existing file. The file must have been
// opened with DELETE access.
func (p *shipmentReconcilePinnedDir) renameFile(file *os.File, name string) error {
	if err := validateShipmentReconcileFileName(name); err != nil {
		return err
	}
	nameUTF16, err := windows.UTF16FromString(name)
	if err != nil {
		return fmt.Errorf("encode file name %q: %w", name, err)
	}
	nameBytes := (len(nameUTF16) - 1) * 2
	// FILE_RENAME_INFORMATION is a variable-length struct, so it is laid out
	// by hand in a byte buffer with encoding/binary (no unsafe pointer
	// arithmetic): ULONG flags (padded to pointer size), HANDLE RootDirectory,
	// ULONG FileNameLength, WCHAR FileName[].
	ptrSize := bits.UintSize / 8
	lengthOffset := 2 * ptrSize
	buf := make([]byte, lengthOffset+4+nameBytes)
	binary.LittleEndian.PutUint32(buf[0:4], windows.FILE_RENAME_REPLACE_IF_EXISTS)
	if ptrSize == 8 {
		binary.LittleEndian.PutUint64(buf[ptrSize:], uint64(p.handle))
	} else {
		binary.LittleEndian.PutUint32(buf[ptrSize:], uint32(p.handle))
	}
	binary.LittleEndian.PutUint32(buf[lengthOffset:], uint32(nameBytes))
	for i, unit := range nameUTF16[:len(nameUTF16)-1] {
		binary.LittleEndian.PutUint16(buf[lengthOffset+4+2*i:], unit)
	}
	var iosb windows.IO_STATUS_BLOCK
	if err := windows.NtSetInformationFile(windows.Handle(file.Fd()), &iosb, &buf[0], uint32(len(buf)), windows.FileRenameInformation); err != nil {
		return fmt.Errorf("rename to %s in pinned directory: %w", name, err)
	}
	return nil
}

// deleteFile marks the open file for deletion (cleanup of a staged temp file)
// and reports a failure so the caller can surface a leaked temp file.
func (p *shipmentReconcilePinnedDir) deleteFile(file *os.File) error {
	var iosb windows.IO_STATUS_BLOCK
	mark := []byte{1}
	if err := windows.NtSetInformationFile(windows.Handle(file.Fd()), &iosb, &mark[0], 1, windows.FileDispositionInformation); err != nil {
		return fmt.Errorf("mark staged file for deletion: %w", err)
	}
	return nil
}

func shipmentReconcileTempName(fileName string) (string, error) {
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("generate temp file suffix: %w", err)
	}
	return "." + fileName + "-" + hex.EncodeToString(suffix[:]) + ".tmp", nil
}
