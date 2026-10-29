package core

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/softwaresalt/backlogit/internal/canonical"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// EventShipmentReconciledShipped is the event type recorded when the
// governed shipment-reconciliation-to-shipped transaction (167-F, #423)
// durably commits an archived_status:shipped repair. It is DISTINCT from
// the plain "shipment_status_changed" event ArchiveItem/ShipShipment
// already use, so doctor recognition (and any future consumer) can tell a
// GOVERNED reconciliation apart from a routine status transition
// (167.002-T, U3).
const EventShipmentReconciledShipped = "shipment_reconciled_shipped"

// maxShipmentReconcileEventBytes bounds one serialized event line below the
// 1 MiB line cap the event log readers enforce.
const maxShipmentReconcileEventBytes = 1 << 20

// ShipmentReconciledShippedBefore is the "before" sub-object of an
// EventShipmentReconciledShipped event's delta (167.002-T).
type ShipmentReconciledShippedBefore struct {
	Status         string `json:"status"`
	ArchivedStatus string `json:"archived_status"`
}

// ShipmentReconciledShippedAfter is the "after" sub-object of an
// EventShipmentReconciledShipped event's delta (167.002-T).
type ShipmentReconciledShippedAfter struct {
	ArchivedStatus string `json:"archived_status"`
}

// ShipmentReconciledShippedEvidence is the "evidence" sub-object of an
// EventShipmentReconciledShipped event's delta (167.002-T). EvidenceRefs is
// always persisted as a (possibly empty) array, never omitted, per the
// task's binding contract.
type ShipmentReconciledShippedEvidence struct {
	MergeSHA           string   `json:"merge_sha"`
	ClosureEvidence    string   `json:"closure_evidence"`
	ManifestDigest     string   `json:"manifest_digest"`
	MemberTerminalIDs  []string `json:"member_terminal_ids"`
	ClosureContentHash string   `json:"closure_content_hash"`
	EvidenceRefs       []string `json:"evidence_refs"`
	EvidenceDigest     string   `json:"evidence_digest"`
}

// ShipmentReconciledShippedAttestation is the "attestation" sub-object of an
// EventShipmentReconciledShipped event's delta (169.003-T). It records which
// verified signed delivery-evidence attestation backed the repair. It is
// omitted entirely from unsigned (#423 v1) events.
type ShipmentReconciledShippedAttestation struct {
	KeyID           string `json:"key_id"`
	SignerIdentity  string `json:"signer_identity"`
	SigAlgo         string `json:"sig_algo"`
	StatementDigest string `json:"statement_digest"`
	// Envelope is the durable signed attestation envelope (raw JSON) that
	// doctor and auditors re-verify offline against the external root of trust
	// (169.007-T).
	Envelope string `json:"envelope,omitempty"`
}

// ShipmentReconciledShippedDelta is the canonical delta payload of an
// EventShipmentReconciledShipped event (167.002-T, U3). second_approver and
// evidence.evidence_refs are always persisted (empty when unset) so their
// absence from a logged event is itself a validation signal, not merely an
// unset optional field. RequestIdentityDigest and TrustedRefTip are covered
// by the full-event digest (ShipmentReconciledShippedEventDigest) so a
// frontmatter-only edit of either cannot be made to agree with the durable
// event undetected (PR #424 review, binding).
type ShipmentReconciledShippedDelta struct {
	Before                ShipmentReconciledShippedBefore       `json:"before"`
	After                 ShipmentReconciledShippedAfter        `json:"after"`
	Reason                string                                `json:"reason"`
	Actor                 string                                `json:"actor"`
	SecondApprover        string                                `json:"second_approver"`
	IdempotencyKey        string                                `json:"idempotency_key"`
	RequestIdentityDigest string                                `json:"request_identity_digest"`
	TrustedRefName        string                                `json:"trusted_ref_name"`
	TrustedRefTip         string                                `json:"trusted_ref_tip"`
	Evidence              ShipmentReconciledShippedEvidence     `json:"evidence"`
	Attestation           *ShipmentReconciledShippedAttestation `json:"attestation,omitempty"`
}

// ShipmentReconciledShippedEventDigest returns the sha256 hex digest of the
// exact, canonical serialized event bytes (167.002-T). Both the doctor
// recognizer and the transaction's resume-comparison path (167.008-T)
// recompute this over the FULL wrapper+delta bytes — never the delta alone
// — so the digest covers the replay identity (request_identity_digest) as
// well as the audit content (PR #424 review, binding).
func ShipmentReconciledShippedEventDigest(eventBytes []byte) string {
	return canonical.HashBytes(eventBytes)
}

// ValidateShipmentReconciledShippedEvent validates that rawEventBytes — a
// single raw JSONL line as read directly from an item's event log, WITHOUT
// going through events.ReadAllEvents' backfilled reconstruction (167.002-T:
// doctor recognition works from the raw serialized bytes so a byte-level
// tamper or duplicate member is never silently normalized away) — is a
// well-formed, duplicate-member-free EventShipmentReconciledShipped event.
//
// rawEventBytes MUST be newline-terminated (a single complete JSONL line).
// The raw bytes are scanned for duplicate JSON object members (at every
// nesting depth, via a raw token-level walk) BEFORE any canonicalization:
// encoding/json's map decode silently keeps the LAST duplicate value, which
// would let two conflicting members (e.g. two "reason" entries) pass an
// after-the-fact canonicalized comparison undetected.
//
// When preparedEventBytes is non-empty (or expectedDigest is non-blank),
// rawEventBytes must match preparedEventBytes exactly (byte-for-byte,
// newline-normalized) and ShipmentReconciledShippedEventDigest(rawEventBytes)
// must equal expectedDigest; either mismatch is reported invalid. This is
// the "compare the full logged event against BOTH the persisted prepared
// event and the persisted digest" contract 167.008-T's resume path and the
// doctor recognizer both rely on. A blank preparedEventBytes/expectedDigest
// pair validates rawEventBytes' own shape and required-field contract only
// (the doctor-local recognition path, which has no persisted prepared-event
// artifact to compare against — that persistence lands in 167.008-T).
//
// When expectedShipmentID is non-blank, the event's item_id MUST equal it
// exactly, or the event is rejected as invalid — regardless of whether it is
// otherwise well-formed (PR #440 review, 167.002-T/167.010-T): a
// shape-valid, even digest-matching, event that names a DIFFERENT shipment
// must never be accepted as evidence for THIS shipment. A blank
// expectedShipmentID skips this check (used only where the caller genuinely
// has no specific shipment identity to bind against).
func ValidateShipmentReconciledShippedEvent(rawEventBytes []byte, preparedEventBytes []byte, expectedDigest string, expectedShipmentID string) error {
	if len(rawEventBytes) == 0 || len(bytes.TrimSpace(rawEventBytes)) == 0 {
		return fmt.Errorf("%w: event bytes are empty", blerrors.ErrValidation)
	}
	if len(rawEventBytes) >= maxShipmentReconcileEventBytes {
		return fmt.Errorf("%w: event is %d bytes, must be under %d so the item log stays readable", blerrors.ErrValidation, len(rawEventBytes), maxShipmentReconcileEventBytes)
	}
	if !bytes.HasSuffix(rawEventBytes, []byte("\n")) {
		return fmt.Errorf("%w: event bytes are not newline-terminated", blerrors.ErrValidation)
	}
	trimmed := bytes.TrimSuffix(rawEventBytes, []byte("\n"))

	if err := detectDuplicateJSONMembers(trimmed); err != nil {
		return err
	}

	var wrapper events.Event
	if err := json.Unmarshal(trimmed, &wrapper); err != nil {
		return fmt.Errorf("%w: parse event: %w", blerrors.ErrValidation, err)
	}
	if wrapper.EventType != EventShipmentReconciledShipped {
		return fmt.Errorf("%w: event_type is %q, want %q", blerrors.ErrValidation, wrapper.EventType, EventShipmentReconciledShipped)
	}
	if wrapper.ItemID == "" {
		return fmt.Errorf("%w: item_id is required", blerrors.ErrValidation)
	}
	if expectedShipmentID != "" && wrapper.ItemID != expectedShipmentID {
		return fmt.Errorf("%w: item_id is %q, want %q", blerrors.ErrValidation, wrapper.ItemID, expectedShipmentID)
	}

	if wrapper.Actor == "" {
		return fmt.Errorf("%w: actor is required", blerrors.ErrValidation)
	}
	if wrapper.Timestamp.IsZero() {
		return fmt.Errorf("%w: timestamp is required", blerrors.ErrValidation)
	}

	delta, err := decodeShipmentReconciledShippedDelta(wrapper.Delta)
	if err != nil {
		return err
	}
	if delta.Before.Status == "" {
		return fmt.Errorf("%w: delta.before.status is required", blerrors.ErrValidation)
	}
	if delta.Before.ArchivedStatus == "" {
		return fmt.Errorf("%w: delta.before.archived_status is required", blerrors.ErrValidation)
	}
	if delta.After.ArchivedStatus != string(ShipmentShipped) {
		return fmt.Errorf("%w: delta.after.archived_status is %q, want %q", blerrors.ErrValidation, delta.After.ArchivedStatus, string(ShipmentShipped))
	}
	if delta.Reason == "" {
		return fmt.Errorf("%w: delta.reason is required", blerrors.ErrValidation)
	}
	if delta.Actor == "" {
		return fmt.Errorf("%w: delta.actor is required", blerrors.ErrValidation)
	}
	if delta.IdempotencyKey == "" {
		return fmt.Errorf("%w: delta.idempotency_key is required", blerrors.ErrValidation)
	}
	if delta.RequestIdentityDigest == "" {
		return fmt.Errorf("%w: delta.request_identity_digest is required", blerrors.ErrValidation)
	}
	if delta.TrustedRefName == "" {
		return fmt.Errorf("%w: delta.trusted_ref_name is required", blerrors.ErrValidation)
	}
	if delta.TrustedRefTip == "" {
		return fmt.Errorf("%w: delta.trusted_ref_tip is required", blerrors.ErrValidation)
	}
	if delta.Evidence.MergeSHA == "" {
		return fmt.Errorf("%w: delta.evidence.merge_sha is required", blerrors.ErrValidation)
	}
	if delta.Evidence.ClosureEvidence == "" {
		return fmt.Errorf("%w: delta.evidence.closure_evidence is required", blerrors.ErrValidation)
	}
	if delta.Evidence.ManifestDigest == "" {
		return fmt.Errorf("%w: delta.evidence.manifest_digest is required", blerrors.ErrValidation)
	}
	if len(delta.Evidence.MemberTerminalIDs) == 0 {
		return fmt.Errorf("%w: delta.evidence.member_terminal_ids must be non-empty", blerrors.ErrValidation)
	}
	if delta.Evidence.ClosureContentHash == "" {
		return fmt.Errorf("%w: delta.evidence.closure_content_hash is required", blerrors.ErrValidation)
	}
	if delta.Evidence.EvidenceRefs == nil {
		return fmt.Errorf("%w: delta.evidence.evidence_refs must be present (an array, possibly empty, never omitted)", blerrors.ErrValidation)
	}
	if delta.Evidence.EvidenceDigest == "" {
		return fmt.Errorf("%w: delta.evidence.evidence_digest is required", blerrors.ErrValidation)
	}

	if err := validateShipmentReconciledShippedAttestation(delta.Attestation); err != nil {
		return err
	}

	if len(preparedEventBytes) == 0 && expectedDigest == "" {
		return nil
	}

	trimmedPrepared := bytes.TrimSuffix(preparedEventBytes, []byte("\n"))
	if !bytes.Equal(trimmed, trimmedPrepared) {
		return fmt.Errorf("%w: logged event does not match the persisted prepared event", blerrors.ErrValidation)
	}
	if gotDigest := ShipmentReconciledShippedEventDigest(rawEventBytes); gotDigest != expectedDigest {
		return fmt.Errorf("%w: event digest mismatch: got %s want %s", blerrors.ErrValidation, gotDigest, expectedDigest)
	}
	return nil
}

// validateShipmentReconciledShippedAttestation requires a present attestation
// to be complete. A nil attestation is the valid unsigned (#423 v1) shape.
func validateShipmentReconciledShippedAttestation(att *ShipmentReconciledShippedAttestation) error {
	if att == nil {
		return nil
	}
	for _, field := range []struct{ name, value string }{
		{"key_id", att.KeyID},
		{"signer_identity", att.SignerIdentity},
		{"sig_algo", att.SigAlgo},
		{"statement_digest", att.StatementDigest},
		{"envelope", att.Envelope},
	} {
		if field.value == "" {
			return fmt.Errorf("%w: delta.attestation.%s is required when attestation is present", blerrors.ErrValidation, field.name)
		}
	}
	return nil
}

// decodeShipmentReconciledShippedDelta round-trips the already-canonicalized
// (map-decoded) delta into the typed schema. Duplicate-member rejection has
// already run against the raw bytes by the time this is called, so a
// last-wins map decode here is safe: any ambiguity was already refused.
func decodeShipmentReconciledShippedDelta(delta map[string]any) (ShipmentReconciledShippedDelta, error) {
	var typed ShipmentReconciledShippedDelta
	raw, err := json.Marshal(delta)
	if err != nil {
		return typed, fmt.Errorf("%w: marshal delta: %w", blerrors.ErrValidation, err)
	}
	if err := json.Unmarshal(raw, &typed); err != nil {
		return typed, fmt.Errorf("%w: unmarshal delta: %w", blerrors.ErrValidation, err)
	}
	return typed, nil
}

// detectDuplicateJSONMembers rejects duplicate object members at any depth
// (key equivalence matches encoding/json binding), malformed JSON, and
// trailing data, delegating to the shared single-pass walker. A map decode
// would silently keep the LAST duplicate value (167.002-T).
func detectDuplicateJSONMembers(data []byte) error {
	return rejectDuplicateJSONMembers(data)
}
