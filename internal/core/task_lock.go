package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
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

func newTaskLockToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate task lock token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func taskLockOwned(lockPath, token string) bool {
	data, err := os.ReadFile(lockPath)
	return err == nil && string(data) == token
}

// removeOwnedTaskLock atomically moves an owned sidecar to a token-specific
// recovery path before removing it. A replacement owner can therefore not be
// removed by a stale heartbeat or unlock callback.
func removeOwnedTaskLock(lockPath, token string) error {
	if !taskLockOwned(lockPath, token) {
		return nil
	}
	recoveryPath := fmt.Sprintf("%s.releasing-%s", lockPath, token)
	if err := os.Rename(lockPath, recoveryPath); err != nil {
		if os.IsNotExist(err) || !taskLockOwned(lockPath, token) {
			return nil
		}
		return err
	}
	if err := os.Remove(recoveryPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// reclaimStaleTaskLock atomically claims the observed stale sidecar before
// removing it. Competing reclaimers can only succeed once; a new owner is
// never removed by a second stale cleanup based on an old stat result.
func reclaimStaleTaskLock(lockPath, token string) (bool, error) {
	recoveryPath := fmt.Sprintf("%s.reclaim-%s", lockPath, token)
	if err := os.Rename(lockPath, recoveryPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if err := os.Remove(recoveryPath); err != nil && !os.IsNotExist(err) {
		return false, err
	}
	return true, nil
}

func createTaskLockSidecar(lockPath, token string) (*os.File, error) {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	if _, err := file.WriteString(token); err != nil {
		_ = file.Close()
		_ = os.Remove(lockPath)
		return nil, fmt.Errorf("write task lock token: %w", err)
	}
	return file, nil
}

// lockTaskFile acquires a per-task advisory lock for taskFilePath.
func lockTaskFile(taskFilePath string) (func() error, error) {
	unlock, _, err := lockTaskFileOwned(taskFilePath)
	return unlock, err
}

// lockTaskFileOwned acquires a per-task advisory lock and returns its ownership
// token for callers that need to refresh the sidecar without touching a
// replacement owner's lock.
// It first makes a NON-BLOCKING attempt on the per-path in-process mutex
// (TryLock), then
// creates an O_CREATE|O_EXCL sidecar for cross-process safety. A sidecar older
// than taskStaleLockTTL is treated as crash residue: it is removed with a WARN
// and creation is retried once. On a held mutex or a live sidecar it returns
// ErrTaskBusy without blocking. A sidecar-creation failure for any reason OTHER
// than "already exists" (permission, missing directory, read-only filesystem)
// is returned as an ordinary wrapped error — NOT ErrTaskBusy — so gate consumers
// preserve the busy-vs-IO exit-code contract. The returned unlock releases BOTH
// the sidecar and the mutex and is safe to call multiple times; callers MUST
// defer it so every error path releases both.
func lockTaskFileOwned(taskFilePath string) (unlock func() error, token string, err error) {
	resolved, err := filepath.Abs(taskFilePath)
	if err != nil {
		return nil, "", fmt.Errorf("resolve task path: %w", err)
	}
	resolved = filepath.Clean(resolved)

	mu := taskMutexFor(resolved)
	if !mu.TryLock() {
		// Another goroutine in this process holds the lock — non-blocking busy.
		return nil, "", ErrTaskBusy
	}

	lockPath := taskLockSidecarPath(resolved)
	token, err = newTaskLockToken()
	if err != nil {
		mu.Unlock()
		return nil, "", err
	}
	f, createErr := createTaskLockSidecar(lockPath, token)
	if createErr != nil {
		// Only an "already exists" (EEXIST) failure means contention. Any other
		// error (permission denied, missing parent directory, read-only
		// filesystem) is a genuine IO fault and must NOT be classified as busy:
		// doing so would break the gate exit-code contract (busy=4 vs io=3) and
		// trigger misleading contention retries. Surface those as ordinary errors.
		if !errors.Is(createErr, os.ErrExist) {
			mu.Unlock()
			return nil, "", fmt.Errorf("create task lock sidecar %s: %w", lockPath, createErr)
		}
		// The sidecar already exists. Inspect its age to distinguish a live lock
		// (busy) from crash residue (reclaimable).
		info, statErr := os.Stat(lockPath)
		switch {
		case statErr == nil && time.Since(info.ModTime()) <= taskStaleLockTTL:
			// A fresh sidecar is a live lock held by another operation.
			mu.Unlock()
			return nil, "", ErrTaskBusy
		case statErr != nil && errors.Is(statErr, os.ErrNotExist):
			// The sidecar vanished between OpenFile(EEXIST) and Stat — a race
			// with a concurrent release. Stay non-blocking: report busy so the
			// caller can retry rather than silently proceeding.
			mu.Unlock()
			return nil, "", ErrTaskBusy
		case statErr != nil:
			// A permission/IO error stat-ing the sidecar is NOT contention:
			// classifying it as busy would break the busy-vs-IO exit-code
			// contract. Surface it as an ordinary error.
			mu.Unlock()
			return nil, "", fmt.Errorf("stat task lock sidecar %s: %w", lockPath, statErr)
		}
		// Stale (older than the TTL) → crash residue: atomically claim it before
		// removing it so competing reclaimers cannot remove a replacement lock.
		slog.Warn("removing stale task lock file", "path", lockPath, "age", time.Since(info.ModTime()))
		claimed, claimErr := reclaimStaleTaskLock(lockPath, token)
		if claimErr != nil {
			// Failing to reclaim the stale sidecar (permission denied, read-only
			// filesystem) is an IO fault, not contention. Surface it as an ordinary
			// error instead of mapping it to the busy exit code.
			mu.Unlock()
			return nil, "", fmt.Errorf("reclaim stale task lock sidecar %s: %w", lockPath, claimErr)
		}
		if !claimed {
			mu.Unlock()
			return nil, "", ErrTaskBusy
		}
		f, createErr = createTaskLockSidecar(lockPath, token)
		if createErr != nil {
			mu.Unlock()
			if errors.Is(createErr, os.ErrExist) {
				// Another operation re-created the sidecar between reclaim and
				// recreate — genuine contention.
				return nil, "", ErrTaskBusy
			}
			return nil, "", fmt.Errorf("recreate task lock sidecar %s: %w", lockPath, createErr)
		}
	}
	_ = f.Close()

	var once sync.Once
	return func() error {
		once.Do(func() {
			if rmErr := removeOwnedTaskLock(lockPath, token); rmErr != nil {
				slog.Warn("failed to remove task lock file", "path", lockPath, "error", rmErr)
			}
			mu.Unlock()
		})
		return nil
	}, token, nil
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
	var lockToken string
	for {
		var err error
		unlock, lockToken, err = lockTaskFileOwned(taskFilePath)
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
					if !taskLockOwned(sidecar, lockToken) {
						return
					}
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
