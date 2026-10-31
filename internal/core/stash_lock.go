package core

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// staleLockTTL is the age threshold after which an unremoved .lock sidecar is
// treated as stale (left by a crashed process) and automatically cleaned up.
const staleLockTTL = 60 * time.Second

// stashMu serializes concurrent same-process stash mutations. Combined with the
// .lock sidecar file, this provides protection within a single process (mutex)
// and across processes (file lock).
var stashMu sync.Mutex

// lockStashFile creates an advisory lock sidecar (.lock) next to path.
// It first acquires the in-process mutex to serialize goroutines in the same
// process, then creates the sidecar file to guard against other processes.
// If the sidecar already exists but is older than staleLockTTL, it is treated
// as stale (left by a crashed process) and removed before retrying.
// Returns an unlock function that removes the sidecar and releases the mutex.
func lockStashFile(path string) (unlock func() error, err error) {
	stashMu.Lock()
	lockPath := path + ".lock"
	f, createErr := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if createErr != nil {
		// Check whether the existing lock file is stale before giving up.
		if info, statErr := os.Stat(lockPath); statErr == nil {
			age := time.Since(info.ModTime())
			if age > staleLockTTL {
				slog.Warn("removing stale stash lock file", "path", lockPath, "age", age)
				_ = os.Remove(lockPath)
				f, createErr = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
			}
		}
		if createErr != nil {
			stashMu.Unlock()
			return nil, fmt.Errorf("stash locked by another process: %w", createErr)
		}
	}
	_ = f.Close()
	return func() error {
		defer stashMu.Unlock()
		return unlockStashFile(path)
	}, nil
}

// lockActiveStash acquires the canonical cross-process stash lock: the sidecar
// of the ACTIVE stash file (StashFilePath). HarvestStashEntry,
// ArchiveStashEntry and every other stash mutator serialize on this lock, and
// they append to archive/stash.jsonl while holding it, so any code that
// rewrites the archive must hold the same lock. The lock is not re-entrant:
// callers that already hold it must not call this helper.
func lockActiveStash(ws *Workspace) (unlock func() error, err error) {
	path := StashFilePath(ws.RootPath)
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil {
		return nil, fmt.Errorf("ensure stash directory: %w", mkErr)
	}
	return lockStashFile(path)
}

// unlockStashFile removes the advisory lock sidecar created by lockStashFile.
// On Windows, os.Remove may fail if the handle is still open; failures are logged
// as warnings and the caller proceeds without error.
func unlockStashFile(path string) error {
	lockPath := path + ".lock"
	if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
		slog.Warn("failed to remove stash lock file", "path", lockPath, "error", err)
	}
	return nil
}
