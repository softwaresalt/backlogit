package core

import (
	"bytes"
	"fmt"
	"strings"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

const (
	shipmentReconcileIdempotencyKeyField        = "shipment_reconciliation_idempotency_key"
	shipmentReconcileRequestIdentityDigestField = "shipment_reconciliation_request_identity_digest"
	shipmentReconcilePreparedEventField         = "shipment_reconciliation_prepared_event"
	shipmentReconcileEventDigestField           = "shipment_reconciliation_event_digest"
)

type shipmentReconcileOptionalString struct {
	value   string
	present bool
}

type shipmentReconcileRawField struct {
	value   any
	present bool
}

type shipmentReconcilePersistedState struct {
	archivedStatus        string
	idempotencyKey        shipmentReconcileOptionalString
	requestIdentityDigest shipmentReconcileOptionalString
	preparedEvent         shipmentReconcileRawField
	eventDigest           shipmentReconcileRawField
}

type shipmentReconcileLoggedEvent struct {
	delta ShipmentReconciledShippedDelta
}

// classifyShipmentReconcileStateImpl is 167.010-T's shared TOTAL state
// classifier. It is pure and read-only: callers must supply already-read log
// bytes and parsed frontmatter, and the caller owns branch 0
// (unreadable/path-unsafe log) by returning indeterminate before calling this
// function when those bytes cannot be obtained safely. A nil/blank log here is
// therefore treated as a readable event-absent log, not an unreadable one.
//
// shipmentID binds every event this classifier accepts (logged or persisted
// prepared-event) to the shipment actually being classified (PR #440 review,
// 167.010-T): without it, a misplaced/forged event naming a DIFFERENT
// shipment could otherwise be classified as evidence for this one.
func classifyShipmentReconcileStateImpl(log []byte, frontmatter map[string]any, reqIdempotencyKey string, reqRequestIdentityDigest string, shipmentID string) (ShipmentReconcileOutcome, error) {
	persisted, err := parseShipmentReconcilePersistedState(frontmatter)
	if err != nil {
		return ShipmentReconcileOutcomeIndeterminate, err
	}

	// Whenever the persisted prepared-event bytes/digest ARE already
	// available in frontmatter, thread them through so every logged event is
	// held to the STRICT byte+digest comparison instead of shape-only
	// validation (167.002-T/167.010-T, PR #440 review finding 3a/6): a
	// tampered or mismatched-digest logged event must never be accepted just
	// because it happens to be shape-valid.
	preparedEventBytes, eventDigest := persisted.preparedEventBytesAndDigestIfPresent()

	loggedEvents, conflictingPlainShippedEvent, err := scanShipmentReconcileLog(log, shipmentID, preparedEventBytes, eventDigest)
	if err != nil {
		return ShipmentReconcileOutcomeIndeterminate, err
	}
	// A plain shipment_status_changed:shipped event in this shipment's own
	// item log is a real state-integrity discrepancy this governed
	// transaction must never silently paper over (Copilot PR #440 review,
	// finding 4): EventShipmentReconciledShipped exists specifically so a
	// GOVERNED reconciliation can be told apart from a routine transition
	// (167.002-T design intent), which only matters if this classifier
	// actually treats a routine transition's presence as evidence the
	// shipment already has a normal, non-governed shipping history that
	// conflicts with running (or having run) a fresh legacy repair here.
	// This check runs BEFORE the reconcile-event-present/absent branching
	// below so it can never be silently bypassed on the path that would
	// otherwise reach ShipmentReconcileOutcomeReconciled.
	if conflictingPlainShippedEvent {
		return ShipmentReconcileOutcomeConflict, nil
	}
	if len(loggedEvents) > 0 {
		return classifyShipmentReconcileEventPresent(loggedEvents, persisted, reqIdempotencyKey, reqRequestIdentityDigest)
	}
	return classifyShipmentReconcileEventAbsent(persisted, reqIdempotencyKey, reqRequestIdentityDigest, shipmentID)
}

func classifyShipmentReconcileEventPresent(loggedEvents []shipmentReconcileLoggedEvent, persisted shipmentReconcilePersistedState, reqIdempotencyKey string, reqRequestIdentityDigest string) (ShipmentReconcileOutcome, error) {
	if len(loggedEvents) != 1 {
		return ShipmentReconcileOutcomeConflict, nil
	}
	if persisted.archivedStatus != string(ShipmentShipped) {
		return ShipmentReconcileOutcomeConflict, nil
	}
	if !persisted.idempotencyKey.present || !persisted.requestIdentityDigest.present {
		return ShipmentReconcileOutcomeConflict, nil
	}

	logged := loggedEvents[0].delta
	if logged.IdempotencyKey != reqIdempotencyKey || logged.RequestIdentityDigest != reqRequestIdentityDigest {
		return ShipmentReconcileOutcomeConflict, nil
	}
	if persisted.idempotencyKey.value != reqIdempotencyKey || persisted.requestIdentityDigest.value != reqRequestIdentityDigest {
		return ShipmentReconcileOutcomeConflict, nil
	}
	if logged.IdempotencyKey != persisted.idempotencyKey.value || logged.RequestIdentityDigest != persisted.requestIdentityDigest.value {
		return ShipmentReconcileOutcomeConflict, nil
	}
	return ShipmentReconcileOutcomeNoOp, nil
}

func classifyShipmentReconcileEventAbsent(persisted shipmentReconcilePersistedState, reqIdempotencyKey string, reqRequestIdentityDigest string, shipmentID string) (ShipmentReconcileOutcome, error) {
	if persisted.archivedStatus != string(ShipmentShipped) {
		if persisted.hasAnyResumeMarkers() {
			return ShipmentReconcileOutcomeIndeterminate, fmt.Errorf("classify shipment reconcile state: resume markers present while archived_status=%q: %w", persisted.archivedStatus, blerrors.ErrValidation)
		}
		return ShipmentReconcileOutcomeReconciled, nil
	}

	if !persisted.hasAnyResumeMarkers() {
		return ShipmentReconcileOutcomeConflict, nil
	}
	if !persisted.resumeMarkersComplete() {
		return ShipmentReconcileOutcomeIndeterminate, fmt.Errorf("classify shipment reconcile state: torn resume state: %w", blerrors.ErrValidation)
	}

	preparedDelta, err := persisted.validatePreparedEvent(shipmentID)
	if err != nil {
		return ShipmentReconcileOutcomeIndeterminate, err
	}
	if persisted.idempotencyKey.value != reqIdempotencyKey || persisted.requestIdentityDigest.value != reqRequestIdentityDigest {
		return ShipmentReconcileOutcomeConflict, nil
	}
	if preparedDelta.IdempotencyKey != reqIdempotencyKey || preparedDelta.RequestIdentityDigest != reqRequestIdentityDigest {
		return ShipmentReconcileOutcomeConflict, nil
	}
	if preparedDelta.IdempotencyKey != persisted.idempotencyKey.value || preparedDelta.RequestIdentityDigest != persisted.requestIdentityDigest.value {
		return ShipmentReconcileOutcomeConflict, nil
	}
	return ShipmentReconcileOutcomeNoOp, nil
}

func parseShipmentReconcilePersistedState(frontmatter map[string]any) (shipmentReconcilePersistedState, error) {
	var state shipmentReconcilePersistedState

	archivedStatus, err := readOptionalFrontmatterString(frontmatter, "archived_status", false)
	if err != nil {
		return state, fmt.Errorf("classify shipment reconcile state: %w", err)
	}
	state.archivedStatus = archivedStatus.value

	state.preparedEvent = readRawFrontmatterField(frontmatter, shipmentReconcilePreparedEventField)
	state.eventDigest = readRawFrontmatterField(frontmatter, shipmentReconcileEventDigestField)

	customFields, err := readOptionalFrontmatterMap(frontmatter, "custom_fields")
	if err != nil {
		return state, fmt.Errorf("classify shipment reconcile state: %w", err)
	}
	state.idempotencyKey, err = readOptionalFrontmatterString(customFields, shipmentReconcileIdempotencyKeyField, true)
	if err != nil {
		return state, fmt.Errorf("classify shipment reconcile state: %w", err)
	}
	state.requestIdentityDigest, err = readOptionalFrontmatterString(frontmatter, shipmentReconcileRequestIdentityDigestField, true)
	if err != nil {
		return state, fmt.Errorf("classify shipment reconcile state: %w", err)
	}
	return state, nil
}

// scanShipmentReconcileLog scans log's JSONL lines for two DISTINCT
// signals: (1) EventShipmentReconciledShipped events for expectedShipmentID
// (validated and decoded into loggedEvents, as before), and (2) a plain
// "shipment_status_changed" event whose delta shows the shipment was
// already shipped through the ORDINARY, non-governed path (Copilot PR #440
// review, finding 4). The two event types are mutually exclusive evidence:
// EventShipmentReconciledShipped exists specifically so a GOVERNED
// reconciliation can be told apart from a routine transition (167.002-T
// design intent), and this classifier must actually use that distinction
// defensively rather than silently ignoring every other event type in the
// log. A conflicting plain shipped event is reported via the returned bool
// so the caller can route it to ShipmentReconcileOutcomeConflict — a real
// state-integrity discrepancy this governed transaction must never
// silently paper over by reaching ShipmentReconcileOutcomeReconciled for a
// shipment that was, in fact, already shipped normally.
func scanShipmentReconcileLog(log []byte, expectedShipmentID string, preparedEventBytes []byte, expectedDigest string) ([]shipmentReconcileLoggedEvent, bool, error) {
	trimmed := bytes.TrimSpace(log)
	if len(trimmed) == 0 {
		return nil, false, nil
	}
	if !bytes.HasSuffix(log, []byte("\n")) {
		return nil, false, fmt.Errorf("classify shipment reconcile state: log ends with a partial JSONL line: %w", blerrors.ErrValidation)
	}

	loggedEvents := make([]shipmentReconcileLoggedEvent, 0, 1)
	conflictingPlainShippedEvent := false
	for i, line := range bytes.Split(log, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if err := detectDuplicateJSONMembers(line); err != nil {
			return nil, false, fmt.Errorf("classify shipment reconcile state: log line %d has duplicate JSON object members: %w", i+1, err)
		}
		event, ok, err := events.ParseEventLine(string(line), "")
		if err != nil {
			return nil, false, fmt.Errorf("classify shipment reconcile state: parse log line %d: %w", i+1, fmt.Errorf("%w: %v", blerrors.ErrValidation, err))
		}
		if !ok {
			continue
		}
		if event.EventType == shipmentStatusChangedEventType {
			if (expectedShipmentID == "" || event.ItemID == expectedShipmentID) && shipmentStatusChangedDeltaIsShipped(event.Delta) {
				conflictingPlainShippedEvent = true
			}
			continue
		}
		if event.EventType != EventShipmentReconciledShipped {
			continue
		}

		rawLine := append(append([]byte{}, line...), '\n')
		if err := ValidateShipmentReconciledShippedEvent(rawLine, preparedEventBytes, expectedDigest, expectedShipmentID); err != nil {
			return nil, false, fmt.Errorf("classify shipment reconcile state: validate log line %d: %w", i+1, err)
		}
		delta, err := decodeShipmentReconciledShippedDelta(event.Delta)
		if err != nil {
			return nil, false, fmt.Errorf("classify shipment reconcile state: decode log line %d reconcile delta: %w", i+1, err)
		}
		loggedEvents = append(loggedEvents, shipmentReconcileLoggedEvent{delta: delta})
	}
	return loggedEvents, conflictingPlainShippedEvent, nil
}

// shipmentStatusChangedEventType is the plain, non-governed event type
// ArchiveItem/ShipShipment already use (shipment.go, archive.go) — distinct
// from EventShipmentReconciledShipped by design (167.002-T).
const shipmentStatusChangedEventType = "shipment_status_changed"

// shipmentStatusChangedDeltaIsShipped reports whether a
// "shipment_status_changed" event's delta records the shipment as having
// been shipped, mirroring doctor.go's shippedEventPresence recognition
// (delta.status == string(ShipmentShipped)).
func shipmentStatusChangedDeltaIsShipped(delta map[string]any) bool {
	status, ok := delta["status"].(string)
	return ok && status == string(ShipmentShipped)
}

func (state shipmentReconcilePersistedState) hasAnyResumeMarkers() bool {
	return state.idempotencyKey.present || state.requestIdentityDigest.present || state.preparedEvent.present || state.eventDigest.present
}

func (state shipmentReconcilePersistedState) resumeMarkersComplete() bool {
	return state.idempotencyKey.present && state.requestIdentityDigest.present && state.preparedEvent.present && state.eventDigest.present
}

// preparedEventBytesAndDigestIfPresent returns the persisted prepared-event
// raw bytes and digest when BOTH frontmatter fields are present and
// string-typed, or (nil, "") otherwise. It never itself validates JSON shape
// — callers pass the result straight through to
// ValidateShipmentReconciledShippedEvent, which performs that validation. A
// partial/malformed pair degrades to shape-only comparison here rather than
// erroring, matching this being a best-effort strengthening of an existing
// shape-only call site, not a new hard precondition: any resume-marker
// completeness/torn-state problem is already caught separately by
// hasAnyResumeMarkers/resumeMarkersComplete before this matters.
func (state shipmentReconcilePersistedState) preparedEventBytesAndDigestIfPresent() ([]byte, string) {
	if !state.preparedEvent.present || !state.eventDigest.present {
		return nil, ""
	}
	preparedText, ok := state.preparedEvent.value.(string)
	if !ok {
		return nil, ""
	}
	digest, ok := state.eventDigest.value.(string)
	if !ok {
		return nil, ""
	}
	return []byte(preparedText), digest
}

func (state shipmentReconcilePersistedState) validatePreparedEvent(shipmentID string) (ShipmentReconciledShippedDelta, error) {
	var zero ShipmentReconciledShippedDelta

	preparedEvent, err := readRequiredRawStringField(state.preparedEvent, shipmentReconcilePreparedEventField)
	if err != nil {
		return zero, fmt.Errorf("classify shipment reconcile state: %w", err)
	}
	eventDigest, err := readRequiredRawStringField(state.eventDigest, shipmentReconcileEventDigestField)
	if err != nil {
		return zero, fmt.Errorf("classify shipment reconcile state: %w", err)
	}

	preparedBytes := []byte(preparedEvent)
	if err := ValidateShipmentReconciledShippedEvent(preparedBytes, preparedBytes, eventDigest, shipmentID); err != nil {
		return zero, fmt.Errorf("classify shipment reconcile state: validate persisted prepared event: %w", err)
	}
	loggedEvents, _, err := scanShipmentReconcileLog(preparedBytes, shipmentID, preparedBytes, eventDigest)
	if err != nil {
		return zero, fmt.Errorf("classify shipment reconcile state: parse persisted prepared event: %w", err)
	}
	if len(loggedEvents) != 1 {
		return zero, fmt.Errorf("classify shipment reconcile state: persisted prepared event must contain exactly one reconcile event: %w", blerrors.ErrValidation)
	}
	return loggedEvents[0].delta, nil
}

func readOptionalFrontmatterMap(frontmatter map[string]any, key string) (map[string]any, error) {
	if frontmatter == nil {
		return nil, nil
	}
	raw, ok := frontmatter[key]
	if !ok {
		return nil, nil
	}
	if raw == nil {
		return nil, fmt.Errorf("frontmatter %q must be an object: %w", key, blerrors.ErrValidation)
	}
	mapped, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("frontmatter %q must be an object: %w", key, blerrors.ErrValidation)
	}
	return mapped, nil
}

func readOptionalFrontmatterString(frontmatter map[string]any, key string, nonEmpty bool) (shipmentReconcileOptionalString, error) {
	var field shipmentReconcileOptionalString
	if frontmatter == nil {
		return field, nil
	}
	raw, ok := frontmatter[key]
	if !ok {
		return field, nil
	}
	text, ok := raw.(string)
	if !ok {
		return field, fmt.Errorf("frontmatter %q must be a string: %w", key, blerrors.ErrValidation)
	}
	if nonEmpty && strings.TrimSpace(text) == "" {
		return field, fmt.Errorf("frontmatter %q must be a non-empty string: %w", key, blerrors.ErrValidation)
	}
	field.present = true
	field.value = text
	return field, nil
}

func readRawFrontmatterField(frontmatter map[string]any, key string) shipmentReconcileRawField {
	if frontmatter == nil {
		return shipmentReconcileRawField{}
	}
	raw, ok := frontmatter[key]
	if !ok {
		return shipmentReconcileRawField{}
	}
	return shipmentReconcileRawField{value: raw, present: true}
}

func readRequiredRawStringField(field shipmentReconcileRawField, key string) (string, error) {
	if !field.present {
		return "", fmt.Errorf("frontmatter %q is required: %w", key, blerrors.ErrValidation)
	}
	text, ok := field.value.(string)
	if !ok {
		return "", fmt.Errorf("frontmatter %q must be a string: %w", key, blerrors.ErrValidation)
	}
	if len(bytes.TrimSpace([]byte(text))) == 0 {
		return "", fmt.Errorf("frontmatter %q must be a non-empty string: %w", key, blerrors.ErrValidation)
	}
	return text, nil
}
