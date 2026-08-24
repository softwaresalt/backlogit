package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/gateevidence"
)

// TestRepairShipmentMemberEvidence_AppendsForcedEvent verifies the happy path:
// RepairShipmentMemberEvidence appends a pre_task_completion_gate_forced event
// with the current HEAD sha for the specified member, such that
// validateMemberGateEvidence subsequently accepts the member.
func TestRepairShipmentMemberEvidence_AppendsForcedEvent(t *testing.T) {
	ws := newGateTestWorkspace(t)
	// Use a real git repo so headSHABounded resolves a non-empty HEAD sha.
	_, head, _ := initGitRepoWithCommits(t, ws.RootPath)
	ctx := context.Background()

	// Create a feature+task, add both to a claimed shipment.
	feat, err := CreateArtifact(ctx, ws, "repair feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "repair task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "repair shipment", []string{task.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	// Mark task done (ungated, so no evidence exists yet) and archive it.
	_, err = updateArtifactUngated(ctx, ws, task.ID, map[string]any{"status": "done"})
	require.NoError(t, err)

	// Add a stale (divergent) EventGatePassed event to simulate the real scenario.
	staleHead := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" // non-existent sha
	require.NoError(t, appendItemEventErr(ctx, ws, task.ID, EventGatePassed, map[string]any{
		"outcome":  "passed",
		"ran":      true,
		"head_sha": staleHead,
	}))

	// Repair the stale evidence.
	require.NoError(t, RepairShipmentMemberEvidence(ctx, ws, shipment.ID, task.ID, "test repair reason"))

	// Verify a forced event was appended with the current HEAD sha.
	logsRoot := WorkspaceLogsRoot(ws.RootPath)
	evs, err := events.ReadAllEvents(ctx, logsRoot, task.ID)
	require.NoError(t, err)

	evidence := gateevidence.Latest(evs)
	require.NotNil(t, evidence.Event, "a forced gate event must be present after repair")
	assert.Equal(t, EventGateForced, evidence.Event.EventType,
		"repair must append a %s event", EventGateForced)
	h, _ := evidence.Event.Delta["head_sha"].(string)
	assert.Equal(t, head, h, "repaired event must carry the current HEAD sha")
	reason, _ := evidence.Event.Delta["force_reason"].(string)
	assert.Equal(t, "test repair reason", reason)

	// Verify validateMemberGateEvidence now accepts the member with the current HEAD.
	require.NoError(t, validateMemberGateEvidence(ctx, ws, []string{task.ID}, head),
		"member must pass evidence validation after repair")
}

// TestRepairShipmentMemberEvidence_EmptyReason_Errors verifies that an empty
// reason is rejected (operator justification is required for audited repairs).
func TestRepairShipmentMemberEvidence_EmptyReason_Errors(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()
	err := RepairShipmentMemberEvidence(ctx, ws, "001-S", "001.001-T", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason")
}

// TestRepairShipmentMemberEvidence_ShipmentNotActive_Errors verifies that
// RepairShipmentMemberEvidence returns an error when the shipment is queued
// (not active).
func TestRepairShipmentMemberEvidence_ShipmentNotActive_Errors(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "repair feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "repair task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "repair shipment", []string{task.ID})
	require.NoError(t, err)
	// Shipment stays queued (not claimed).

	err = RepairShipmentMemberEvidence(ctx, ws, shipment.ID, task.ID, "some reason")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not active")
}

// TestRepairShipmentMemberEvidence_MemberNotInManifest_Errors verifies that
// RepairShipmentMemberEvidence returns an error when the member is not present
// in the shipment manifest.
func TestRepairShipmentMemberEvidence_MemberNotInManifest_Errors(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "repair feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "repair task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	otherTask, err := CreateArtifact(ctx, ws, "other task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	// Shipment contains only task, not otherTask.
	shipment, err := CreateShipment(ctx, ws, "repair shipment", []string{task.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	err = RepairShipmentMemberEvidence(ctx, ws, shipment.ID, otherTask.ID, "some reason")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in shipment")
}

// TestRepairShipmentMemberEvidence_MemberNotTerminal_Errors verifies that
// RepairShipmentMemberEvidence returns an error when the member is still active
// (not yet in a terminal status).
func TestRepairShipmentMemberEvidence_MemberNotTerminal_Errors(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "repair feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "repair task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "repair shipment", []string{task.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	// Task remains active (claimed), not done.

	err = RepairShipmentMemberEvidence(ctx, ws, shipment.ID, task.ID, "some reason")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminal")
}

// TestRepairShipmentMemberEvidence_NoRepoHead_Errors verifies that
// RepairShipmentMemberEvidence returns an error when the workspace is not inside
// a git repository with a resolvable HEAD (no commit yet).
func TestRepairShipmentMemberEvidence_NoRepoHead_Errors(t *testing.T) {
	ws := newGateTestWorkspace(t)
	// No git repo initialised -> headSHABounded returns "".
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "repair feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "repair task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "repair shipment", []string{task.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	_, err = updateArtifactUngated(ctx, ws, task.ID, map[string]any{"status": "done"})
	require.NoError(t, err)

	err = RepairShipmentMemberEvidence(ctx, ws, shipment.ID, task.ID, "some reason")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HEAD")
}
