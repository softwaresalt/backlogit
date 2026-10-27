//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// readShipmentReconcileMemberFile reads a manifest member or snapshot file
// that must lie inside storageRoot. Containment is enforced by construction,
// not re-derived afterwards by path: the storage root is opened once, then
// every path component below it is opened with Openat relative to the
// previous descriptor using O_NOFOLLOW (directories also O_DIRECTORY). A
// symlink at ANY component therefore fails the open, and swapping an
// intermediate directory for a symlink after the check cannot redirect the
// read because later opens are bound to the already-held descriptors. The
// final file is opened O_NONBLOCK (a FIFO cannot block the open), must be a
// regular file per fstat on the opened descriptor, and is read from that same
// descriptor with a size cap. The storage root itself is trusted and is
// resolved once via EvalSymlinks (it may legitimately be reached through a
// symlink such as /var on macOS). Platforms without this support fail closed
// (see shipment_reconcile_member_read_unsupported.go).
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
	components, err := shipmentReconcileRelativeComponents(pathAbs, rootAbs, realRoot)
	if err != nil {
		return nil, err
	}

	dirFD, err := unix.Open(realRoot, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open storage root %s: %w", realRoot, err)
	}
	last := len(components) - 1
	for _, component := range components[:last] {
		next, openErr := unix.Openat(dirFD, component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		// The parent descriptor is read-only and no longer needed either way.
		_ = unix.Close(dirFD)
		if openErr != nil {
			return nil, shipmentReconcileOpenError(pathAbs, openErr)
		}
		dirFD = next
	}
	fd, err := unix.Openat(dirFD, components[last], unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	_ = unix.Close(dirFD)
	if err != nil {
		return nil, shipmentReconcileOpenError(pathAbs, err)
	}
	file := os.NewFile(uintptr(fd), pathAbs)
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			content = nil
			retErr = errors.Join(retErr, fmt.Errorf("close member file %s: %w", pathAbs, closeErr))
		}
	}()

	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return nil, fmt.Errorf("stat member file %s: %w", pathAbs, err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, fmt.Errorf("%w: member file %s is not a regular file", blerrors.ErrValidation, pathAbs)
	}
	return readAllShipmentReconcileBounded(file, "member file "+pathAbs)
}

// shipmentReconcileRelativeComponents returns the path components of pathAbs
// below whichever of the given roots contains it, rejecting anything outside
// the root or addressing the root itself.
func shipmentReconcileRelativeComponents(pathAbs string, roots ...string) ([]string, error) {
	for _, root := range roots {
		rel, err := filepath.Rel(root, pathAbs)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		components := strings.Split(rel, string(filepath.Separator))
		for _, component := range components {
			if err := validateShipmentReconcileFileName(component); err != nil {
				return nil, fmt.Errorf("member file %s: %w", pathAbs, err)
			}
		}
		return components, nil
	}
	return nil, fmt.Errorf("%w: member file %s is outside storage root %s", blerrors.ErrValidation, pathAbs, roots[len(roots)-1])
}

func shipmentReconcileOpenError(path string, err error) error {
	if errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR) {
		return fmt.Errorf("%w: member file %s traverses a symlink or non-directory: %w", blerrors.ErrValidation, path, err)
	}
	return &os.PathError{Op: "open", Path: path, Err: err}
}
