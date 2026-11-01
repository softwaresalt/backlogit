package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// staleLockTTL is the age threshold after which an unremoved .lock sidecar is
// eligible for recovery only after OS-exclusive ownership is acquired.
const staleLockTTL = 60 * time.Second

// stashMu serializes concurrent same-process stash mutations. Combined with the
// .lock sidecar file, this provides protection within a single process (mutex)
// and across processes (file lock).
var stashMu sync.Mutex

// lockStashFile creates an advisory lock sidecar (.lock) next to path.
// It first acquires the in-process mutex to serialize goroutines in the same
// process, then creates the sidecar file to guard against other processes.
// Old sidecars are recovered only if their OS lock is not owned by a live process.
// Returns an unlock function that removes the sidecar and releases the mutex.
func lockStashFile(path string) (unlock func() error, err error) {
	stashMu.Lock()
	lockPath := path + ".lock"
	f, createErr := acquireStashSidecarFile(lockPath)
	if createErr != nil {
		stashMu.Unlock()
		return nil, fmt.Errorf("stash locked by another process: %w", createErr)
	}
	return func() error {
		defer stashMu.Unlock()
		return releaseStashSidecarFile(f)
	}, nil
}

// acquireStashSidecarFile retains OS ownership until release. Age alone never
// authorizes stealing a live lock, including one whose owner has been suspended.
func acquireStashSidecarFile(lockPath string) (*os.File, error) {
	lockPath, err := filepath.Abs(lockPath)
	if err != nil {
		return nil, fmt.Errorf("resolve stash sidecar: %w", err)
	}
	if err := validateStashSidecarComponents(lockPath); err != nil {
		return nil, err
	}
	info, err := os.Lstat(lockPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat stash sidecar: %w", err)
	}
	if err == nil && time.Since(info.ModTime()) <= staleLockTTL {
		return nil, fmt.Errorf("fresh stash sidecar: %w", os.ErrExist)
	}
	f, busy, err := openStashLockHandle(lockPath)
	if err != nil {
		return nil, fmt.Errorf("open stash sidecar ownership: %w", err)
	}
	if busy {
		return nil, fmt.Errorf("live stash sidecar owner: %w", os.ErrExist)
	}
	fail := func(err error) (*os.File, error) {
		return nil, errors.Join(err, f.Close())
	}
	owned, err := f.Stat()
	if err != nil {
		return fail(fmt.Errorf("stat owned stash sidecar: %w", err))
	}
	if !owned.Mode().IsRegular() {
		return fail(fmt.Errorf("nonregular stash sidecar: %w", os.ErrPermission))
	}
	current, err := os.Lstat(lockPath)
	if err != nil {
		return fail(fmt.Errorf("stat current stash sidecar: %w", err))
	}
	// An opener racing an unlink must not own an orphaned inode. An opener
	// that observed an old lock must not take its fresh replacement either.
	if !os.SameFile(owned, current) || (info != nil &&
		(!os.SameFile(info, current) || !info.ModTime().Equal(current.ModTime()))) {
		return fail(fmt.Errorf("stash sidecar changed during acquisition: %w", os.ErrExist))
	}
	// Truncate/write via the retained handle refreshes mtime on Windows too,
	// where path-based Chtimes cannot open our exclusively held sidecar.
	if err := f.Truncate(0); err != nil {
		return fail(fmt.Errorf("truncate owned stash sidecar: %w", err))
	}
	if _, err := f.WriteString(time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fail(fmt.Errorf("refresh owned stash sidecar: %w", err))
	}
	return f, nil
}

// validateStashSidecarComponents rejects links before any creation or refresh.
// Platform openers additionally bind acquisition to a checked directory and
// reject linked handles; pathname checks alone are not a TOCTOU defense.
func validateStashSidecarComponents(path string) error {
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) && current == path {
			continue
		}
		if err != nil {
			return fmt.Errorf("lstat stash sidecar component: %w", err)
		}
		linked, err := isSymlinkOrReparse(info, current)
		if err != nil {
			return fmt.Errorf("check stash sidecar component: %w", err)
		}
		if linked || (current == path && !info.Mode().IsRegular()) ||
			(current != path && !info.IsDir()) {
			return fmt.Errorf("linked or nonregular stash sidecar component: %w", os.ErrPermission)
		}
		if filepath.Dir(current) == current {
			return nil
		}
	}
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

// stashLockHeldContextKey marks a context whose call tree already holds the
// active stash lock, so nested helpers must not re-acquire the non-reentrant
// mutex.
type stashLockHeldContextKey struct{}

// withStashLockHeld marks ctx as running under the active stash lock.
func withStashLockHeld(ctx context.Context) context.Context {
	return context.WithValue(ctx, stashLockHeldContextKey{}, struct{}{})
}

// stashLockHeld reports whether ctx runs under the active stash lock.
func stashLockHeld(ctx context.Context) bool {
	_, held := ctx.Value(stashLockHeldContextKey{}).(struct{})
	return held
}

// tryLockActiveStash is a non-blocking lockActiveStash. Recovery runs under the
// lifecycle-global lock, which sits after the stash lock in the canonical
// order, so it must never wait for the stash lock; a busy lock is reported as
// an error and the recovery journal is retained for the next barrier.
func tryLockActiveStash(ws *Workspace) (unlock func() error, err error) {
	path := StashFilePath(ws.RootPath)
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil {
		return nil, fmt.Errorf("ensure stash directory: %w", mkErr)
	}
	if !stashMu.TryLock() {
		return nil, fmt.Errorf("stash lock is busy: %w", os.ErrExist)
	}
	f, createErr := acquireStashSidecarFile(path + ".lock")
	if createErr != nil {
		stashMu.Unlock()
		return nil, fmt.Errorf("stash locked by another process: %w", createErr)
	}
	return func() error {
		defer stashMu.Unlock()
		return releaseStashSidecarFile(f)
	}, nil
}
