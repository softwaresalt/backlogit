package core

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// taskStaleLockTTL is the age threshold after which an unremoved per-task .lock
// sidecar is treated as stale (left by a crashed process) and auto-cleaned.
// Windows PID-liveness is unreliable, so recovery is TTL-based, not PID-based.
const taskStaleLockTTL = 60 * time.Second

// taskLockRegistry provides genuine per-task granularity: a registry-guarded
// map of one *sync.Mutex per resolved task path, so locking task A never
// serializes task B (unlike a single package-global mutex). The map is never
// pruned — growth is bounded by the number of distinct task paths touched in a
// process lifetime, which is acceptable for the CLI-subprocess gate.
var (
	taskLockRegistryMu sync.Mutex
	taskLockRegistry   = map[string]*sync.Mutex{}
)

// ErrTaskBusy signals that a task's advisory lock is already held — either by
// another goroutine in this process or by a live cross-process sidecar.
// lockTaskFile returns it WITHOUT blocking so gate consumers (doctor --target,
// update --size) get deterministic contention behavior.
var ErrTaskBusy = fmt.Errorf("task is locked by a concurrent operation")

// taskMutexFor returns the process-wide mutex for a resolved task path,
// creating it on first use.
func taskMutexFor(resolvedPath string) *sync.Mutex {
	taskLockRegistryMu.Lock()
	defer taskLockRegistryMu.Unlock()
	m, ok := taskLockRegistry[resolvedPath]
	if !ok {
		m = &sync.Mutex{}
		taskLockRegistry[resolvedPath] = m
	}
	return m
}

// taskLockSidecarPath returns the .<name>.lock sidecar path adjacent to a task
// file, per concurrency.instructions.md (dotted-prefix, ephemeral, gitignored
// via .*.lock).
func taskLockSidecarPath(taskFilePath string) string {
	dir := filepath.Dir(taskFilePath)
	base := filepath.Base(taskFilePath)
	return filepath.Join(dir, "."+base+".lock")
}

// lockTaskFile acquires a per-task advisory lock for taskFilePath. It first
// makes a NON-BLOCKING attempt on the per-path in-process mutex (TryLock), then
// creates an O_CREATE|O_EXCL sidecar for cross-process safety. A sidecar older
// than taskStaleLockTTL is treated as crash residue: it is removed with a WARN
// and creation is retried once. On a held mutex or a live sidecar it returns
// ErrTaskBusy without blocking. A sidecar-creation failure for any reason OTHER
// than "already exists" (permission, missing directory, read-only filesystem)
// is returned as an ordinary wrapped error — NOT ErrTaskBusy — so gate consumers
// preserve the busy-vs-IO exit-code contract. The returned unlock releases BOTH
// the sidecar and the mutex and is safe to call multiple times; callers MUST
// defer it so every error path releases both.
func lockTaskFile(taskFilePath string) (unlock func() error, err error) {
	resolved, err := filepath.Abs(taskFilePath)
	if err != nil {
		return nil, fmt.Errorf("resolve task path: %w", err)
	}
	resolved = filepath.Clean(resolved)

	mu := taskMutexFor(resolved)
	if !mu.TryLock() {
		// Another goroutine in this process holds the lock — non-blocking busy.
		return nil, ErrTaskBusy
	}

	lockPath := taskLockSidecarPath(resolved)
	f, createErr := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if createErr != nil {
		// Only an "already exists" (EEXIST) failure means contention. Any other
		// error (permission denied, missing parent directory, read-only
		// filesystem) is a genuine IO fault and must NOT be classified as busy:
		// doing so would break the gate exit-code contract (busy=4 vs io=3) and
		// trigger misleading contention retries. Surface those as ordinary errors.
		if !errors.Is(createErr, os.ErrExist) {
			mu.Unlock()
			return nil, fmt.Errorf("create task lock sidecar %s: %w", lockPath, createErr)
		}
		// The sidecar exists. Reclaim it only if it is older than the TTL
		// (crash residue); an in-TTL sidecar is a live lock → busy.
		info, statErr := os.Stat(lockPath)
		if statErr != nil || time.Since(info.ModTime()) <= taskStaleLockTTL {
			mu.Unlock()
			return nil, ErrTaskBusy
		}
		slog.Warn("removing stale task lock file", "path", lockPath, "age", time.Since(info.ModTime()))
		_ = os.Remove(lockPath)
		f, createErr = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if createErr != nil {
			mu.Unlock()
			if errors.Is(createErr, os.ErrExist) {
				// Another operation re-created the sidecar between our remove and
				// recreate — genuine contention.
				return nil, ErrTaskBusy
			}
			return nil, fmt.Errorf("recreate task lock sidecar %s: %w", lockPath, createErr)
		}
	}
	_ = f.Close()

	var once sync.Once
	return func() error {
		once.Do(func() {
			// On Windows os.Remove may fail if a handle is still open; a warn is
			// sufficient — the stale-TTL reclaims it later.
			if rmErr := os.Remove(lockPath); rmErr != nil && !os.IsNotExist(rmErr) {
				slog.Warn("failed to remove task lock file", "path", lockPath, "error", rmErr)
			}
			mu.Unlock()
		})
		return nil
	}, nil
}
