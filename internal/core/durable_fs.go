package core

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/fsutil"
)

// mkdirDirSyncEnabled gates whether mkdirAllDurable fsyncs the parent of each
// newly created directory. It defaults to (runtime.GOOS != "windows") because
// Windows exposes no directory-handle flush (dirent durability is best-effort
// there). It is a package-global seam so the POSIX ordering is exercisable
// in-process on a Windows host (a process kill cannot simulate power loss).
//
// Must not run with t.Parallel: tests that swap this seam read on the production
// write path.
var mkdirDirSyncEnabled = runtime.GOOS != "windows"

// mkdirDirSyncFn is the directory-fsync seam used by mkdirAllDurable; overridden
// by tests to observe or fail the per-ancestor dirent flush.
//
// Must not run with t.Parallel: tests that swap this seam read on the production
// write path.
var mkdirDirSyncFn = fsutil.FsyncDir

// mkdirAllDurable creates dir and any missing ancestors. When durable is false it
// is exactly os.MkdirAll. When durable is true it creates the missing ancestors
// shallowest-first and, on POSIX, fsyncs each newly created directory's parent so
// the new dirent survives power loss (mirroring the U3 events level-by-level
// durable-mkdir). When dir already exists and durable is true, the parent is
// re-fsynced to confirm dirent durability after a possible prior incomplete attempt
// (returns ErrWriteNotApplied on failure: the dir is already present, no new write
// is in flight). Existing ancestors are left untouched; Windows dirent durability
// is best-effort (the fsync is skipped).
//
// The implementation delegates to fsutil.MkdirAllDurable (the shared stdlib leaf)
// and wraps any non-nil result with ErrWriteNotApplied at this boundary: every
// fsutil failure is pre-write and retry-idempotent (D1/D3 design decisions).
func mkdirAllDurable(dir string, durable bool) error {
	if err := fsutil.MkdirAllDurable(dir, durable, mkdirDirSyncEnabled, mkdirDirSyncFn); err != nil {
		return fmt.Errorf("%w: %w", blerrors.ErrWriteNotApplied, err)
	}
	return nil
}

// durableSyncDirDetailed fsyncs dir when durable is enabled, logging a slog.Warn
// on failure (preserving the observability of the former best-effort helper) AND
// returning the error so callers can enforce the two-class contract. It is used
// where a move is within a single directory (for example the adopt ID-rename,
// which rewrites and removes entries in the same dir): the destination dirent was
// made durable by the artifact write, but the subsequent same-dir removal/rename
// is not durable until the directory is fsynced again. Callers that run after
// DB/tx mutations must NOT roll back on this error (the mutation likely
// persisted); they surface it as blerrors.ErrWriteIndeterminate instead. Via
// fsyncDirIfDurable it is a no-op on Windows or when durable is off, so it never
// produces a false indeterminate signal.
func durableSyncDirDetailed(ws *Workspace, dir, op string) error {
	if err := fsyncDirIfDurable(dir, WorkspaceDurableWrites(ws)); err != nil {
		slog.Warn("durable move: directory fsync failed (best-effort)",
			"op", op, "dir", dir, "error", err)
		return err
	}
	return nil
}

// durableSyncMovedFromDir best-effort fsyncs the SOURCE parent directory after a
// durable cross-directory move so the removal of the old dirent is durable. It is
// a no-op unless durable is enabled and the source and destination directories
// differ. A fsync failure is logged (slog.Warn) rather than returned: these
// call sites (archive/unarchive) run after DB/state mutations or after a
// completed git/file move whose rollback is already wired, so surfacing an error
// would either diverge the FS from the DB or incorrectly roll back a completed
// move — a worse outcome than the narrow duplicate this flush prevents.
func durableSyncMovedFromDir(ws *Workspace, srcPath, dstPath, op string) {
	if !WorkspaceDurableWrites(ws) {
		return
	}
	srcDir := filepath.Dir(srcPath)
	if srcDir == filepath.Dir(dstPath) {
		return
	}
	if err := fsyncDirIfDurable(srcDir, true); err != nil {
		slog.Warn("durable move: source-parent fsync failed (best-effort)",
			"op", op, "source_dir", srcDir, "error", err)
	}
}

// fsyncDirIfDurable fsyncs dir when durable is true and directory syncing is
// enabled (POSIX). It exists so a cross-directory durable move can flush the
// SOURCE parent after the old dirent is removed/renamed, mirroring how
// mkdirAllDurable and WriteFileAtomicWithOptions flush the DESTINATION parent.
// Without it a power loss could resurrect the removed source entry alongside the
// durable new one, leaving a duplicate canonical artifact. It routes through the
// mkdirDirSyncFn seam so tests can observe or fail the flush; on Windows (dir
// syncing disabled) it is a no-op (best-effort dirent durability).
func fsyncDirIfDurable(dir string, durable bool) error {
	if durable && mkdirDirSyncEnabled {
		return mkdirDirSyncFn(dir)
	}
	return nil
}
