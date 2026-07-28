package errors

import "errors"

// Two-class outcome-based durability error contract (123-F).
//
// These sentinels classify a failed durable write by its OUTCOME on the
// canonical file/log, not by the write phase. They live in the errors leaf so
// atomicfile, events, core, cli, and mcp classify via errors.Is/As without
// importing internal/core, preserving the leaf-primitive dependency direction.
// Both are always wrapped around the underlying cause with %w so callers can
// still inspect the root failure.
var (
	// ErrWriteNotApplied indicates the mutation DEFINITELY did not apply: the
	// canonical file/log is untouched (for example any atomic write that fails
	// before the rename commits). The failed atomic write is safe to retry. This
	// is a property of the single write, not of a composite operation: a caller
	// that has already appended an audit event before the file write must retry
	// only the atomic write, never the whole op, or it double-appends the event.
	ErrWriteNotApplied = errors.New("backlogit: durable write not applied")

	// ErrWriteIndeterminate indicates the mutation is possibly-applied and its
	// outcome is uncertain (for example a parent-dir fsync failure AFTER a
	// successful rename, or an AppendEvent partial write / post-write fsync
	// failure, since an append is not atomic). Callers MUST NOT blindly retry: a
	// retry may double-apply a status transition or duplicate an audit event.
	ErrWriteIndeterminate = errors.New("backlogit: durable write outcome indeterminate")
)

// IsWriteNotApplied reports whether err (or any error it wraps) is the
// ErrWriteNotApplied class, so callers can route a safe retry of the failed
// atomic write via a single predicate.
func IsWriteNotApplied(err error) bool {
	return errors.Is(err, ErrWriteNotApplied)
}

// IsWriteIndeterminate reports whether err (or any error it wraps) is the
// ErrWriteIndeterminate class, so callers can surface a possibly-committed
// operation instead of blindly retrying it.
func IsWriteIndeterminate(err error) bool {
	return errors.Is(err, ErrWriteIndeterminate)
}
