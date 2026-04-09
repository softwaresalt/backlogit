---
title: "Advisory File Lock with Stale TTL — Go Pattern for Crash-Safe Sidecar Locks"
problem_type: best_practice
category: best_practice
component: task_manager
root_cause: sqlite_locking
resolution_type: code_fix
severity: medium
message: "O_CREATE|O_EXCL sidecar lock files persist after process crash; subsequent writes fail until manual cleanup."
file_path: "internal/core/stash_lock.go"
resolved: true
tags: [go, file-lock, sidecar, crash-safety, stash, concurrency, advisory-lock, ttl]
date: 2026-04-08
---

## Problem

Advisory file locks created with `os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL, 0600)`
persist on disk when the process crashes or is killed while holding the lock.
The next caller attempts `O_CREATE|O_EXCL`, receives `EEXIST`, and returns an
error without any recovery mechanism. All subsequent stash writes fail until an
operator manually removes the sidecar file.

## Symptoms

* `"stash locked by another process"` error on every stash write after a crash
* Lock sidecar (`.stash.jsonl.lock`) exists with an old modification time
* `backlogit_stash` and `backlogit_harvest_stash` both fail
* No recovery without manual `rm .backlogit/stash.jsonl.lock`

## What Did Not Work

Using a `sync.Mutex` alone — it protects in-process concurrency but resets on
restart, leaving the on-disk sidecar orphaned. Ignoring the error on `O_EXCL`
failure and retrying immediately caused a busy-loop that consumed CPU without
recovery. Using the lock file's content (PID) to detect dead processes works
on Unix but is unreliable on Windows where PIDs are reused.

## Solution

On `O_CREATE|O_EXCL` failure, check the sidecar's modification time. If it is
older than a configurable TTL (60 seconds is appropriate for interactive
operations), log a warning, remove the sidecar, and retry once. If the retry
also fails, propagate the error — a second failure indicates a live concurrent
writer.

```go
package core

import (
    "fmt"
    "log/slog"
    "os"
    "sync"
    "time"
)

const staleLockTTL = 60 * time.Second

var stashMu sync.Mutex

func lockStashFile(lockPath string) (func() error, error) {
    stashMu.Lock()

    unlock := func() error {
        defer stashMu.Unlock()
        return os.Remove(lockPath)
    }

    if err := tryCreateLock(lockPath); err != nil {
        stashMu.Unlock()
        return nil, err
    }
    return unlock, nil
}

func tryCreateLock(lockPath string) error {
    f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
    if err == nil {
        _ = f.Close()
        return nil
    }

    // Lock exists — check if it is stale.
    info, statErr := os.Stat(lockPath)
    if statErr != nil {
        return fmt.Errorf("stash locked and stat failed: %w", err)
    }
    age := time.Since(info.ModTime())
    if age <= staleLockTTL {
        return fmt.Errorf("stash locked by another process (lock age: %s)", age.Round(time.Second))
    }

    // Stale lock — remove and retry once.
    slog.Warn("stale stash lock detected, removing",
        "lock_path", lockPath,
        "age", age.Round(time.Second),
    )
    if removeErr := os.Remove(lockPath); removeErr != nil {
        return fmt.Errorf("stash locked and stale lock removal failed: %w", removeErr)
    }

    f2, err2 := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
    if err2 != nil {
        return fmt.Errorf("stash locked even after stale lock removal: %w", err2)
    }
    _ = f2.Close()
    return nil
}
```

### Correct Lock Scope in Harvest

Acquire the lock before any read of the stash file, hold it through the write,
and release it before any non-stash I/O (e.g., `CreateArtifact`). Releasing
before artifact creation avoids holding the lock during potentially slow
filesystem operations.

```go
func HarvestStashEntry(ws, stashID string) (*models.Artifact, error) {
    lockPath := stashLockPath(ws)
    unlock, err := lockStashFile(lockPath)
    if err != nil {
        return nil, fmt.Errorf("harvest stash: %w", err)
    }

    entries, err := readStashEntries(ws)
    if err != nil {
        _ = unlock()
        return nil, err
    }
    filtered := removeEntry(entries, stashID)
    if err := writeStashEntries(ws, filtered); err != nil {
        _ = unlock()
        return nil, err
    }
    _ = unlock() // Release before CreateArtifact — not a stash-file operation

    return CreateArtifact(ws, ...)
}
```

## Why This Works

The TTL check converts an indefinite failure mode (permanent orphaned lock) into
a bounded one (60 s stale window). The `sync.Mutex` prevents in-process races;
the sidecar prevents cross-process races. The single retry after stale removal
handles the race between two processes that both detect stale and both try to
remove — only one succeeds the retry; the other gets a proper "locked" error
and backs off.

Releasing the lock before non-stash operations is correct because the lock
protects only the stash file. Holding it longer would block concurrent stash
reads unnecessarily.

## Prevention

* Always pair in-process `sync.Mutex` with an on-disk sidecar for file locks
  that must survive process restart visibility.
* Set a `staleLockTTL` that is comfortably longer than any expected normal
  operation but short enough to recover from a crash within one agent session.
* Log a `WARN` on stale detection so operators know a crash recovery occurred.
* Test with a scenario that leaves a stale sidecar (create it manually with an
  old mtime) and verify the next `lockStashFile` succeeds after one warning.
* Do not defer `unlock()` when the unlock must happen before non-stash
  operations — use explicit `_ = unlock()` calls on each error path instead.

## Related Solutions

* `go-patterns/f015-shipment-stash-patterns.md` — JSONL append patterns that
  this locking mechanism protects
