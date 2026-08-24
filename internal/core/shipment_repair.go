package core

import (
	"context"
	"fmt"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/gateevidence"
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
//   - The workspace must NOT have formal gate enforcement enabled
//     (EventGateForced is not admissible under FormalAdmit).
//   - The member MUST have existing gate evidence whose head_sha is NOT an
//     ancestor of the current HEAD (the narrow stale-evidence scenario this
//     command addresses). Members with no evidence or already-current evidence
//     are rejected.
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

	// Reject under formal gate enforcement: EventGateForced is not admissible
	// by FormalAdmit (internal/gateevidence/formal.go), so the repair would
	// succeed locally but fail again on the next ship_shipment call.
	if ws.formalGateEnforced() {
		return fmt.Errorf("repair member evidence: formal gate enforcement is active; " +
			"EventGateForced is not admissible under FormalAdmit — a separate " +
			"authenticated formal-repair contract is required")
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

	// Validate member type and status.
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

	// Resolve the current workspace HEAD (bounded to prevent workspace-lock
	// contention). This also confirms we are inside a real git repository —
	// a prerequisite for the ancestry check below.
	headSHA, headErr := ws.headSHABounded(ctx)
	if headErr != nil {
		return fmt.Errorf("repair member evidence: resolve workspace HEAD: %w", headErr)
	}
	if headSHA == "" {
		return fmt.Errorf("repair member evidence: workspace HEAD is unresolvable " +
			"(workspace must be inside a git repository with a committed HEAD)")
	}

	// Verify the member has existing stale evidence. The repair is narrowly
	// scoped to members whose prior gate evidence IS present but whose recorded
	// head_sha is NOT an ancestor of the current HEAD (dangling or divergent).
	// Members with no evidence at all must go through the normal gate path.
	// Members whose evidence is already current do not need a repair event.
	logsRoot := WorkspaceLogsRoot(ws.RootPath)
	evs, rerr := events.ReadAllEvents(ctx, logsRoot, memberID)
	if rerr != nil {
		return fmt.Errorf("repair member evidence: read events for %s: %w", memberID, rerr)
	}
	latest := gateevidence.Latest(evs).Event
	if latest == nil {
		return fmt.Errorf("repair member evidence: member %s has no prior gate evidence; "+
			"repair only applies to members with existing but stale evidence "+
			"(use move --force-gates to complete a never-gated member)",
			memberID)
	}
	existingHead, _ := latest.Delta["head_sha"].(string)
	if existingHead == "" {
		return fmt.Errorf("repair member evidence: member %s gate evidence has no recorded head_sha; "+
			"only evidence with a non-empty head_sha is eligible for ancestry-based repair",
			memberID)
	}
	// Check whether the existing evidence is already an ancestor of HEAD. If it
	// is, the evidence is current and no repair is needed.
	if isGitObjectName(existingHead) {
		alreadyCurrent, aerr := ws.isAncestor(ctx, existingHead, headSHA)
		if aerr == nil && alreadyCurrent {
			return fmt.Errorf("repair member evidence: member %s evidence head_sha %s is already "+
				"an ancestor of (or equal to) the current HEAD %s; no repair is needed",
				memberID, existingHead, headSHA)
		}
		// aerr != nil means the lineage check failed (e.g. dangling object),
		// which is precisely the stale-evidence scenario we are here to repair.
	}

	// Append the audited forced gate event. This is the same event type the
	// existing move --force-gates path produces; gateevidence.Latest accepts it as
	// qualifying evidence (when formal enforcement is not active) so
	// validateMemberGateEvidence passes on the next ship_shipment call. The
	// repair: true marker distinguishes this from a live-run force so the audit
	// log is self-explanatory.
	if err := ws.appendGateEvent(ctx, memberID, EventGateForced, map[string]any{
		"head_sha":            headSHA,
		"ran":                 false,
		"outcome":             "forced",
		"force_reason":        reason,
		"repair":              true,
		"stale_evidence_head": existingHead,
	}); err != nil {
		return fmt.Errorf("repair member evidence: append gate event for %s: %w", memberID, err)
	}
	return nil
}
