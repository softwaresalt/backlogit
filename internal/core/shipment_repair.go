package core

import (
	"context"
	"fmt"

	"github.com/softwaresalt/backlogit/internal/models"
)

// RepairShipmentMemberEvidence appends a forced gate pass event for a shipment
// member whose recorded gate evidence head_sha is stale (dangling or divergent
// from the current workspace HEAD) but whose implementation is verified present
// in the shipped scope. The repair records a pre_task_completion_gate_forced
// event with the current workspace HEAD so subsequent backlogit_ship_shipment
// ancestry checks pass.
//
// Prerequisites:
//   - shipmentID must identify an active shipment.
//   - memberID must be present in the shipment manifest and be a task or subtask
//     (features are not gated and do not carry per-member gate evidence).
//   - The member must be in a terminal release status (done or archived).
//   - The workspace must be inside a git repository with a resolvable HEAD.
//   - reason must be non-empty (operator-provided justification is required).
//
// The repair is audited: the appended EventGateForced event records the force
// reason, repair: true, and the resolved HEAD sha so the audit trail is
// self-explanatory without requiring cross-log lookups.
func RepairShipmentMemberEvidence(ctx context.Context, ws *Workspace, shipmentID, memberID, reason string) error {
	if reason == "" {
		return fmt.Errorf("repair member evidence: reason is required (operator justification must be non-empty)")
	}

	// Validate shipment state.
	shipment, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		return fmt.Errorf("repair member evidence: load shipment %s: %w", shipmentID, err)
	}
	if shipment.Status != models.StatusActive {
		return fmt.Errorf("repair member evidence: shipment %s is not active (status: %s)",
			shipmentID, shipment.Status)
	}

	// Validate member is in the manifest.
	manifestItems := NormalizeShipmentItems(shipment)
	var found bool
	for _, id := range manifestItems {
		if id == memberID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("repair member evidence: member %s is not in shipment %s manifest",
			memberID, shipmentID)
	}

	// Validate member is terminal.
	member, err := loadArtifact(ctx, ws, memberID)
	if err != nil {
		return fmt.Errorf("repair member evidence: load member %s: %w", memberID, err)
	}
	if member.ArtifactType != "task" && member.ArtifactType != "subtask" {
		return fmt.Errorf("repair member evidence: member %s has type %q; "+
			"only task and subtask members carry gate evidence (features are skipped by the gate)",
			memberID, member.ArtifactType)
	}
	if !isTerminalReleaseStatus(member.Status) {
		return fmt.Errorf("repair member evidence: member %s is not in a terminal status (status: %s); "+
			"only done or archived members are eligible for evidence repair",
			memberID, member.Status)
	}

	// Resolve the current workspace HEAD (bounded to prevent workspace-lock contention).
	headSHA, headErr := ws.headSHABounded(ctx)
	if headErr != nil {
		return fmt.Errorf("repair member evidence: resolve workspace HEAD: %w", headErr)
	}
	if headSHA == "" {
		return fmt.Errorf("repair member evidence: workspace HEAD is unresolvable " +
			"(workspace must be inside a git repository with a committed HEAD)")
	}

	// Append the audited forced gate event. This is the same event type the
	// existing move --force-gates path produces; gateevidence.Latest accepts it as
	// qualifying evidence so validateMemberGateEvidence passes on the next
	// ship_shipment call. The repair: true marker distinguishes this forced event
	// from a live-run force so the audit log is self-explanatory.
	if err := ws.appendGateEvent(ctx, memberID, EventGateForced, map[string]any{
		"head_sha":     headSHA,
		"ran":          false,
		"outcome":      "forced",
		"force_reason": reason,
		"repair":       true,
	}); err != nil {
		return fmt.Errorf("repair member evidence: append gate event for %s: %w", memberID, err)
	}
	return nil
}
