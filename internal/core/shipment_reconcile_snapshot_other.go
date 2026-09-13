//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package core

import (
	"fmt"
	"os"
	"path/filepath"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// readShipmentReconcileArchiveSnapshotFile is the portable (Windows and any
// other non-Unix-tagged platform) fallback for reading the pre-mutation
// archive file bytes. It preserves the pre-167.016-T-hardening behavior
// (os.Lstat symlink/type check, then os.ReadFile) byte-for-byte: a true
// handle-relative, single-open read for this path is tracked as a follow-up
// for the platform-specific (Windows) hardening pass, mirroring the same
// documented Windows/Unix asymmetry already established elsewhere in this
// package (e.g. shipment_reconcile_lock_windows.go,
// shipment_reconcile_evidence_windows.go).
func readShipmentReconcileArchiveSnapshotFile(archiveDir, fileName string) ([]byte, error) {
	archivePath := filepath.Join(archiveDir, fileName)

	info, err := os.Lstat(archivePath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("archive file %s is a directory", archivePath)
	}
	symlink, err := IsSymlinkOrReparsePoint(info, archivePath)
	if err != nil {
		return nil, err
	}
	if symlink {
		return nil, fmt.Errorf("archive file %s: %w", archivePath, blerrors.ErrValidation)
	}

	content, err := os.ReadFile(archivePath)
	if err != nil {
		return nil, err
	}
	return content, nil
}
