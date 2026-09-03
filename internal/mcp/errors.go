package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type workspaceRootAmbiguousResponse struct {
	Error                string   `json:"error"`
	Message              string   `json:"message"`
	Roots                []string `json:"roots"`
	SupportedResolutions []string `json:"supported_resolutions"`
	Override             string   `json:"override"`
	Retryable            bool     `json:"retryable"`
}

// checkpointUnknownFieldsResponse is the structured MCP error shape for a
// checkpoint create request rejected by the closed CheckpointV1 top-level /
// nested-progress schema namespace (146.011-T / U4). UnknownFields names
// every offending key path (e.g. "unexpected_key" or
// "progress.unexpected_key"), sorted and de-duplicated, so a caller does not
// need to parse Message to recover the offending fields.
type checkpointUnknownFieldsResponse struct {
	Error         string   `json:"error"`
	Message       string   `json:"message"`
	UnknownFields []string `json:"unknown_fields"`
}

type mutationPartialResponse struct {
	Error             string   `json:"error"`
	Message           string   `json:"message"`
	Classification    string   `json:"classification"`
	CompletedSteps    []string `json:"completed_steps"`
	FailedStep        string   `json:"failed_step"`
	CompensationState string   `json:"compensation_state"`
	Retryable         bool     `json:"retryable"`
	Recovery          string   `json:"recovery"`
	// QuarantineRequired is true when the underlying cause (traversable via
	// errors.Is through MutationPartialError.Unwrap()) indicates the target
	// checkpoint is malformed or non-conforming and quarantine is the only
	// accepting verb, so a caller does not have to parse Recovery text to
	// discover this (147-F, found during 130-S adversarial review: a
	// between-read race can surface a quarantine-only verdict through this
	// partial-mutation shape rather than the dedicated disposition-error
	// shape). Omitted (false) for every other partial-mutation cause.
	QuarantineRequired bool `json:"quarantine_required,omitempty"`
}

func makeErrorResult(errType, message string) *mcplib.CallToolResult {
	resp := errorResponse{Error: errType, Message: message}
	data, err := json.Marshal(resp)
	if err != nil {
		// Fallback to manually formatted JSON to avoid recursive error handling.
		return mcplib.NewToolResultError(fmt.Sprintf(`{"error":%q,"message":%q}`, errType, message))
	}
	return mcplib.NewToolResultError(string(data))
}

// WorkspaceNotInitialized returns an MCP error for missing workspace.
func WorkspaceNotInitialized() *mcplib.CallToolResult {
	return makeErrorResult("workspace_not_initialized", "No supported workspace directory found. Run backlogit init first.")
}

// WorkspaceRootAmbiguous returns an MCP error when both workspace roots exist.
func WorkspaceRootAmbiguous(roots []string) *mcplib.CallToolResult {
	resp := workspaceRootAmbiguousResponse{
		Error:                "workspace_root_ambiguous",
		Message:              "Both supported workspace directories exist. Set BACKLOGIT_WORKSPACE_DIR to one supported name or remove one directory.",
		Roots:                append([]string(nil), roots...),
		SupportedResolutions: core.WorkspaceRootCandidates(),
		Override:             "BACKLOGIT_WORKSPACE_DIR",
		Retryable:            false,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return InternalError(fmt.Sprintf("marshal workspace root ambiguous response: %v", err))
	}
	return mcplib.NewToolResultError(string(data))
}

// ValidationFailed returns an MCP error for parameter validation failures.
func ValidationFailed(detail string) *mcplib.CallToolResult {
	return makeErrorResult("validation_failed", detail)
}

// NotFound returns an MCP error for missing resources.
func NotFound(detail string) *mcplib.CallToolResult {
	return makeErrorResult("not_found", detail)
}

// Conflict returns an MCP error for state conflicts.
func Conflict(detail string) *mcplib.CallToolResult {
	return makeErrorResult("conflict", detail)
}

// InternalError returns an MCP error for internal failures.
func InternalError(detail string) *mcplib.CallToolResult {
	return makeErrorResult("internal", detail)
}

// checkpointUnknownFields returns a dedicated MCP error for a checkpoint
// create request rejected by the closed schema namespace (146.011-T / U4,
// 146.012-T / U4b). fields is the sorted, de-duplicated set of offending key
// paths (e.g. "unexpected_key" or "progress.unexpected_key").
func checkpointUnknownFields(fields []string) *mcplib.CallToolResult {
	resp := checkpointUnknownFieldsResponse{
		Error:         "validation_failed",
		Message:       "checkpoint carries unknown schema field(s): " + strings.Join(fields, ", "),
		UnknownFields: append([]string(nil), fields...),
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return InternalError(fmt.Sprintf("marshal checkpoint unknown fields response: %v", err))
	}
	return mcplib.NewToolResultError(string(data))
}

// domainError routes domain sentinel errors to the correct MCP error category
// and falls back to InternalError for unknown failures.
//
// Error mapping table (sentinel → MCP error type):
//
//	Sentinel                          | MCP Type                              | HTTP analogue
//	----------------------------------|---------------------------------------|---------------
//	ErrNotFound                       | not_found                             | 404
//	ErrShipmentNotFound               | not_found                             | 404
//	ErrCheckpointNotFound             | not_found                             | 404
//	ErrShipmentConflict               | conflict                              | 409
//	ErrItemAlreadyAssigned            | conflict                              | 409
//	ErrCannotReturnItem               | conflict                              | 409
//	ErrChildrenNotTerminal            | conflict                              | 409
//	ErrShipmentShippedRequiresEnv…    | shipment_shipped_requires_envelope    | 409
//	ErrArchiveShippedRequiresEvent    | archive_shipped_requires_event        | 409
//	ErrCheckpointUnknownField         | validation_failed (+ unknown_fields)  | 422
//	ErrCheckpointMalformedInput       | validation_failed                     | 422
//	ErrCheckpointStateDumpTooLarge    | validation_failed                     | 422
//	ErrCheckpointStateDumpSecretDetected | validation_failed                  | 422
//	ErrCheckpointDuplicateContextKey  | validation_failed (+ dup keys)        | 422
//	ErrValidation                     | validation_failed                     | 422
//	ErrInvalidLinkType                | validation_failed                     | 422
//	ErrTelemetrySourceMissing         | validation_failed                     | 422
//	ErrCheckpointInvalid              | validation_failed                     | 422
//	ErrCheckpointCorrupt              | validation_failed                     | 422
//	ErrCheckpointCannotResolveAbandoned | validation_failed                   | 422
//	ErrTelemetryParseFailed           | internal                              | 500
//	(all others)                      | internal                              | 500
//
// op is a short camelCase description of the operation, prepended to
// InternalError messages to aid diagnosis (e.g. "archive item").
func domainError(op string, err error) *mcplib.CallToolResult {
	var partialErr *corerrors.MutationPartialError
	if errors.As(err, &partialErr) {
		return mutationPartialError(op, partialErr)
	}
	switch {
	case errors.Is(err, corerrors.ErrShipmentNotFound), errors.Is(err, corerrors.ErrNotFound),
		errors.Is(err, corerrors.ErrCheckpointNotFound):
		return NotFound(err.Error())
	case errors.Is(err, corerrors.ErrShipmentConflict),
		errors.Is(err, corerrors.ErrItemAlreadyAssigned),
		errors.Is(err, corerrors.ErrCannotReturnItem),
		errors.Is(err, core.ErrTaskBusy),
		errors.Is(err, corerrors.ErrChildrenNotTerminal):
		return Conflict(err.Error())
	case errors.Is(err, corerrors.ErrShipmentShippedRequiresEnvelope):
		// 144-F guard 1: generic shipped-transition refusal; distinct from
		// generic conflict so agents can tailor remediation (use ship_shipment).
		return makeErrorResult("shipment_shipped_requires_envelope",
			fmt.Sprintf("%s: %v", op, err))
	case errors.Is(err, corerrors.ErrArchiveShippedRequiresEvent):
		// 144-F guard 2: archive of shipped shipment without durable event.
		return makeErrorResult("archive_shipped_requires_event",
			fmt.Sprintf("%s: %v", op, err))
	case errors.Is(err, corerrors.ErrAmbiguousWorkspaceRoot):
		var ambiguous *corerrors.AmbiguousWorkspaceRootError
		if errors.As(err, &ambiguous) {
			return WorkspaceRootAmbiguous(ambiguous.Roots)
		}
		return WorkspaceRootAmbiguous(core.WorkspaceRootCandidates())
	case errors.Is(err, corerrors.ErrCheckpointUnknownField):
		// Dedicated case (146.012-T / U4b), placed before the general
		// ErrValidation case: the general ValidationFailed path returns a
		// two-field {error, message} struct with no room for the offending
		// field list, mirroring the WorkspaceRootAmbiguous precedent above.
		var typed *corerrors.CheckpointUnknownFieldError
		if errors.As(err, &typed) {
			return checkpointUnknownFields(typed.Fields)
		}
		return ValidationFailed(err.Error())
	case errors.Is(err, corerrors.ErrCheckpointMalformedInput):
		// 148-F / U1: state_dump is not valid JSON. Mapped to validation_failed
		// (not internal_error) so callers can distinguish a client-supplied
		// malformed payload from a server-side fault.
		return ValidationFailed(err.Error())
	case errors.Is(err, corerrors.ErrCheckpointStateDumpTooLarge):
		// 153.003-T / S1 U3: state_dump exceeds the 64 KiB fail-closed size
		// limit. Mapped to validation_failed so MCP callers can distinguish this
		// client-input rejection from a server-side fault.
		return ValidationFailed(err.Error())
	case errors.Is(err, corerrors.ErrCheckpointStateDumpSecretDetected):
		// 153.003-T / S1 U3: state_dump contains a heuristically detected
		// secret pattern. Mapped to validation_failed so callers can distinguish
		// this client-input rejection from a server-side fault. The error message
		// contains no payload bytes (Constitution III).
		return ValidationFailed(err.Error())
	case errors.Is(err, corerrors.ErrCheckpointDuplicateContextKey):
		// 148-F / U2: context object carries duplicate or case-fold-aliased
		// member names. Mapped to validation_failed with the offending keys
		// for recovery guidance, matching the CheckpointUnknownField pattern.
		var typed *corerrors.CheckpointDuplicateContextKeyError
		if errors.As(err, &typed) {
			return makeErrorResult("validation_failed",
				fmt.Sprintf("checkpoint context carries duplicate key(s): %v", typed.Keys))
		}
		return ValidationFailed(err.Error())
	case errors.Is(err, corerrors.ErrValidation),
		errors.Is(err, corerrors.ErrInvalidLinkType),
		errors.Is(err, corerrors.ErrTelemetrySourceMissing),
		errors.Is(err, corerrors.ErrCheckpointInvalid),
		errors.Is(err, corerrors.ErrCheckpointCorrupt),
		errors.Is(err, corerrors.ErrCheckpointCannotResolveAbandoned):
		return ValidationFailed(err.Error())
	case errors.Is(err, corerrors.ErrWriteIndeterminate):
		// A durable write's outcome is uncertain (e.g. a parent-directory
		// fsync failure after the rename already committed). Distinct from
		// the default InternalError fallback because callers MUST NOT
		// blindly retry an indeterminate write — it may already have
		// applied. Reached by callers (e.g. ResolveCheckpoint) that do not
		// wrap RewriteCheckpointFile in a MutationEnvelope; envelope-wrapped
		// callers (AbandonCheckpoint, QuarantineCheckpoint) already surface
		// this via mutationPartialError above.
		return makeErrorResult("write_indeterminate", fmt.Sprintf("%s: %v", op, err))
	case errors.Is(err, corerrors.ErrWriteNotApplied):
		// A durable write definitely did not apply (a pre-rename failure);
		// the target is untouched and the write is safe to retry.
		return makeErrorResult("write_not_applied", fmt.Sprintf("%s: %v", op, err))
	default:
		return InternalError(fmt.Sprintf("%s: %v", op, err))
	}
}

func mutationPartialError(op string, err *corerrors.MutationPartialError) *mcplib.CallToolResult {
	recovery := mutationPartialRecoveryFor(err.Class, err.FailedStep, err.CompensationState)
	quarantineRequired := corerrors.QuarantineIsRemedy(err)
	if quarantineRequired {
		// 147-F: err.Cause is malformed or non-conforming (a between-read
		// race between the caller's classification and the guarded seam's
		// own read surfaced it here, rather than through the dedicated
		// checkpointDispositionError path). Append the quarantine
		// remediation to the class-based recovery text rather than
		// replacing it, so the audit-trail/compensation context this shape
		// exists for is not lost.
		recovery += "; the current checkpoint bytes are malformed or non-conforming — quarantine is the only " +
			"accepting verb for this target (backlogit_quarantine_checkpoint / checkpoint quarantine)"
	}
	resp := mutationPartialResponse{
		Error:             "mutation_partial",
		Message:           fmt.Sprintf("%s: %s", op, err.Error()),
		Classification:    err.Class,
		CompletedSteps:    append([]string(nil), err.Completed...),
		FailedStep:        err.FailedStep,
		CompensationState: err.CompensationState,
		// 143.010-T: a partially-compensated result must never advertise itself
		// as safe to retry while release-scope items remain un-restored.
		// 147-F: also gated on !quarantineRequired (found during 130-S
		// adversarial review) — retrying the same resolve/abandon call
		// cannot succeed against bytes that are malformed or non-conforming;
		// "safe to retry" would contradict the quarantine-only guidance
		// above.
		Retryable:          err.Class == "not-applied" && err.CompensationState == "compensated" && !quarantineRequired,
		Recovery:           recovery,
		QuarantineRequired: quarantineRequired,
	}
	data, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		return InternalError(fmt.Sprintf("marshal mutation partial response: %v", marshalErr))
	}
	return mcplib.NewToolResultError(string(data))
}

// mutationPartialRecoveryFor selects recovery guidance by PRODUCER first and
// class second.
//
// mutationPartialRecovery is keyed on class only and is shared by three
// producers (domainError, handleTrackCommit, checkpointDispositionError), so
// repointing its "indeterminate" case at the shipped-event audit would
// mis-advise all three. The shipped-event branch is therefore selected on
// FailedStep, leaving the class-only default text untouched for every other
// producer.
func mutationPartialRecoveryFor(classification, failedStep, compensationState string) string {
	if failedStep != corerrors.StepShippedEventAppend {
		return mutationPartialRecovery(classification)
	}
	const audit = "run the shipped-event reconciliation audit (MCP: backlogit_doctor with check_shipped_event_completeness; CLI: backlogit doctor --check-shipped-event-completeness)"
	switch compensationState {
	case "compensated":
		return "safe to retry the ship; compensation restored the release scope. " + audit + " to confirm the clean state"
	case "partially-compensated":
		return "do not retry the ship yet; compensation could not restore every release-scope item and the un-restored IDs named in this message must be reconciled first. " + audit +
			" -- note it may report clean while those items remain torn, so verify each named ID directly. Reconcile before archiving"
	default:
		return "do not retry the ship; the shipped-event append outcome is unknown, so the shipment is left shipped and unarchived. " + audit +
			", read {workspace-storage-root}/logs/{shipment-id}.jsonl (e.g. .backlogit/logs/ or .backlog/logs/ depending on your workspace configuration) to determine whether the shipped event actually landed, and never synthesize it. Reconcile before archiving"
	}
}

func mutationPartialRecovery(classification string) string {
	switch classification {
	case "not-applied":
		return "safe to retry the mutation; the envelope compensated earlier steps, and doctor with check_partial_mutations can confirm the clean state"
	case "indeterminate":
		return "do not blindly retry; inspect doctor with check_partial_mutations to reconcile state"
	case "double-fault":
		return "manual recovery required; inspect doctor with check_partial_mutations and review compensation failures"
	default:
		return "inspect doctor with check_partial_mutations before retrying"
	}
}

// blockingChildrenResult returns a structured error response when a parent
// artifact cannot move to a terminal status because non-terminal children exist.
func blockingChildrenResult(children []core.ChildStatus) *mcplib.CallToolResult {
	type childEntry struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	type resp struct {
		Error    string       `json:"error"`
		Message  string       `json:"message"`
		Children []childEntry `json:"children"`
	}
	entries := make([]childEntry, len(children))
	for i, c := range children {
		entries[i] = childEntry{ID: c.ID, Status: c.Status}
	}
	r := resp{
		Error:    "blocking_children",
		Message:  fmt.Sprintf("%d non-terminal child(ren) are blocking this status transition", len(children)),
		Children: entries,
	}
	data, err := json.Marshal(r)
	if err != nil {
		return InternalError(fmt.Sprintf("marshal blocking children response: %v", err))
	}
	return mcplib.NewToolResultError(string(data))
}

// checkpointDispositionErrorResponse is the structured MCP error shape for
// checkpoint disposition (abandon/quarantine/resolve) failures. It carries a
// stable code, the offending checkpoint filename, a class-derived retryable
// flag, and an actionable remediation hint so an agent caller does not need
// to parse the error message to decide what to do next.
//
// UnknownFields, UnknownFieldsTruncated, UnknownFieldsOmitted, and
// UnknownFieldsShortened (147-F / U7) carry the bounded, raw machine
// projection of a non-conforming refusal's offender key paths (populated via
// errors.As from *corerrors.CheckpointNonConformingError.BoundedFieldPaths()).
// None of the four carry omitempty: a refusal unrelated to non-conformance
// still reports unknown_fields: [] and the three scalars at their zero value,
// so a caller can rely on all four keys always being present.
type checkpointDispositionErrorResponse struct {
	Error                  string   `json:"error"`
	Message                string   `json:"message"`
	Code                   string   `json:"code"`
	Filename               string   `json:"filename"`
	Retryable              bool     `json:"retryable"`
	Outcome                string   `json:"outcome,omitempty"`
	Remediation            string   `json:"remediation"`
	UnknownFields          []string `json:"unknown_fields"`
	UnknownFieldsTruncated bool     `json:"unknown_fields_truncated"`
	UnknownFieldsOmitted   int      `json:"unknown_fields_omitted"`
	UnknownFieldsShortened int      `json:"unknown_fields_shortened"`
}

// dispositionOperatorVerb derives the operator-facing verb from the first
// word of op (e.g. "abandon checkpoint" -> "abandon") and returns the
// corresponding backlogit_<verb>_checkpoint tool name, so a remediation
// string's "instead of" clause always names the caller's actual originating
// verb instead of a hardcoded one (147-F / U7). This lets
// checkpointDispositionError serve resolve refusals (147.025-T / U7d) with
// the same accurate wording it already gives abandon and quarantine.
func dispositionOperatorVerb(op string) string {
	verb, _, _ := strings.Cut(op, " ")
	return "backlogit_" + verb + "_checkpoint"
}

// checkpointDispositionError maps a checkpoint disposition sentinel error
// (internal/errors checkpoint_errors.go) to a structured MCP error result.
// A *corerrors.MutationPartialError from the underlying MutationEnvelope (the
// rewrite/move/sidecar steps) is detected first and routed through the same
// mutationPartialError formatter domainError uses, so a partial-mutation
// failure surfaces its classification, completed steps, compensation state,
// and retryable flag instead of falling through to a generic internal error.
// Any other unrecognized error also falls back to InternalError.
func checkpointDispositionError(op, filename string, err error) *mcplib.CallToolResult {
	var partialErr *corerrors.MutationPartialError
	if errors.As(err, &partialErr) {
		return mutationPartialError(op, partialErr)
	}

	resp := checkpointDispositionErrorResponse{
		Error:    "checkpoint_disposition_failed",
		Message:  fmt.Sprintf("%s: %v", op, err),
		Filename: filename,
		// Default to a non-nil empty slice (never left nil) so a refusal
		// unrelated to non-conformance still marshals "unknown_fields": []
		// rather than "unknown_fields": null
		// (docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md).
		UnknownFields: []string{},
	}
	switch {
	case errors.Is(err, corerrors.ErrCheckpointNonConforming):
		// 147-F / U7: distinct from ErrCheckpointUseQuarantine — this
		// sentinel means the document IS schema-valid but carries
		// unmodeled/duplicate top-level or nested keys (post-U5, quarantine
		// itself no longer returns this; it now originates from abandon and
		// resolve refusing to rewrite such a document).
		resp.Code = "checkpoint_non_conforming"
		resp.Retryable = false
		resp.Remediation = fmt.Sprintf("this target has unmodeled or duplicate keys; call backlogit_quarantine_checkpoint instead of %s", dispositionOperatorVerb(op))
		var typed *corerrors.CheckpointNonConformingError
		if errors.As(err, &typed) {
			bounded := typed.BoundedFieldPaths()
			resp.UnknownFields = bounded.Paths
			resp.UnknownFieldsTruncated = bounded.Truncated
			resp.UnknownFieldsOmitted = bounded.OmittedPaths
			resp.UnknownFieldsShortened = bounded.TruncatedPaths
		}
	case errors.Is(err, corerrors.ErrCheckpointUseQuarantine):
		resp.Code = "checkpoint_use_quarantine"
		resp.Retryable = false
		resp.Remediation = fmt.Sprintf("this target is malformed; call backlogit_quarantine_checkpoint instead of %s", dispositionOperatorVerb(op))
	case errors.Is(err, corerrors.ErrCheckpointUseAbandon):
		resp.Code = "checkpoint_use_abandon"
		resp.Retryable = false
		resp.Remediation = fmt.Sprintf("this target is valid; call backlogit_abandon_checkpoint instead of %s", dispositionOperatorVerb(op))
	case errors.Is(err, corerrors.ErrCheckpointNotActive):
		resp.Code = "checkpoint_not_active"
		resp.Retryable = false
		resp.Remediation = "abandon requires an active checkpoint; this checkpoint has a different status (e.g. resolved) and is not eligible"
	case errors.Is(err, corerrors.ErrCheckpointTargetUnsafe):
		resp.Code = "checkpoint_target_unsafe"
		resp.Retryable = false
		resp.Remediation = "supply a bare checkpoint filename (basename only, no path separators, no traversal, no symlink)"
	case errors.Is(err, corerrors.ErrCheckpointReasonRequired):
		resp.Code = "checkpoint_reason_required"
		resp.Retryable = false
		resp.Remediation = "retry with a non-empty reason parameter"
	case errors.Is(err, corerrors.ErrCheckpointOperatorRequired):
		resp.Code = "checkpoint_operator_required"
		resp.Retryable = false
		resp.Remediation = "retry with a non-empty operator parameter; operator is never inferred on the MCP surface"
	case errors.Is(err, corerrors.ErrCheckpointDestinationOccupied):
		resp.Code = "checkpoint_destination_occupied"
		resp.Retryable = false
		resp.Remediation = "an existing quarantined file already occupies the destination; resolve the conflict manually before retrying"
	case errors.Is(err, corerrors.ErrCheckpointAuditIndeterminate):
		resp.Code = "checkpoint_audit_indeterminate"
		resp.Retryable = false
		resp.Outcome = "indeterminate"
		resp.Remediation = "do not blindly retry; inspect the checkpoint disposition audit log to reconcile state before retrying"
	case errors.Is(err, corerrors.ErrCheckpointAuditNotApplied):
		resp.Code = "checkpoint_audit_not_applied"
		resp.Retryable = true
		resp.Remediation = "the audit append definitely did not apply and nothing was moved or rewritten; safe to retry"
	case errors.Is(err, corerrors.ErrCheckpointContentChanged):
		// 147-F: a concurrent writer mutated the checkpoint between the
		// classification read and the guarded disposition write (quarantine's
		// move, or the conforming-rewrite seam resolve/abandon share). No
		// data was lost or overwritten; the caller's own read is now stale.
		resp.Code = "checkpoint_content_changed"
		resp.Retryable = true
		resp.Remediation = "the checkpoint was modified by another process between classification and write; re-read the checkpoint and retry the operation"
	case errors.Is(err, corerrors.ErrCheckpointNotFound):
		return NotFound(err.Error())
	default:
		return InternalError(fmt.Sprintf("%s: %v", op, err))
	}
	data, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		return InternalError(fmt.Sprintf("marshal checkpoint disposition error: %v", marshalErr))
	}
	return mcplib.NewToolResultError(string(data))
}
