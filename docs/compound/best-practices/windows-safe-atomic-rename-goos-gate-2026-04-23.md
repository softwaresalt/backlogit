---
chunk_strategy: h1-h2-h3
description: Gate pre-rename removal to Windows so os.Rename works on Windows without breaking POSIX atomic replacement semantics.
doc_type: learning
docline:
    category: best_practice
    component: event_log
    date: 2026-04-23T00:00:00Z
    file_path: internal/events/fsutil.go
    message: os.Rename fails with ERROR_ALREADY_EXISTS on Windows; gating os.Remove on runtime.GOOS == "windows" preserves POSIX atomicity while fixing Windows compat
    problem_type: best_practice
    resolution_type: feature_gate
    resolved: true
    root_cause: platform_compat
    severity: high
    tags:
        - go
        - os.Rename
        - Windows
        - atomic
        - rename
        - cross-platform
        - runtime.GOOS
        - temp-file-rename
        - write-durability
        - fsync
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/best-practices/windows-safe-atomic-rename-goos-gate-2026-04-23.md
title: 'Windows-Safe Atomic Rename: Gate os.Remove on runtime.GOOS'
---

## Problem

A temp-file-then-rename write pattern requires different pre-rename behavior on
Windows vs POSIX. Unconditionally calling `os.Remove(dst)` before `os.Rename(src,
dst)` introduces an ENOENT window on POSIX that can cause readers to observe a
missing file during a crash. Omitting the pre-remove causes `os.Rename` to fail
with `ERROR_ALREADY_EXISTS` on Windows.

## Symptoms

**On Windows (without pre-remove):**
```text
rename tmp.json path.json: The file cannot be accessed by the system.
// or: The process cannot access the file because it is being used by another process.
```

**On POSIX (with unconditional pre-remove):**
- Brief ENOENT window between `Remove` and `Rename` if a crash occurs in that gap
- Reader processes can observe a missing file for regenerable content (checkpoints)
- Reader processes can permanently lose data for non-regenerable content (event queues)

## What Did Not Work

**Unconditional pre-remove** — `_ = os.Remove(dst); os.Rename(tmp, dst)`:
- POSIX: introduces ENOENT crash window; breaks atomicity guarantee
- Windows: works, but correctness depended on an accidental property

**Build-tag-based platform files** (`_windows.go` / `_unix.go`):
- Overkill for a single conditional
- Duplicate function bodies increase maintenance surface

**`os.Link` + `os.Remove(src)`**:
- `os.Link` requires same filesystem (fails across `/tmp` → data drive)
- Not universally supported on Windows

## Solution

Gate the pre-remove exclusively on `runtime.GOOS == "windows"`:

### Before

```go
// Unconditional pre-remove — breaks POSIX atomicity
_ = os.Remove(dst)
if err := os.Rename(tmp, dst); err != nil {
    _ = os.Remove(tmp)
    return fmt.Errorf("rename: %w", err)
}
```

### After

```go
// On POSIX, os.Rename atomically replaces the destination (no pre-remove needed).
// On Windows, os.Rename fails when the destination already exists; remove first.
if runtime.GOOS == "windows" {
    _ = os.Remove(dst)
}
if err := os.Rename(tmp, dst); err != nil {
    _ = os.Remove(tmp)
    return fmt.Errorf("rename: %w", err)
}
```

Required import: `"runtime"` (standard library, no new dependency).

## Why This Works

POSIX specifies (IEEE Std 1003.1) that `rename(2)` shall atomically replace
`newpath` with `oldpath` when both exist. The kernel swaps the directory entry
in a single syscall — there is no window during which neither file exists. Go's
`os.Rename` maps directly to `rename(2)` on POSIX.

Windows NTFS does not expose an equivalent atomic replace syscall through the
standard Win32 API. `MoveFileEx` with `MOVEFILE_REPLACE_EXISTING` provides
near-atomic replace semantics, but Go's `os.Rename` currently falls back to a
non-atomic approach when `MOVEFILE_REPLACE_EXISTING` is unavailable for the
given path. The simplest correct fix is to pre-remove before rename on Windows.

`runtime.GOOS` is evaluated at program runtime; it has no compile-time cost
and produces no dead code. It is the idiomatic Go mechanism for narrow OS
bifurcations that do not warrant separate build-tag files.

## Prevention

- **Treat the POSIX atomic guarantee as a correctness invariant**, not a
  performance optimization. Remove the pre-remove for POSIX paths.
- **Add a Windows gate whenever you write a temp-then-rename helper.** The
  pattern appears in checkpoint writers, JSONL appenders, config writers, and
  any other "write to .tmp, then rename" flow.
- **Gate on `runtime.GOOS == "windows"` rather than `runtime.GOOS != "linux"`**.
  POSIX behavior holds on Linux, macOS, and all Unix variants; listing them
  individually is error-prone.
- **Document the gate with a comment** explaining the POSIX/Windows semantic
  difference so future readers don't "clean it up".

## Related Solutions

- `docs/compound/runtime-errors/windows-mojibake-utf8-powershell-fix-2026-04-08.md`
  — another Windows platform-compatibility fix in the same codebase; illustrates
  the `runtime.GOOS == "windows"` branching pattern.
