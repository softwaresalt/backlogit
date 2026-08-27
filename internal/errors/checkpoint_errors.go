package errors

import (
	"errors"
	"fmt"
	"strings"
)

// Checkpoint administrative disposition sentinel errors (136-F).
//
// These sentinels govern the abandon/quarantine verb pair, which is disjoint by
// design: abandon operates only on parseable, schema-valid checkpoints;
// quarantine operates only on malformed (unparseable or schema-invalid)
// checkpoints. Each verb refuses to operate on the other's target class and
// names the correct verb in its error so callers can recover without guessing.
var (
	// ErrCheckpointUseQuarantine indicates AbandonCheckpoint was called on a
	// malformed (unparseable or schema-invalid) checkpoint target. The caller
	// should retry with QuarantineCheckpoint instead.
	ErrCheckpointUseQuarantine = errors.New("backlogit: checkpoint is malformed; use quarantine instead of abandon")

	// ErrCheckpointUseAbandon indicates QuarantineCheckpoint was called on a
	// parseable, schema-valid checkpoint target. The caller should retry with
	// AbandonCheckpoint instead.
	ErrCheckpointUseAbandon = errors.New("backlogit: checkpoint is valid; use abandon instead of quarantine")

	// ErrCheckpointTargetUnsafe indicates the requested checkpoint filename
	// failed confinement validation: it was empty, contained a path separator,
	// contained "..", was absolute, was volume-qualified, was a UNC path, or
	// resolved to a symlink.
	ErrCheckpointTargetUnsafe = errors.New("backlogit: checkpoint target failed confinement validation")

	// ErrCheckpointReasonRequired indicates a disposition operation (abandon or
	// quarantine) was invoked without a non-empty reason.
	ErrCheckpointReasonRequired = errors.New("backlogit: checkpoint disposition reason is required")

	// ErrCheckpointOperatorRequired indicates a disposition operation (abandon
	// or quarantine) could not resolve a non-empty operator identity. Operator
	// identity is never defaulted to a fixed string such as "backlogit" — an
	// unresolvable operator is a hard refusal.
	ErrCheckpointOperatorRequired = errors.New("backlogit: checkpoint disposition operator is required")

	// ErrCheckpointDestinationOccupied indicates the quarantine destination
	// path already exists, so the move was refused rather than clobbering an
	// existing quarantined file.
	ErrCheckpointDestinationOccupied = errors.New("backlogit: checkpoint quarantine destination already occupied")

	// ErrCheckpointAuditNotApplied indicates the pre-move/pre-rewrite audit
	// append definitely did not apply (ErrWriteNotApplied class). The
	// disposition operation refuses and nothing is moved or rewritten; the
	// operation is safely retryable.
	ErrCheckpointAuditNotApplied = errors.New("backlogit: checkpoint disposition audit append not applied")

	// ErrCheckpointAuditIndeterminate indicates the pre-move/pre-rewrite audit
	// append outcome is indeterminate (ErrWriteIndeterminate class). The
	// disposition operation refuses and nothing is moved or rewritten; the
	// caller must NOT blindly retry and should reconcile state before retrying.
	ErrCheckpointAuditIndeterminate = errors.New("backlogit: checkpoint disposition audit append outcome indeterminate")

	// ErrCheckpointCannotResolveAbandoned indicates ResolveCheckpoint was
	// called on a checkpoint that carries an administrative "abandoned"
	// disposition. Abandon is a terminal, non-resumable disposition; resolve
	// must not silently transition an abandoned checkpoint back to
	// "resolved" and erase that terminal state.
	ErrCheckpointCannotResolveAbandoned = errors.New("backlogit: checkpoint has been administratively abandoned; resolve is refused")

	// ErrCheckpointNotActive indicates AbandonCheckpoint was called on a
	// parseable, schema-valid checkpoint whose Status is neither "active" nor
	// already "abandoned" (the sole idempotent exception). Per the U6
	// contract, abandon requires an active checkpoint; any other non-active,
	// non-abandoned status (e.g. "resolved") is a state conflict, not a
	// silent transition to "abandoned".
	ErrCheckpointNotActive = errors.New("backlogit: checkpoint is not active; abandon requires an active checkpoint")

	// ErrCheckpointContentChanged indicates that the content of a checkpoint
	// file changed between the classification read and the quarantine move.
	// This closes the TOCTOU race in QuarantineCheckpoint: if another process
	// replaces the malformed file with a valid one before the link executes,
	// the move is refused so a valid replacement is never quarantined.
	ErrCheckpointContentChanged = errors.New("backlogit: checkpoint content changed since classification; refusing quarantine move")

	// ErrCheckpointUnknownField indicates a checkpoint create request carried a
	// key outside the closed CheckpointV1 top-level or nested progress schema
	// namespace (the context namespace remains open). Callers that need the
	// offending field names should use errors.As to recover a
	// *CheckpointUnknownFieldError rather than parsing this sentinel's message.
	ErrCheckpointUnknownField = errors.New("backlogit: checkpoint carries unknown schema field")

	// ErrCheckpointNonConforming indicates a checkpoint disposition rewrite
	// (abandon or resolve) was refused because the stored document carries
	// one or more top-level keys outside the read-boundary conformance set:
	// an unmodeled key, a duplicate or case-fold-variant key, or a nested
	// progress key outside its own closed set. Rewriting such a document
	// would silently drop those keys on re-marshal, so the operation refuses
	// rather than rewriting; QuarantineCheckpoint is the remedy. Callers that
	// need the offending key paths should use errors.As to recover a
	// *CheckpointNonConformingError rather than parsing this sentinel's
	// message.
	ErrCheckpointNonConforming = errors.New("backlogit: checkpoint carries unmodeled top-level key(s); rewrite refused")
)

// CheckpointUnknownFieldError is returned when a checkpoint create request
// carries one or more keys outside the closed schema namespace (the
// CheckpointV1 top level and the nested progress object). Fields is the
// sorted, de-duplicated set of offending key paths (for example
// "unexpected_key" or "progress.unexpected_key"). Recover Fields with
// errors.As rather than parsing Error()'s message.
type CheckpointUnknownFieldError struct {
	Fields []string
}

// Error returns the formatted error string for CheckpointUnknownFieldError.
func (e *CheckpointUnknownFieldError) Error() string {
	return "backlogit: checkpoint carries unknown schema field(s): " + strings.Join(e.Fields, ", ")
}

// Unwrap returns ErrCheckpointUnknownField so errors.Is matches through this
// typed error.
func (e *CheckpointUnknownFieldError) Unwrap() error {
	return ErrCheckpointUnknownField
}

// CheckpointNonConformingError is returned when a checkpoint disposition
// rewrite is refused because the stored document carries one or more
// top-level keys outside the read-boundary conformance set. Fields is the
// sorted, de-duplicated set of offending key paths only — never key values.
// Recover Fields with errors.As rather than parsing Error()'s message.
type CheckpointNonConformingError struct {
	Fields []string
}

// Error returns the formatted error string for CheckpointNonConformingError,
// naming the offending field count.
func (e *CheckpointNonConformingError) Error() string {
	return fmt.Sprintf("backlogit: checkpoint carries %d unmodeled top-level key(s): %s",
		len(e.Fields), strings.Join(e.Fields, ", "))
}

// Unwrap returns ErrCheckpointNonConforming so errors.Is matches through this
// typed error.
func (e *CheckpointNonConformingError) Unwrap() error {
	return ErrCheckpointNonConforming
}

// QuarantineIsRemedy reports whether err means "this checkpoint cannot be
// rewritten; route it to QuarantineCheckpoint". It matches both the
// malformed-document refusal (ErrCheckpointUseQuarantine) and the
// non-conforming-document refusal (ErrCheckpointNonConforming) added for the
// top-level-key disposition rewrite refusal (147-F / U1, Q1).
func QuarantineIsRemedy(err error) bool {
	return errors.Is(err, ErrCheckpointUseQuarantine) || errors.Is(err, ErrCheckpointNonConforming)
}
