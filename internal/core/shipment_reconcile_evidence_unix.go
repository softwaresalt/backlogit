//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package core

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// readShipmentReconcileClosureEvidenceFile reads the closure-evidence file
// TRUE directory-handle-relative on Unix (167.001-T hardening): after
// resolving and validating the workspace root itself, it walks EVERY
// remaining path segment (each ancestor directory, then the final file)
// relative to the previously verified directory file descriptor via
// unix.Openat with O_NOFOLLOW, never by re-deriving and opening a fresh
// joined pathname for an ancestor a second time. This defeats an attacker
// swapping an ancestor directory (e.g. "docs/") for a symlink between an
// earlier path resolution and this open, because every open in the chain is
// bound to the previous directory's HANDLE, not a re-resolved path —
// mirroring the same discipline already established by
// openShipmentReconcileLockHandleRelative (shipment_reconcile_lock_unix.go)
// and writeShipmentReconcileArchiveFileHandleRelative
// (shipment_reconcile_fs_unix.go).
func readShipmentReconcileClosureEvidenceFile(rootPath, closurePath string) ([]byte, string, error) {
	rootAbs, err := filepath.Abs(filepath.Clean(rootPath))
	if err != nil {
		return nil, "", fmt.Errorf("resolve workspace root %s: %w", rootPath, err)
	}
	realRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, "", fmt.Errorf("resolve workspace root real path: %w", err)
	}

	candidateAbs := closurePath
	if !filepath.IsAbs(candidateAbs) {
		candidateAbs = filepath.Join(realRoot, closurePath)
	}
	candidateAbs = filepath.Clean(candidateAbs)

	rel, err := filepath.Rel(realRoot, candidateAbs)
	if err != nil {
		return nil, "", fmt.Errorf("resolve closure evidence %s relative to workspace root %s: %w", closurePath, realRoot, err)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return nil, "", fmt.Errorf("closure evidence %s resolves outside workspace root %s", closurePath, realRoot)
	}
	segments := strings.Split(rel, string(filepath.Separator))
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return nil, "", fmt.Errorf("closure evidence %s contains an invalid path segment", closurePath)
		}
	}

	dirFD, err := unix.Open(realRoot, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open workspace root %s: %w", realRoot, err)
	}
	dirFile := os.NewFile(uintptr(dirFD), realRoot)
	defer func() {
		if dirFile != nil {
			_ = dirFile.Close()
		}
	}()

	currentPath := realRoot
	for _, segment := range segments[:len(segments)-1] {
		nextFD, openErr := unix.Openat(int(dirFile.Fd()), segment, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
		if openErr != nil {
			if errors.Is(openErr, unix.ELOOP) {
				return nil, "", fmt.Errorf("%w: closure evidence ancestor %s is a symlink", blerrors.ErrValidation, filepath.Join(currentPath, segment))
			}
			return nil, "", fmt.Errorf("open closure evidence ancestor %s relative to %s: %w", segment, currentPath, openErr)
		}
		nextPath := filepath.Join(currentPath, segment)
		nextFile := os.NewFile(uintptr(nextFD), nextPath)
		_ = dirFile.Close()
		dirFile = nextFile
		currentPath = nextPath
	}

	fileName := segments[len(segments)-1]
	fd, err := unix.Openat(int(dirFile.Fd()), fileName, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return nil, "", fmt.Errorf("%w: closure evidence %s is a symlink", blerrors.ErrValidation, fileName)
		}
		return nil, "", fmt.Errorf("open closure evidence %s relative to %s: %w", fileName, currentPath, err)
	}
	openedPath := filepath.Join(currentPath, fileName)
	file := os.NewFile(uintptr(fd), openedPath)
	defer file.Close()

	var stat unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil {
		return nil, "", fmt.Errorf("stat closure evidence %s: %w", fileName, err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, "", fmt.Errorf("closure evidence %s is not a regular file", closurePath)
	}
	if stat.Size <= 0 {
		return nil, "", fmt.Errorf("closure evidence %s is empty", closurePath)
	}

	body, err := io.ReadAll(file)
	if err != nil {
		return nil, "", fmt.Errorf("read closure evidence %s: %w", closurePath, err)
	}
	if len(body) == 0 {
		return nil, "", fmt.Errorf("closure evidence %s is empty", closurePath)
	}
	if openedPath != realRoot && !pathContained(realRoot, openedPath) {
		return nil, "", fmt.Errorf("opened closure evidence %s resolves outside workspace root %s", openedPath, realRoot)
	}
	return body, openedPath, nil
}
