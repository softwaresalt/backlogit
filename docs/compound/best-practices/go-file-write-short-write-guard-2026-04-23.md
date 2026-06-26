---
chunk_strategy: h1-h2-h3
description: Check that os.File.Write writes all bytes because short writes can occur without returning an error.
doc_type: learning
docline:
    category: best_practice
    component: event_log
    date: 2026-04-23T00:00:00Z
    file_path: internal/events/fsutil.go
    message: os.File.Write can return n < len(data) with err == nil; not checking n causes silent partial writes that corrupt JSONL and checkpoint files
    problem_type: best_practice
    resolution_type: code_fix
    resolved: true
    root_cause: incorrect_error_type
    severity: high
    tags:
        - go
        - os.File
        - Write
        - short-write
        - data-corruption
        - JSONL
        - write-durability
        - fsync
        - file-io
        - event_log
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/best-practices/go-file-write-short-write-guard-2026-04-23.md
title: 'Go os.File.Write Short-Write Guard: Always Check n == len(data)'
---

## Problem

`os.File.Write(data []byte) (n int, err error)` is a legal short-write path in
Go: the call can return `n < len(data)` with `err == nil`. When this happens,
only part of `data` was written to the file. If the caller ignores `n` and
proceeds to `f.Sync()` and `os.Rename`, the destination file silently contains
truncated content.

## Symptoms

- JSONL event queue contains a partial JSON object on the last line (parse error
  on next read)
- Checkpoint file is valid JSON for a portion of the payload; the rest is missing
- Corruption is non-deterministic and rare, making it difficult to reproduce in
  tests
- No error is returned by the write call; callers have no indication anything
  went wrong

## What Did Not Work

**Ignoring the return value** — `_, writeErr := f.Write(data)`:
- This is the default pattern copied from documentation examples that assume
  writing to bytes.Buffer or strings.Builder, where short writes cannot occur
- On `os.File`, it is silently incorrect

**Using `io.WriteString`** (for string payloads):
- Same issue: `io.WriteString` calls `Write` once internally and does not retry
  on short write

**Relying on `f.Sync()` to surface the error**:
- `f.Sync()` flushes kernel buffers to storage — it does not detect that fewer
  bytes than expected were written to the kernel buffer in the first place

## Solution

Capture `n` and treat `n != len(data)` as a hard error immediately after the
`Write` call:

### Before

```go
_, writeErr := f.Write(data)
syncErr := f.Sync()
closeErr := f.Close()
if writeErr != nil {
    _ = os.Remove(tmp)
    return fmt.Errorf("write data: %w", writeErr)
}
```

### After

```go
n, writeErr := f.Write(data)
if writeErr == nil && n != len(data) {
    writeErr = fmt.Errorf("short write: wrote %d of %d bytes", n, len(data))
}
syncErr := f.Sync()
closeErr := f.Close()
if writeErr != nil {
    _ = os.Remove(tmp)
    return fmt.Errorf("write data: %w", writeErr)
}
```

The short-write check MUST appear before `f.Sync()`. Calling `Sync()` on a
short-written file flushes the partial data to storage; subsequent callers may
then read truncated content even though we clean up with `os.Remove(tmp)`.

## Why This Works

The Go `io.Writer` interface contract (and the POSIX `write(2)` syscall it
ultimately calls) allows `n < len(p)` returns with nil error. This is rare for
`os.File` writes to local disk but can occur:

- Under low virtual memory conditions where the kernel truncates a buffer write
- On some network-mounted filesystems (NFS, FUSE drivers) with small write limits
- Under kernel stress tests or certain container memory-limit scenarios
- On non-standard `io.Writer` wrappers passed through interface values

`io.ReadFull` exists precisely because this problem is well-known for reads.
There is no `io.WriteFull` equivalent in the standard library, so the guard must
be written explicitly.

Checking before `f.Sync()` matters: sync flushes whatever is in the kernel
buffer for that file descriptor. A short write is in the buffer; sync would
durably store the partial data. Our cleanup (`os.Remove(tmp)`) then removes a
file that has already been fsynced — the remove is correct, but the window
during which the partial data was durable is real on some kernels.

## Prevention

- **Always capture the `n` return from `f.Write`.** If `n != len(data)` with
  nil error, synthesize an error immediately.
- **Apply the guard before any `f.Sync()` call.** Syncing partial data and then
  removing the file is safer than syncing after detecting the short write, but
  not syncing at all is best.
- **Use `fmt.Errorf("short write: wrote %d of %d bytes", n, len(data))`** — the
  error message provides enough context to distinguish this from a genuine write
  error in logs.
- **Consider wrapping repeated write-sync-rename logic in a single helper** (like
  `syncWriteFileAtomic`) so the guard is applied once and reused everywhere,
  rather than requiring every call site to remember the pattern.
- **In tests, consider wrapping `os.File` in a mock that returns a short write**
  to verify the guard is exercised.

## Related Solutions

- `docs/compound/best-practices/windows-safe-atomic-rename-goos-gate-2026-04-23.md`
  — companion fix applied in the same shipment (041-S); covers the rename
  atomicity pattern that the `syncWriteFileAtomic` helper also implements.
- `docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md`
  — related write-durability pattern at the SQLite layer.
