package core

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
)

// stashMu serializes concurrent same-process stash mutations. Combined with the
// .lock sidecar file, this provides protection within a single process (mutex)
// and across processes (file lock).
var stashMu sync.Mutex

// lockStashFile creates an advisory lock sidecar (.lock) next to path.
// It first acquires the in-process mutex to serialize goroutines in the same
// process, then creates the sidecar file to guard against other processes.
// Returns an unlock function that removes the sidecar and releases the mutex.
// Returns an error if the lock file already exists (another process holds the lock).
func lockStashFile(path string) (unlock func() error, err error) {
	stashMu.Lock()
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		stashMu.Unlock()
		return nil, fmt.Errorf("stash locked by another process: %w", err)
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
