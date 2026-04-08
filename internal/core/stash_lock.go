package core

import (
	"fmt"
	"log/slog"
	"os"
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
