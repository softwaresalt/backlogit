package core

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

type blockedShipmentEnvelope struct {
	reason              string
	blockedAt           string
	blockedBy           string
	branch              string
	resumeCheckpointRef string
	memberStatuses      map[string]string
}

func validatePersistedBlockedShipmentEnvelope(
	ctx context.Context,
	ws *Workspace,
	shipment *models.Artifact,
) (blockedShipmentEnvelope, error) {
	var envelope blockedShipmentEnvelope
	if shipment == nil {
		return envelope, blockedShipmentEnvelopeError("", "shipment is nil")
	}
	itemEvents, err := events.ReadAllEvents(ctx, WorkspaceLogsRoot(ws.RootPath), shipment.ID)
	if err != nil {
		return envelope, fmt.Errorf("read shipment %s blocked lifecycle evidence: %w", shipment.ID, err)
	}
	return validateBlockedShipmentEnvelope(shipment, NormalizeShipmentItems(shipment), itemEvents)
}

func validateBlockedShipmentEnvelope(
	shipment *models.Artifact,
	memberIDs []string,
	itemEvents []events.Event,
) (blockedShipmentEnvelope, error) {
	var envelope blockedShipmentEnvelope
	if shipment == nil ||
		shipment.ArtifactType != "shipment" ||
		shipment.Status != models.StatusBlocked {
		id := ""
		if shipment != nil {
			id = shipment.ID
		}
		return envelope, blockedShipmentEnvelopeError(id, "record is not a blocked shipment")
	}
	if shipment.CustomFields == nil {
		return envelope, blockedShipmentEnvelopeError(shipment.ID, "blocked metadata is absent")
	}

	reason, ok := shipment.CustomFields["blocked_reason"].(string)
	if !ok || strings.TrimSpace(reason) == "" {
		return envelope, blockedShipmentEnvelopeError(shipment.ID, "blocked_reason must be a non-empty string")
	}
	blockedAt, ok := canonicalBlockedTimestamp(shipment.CustomFields["blocked_at"])
	if !ok {
		return envelope, blockedShipmentEnvelopeError(shipment.ID, "blocked_at must be an RFC3339 timestamp")
	}
	blockedBy, err := persistedBlockedEnvelopeOptionalString(shipment.CustomFields, "blocked_by")
	if err != nil {
		return envelope, blockedShipmentEnvelopeError(shipment.ID, err.Error())
	}
	branch, err := persistedBlockedEnvelopeOptionalString(shipment.CustomFields, "branch")
	if err != nil {
		return envelope, blockedShipmentEnvelopeError(shipment.ID, err.Error())
	}
	resumeCheckpointRef, err := persistedBlockedEnvelopeOptionalString(
		shipment.CustomFields,
		"resume_checkpoint_ref",
	)
	if err != nil {
		return envelope, blockedShipmentEnvelopeError(shipment.ID, err.Error())
	}
	memberStatuses, err := decodeBlockedMemberStatusSnapshot(
		shipment.CustomFields["member_status_snapshot"],
		memberIDs,
	)
	if err != nil {
		return envelope, blockedShipmentEnvelopeError(shipment.ID, err.Error())
	}

	envelope = blockedShipmentEnvelope{
		reason:              reason,
		blockedAt:           blockedAt,
		blockedBy:           blockedBy,
		branch:              branch,
		resumeCheckpointRef: resumeCheckpointRef,
		memberStatuses:      memberStatuses,
	}
	if !hasCorrelatedCommittedBlockedEvidence(envelope, itemEvents) {
		return blockedShipmentEnvelope{}, blockedShipmentEnvelopeError(
			shipment.ID,
			"no correlated committed block or normalize lifecycle evidence matches the blocked record",
		)
	}
	return envelope, nil
}

func normalizeBlockedEnvelopeOptionalString(value string) string {
	return strings.TrimSpace(value)
}

func blockedEnvelopeOptionalString(fields map[string]any, key string) (string, error) {
	raw, found := fields[key]
	if !found {
		return "", nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string when present", key)
	}
	return normalizeBlockedEnvelopeOptionalString(value), nil
}

func persistedBlockedEnvelopeOptionalString(fields map[string]any, key string) (string, error) {
	value, err := blockedEnvelopeOptionalString(fields, key)
	if err != nil {
		return "", err
	}
	raw, found := fields[key]
	if !found {
		return "", nil
	}
	if value == "" || raw.(string) != value {
		return "", fmt.Errorf("%s must be a canonical non-empty string when present", key)
	}
	return value, nil
}

func setBlockedEnvelopeOptionalString(fields map[string]any, key, value string) {
	value = normalizeBlockedEnvelopeOptionalString(value)
	if value == "" {
		delete(fields, key)
		return
	}
	fields[key] = value
}

func decodeBlockedMemberStatusSnapshot(raw any, memberIDs []string) (map[string]string, error) {
	statuses := make(map[string]string, len(memberIDs))
	switch snapshot := raw.(type) {
	case map[string]string:
		maps.Copy(statuses, snapshot)
	case map[string]any:
		for memberID, rawStatus := range snapshot {
			status, ok := rawStatus.(string)
			if !ok {
				return nil, fmt.Errorf("member_status_snapshot status for %s must be a string", memberID)
			}
			statuses[memberID] = status
		}
	default:
		return nil, fmt.Errorf("member_status_snapshot must be a status map")
	}
	if len(statuses) != len(memberIDs) {
		return nil, fmt.Errorf("member_status_snapshot must exactly cover the shipment manifest")
	}
	for _, memberID := range memberIDs {
		status, found := statuses[memberID]
		if !found {
			return nil, fmt.Errorf("member_status_snapshot is missing member %s", memberID)
		}
		if !isBlockedSnapshotStatus(models.ArtifactStatus(status)) {
			return nil, fmt.Errorf("member_status_snapshot has invalid status %q for member %s", status, memberID)
		}
	}
	return statuses, nil
}

func isBlockedSnapshotStatus(status models.ArtifactStatus) bool {
	switch status {
	case models.StatusQueued, models.StatusActive, models.StatusBlocked, models.StatusReview,
		models.StatusDone, models.StatusAccepted, models.StatusRejected, models.StatusArchived,
		models.StatusShipped, models.StatusAbandoned:
		return true
	default:
		return false
	}
}

func canonicalBlockedTimestamp(raw any) (string, bool) {
	var parsed time.Time
	switch value := raw.(type) {
	case string:
		var err error
		parsed, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return "", false
		}
	case time.Time:
		parsed = value
	default:
		return "", false
	}
	if parsed.IsZero() {
		return "", false
	}
	return parsed.UTC().Format(time.RFC3339), true
}

func hasCorrelatedCommittedBlockedEvidence(
	envelope blockedShipmentEnvelope,
	itemEvents []events.Event,
) bool {
	applied := make(map[string]int)
	for index, event := range itemEvents {
		correlationID, _ := event.Delta["correlation_id"].(string)
		operation, _ := event.Delta["operation"].(string)
		if correlationID == "" || (operation != "block" && operation != "normalize") {
			continue
		}
		target, targetErr := shipmentLifecycleEvidenceTarget(event.Delta)
		if targetErr != nil || target != string(ShipmentBlocked) {
			continue
		}
		if !blockedEvidenceMatchesEnvelope(event.Delta, envelope) {
			continue
		}
		key := correlationID + "\x00" + operation
		phase, _ := event.Delta["phase"].(string)
		switch {
		case event.EventType == "shipment_status_changed" && phase == "applied":
			applied[key] = index
		case event.EventType == "shipment_lifecycle" && phase == "committed":
			appliedIndex, found := applied[key]
			if found && appliedIndex < index {
				return true
			}
		}
	}
	return false
}

func blockedEvidenceMatchesEnvelope(delta map[string]any, envelope blockedShipmentEnvelope) bool {
	reason, _ := delta["reason"].(string)
	blockedBy, blockedByOK := optionalBlockedEvidenceString(delta, "blocked_by")
	blockedAt, blockedAtOK := canonicalBlockedTimestamp(delta["blocked_at"])
	branch, branchOK := optionalBlockedEvidenceString(delta, "branch")
	resumeCheckpointRef, resumeOK := optionalBlockedEvidenceString(delta, "resume_checkpoint_ref")
	memberStatuses, snapshotErr := decodeBlockedMemberStatusSnapshot(
		delta["member_status_snapshot"],
		sortedBlockedSnapshotMemberIDs(envelope.memberStatuses),
	)
	return reason == envelope.reason &&
		blockedByOK && blockedBy == envelope.blockedBy &&
		blockedAtOK && blockedAt == envelope.blockedAt &&
		branchOK && branch == envelope.branch &&
		resumeOK && resumeCheckpointRef == envelope.resumeCheckpointRef &&
		snapshotErr == nil && maps.Equal(memberStatuses, envelope.memberStatuses)
}

func optionalBlockedEvidenceString(delta map[string]any, key string) (string, bool) {
	raw, found := delta[key]
	if !found {
		return "", true
	}
	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	canonical := normalizeBlockedEnvelopeOptionalString(value)
	return canonical, canonical != "" && canonical == value
}

func sortedBlockedSnapshotMemberIDs(statuses map[string]string) []string {
	ids := make([]string, 0, len(statuses))
	for id := range statuses {
		ids = append(ids, id)
	}
	return ids
}

func blockedShipmentEnvelopeError(shipmentID, detail string) error {
	return fmt.Errorf(
		"shipment %s blocked envelope is not canonical: %s: %w",
		shipmentID,
		detail,
		blerrors.ErrShipmentBlockedRequiresEnvelope,
	)
}
