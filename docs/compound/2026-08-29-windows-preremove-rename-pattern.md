---
chunk_strategy: h1-h2-h3
description: "Windows os.Rename pre-Remove blocks are universally unnecessary in Go 1.24.0"
doc_type: compound
schema_version: "1.0"
source: 133-S closure
title: Go 1.24.0 os.Rename on Windows — pre-Remove is unnecessary and dangerous
---

# Go 1.24.0 os.Rename on Windows — pre-Remove is Unnecessary and Dangerous

## Pattern

Code like:

```go
if runtime.GOOS == "windows" {
    _ = os.Remove(dst)
}
if err := os.Rename(src, dst); err != nil { ... }
```

This is a legacy workaround for old Go versions where `os.Rename` on Windows could not replace an existing file. It creates a data-loss window: if `os.Rename` fails after `os.Remove` succeeds, any file previously at `dst` is permanently destroyed.

## Why it is unnecessary

Go 1.24.0 `os.Rename` on Windows calls `MoveFileExW(MOVEFILE_REPLACE_EXISTING)`, which handles an existing `dst` without requiring a pre-Remove. The property the fix relies on: replacement is attempted without an explicit delete. This does not guarantee crash-atomic replacement, but it does eliminate the data-loss window.

## Locations found and fixed in this codebase

| Shipment | File | Function | Variable | Commit |
|----------|------|----------|----------|--------|
| 132-S (CB71B412) | internal/events/fsutil.go | syncWriteFileAtomic | path | (149-F) |
| 133-S (11FFF601) | internal/events/checkpoint_lifecycle.go | CleanupCheckpoints | dst | 1ace3861 |

## Detection

The pattern is detectable via AST inspection. Example: `TestCleanupCheckpoints_NoPreRemoveInAST` and `TestSyncWriteFileAtomic_NoPreRemoveInAST` are P-002 FC-1 regression guards.

## Scan recommendation

Before adding any new archive/move/rename patterns that target Windows, verify no pre-Remove block is added. Run `grep -rn "os.Remove" internal/` after any rename-related change to ensure no new instances appear.

## Related stash entries

- CB71B412 (fixed, 132-S) — syncWriteFileAtomic
- 11FFF601 (fixed, 133-S) — CleanupCheckpoints
- 302EFF07 (pending, symlink rejection for read paths) — related safety theme, different package
