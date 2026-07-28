package core

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
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
var mkdirDirSyncFn = fsyncDirCore

// mkdirAllDurable creates dir and any missing ancestors. When durable is false it
// is exactly os.MkdirAll. When durable is true it creates the missing ancestors
// shallowest-first and, on POSIX, fsyncs each newly created directory's parent so
// the new dirent survives power loss (mirroring the U5 events level-by-level
// durable-mkdir). Existing ancestors are left untouched; Windows dirent
// durability is best-effort (the fsync is skipped).
func mkdirAllDurable(dir string, durable bool) error {
	if !durable {
		return os.MkdirAll(dir, 0o755)
	}
	if _, err := os.Stat(dir); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", dir, err)
	}
	// Collect the missing ancestors, deepest-first.
	var missing []string
	cur := dir
	for {
		if _, err := os.Stat(cur); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat %s: %w", cur, err)
		}
		missing = append(missing, cur)
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	// Create shallowest-first so each parent exists when its child lands, and
	// fsync each new directory's parent so the new dirent is durable (POSIX).
	for i := len(missing) - 1; i >= 0; i-- {
		d := missing[i]
		if err := os.Mkdir(d, 0o755); err != nil && !os.IsExist(err) {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
		if mkdirDirSyncEnabled {
			if err := mkdirDirSyncFn(filepath.Dir(d)); err != nil {
				return fmt.Errorf("fsync parent of %s: %w", d, err)
			}
		}
	}
	return nil
}

// durableSyncDir best-effort fsyncs dir when durable is enabled, logging a
// slog.Warn on failure rather than returning an error. It is used where a move is
// within a single directory (for example the adopt ID-rename, which rewrites and
// removes entries in the same dir): the destination dirent was made durable by
// the artifact write, but the subsequent same-dir removal/rename is not durable
// until the directory is fsynced again. These call sites run after DB/tx
// mutations, so a failure is logged, not surfaced (see durableSyncMovedFromDir).
func durableSyncDir(ws *Workspace, dir, op string) {
	if err := fsyncDirIfDurable(dir, WorkspaceDurableWrites(ws)); err != nil {
		slog.Warn("durable move: directory fsync failed (best-effort)",
			"op", op, "dir", dir, "error", err)
	}
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

// fsyncDirCore opens the directory at path and fsyncs its handle so a new dirent
// within it is durable. POSIX-only; callers gate on mkdirDirSyncEnabled.
func fsyncDirCore(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open dir %s: %w", path, err)
	}
	syncErr := d.Sync()
	closeErr := d.Close()
	if syncErr != nil {
		return fmt.Errorf("fsync dir %s: %w", path, syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close dir %s: %w", path, closeErr)
	}
	return nil
}
