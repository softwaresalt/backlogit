---
chunk_strategy: h1-h2-h3
description: ""
doc_type: learning
docline:
    category: best_practice
    component: cli
    date: 2026-04-23T00:00:00Z
    file_path: internal/core/delete_crashsafe_042.go
    message: when os.Remove(tempPath) fails after a DB delete in a rename→DB→remove sequence, rename temp back to the original canonical path before returning the error
    problem_type: best_practice
    resolution_type: code_fix
    resolved: true
    root_cause: incorrect_error_type
    severity: medium
    tags:
        - crash-safety
        - delete
        - atomicity
        - file-rename
        - rollback
        - recovery
        - golang
        - best-practice
        - workspace-integrity
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/best-practices/crash-safe-delete-rename-rollback-go-2026-04-23.md
title: 'Crash-Safe Delete: Rename Temp Back to Original Path on os.Remove Failure'
---

# Crash-Safe Delete: Rename Temp Back to Original Path on os.Remove Failure

## Problem

Implementing a crash-safe `DeleteArtifact` follows the sequence:

1. Rename `artifact.md` → `artifact.deleting.md`
2. Delete from DB
3. `os.Remove("artifact.deleting.md")`

If step 3 fails (file locked on Windows, permission error, etc.) after step 2
already succeeded, the artifact is deleted from the DB but the file sits at
`artifact.deleting.md`. The file is orphaned: not at its canonical `.md` path,
so `FindArtifactPath`, rehydration, and `backlogit sync` cannot discover it
without a full directory scan for `.deleting.md` files.

## Symptoms

- `os.Remove` returns an error.
- `backlogit list` no longer shows the artifact (DB entry gone).
- The `.md` file is absent from `.backlogit/queue/` but a `.deleting.md` file
  exists there.
- `backlogit sync` does not restore the artifact because it only looks for `.md`
  extension files.
- Orphaned `.deleting.md` files accumulate over sessions on Windows.

## What Did Not Work

Returning the error directly with no recovery:

```go
if err := os.Remove(tempPath); err != nil {
    return fmt.Errorf("remove artifact file: %w", err)
    // File stuck at .deleting.md, DB entry already gone → silently orphaned.
}
```

## Solution

After a failed `os.Remove`, rename the temp file back to the original canonical
path before returning the error. This makes the file discoverable by rehydration
even though its DB entry is gone — the next `backlogit sync` will recreate the
DB entry from the file.

### Before

```go
if err := os.Remove(tempPath); err != nil {
    return fmt.Errorf("remove artifact file: %w", err)
}
```

### After

```go
if err := os.Remove(tempPath); err != nil {
    // Best-effort: rename temp back to its original path so it remains
    // discoverable and can be cleaned up by the next rehydration or sync.
    if renameErr := os.Rename(tempPath, filePath); renameErr != nil {
        slog.Error("delete artifact: DB deleted but temp file stuck; workspace may be inconsistent",
            "temp_path", tempPath, "original_path", filePath,
            "error", err, "rename_error", renameErr)
    }
    return fmt.Errorf("remove artifact file: %w", err)
}
```

The same rollback pattern applies to the DB-delete failure case (step 2):

```go
if err := s.db.DeleteItemCascade(ctx, artifactID); err != nil {
    // Rollback: rename temp back so the artifact is fully restored.
    if renameErr := os.Rename(tempPath, filePath); renameErr != nil {
        slog.Error("delete artifact: DB delete failed and rename rollback failed",
            "temp_path", tempPath, "original_path", filePath,
            "db_error", err, "rename_error", renameErr)
    }
    return fmt.Errorf("delete artifact from db: %w", err)
}
```

## Why This Works

`FindArtifactPath` and the rehydration engine search for files with `.md`
extension. Renaming the temp file back to `artifact.md` puts it at the exact
path both systems expect. The next `backlogit sync` detects the file, finds no
matching DB row, and recreates the entry from the YAML frontmatter — completing
automatic recovery with no manual intervention.

If the rename-back also fails (double fault), the error is logged with all four
diagnostic values (`temp_path`, `original_path`, `error`, `rename_error`) so an
operator can manually move the file. The original `os.Remove` error is still
propagated to the caller.

## Prevention

- **Name the temp file with `.deleting.md`** (marker before the extension), not
  `.md.deleting` (extension after). This keeps the extension intact so that a
  future extension-aware scanner could find it if rename-back fails.
- **Always rename temp back before returning any error** that follows a
  successful DB delete. The DB delete is the point of no return for the DB state;
  the file system must be left in the most discoverable state possible.
- **Design `sync` to treat any `.deleting.md` file as a recovery artifact** —
  either complete the deletion or rename it back, depending on whether the DB
  entry exists.
- Test `os.Remove` failure explicitly in harness tests using a file that is made
  read-only or locked before calling `DeleteArtifact`.

## Related Solutions

- [windows-safe-atomic-rename-goos-gate-2026-04-23.md](windows-safe-atomic-rename-goos-gate-2026-04-23.md) —
  cross-platform `os.Rename` considerations; Windows holds file locks longer
  than Linux, which is the primary trigger for this failure mode.
- [go-file-write-short-write-guard-2026-04-23.md](go-file-write-short-write-guard-2026-04-23.md) —
  related defensive file I/O pattern using guard checks before write operations.
