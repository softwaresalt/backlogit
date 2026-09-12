package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// shipmentReconcileArchiveDirName is the fixed archive subdirectory (under
// the workspace storage root) that the governed reconciliation transaction
// writes its archived shipment file into — the SAME directory the rest of
// the codebase's archive path uses (internal/core/archive.go).
const shipmentReconcileArchiveDirName = "archive"

// shipmentReconcileFSSeams carries the injectable fsync seams and the
// platform durability gate for the handle-relative archive-file writer,
// mirroring internal/atomicfile's durableSeams: production wires real
// *os.File.Sync calls with dirSyncEnabled gated by GOOS (Windows exposes no
// directory-handle flush); tests override the seam functions directly so
// the pre-rename/post-rename failure classification is deterministically
// exercisable on ANY host OS, including a Windows dev machine, without
// depending on the real platform's fsync-on-directory behavior.
//
// Must not run with t.Parallel: tests that override these read on the
// production write path.
type shipmentReconcileFSSeams struct {
	dirSyncEnabled bool
	syncFile       func(*os.File) error
	syncDir        func(*os.File) error
}

// defaultShipmentReconcileFSSeams returns the production fsync seams: always
// fsync the temp file before rename (this writer precedes an always-fsync
// audit append, 167.007-T, and must provide equivalent crash-durability for
// the archive artifact itself), and fsync the archive directory handle after
// rename on POSIX only.
func defaultShipmentReconcileFSSeams() shipmentReconcileFSSeams {
	return shipmentReconcileFSSeams{
		dirSyncEnabled: runtime.GOOS != "windows",
		syncFile:       func(f *os.File) error { return f.Sync() },
		syncDir:        func(f *os.File) error { return f.Sync() },
	}
}

// writeShipmentReconcileArchiveFileWithSeams is the seam-injectable
// implementation shared by writeShipmentReconcileArchiveFile and the
// durability tests.
func writeShipmentReconcileArchiveFileWithSeams(ctx context.Context, ws *Workspace, shipmentID string, content []byte, seams shipmentReconcileFSSeams) error {
	if ws == nil {
		return fmt.Errorf("write shipment reconcile archive file: workspace is required: %w", blerrors.ErrValidation)
	}
	if shipmentID == "" {
		return fmt.Errorf("write shipment reconcile archive file: shipment id is required: %w", blerrors.ErrValidation)
	}
	// Reject anything that is not a single safe path component. The archived
	// filename convention (<id>.md`) is preserved verbatim for compatibility
	// with every existing reader of the archive directory (FindArtifactPath,
	// doctor scans, ArchiveItem-produced files), so the ID is validated, not
	// encoded.
	if filepath.Base(shipmentID) != shipmentID || shipmentID == "." || shipmentID == ".." {
		return fmt.Errorf("%w: shipment id %q is not a safe filename component", blerrors.ErrValidation, shipmentID)
	}

	storageRoot := workspaceStorageRoot(ws)
	archiveDir := filepath.Join(storageRoot, shipmentReconcileArchiveDirName)
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return fmt.Errorf("%w: create archive directory: %w", blerrors.ErrWriteNotApplied, err)
	}
	realStorageRoot, err := filepath.EvalSymlinks(storageRoot)
	if err != nil {
		return fmt.Errorf("%w: resolve workspace storage root: %w", blerrors.ErrWriteNotApplied, err)
	}
	realArchiveDir, err := filepath.EvalSymlinks(archiveDir)
	if err != nil {
		return fmt.Errorf("%w: resolve archive directory: %w", blerrors.ErrWriteNotApplied, err)
	}
	if !pathContained(realStorageRoot, realArchiveDir) {
		return fmt.Errorf("%w: archive directory resolves outside the workspace storage root", blerrors.ErrValidation)
	}

	fileName := shipmentID + ".md"
	return writeShipmentReconcileArchiveFileHandleRelative(realArchiveDir, fileName, content, seams)
}
