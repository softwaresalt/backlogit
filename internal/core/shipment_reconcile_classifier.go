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
func classifyShipmentReconcileStateImpl(log []byte, frontmatter map[string]any, reqIdempotencyKey string, reqRequestIdentityDigest string) (ShipmentReconcileOutcome, error) {
	persisted, err := parseShipmentReconcilePersistedState(frontmatter)
	if err != nil {
		return ShipmentReconcileOutcomeIndeterminate, err
	}

	loggedEvents, err := scanShipmentReconcileLog(log)
	if err != nil {
		return ShipmentReconcileOutcomeIndeterminate, err
	}
	if len(loggedEvents) > 0 {
		return classifyShipmentReconcileEventPresent(loggedEvents, persisted, reqIdempotencyKey, reqRequestIdentityDigest)
	}
	return classifyShipmentReconcileEventAbsent(persisted, reqIdempotencyKey, reqRequestIdentityDigest)
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

func classifyShipmentReconcileEventAbsent(persisted shipmentReconcilePersistedState, reqIdempotencyKey string, reqRequestIdentityDigest string) (ShipmentReconcileOutcome, error) {
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

	preparedDelta, err := persisted.validatePreparedEvent()
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

func scanShipmentReconcileLog(log []byte) ([]shipmentReconcileLoggedEvent, error) {
	trimmed := bytes.TrimSpace(log)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if !bytes.HasSuffix(log, []byte("\n")) {
		return nil, fmt.Errorf("classify shipment reconcile state: log ends with a partial JSONL line: %w", blerrors.ErrValidation)
	}

	loggedEvents := make([]shipmentReconcileLoggedEvent, 0, 1)
	for i, line := range bytes.Split(log, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if err := detectDuplicateJSONMembers(line); err != nil {
			return nil, fmt.Errorf("classify shipment reconcile state: log line %d has duplicate JSON object members: %w", i+1, err)
		}
		event, ok, err := events.ParseEventLine(string(line), "")
		if err != nil {
			return nil, fmt.Errorf("classify shipment reconcile state: parse log line %d: %w", i+1, fmt.Errorf("%w: %v", blerrors.ErrValidation, err))
		}
		if !ok || event.EventType != EventShipmentReconciledShipped {
			continue
		}

		rawLine := append(append([]byte{}, line...), '\n')
		if err := ValidateShipmentReconciledShippedEvent(rawLine, nil, ""); err != nil {
			return nil, fmt.Errorf("classify shipment reconcile state: validate log line %d: %w", i+1, err)
		}
		delta, err := decodeShipmentReconciledShippedDelta(event.Delta)
		if err != nil {
			return nil, fmt.Errorf("classify shipment reconcile state: decode log line %d reconcile delta: %w", i+1, err)
		}
		loggedEvents = append(loggedEvents, shipmentReconcileLoggedEvent{delta: delta})
	}
	return loggedEvents, nil
}

func (state shipmentReconcilePersistedState) hasAnyResumeMarkers() bool {
	return state.idempotencyKey.present || state.requestIdentityDigest.present || state.preparedEvent.present || state.eventDigest.present
}

func (state shipmentReconcilePersistedState) resumeMarkersComplete() bool {
	return state.idempotencyKey.present && state.requestIdentityDigest.present && state.preparedEvent.present && state.eventDigest.present
}

func (state shipmentReconcilePersistedState) validatePreparedEvent() (ShipmentReconciledShippedDelta, error) {
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
	if err := ValidateShipmentReconciledShippedEvent(preparedBytes, preparedBytes, eventDigest); err != nil {
		return zero, fmt.Errorf("classify shipment reconcile state: validate persisted prepared event: %w", err)
	}
	loggedEvents, err := scanShipmentReconcileLog(preparedBytes)
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
