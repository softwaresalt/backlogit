// Package atomicfile provides a single hardened atomic file-write primitive
// (WriteFileAtomic) shared across the repository's document and archive write
// paths. It is a generic filesystem utility, NOT markdown-specific: it writes a
// byte slice to a destination path via a same-directory temp file plus an
// atomic rename so a partial or interrupted write can never leave a corrupt or
// truncated file in place.
//
// # Leaf package
//
// atomicfile imports only the standard library (fmt, io, os, path/filepath,
// runtime). It has no internal imports, so both internal/docline and
// internal/core can depend on it without an import cycle.
//
// # Path-agnostic contract (security)
//
// WriteFileAtomic performs NO path containment or validation. It writes wherever
// it is told. Callers are responsible for pre-validating the destination path
// (for example via internal/core.SafeResolve, or docline's ValidateApplyPath
// preflight) BEFORE calling WriteFileAtomic. Pushing containment to the caller
// keeps this package a stdlib-only leaf and avoids re-introducing the import
// cycle that core.SafeResolve would create.
//
// The temp file uses a non-".md" prefix (".atomicfile-*.tmp") so that markdown
// scanners in docline and doctor cannot pick up a half-written temp file
// mid-write.
//
// # Mode policy
//
// On overwrite the destination's existing mode is preserved but CLAMPED: the
// group/world write bits are stripped (perm &^ 0o022) so an in-place rewrite
// never perpetuates an over-permissive 0666/0777 source. Preserving the source
// mode also keeps a tightened 0600 record at 0600 rather than resetting it to
// the 0644 new-file default. A newly created file is written at 0644. (POSIX permission bits are not represented on Windows
// filesystems, where the mode is effectively advisory.)
//
// # Durability: opt-in durable_writes protocol
//
// By default WriteFileAtomic is the fast path: it does NOT fsync the temp file
// or its directory. os.Rename provides atomic VISIBILITY of the complete new
// content, which is the corruption guarantee callers need; rollback for docs and
// archive records is provided by git. The default path adds no fsync latency.
//
// WriteFileAtomicWithOptions(path, data, Options{DurableWrites: true}) opts into
// the durable_writes fsync protocol (123-F): it fsyncs the temp file before
// close, opens the parent-directory handle before the rename, and — on POSIX —
// fsyncs that handle after a successful rename so both the new file content and
// its dirent survive a power loss. The workspace-level durable_writes config flag
// (default false) is read at the composition root and threaded in as this option
// value; this package never imports config or core.
//
// # Windows atomic replace (unconditional safety fix)
//
// The rename goes through a build-tagged platform seam. On Windows it uses
// golang.org/x/sys/windows.MoveFileEx with MOVEFILE_REPLACE_EXISTING set
// UNCONDITIONALLY — replacing the legacy remove-before-rename fallback that could
// leave the canonical file MISSING if the process crashed (or a second rename
// failed) between the remove and the retry. That atomic-replace safety half is a
// correctness fix and applies in BOTH durable and non-durable modes. The
// MOVEFILE_WRITE_THROUGH durability flush is a synchronous cost, so it is added
// only when durable_writes is on (flags = REPLACE_EXISTING | (durable ?
// WRITE_THROUGH : 0)). ReplaceFileW is rejected (unreliable write-through; its
// failure modes can leave the destination absent). On POSIX the seam is a plain
// os.Rename, whose durability is provided by the parent-directory fsync above.
//
// # Two-class outcome-based error contract
//
// A durable write classifies its failures by OUTCOME (see internal/errors):
//
//   - ErrWriteNotApplied — any failure BEFORE the rename commits (create temp,
//     write, chmod, temp fsync, close, or the rename itself). The destination is
//     untouched, so the failed atomic write is safe to retry.
//   - ErrWriteIndeterminate — a parent-directory fsync failure AFTER a successful
//     rename. The file is already visibly replaced, so the outcome is uncertain;
//     callers MUST NOT blindly retry.
//
// # Platform-asymmetric guarantee
//
// POSIX gets full power-loss durability in durable mode, including the new-file
// dirent (temp fsync + parent-dir fsync). Windows gets file-content durability
// via WRITE_THROUGH plus the unconditional atomic replace, but directory-entry
// durability is best-effort: Windows exposes no directory-handle flush, so the
// parent-dir fsync is skipped there (runtime.GOOS == "windows").
//
// # fsync micro-benchmark and critical-vs-bulk policy
//
// atomicfile_bench_test.go records the fsync overhead (BenchmarkWriteFileAtomic_
// DurableOff vs _DurableOn and BenchmarkFsyncFile). Because the durable path adds
// measurable per-write latency, durable_writes is applied to CRITICAL single-item
// mutations (for example the size seam and status/artifact rewrites when the flag
// is enabled) while BULK regeneration stays on the fast, sync-free default path.
package atomicfile
