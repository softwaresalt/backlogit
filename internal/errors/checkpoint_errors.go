package errors

import "errors"

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
)

