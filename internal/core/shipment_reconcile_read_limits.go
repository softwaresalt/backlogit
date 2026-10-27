package core

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// maxShipmentReconcileReadBytes caps how much a reconcile reader will load
// from a single member or snapshot file.
const maxShipmentReconcileReadBytes = 16 << 20

// readAllShipmentReconcileBounded reads r to EOF and fails with
// blerrors.ErrValidation when more than maxShipmentReconcileReadBytes remain.
func readAllShipmentReconcileBounded(r io.Reader, name string) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(r, maxShipmentReconcileReadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	if len(content) > maxShipmentReconcileReadBytes {
		return nil, fmt.Errorf("%w: %s exceeds %d bytes", blerrors.ErrValidation, name, maxShipmentReconcileReadBytes)
	}
	return content, nil
}

// validateShipmentReconcileFileName requires name to be a single path
// component: non-empty, not a traversal element, free of separators and of
// ':' (which would address an NTFS alternate data stream).
func validateShipmentReconcileFileName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name ||
		strings.ContainsAny(name, `/\:`) {
		return fmt.Errorf("%w: %q is not a single path component", blerrors.ErrValidation, name)
	}
	return nil
}
