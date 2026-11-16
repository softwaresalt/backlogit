//go:build windows

package atomicfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// moveFileExFlags computes the MoveFileEx flag word for an atomic same-volume
// replace. It splits the two concerns the legacy remove-before-rename fallback
// conflated: MOVEFILE_REPLACE_EXISTING (atomic-replace SAFETY) is set
// UNCONDITIONALLY because removing the destination first was a real data-loss
// defect, while MOVEFILE_WRITE_THROUGH (a synchronous durability flush) is added
// ONLY when durable is on so the off-mode fast path pays no write-through
// latency. No MOVEFILE_COPY_ALLOWED: the temp file is same-directory/same-volume.
func moveFileExFlags(durable bool) uint32 {
	flags := uint32(windows.MOVEFILE_REPLACE_EXISTING)
	if durable {
		flags |= windows.MOVEFILE_WRITE_THROUGH
	}
	return flags
}

// moveFileEx is the injectable MoveFileEx syscall seam. Tests override it to
// assert the gated flag word and to simulate replace failures in-process
// (a process kill cannot return an error to the caller, so it cannot exercise
// the fail-closed classification). Production wires the real Windows syscall.
var moveFileEx = func(from, to string, flags uint32) error {
	fromPtr, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return fmt.Errorf("encode source path: %w", err)
	}
	toPtr, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return fmt.Errorf("encode destination path: %w", err)
	}
	return windows.MoveFileEx(fromPtr, toPtr, flags)
}

const (
	atomicReplaceMaxRetries     = 6
	atomicReplaceInitialBackoff = 1 * time.Millisecond
)

// atomicReplaceSleep makes retry backoff deterministic in tests.
var atomicReplaceSleep = time.Sleep

func isTransientWindowsReplaceError(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) ||
		errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}

// atomicReplace renames tmpName over path using MoveFileEx so the destination
// is replaced atomically and is NEVER removed before the replacement is in
// place. ReplaceFileW is deliberately rejected (its write-through story is
// unreliable and its failure modes can leave the destination absent). On
// failure the destination is untouched, so the error is classified
// ErrWriteNotApplied.
func atomicReplace(tmpName, path string, durable bool) error {
	flags := moveFileExFlags(durable)
	delay := atomicReplaceInitialBackoff
	var err error
	for attempt := 0; attempt <= atomicReplaceMaxRetries; attempt++ {
		err = moveFileEx(tmpName, path, flags)
		if err == nil {
			return nil
		}
		if !isTransientWindowsReplaceError(err) || attempt == atomicReplaceMaxRetries {
			break
		}
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) && atomicReplaceDestinationPermanentDenial(path) {
			break
		}
		atomicReplaceSleep(delay)
		delay *= 2
	}
	return fmt.Errorf("move temp over destination: %w",
		fmt.Errorf("%w: %w", blerrors.ErrWriteNotApplied, err))
}

func atomicReplaceDestinationPermanentDenial(path string) bool {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attributes, err := windows.GetFileAttributes(name)
	// An unavailable attribute query must not replace the original move error
	// or turn an unconfirmed denial into a permanent one.
	return err == nil && attributes&(windows.FILE_ATTRIBUTE_READONLY|windows.FILE_ATTRIBUTE_DIRECTORY) != 0
}

func normalizeWindowsFinalPath(path string) string {
	if len(path) >= len(`\\?\UNC\`) && strings.EqualFold(path[:len(`\\?\UNC\`)], `\\?\UNC\`) {
		return `\\` + path[len(`\\?\UNC\`):]
	}
	return strings.TrimPrefix(path, `\\?\`)
}

func finalPathByHandle(handle windows.Handle) (string, error) {
	buf := make([]uint16, 32768)
	n, err := windows.GetFinalPathNameByHandle(handle, &buf[0], uint32(len(buf)), 0)
	if err != nil {
		return "", fmt.Errorf("get final path by handle: %w", err)
	}
	if n == 0 || n >= uint32(len(buf)) {
		return "", fmt.Errorf("get final path by handle: buffer length %d: %w", n, blerrors.ErrValidation)
	}
	return normalizeWindowsFinalPath(windows.UTF16ToString(buf[:n])), nil
}

func verifyRootRegularLeafHandle(root *os.Root, rel string, expectedInfo os.FileInfo) error {
	f, err := root.Open(rel)
	if err != nil {
		return notApplied("open leaf in root", err)
	}
	defer func() {
		// Read-only verification handle; release before MoveFileExW so sharing does not contend.
		_ = f.Close()
	}()
	openedInfo, err := f.Stat()
	if err != nil {
		return notApplied("stat opened leaf in root", err)
	}
	if !os.SameFile(openedInfo, expectedInfo) {
		return fmt.Errorf("leaf %q identity changed: %w: %w", rel, blerrors.ErrWriteNotApplied, blerrors.ErrValidation)
	}
	var fi windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &fi); err != nil {
		return notApplied("inspect leaf handle", err)
	}
	if fi.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		return fmt.Errorf("leaf %q is a reparse point or directory: %w: %w", rel, blerrors.ErrWriteNotApplied, blerrors.ErrValidation)
	}
	return nil
}

func verifyExpectedDirMatchesHandle(dirAbs string, dirHandleInfo windows.ByHandleFileInformation, liveDirPath string) (string, error) {
	cleanDir := filepath.Clean(dirAbs)
	dirPtr, err := windows.UTF16PtrFromString(cleanDir)
	if err != nil {
		return "", notApplied("encode expected directory path", err)
	}
	expectedHandle, err := windows.CreateFile(
		dirPtr,
		windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return "", notApplied("open expected directory path", err)
	}
	defer func() {
		// Verification handle; close before replacement.
		_ = windows.CloseHandle(expectedHandle)
	}()

	var expectedInfo windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(expectedHandle, &expectedInfo); err != nil {
		return "", notApplied("inspect expected directory handle", err)
	}
	if expectedInfo.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		expectedInfo.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return "", fmt.Errorf("expected directory %q is redirected or not a directory: %w: %w", cleanDir, blerrors.ErrWriteNotApplied, blerrors.ErrValidation)
	}
	expectedFinalPath, err := finalPathByHandle(expectedHandle)
	if err != nil {
		return "", notApplied("resolve expected directory kernel path", err)
	}
	if expectedInfo.VolumeSerialNumber != dirHandleInfo.VolumeSerialNumber ||
		expectedInfo.FileIndexHigh != dirHandleInfo.FileIndexHigh ||
		expectedInfo.FileIndexLow != dirHandleInfo.FileIndexLow ||
		!strings.EqualFold(filepath.Clean(liveDirPath), filepath.Clean(expectedFinalPath)) {
		return "", fmt.Errorf("pinned directory %q redirected to %q: %w: %w", expectedFinalPath, liveDirPath, blerrors.ErrWriteNotApplied, blerrors.ErrValidation)
	}
	return expectedFinalPath, nil
}

func replaceTempFileInRootPlatform(
	root *os.Root,
	dirFile *os.File,
	dirAbs, tmpRel, dstRel string,
	tmpInfo, dstInfo os.FileInfo,
	durable bool,
) error {
	var dirHandleInfo windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(dirFile.Fd()), &dirHandleInfo); err != nil {
		return notApplied("inspect pinned directory handle", err)
	}
	if dirHandleInfo.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		dirHandleInfo.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("pinned directory is not a regular directory: %w: %w", blerrors.ErrWriteNotApplied, blerrors.ErrValidation)
	}

	liveDirPath, err := finalPathByHandle(windows.Handle(dirFile.Fd()))
	if err != nil {
		return notApplied("resolve pinned directory kernel path", err)
	}
	canonDirAbs, err := verifyExpectedDirMatchesHandle(dirAbs, dirHandleInfo, liveDirPath)
	if err != nil {
		return err
	}

	if err := verifyRootRegularLeafHandle(root, tmpRel, tmpInfo); err != nil {
		return err
	}
	if dstInfo != nil {
		if err := verifyRootRegularLeafHandle(root, dstRel, dstInfo); err != nil {
			return err
		}
	}

	return atomicReplace(filepath.Join(canonDirAbs, tmpRel), filepath.Join(canonDirAbs, dstRel), durable)
}
