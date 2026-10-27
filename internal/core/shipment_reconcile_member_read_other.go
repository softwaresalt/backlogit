//go:build !windows

package core

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// readShipmentReconcileMemberFile opens a manifest member file once with
// O_NOFOLLOW, verifies the opened handle is the same file as the fully
// resolved path and that this path lies inside storageRoot, and returns the
// bytes read from that same handle.
func readShipmentReconcileMemberFile(storageRoot, path string) (content []byte, retErr error) {
	rootAbs, err := filepath.Abs(storageRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve storage root %s: %w", storageRoot, err)
	}
	realRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, fmt.Errorf("resolve storage root real path %s: %w", rootAbs, err)
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		if errors.Is(err, syscall.ELOOP) {
			return nil, fmt.Errorf("%w: member file %s is a symlink", blerrors.ErrValidation, path)
		}
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(fd), path)
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close member file %s: %w", path, closeErr))
		}
	}()

	opened, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat member file %s: %w", path, err)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("resolve member file %s: %w", path, err)
	}
	if resolved != realRoot && !pathContained(realRoot, resolved) {
		return nil, fmt.Errorf("%w: member file %s resolves outside storage root %s", blerrors.ErrValidation, resolved, realRoot)
	}
	current, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("stat resolved member file %s: %w", resolved, err)
	}
	if !os.SameFile(opened, current) {
		return nil, fmt.Errorf("%w: member file %s changed while being opened", blerrors.ErrValidation, path)
	}
	content, err = io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read member file %s: %w", path, err)
	}
	return content, nil
}
