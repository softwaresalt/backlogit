package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

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
// file, per concurrency.instructions.md (dotted-prefix, gitignored via
// .*.lock). The file is retained as the stable inode for advisory locking.
func taskLockSidecarPath(taskFilePath string) string {
	dir := filepath.Dir(taskFilePath)
	base := filepath.Base(taskFilePath)
	return filepath.Join(dir, "."+base+".lock")
}

// lockTaskFile acquires a per-task advisory lock for taskFilePath. The
// in-process mutex provides fast local contention detection; the open sidecar
// handle provides an OS-level lock that is released atomically when closed.
// The sidecar itself is deliberately retained after release so stale files
// never need path-based reclamation and cannot race a replacement owner.
func lockTaskFile(taskFilePath string) (unlock func() error, err error) {
	resolved, err := filepath.Abs(taskFilePath)
	if err != nil {
		return nil, fmt.Errorf("resolve task path: %w", err)
	}
	resolved = filepath.Clean(resolved)

	mu := taskMutexFor(resolved)
	if !mu.TryLock() {
		return nil, ErrTaskBusy
	}

	lockPath := taskLockSidecarPath(resolved)
	file, busy, err := openTaskLockHandle(lockPath)
	if err != nil {
		mu.Unlock()
		return nil, err
	}
	if busy {
		mu.Unlock()
		return nil, ErrTaskBusy
	}

	var once sync.Once
	return func() error {
		once.Do(func() {
			if closeErr := file.Close(); closeErr != nil {
				slog.Warn("failed to close task lock file", "path", lockPath, "error", closeErr)
			}
			mu.Unlock()
		})
		return nil
	}, nil
}

// defaultGateLockBoundedWait bounds how long the gated completion path waits to
// acquire a contended task lock before failing fast with a retryable error.
const defaultGateLockBoundedWait = 3 * time.Second

// defaultGateLockHeartbeat is the sidecar ModTime refresh interval used while a
// gate holds the lock. It is strictly less than taskStaleLockTTL so a long gate
// run (timeout_seconds may exceed the 60s TTL) never lets a concurrent
// cross-process caller treat the live lock as crash residue and reap it mid-gate.
const defaultGateLockHeartbeat = 20 * time.Second

// lockTaskFileWithHeartbeat acquires the per-task advisory lock with a bounded
// wait (retrying on ErrTaskBusy up to boundedWait with backoff, honoring
// ctx.Done()) and starts a heartbeat that refreshes the lock sidecar's ModTime on
// the heartbeat interval for the whole hold. On contention past the bounded wait
// it returns a wrapped ErrGateInProgress (retryable). The returned unlock stops
// the heartbeat, waits for it to exit, then releases the underlying lock; it is
// safe to call multiple times and callers MUST defer it.
func lockTaskFileWithHeartbeat(ctx context.Context, taskFilePath string, boundedWait, heartbeat time.Duration) (func() error, error) {
	deadline := time.Now().Add(boundedWait)
	backoff := 20 * time.Millisecond

	var unlock func() error
	for {
		var err error
		unlock, err = lockTaskFile(taskFilePath)
		if err == nil {
			break
		}
		if !errors.Is(err, ErrTaskBusy) {
			// A genuine IO fault (permission, missing dir) — not contention.
			return nil, err
		}
		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("task %s: %w", taskFilePath, bkerrors.ErrGateInProgress)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 250*time.Millisecond {
			backoff *= 2
		}
	}

	resolved, err := filepath.Abs(taskFilePath)
	if err != nil {
		// Best-effort: fall back to the raw path for the sidecar.
		resolved = taskFilePath
	}
	sidecar := taskLockSidecarPath(filepath.Clean(resolved))

	stop := make(chan struct{})
	var wg sync.WaitGroup
	if heartbeat > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(heartbeat)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					now := time.Now()
					if chErr := os.Chtimes(sidecar, now, now); chErr != nil && !os.IsNotExist(chErr) {
						slog.Warn("gate lock heartbeat failed", "path", sidecar, "error", chErr)
					}
				}
			}
		}()
	}

	var once sync.Once
	return func() error {
		once.Do(func() {
			close(stop)
			wg.Wait()
		})
		return unlock()
	}, nil
}
