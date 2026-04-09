---
title: "Atomic SQLite Rehydration — Wrap DELETE + WalkDir in a Single Transaction"
problem_type: database_issue
category: database_issue
component: db_cache
root_cause: timeout
resolution_type: code_fix
severity: high
message: "Rehydrate() cleared items with DELETE then rebuilt via WalkDir with no transaction; crash mid-walk yields empty index."
file_path: "internal/db/rehydration.go"
resolved: true
tags: [sqlite, transaction, rehydration, atomicity, crash-safety, cqrs, go]
date: 2026-04-08
---

## Problem

`Rehydrate()` executed three separate operations without a transaction:

1. `DELETE FROM items`
2. `DELETE FROM item_deps`
3. `WalkDir` + per-file `UpsertItem` calls

Any crash or context cancellation between step 1 and the end of step 3 left the
SQLite index empty. Every subsequent MCP tool call returned zero results until a
successful full rehydration completed.

## Symptoms

* `backlogit_get_queue` returns an empty list immediately after a restart
* `backlogit_list_items` returns zero results despite `.md` files in `.backlogit/`
* Log shows `rehydrate: walk started` but no `rehydrate: commit` line
* Index row count drops to 0 mid-session

## What Did Not Work

Retrying on error without a transaction — the retry itself cleared the index
again before rebuilding, so two rapid failures compounded the problem. Deferring
`tx.Rollback()` alone (without `tx.Commit()` on success) also failed because the
deferred rollback fired whether the walk succeeded or not.

## Solution

Wrap the entire clear-and-rebuild cycle in a single `*sql.Tx`. Use deferred
`tx.Rollback()` as the crash guard; call `tx.Commit()` explicitly only after
`WalkDir` completes without error.

Replace all per-item `db.ExecContext` calls inside the walk with
`tx.ExecContext` equivalents, and thread `*sql.Tx` through a new `upsertItemTx`
helper.

### Before

```go
func Rehydrate(ctx context.Context, db *sql.DB, workspacePath string) (int, error) {
    if _, err := db.ExecContext(ctx, "DELETE FROM items"); err != nil {
        return 0, fmt.Errorf("rehydrate: clear items: %w", err)
    }
    if _, err := db.ExecContext(ctx, "DELETE FROM item_deps"); err != nil {
        return 0, fmt.Errorf("rehydrate: clear deps: %w", err)
    }
    count := 0
    err := filepath.WalkDir(workspacePath, func(path string, d fs.DirEntry, err error) error {
        // ...
        return upsertItem(ctx, db, artifact) // uses db directly
    })
    return count, err
}
```

### After

```go
func Rehydrate(ctx context.Context, db *sql.DB, workspacePath string) (int, error) {
    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return 0, fmt.Errorf("rehydrate: begin tx: %w", err)
    }
    defer func() { _ = tx.Rollback() }() // no-op after Commit

    if _, err := tx.ExecContext(ctx, "DELETE FROM items"); err != nil {
        return 0, fmt.Errorf("rehydrate: clear items: %w", err)
    }
    if _, err := tx.ExecContext(ctx, "DELETE FROM item_deps"); err != nil {
        return 0, fmt.Errorf("rehydrate: clear deps: %w", err)
    }

    count := 0
    err = filepath.WalkDir(workspacePath, func(path string, d fs.DirEntry, walkErr error) error {
        // ...
        return upsertItemTx(ctx, tx, artifact) // uses *sql.Tx
    })
    if err != nil {
        return 0, fmt.Errorf("rehydrate: walk: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return 0, fmt.Errorf("rehydrate: commit: %w", err)
    }
    return count, nil
}

// upsertDependencyTx is the transactional counterpart to the old
// upsertDependencyBestEffort, which used db directly and was deleted.
func upsertDependencyTx(ctx context.Context, tx *sql.Tx, itemID, depID, depType string) error {
    _, err := tx.ExecContext(ctx,
        `INSERT OR REPLACE INTO item_deps (item_id, depends_on, dep_type) VALUES (?, ?, ?)`,
        itemID, depID, depType,
    )
    return err
}
```

## Why This Works

SQLite's BEGIN/COMMIT wraps all mutations into an atomic unit. The deferred
`Rollback()` fires on any error path (including panics), restoring the index to
its pre-rehydration state rather than leaving it empty. Only a successful,
complete walk reaches `Commit()`.

The key is threading `*sql.Tx` through every helper that touches `items` or
`item_deps` during the walk. Helpers that use `*sql.DB` directly bypass the
transaction and can interleave with it — delete those or rename them to make
the boundary explicit.

Operations that run outside the items transaction (`rehydrateStash`,
`rehydrateItemLogs`) are intentionally excluded; they write to separate tables
and can safely run after commit.

## Prevention

* Any function that clears and rebuilds a table should open a transaction before
  the DELETE and commit only after the rebuild is verified complete.
* Name transactional helpers with a `Tx` suffix (`upsertItemTx`,
  `upsertDependencyTx`) to make the boundary visible at the call site.
* Remove `BestEffort` variants that use `*sql.DB` directly once a transactional
  replacement exists — they create silent bypass paths.
* Add a test that cancels the context mid-walk and verifies the row count equals
  the pre-rehydration count (not zero).

## Related Solutions

* `go-patterns/f015-shipment-stash-patterns.md` — JSONL append-only patterns
  that complement the SQLite transaction approach for mixed storage systems
