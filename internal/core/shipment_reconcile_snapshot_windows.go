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

// readShipmentReconcileArchiveSnapshotFile reads the pre-mutation archive
// file bytes on Windows through a single verified handle (Copilot PR #440
// review, finding 1), replacing the prior portable fallback this file used
// to share with shipment_reconcile_snapshot_other.go (an os.Lstat check
// immediately followed by a separate os.ReadFile call — two distinct
// pathname operations, hence a check/use TOCTOU window in which the file
// could be replaced by a symlink/junction between the two).
//
// This mirrors the established pattern already used by
// verifyShipmentReconcileArchiveFileContainment
// (shipment_reconcile_fs_windows.go) and
// readShipmentReconcileClosureEvidenceFile
// (shipment_reconcile_evidence_windows.go): the archive directory is
// real-path-resolved first, the target file is opened with
// FILE_FLAG_OPEN_REPARSE_POINT (so a reparse point AT the file itself is
// opened as the reparse object and rejected rather than transparently
// followed), and the ACTUAL opened handle's own resolved final path (via
// shipmentReconcileWindowsFinalPath) is required to still land inside the
// resolved archive directory — all BEFORE any byte is read, and every byte
// read comes from that SAME single handle.
//
// A missing archive directory or missing archive file surfaces as an error
// satisfying errors.Is(err, os.ErrNotExist): windows.Errno (an alias for
// syscall.Errno) implements the same Is(os.ErrNotExist) contract unix.Errno
// does on the Unix build, so the caller's existing not-exist handling
// (shipment_reconcile_snapshot.go) is unaffected.
func readShipmentReconcileArchiveSnapshotFile(archiveDir, fileName string) ([]byte, error) {
	realArchiveDir, err := filepath.EvalSymlinks(archiveDir)
	if err != nil {
		return nil, fmt.Errorf("resolve archive directory %s: %w", archiveDir, err)
	}

	archivePath := filepath.Join(realArchiveDir, fileName)
	name, err := windows.UTF16PtrFromString(archivePath)
	if err != nil {
		return nil, fmt.Errorf("encode archive file path %s: %w", archivePath, err)
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
		return nil, fmt.Errorf("open archive file %s: %w", archivePath, err)
	}
	file := os.NewFile(uintptr(handle), archivePath)
	defer file.Close()

	var info windows.ByHandleFileInformation
	if infoErr := windows.GetFileInformationByHandle(handle, &info); infoErr != nil {
		return nil, fmt.Errorf("stat archive file %s: %w", archivePath, infoErr)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, fmt.Errorf("%w: archive file %s is a reparse point", blerrors.ErrValidation, archivePath)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return nil, fmt.Errorf("archive file %s is a directory", archivePath)
	}

	finalPath, err := shipmentReconcileWindowsFinalPath(handle)
	if err != nil {
		return nil, fmt.Errorf("resolve final path for archive file %s: %w", archivePath, err)
	}
	if finalPath != realArchiveDir && !pathContained(realArchiveDir, finalPath) {
		return nil, fmt.Errorf("%w: archive file %s resolves outside archive directory %s", blerrors.ErrValidation, finalPath, realArchiveDir)
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read archive file %s: %w", archivePath, err)
	}
	return content, nil
}
