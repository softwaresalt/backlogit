package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// FsyncDir opens the directory at path and fsyncs its handle so a new dirent
// or rename within it is durable. POSIX-only; callers gate on their own
// dir-sync-enabled flag. Returns a neutral error; callers classify it as
// ErrWriteNotApplied for pre-write failures or ErrWriteIndeterminate for
// post-write failures, depending on context.
func FsyncDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("open dir %s: not a directory", path)
	}
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

// MkdirAllDurable creates dir and any missing ancestors. When durable is false
// it is exactly os.MkdirAll(dir, 0o755). When durable and dirSyncEnabled:
//   - if dir exists, re-fsyncs its parent (U4 retry re-confirm);
//   - else collects missing ancestors deepest-first, re-confirms the first
//     existing ancestor's parent via syncDir (Finding-2), then creates
//     shallowest-first fsyncing each new dir's parent via syncDir.
//
// syncDir must not be nil when durable && dirSyncEnabled. All failures are
// pre-write; the caller maps a non-nil error onto blerrors.ErrWriteNotApplied.
func MkdirAllDurable(dir string, durable, dirSyncEnabled bool, syncDir func(string) error) error {
	if !durable {
		return os.MkdirAll(dir, 0o755)
	}
	if _, err := os.Stat(dir); err == nil {
		// Dir already exists. On a durable retry the previous attempt may have
		// created the dir but failed to fsync its parent; re-confirm dirent
		// durability now (U4). No new write is in flight.
		if dirSyncEnabled {
			if fsyncErr := syncDir(filepath.Dir(dir)); fsyncErr != nil {
				return fmt.Errorf("fsync parent of existing %s: %w", dir, fsyncErr)
			}
		}
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
	// Re-confirm the first existing ancestor's dirent durability (Finding-2):
	// a prior attempt may have created cur itself but failed to fsync its parent,
	// leaving that new dirent uncommitted. No new write is in flight.
	if dirSyncEnabled {
		if parentOfAncestor := filepath.Dir(cur); parentOfAncestor != cur {
			if fsyncErr := syncDir(parentOfAncestor); fsyncErr != nil {
				return fmt.Errorf("fsync parent of first existing ancestor %s: %w", cur, fsyncErr)
			}
		}
	}
	// Create shallowest-first so each parent exists when its child lands, and
	// fsync each new directory's parent so the new dirent is durable (POSIX).
	for i := len(missing) - 1; i >= 0; i-- {
		d := missing[i]
		if err := os.Mkdir(d, 0o755); err != nil && !os.IsExist(err) {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
		if dirSyncEnabled {
			if err := syncDir(filepath.Dir(d)); err != nil {
				return fmt.Errorf("fsync parent of %s: %w", d, err)
			}
		}
	}
	return nil
}
