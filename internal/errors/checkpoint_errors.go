package errors

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
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
// naming the offending field count and rendering the offender paths through
// FieldPathsForDisplay (147-F / U1c) so the machine message and the human
// rendering cannot drift apart.
func (e *CheckpointNonConformingError) Error() string {
	return fmt.Sprintf("backlogit: checkpoint carries %d non-conforming field path(s): %s",
		len(e.Fields), e.FieldPathsForDisplay())
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

// boundedFieldPathMaxCount and boundedFieldPathMaxBytes bound
// BoundedFieldPaths' machine projection (147-F / U1b, cycle-17 rewrite).
const (
	boundedFieldPathMaxCount = 16
	boundedFieldPathMaxBytes = 128
)

// BoundedFieldPathSet is the bounded, raw machine projection of a
// CheckpointNonConformingError's offender key paths. Paths carries at most
// boundedFieldPathMaxCount sorted, de-duplicated raw key paths, each capped
// at boundedFieldPathMaxBytes bytes. Truncation — from either cap — is
// reported structurally via Truncated, OmittedPaths, and TruncatedPaths,
// never as a synthetic path element or quoted text: Paths is for machine
// consumption only (human rendering is FieldPathsForDisplay, U1c).
type BoundedFieldPathSet struct {
	Paths          []string `json:"paths"`
	Truncated      bool     `json:"truncated"`
	OmittedPaths   int      `json:"omitted_paths"`
	TruncatedPaths int      `json:"truncated_paths"`
}

// BoundedFieldPaths returns the sorted, de-duplicated offender paths in RAW
// form, bounded for machine consumption. At most boundedFieldPathMaxCount
// paths are returned; each returned path is cut at the last valid UTF-8 rune
// boundary at or before boundedFieldPathMaxBytes bytes, so a returned path is
// always valid UTF-8 and never ends mid-rune. This is the only sanctioned
// source of machine offender data: no other surface may re-derive a list
// from Fields or apply its own cap (147-F / U1b).
func (e *CheckpointNonConformingError) BoundedFieldPaths() BoundedFieldPathSet {
	sorted := dedupeSortedFieldPaths(e.Fields)

	result := BoundedFieldPathSet{}
	limit := len(sorted)
	if limit > boundedFieldPathMaxCount {
		result.Truncated = true
		result.OmittedPaths = limit - boundedFieldPathMaxCount
		limit = boundedFieldPathMaxCount
	}

	result.Paths = make([]string, 0, limit)
	for _, p := range sorted[:limit] {
		capped, wasCapped := capFieldPathBytes(p, boundedFieldPathMaxBytes)
		if wasCapped {
			result.Truncated = true
			result.TruncatedPaths++
		}
		result.Paths = append(result.Paths, capped)
	}
	return result
}

// dedupeSortedFieldPaths returns a sorted, de-duplicated copy of fields.
func dedupeSortedFieldPaths(fields []string) []string {
	sorted := append([]string(nil), fields...)
	sort.Strings(sorted)
	out := make([]string, 0, len(sorted))
	for i, f := range sorted {
		if i > 0 && f == sorted[i-1] {
			continue
		}
		out = append(out, f)
	}
	return out
}

// capFieldPathBytes cuts path at the last valid UTF-8 rune boundary at or
// before maxBytes and reports whether truncation occurred. A path whose
// first rune already exceeds maxBytes is returned as "" (still counted as
// truncated), so the caller learns an offender existed without receiving
// invalid or partial bytes.
func capFieldPathBytes(path string, maxBytes int) (string, bool) {
	if len(path) <= maxBytes {
		return path, false
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(path[cut]) {
		cut--
	}
	return path[:cut], true
}

// FieldPathsForDisplay renders BoundedFieldPaths() for human consumption:
// each path is escaped via strconv.Quote and joined with ", ", followed by
// an explicit clause when the set is truncated (e.g. "(5 more omitted, 1
// shortened)"). This is the only place quoting or escaping happens — no
// machine surface may call it, and no human surface may print Paths
// directly (147-F / U1c, cycle-16 gate finding H4).
func (e *CheckpointNonConformingError) FieldPathsForDisplay() string {
	set := e.BoundedFieldPaths()

	quoted := make([]string, 0, len(set.Paths))
	for _, p := range set.Paths {
		quoted = append(quoted, strconv.Quote(p))
	}
	display := strings.Join(quoted, ", ")

	if !set.Truncated {
		return display
	}
	var clauses []string
	if set.OmittedPaths > 0 {
		clauses = append(clauses, fmt.Sprintf("%d more omitted", set.OmittedPaths))
	}
	if set.TruncatedPaths > 0 {
		clauses = append(clauses, fmt.Sprintf("%d shortened", set.TruncatedPaths))
	}
	if len(clauses) == 0 {
		return display
	}
	return fmt.Sprintf("%s (%s)", display, strings.Join(clauses, ", "))
}
