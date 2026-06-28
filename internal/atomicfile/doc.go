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
// never perpetuates an over-permissive 0666/0777 source and never downgrades a
// tightened 0600 record to the 0600 temp default by accident. A newly created
// file is written at 0644. (POSIX permission bits are not represented on Windows
// filesystems, where the mode is effectively advisory.)
//
// # Sync-free by design
//
// WriteFileAtomic deliberately does NOT fsync the temp file or its directory.
// os.Rename provides atomic VISIBILITY of the complete new content, which is the
// corruption guarantee callers need; durability and rollback for docs and
// archive records are provided by git, not by fsync. Adding fsync is out of
// scope and intentionally omitted to keep the writer fast and simple.
package atomicfile
