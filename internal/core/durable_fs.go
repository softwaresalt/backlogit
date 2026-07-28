package core

import (
	"fmt"
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
