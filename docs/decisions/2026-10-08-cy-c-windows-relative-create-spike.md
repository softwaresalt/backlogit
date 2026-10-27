---
title: "CY-C spike: Windows directory-relative create for reconcile writers"
date: 2026-10-27
status: decided
task: 201.004-T
feature: 201-F
shipment: 201-S
---

# CY-C spike: Windows directory-relative create for reconcile writers

## Question

`writeShipmentReconcileArchiveFileHandleRelative` (Windows) leaks the real
archive payload to an outside location when an intermediate directory of the
archive path is swapped for a junction after validation. Which approach removes
the leak with the primitives available in `golang.org/x/sys/windows`?

## Approaches compared

| Approach | Mechanism | Removes the payload leak? |
|---|---|---|
| A. `NtCreateFile` with a root directory handle | Open the target directory once, then create the temp file with `OBJECT_ATTRIBUTES.RootDirectory` = that handle and rename it with `FileRenameInformation.RootDirectory` = the same handle. | Yes. No create, write, or rename re-walks the directory path. |
| B. Stage in a verified in-workspace temp file, then rename by handle | Create the temp file by path in a verified directory, write, then rename it. | No. The temp create is itself a path walk (the leak step). A handle rename that names its destination by path, not by a `RootDirectory` handle, re-walks the swapped path again. B only works if its rename uses a `RootDirectory` handle, which is approach A. |

## Chosen approach

Approach A. Implemented in `internal/core/shipment_reconcile_dirpin_windows.go`
and used by both Windows reconcile writers (`shipment_reconcile_fs_windows.go`,
`shipment_reconcile_append_windows.go`).

## How the target directory is pinned

1. The caller resolves the directory with `filepath.EvalSymlinks` (unchanged).
2. `pinShipmentReconcileDir` opens that path with `CreateFile`
   (`FILE_FLAG_BACKUP_SEMANTICS | FILE_FLAG_OPEN_REPARSE_POINT`) and
   `FILE_SHARE_READ | FILE_SHARE_WRITE` only, with no `FILE_SHARE_DELETE`.
3. On the opened handle it rejects a reparse point or a non-directory and
   requires `GetFinalPathNameByHandle` to equal the resolved path. A swap that
   happened before the open is rejected with nothing written.
4. Every later create and rename is relative to that handle. The handle names
   the directory object, not a path, so a swap of the path after the pin cannot
   redirect a write. The missing `FILE_SHARE_DELETE` makes Windows refuse a
   rename or delete of the pinned directory while the
   handle is open (Windows also normally refuses to rename an ancestor of an
   open handle, but this is not relied on), so the object cannot be moved outside the workspace after the
   check. A rename by handle still names its target by a single path component
   resolved against the pinned handle, never by an absolute path.

The new file name must be a single path component; separators and `:` are
rejected.

## Evidence

* `TestWriteShipmentReconcileArchiveFileHandleRelative_TOCTOUSwapLeaksRealPayload`
  (flipped by 201.005-T to assert no payload byte reaches the outside location)
  failed on the pre-change writer, with the full payload found in the outside
  directory, and passes with approach A. The test ran on Windows (not skipped).
* The three existing junction TOCTOU tests for the archive writer and the
  append TOCTOU tests still pass, as do the happy-path and fsync-seam tests.
* `NtCreateFile` and `NtSetInformationFile` are provided by
  `golang.org/x/sys/windows` v0.39.0 (`FileRenameInformation` buffer is built by
  hand with `encoding/binary`, so no `unsafe` import is needed).

## Residual risk

* A swap that lands between `EvalSymlinks` and the pin is caught only by the
  handle check (reject, nothing written), not prevented.
* The pin blocks rename and delete of the directory for the duration of the
  write; a concurrent process that needs delete access to the directory in that
  window fails and retries. The window is one write.
* `FileRenameInformation` with `ReplaceIfExists` replaces by NT semantics, not
  `MOVEFILE_REPLACE_EXISTING`; both replace an existing regular file. If the
  existing target is open without `FILE_SHARE_DELETE` the rename fails as before
  (reported as not applied; the temp file is deleted).
* Windows-only behavior: Linux CI cannot exercise it. The CY-C closure attaches
  the Windows run of the C4 test.
* Non-Windows writers are unchanged.
