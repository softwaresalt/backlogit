package core

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core/gate"
	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestComputeManifestDigest_ReorderedMembersChangesDigest verifies member
// order is semantic: two shipments with the same members in a different
// order produce different digests (106-F F1/U7).
func TestComputeManifestDigest_ReorderedMembersChangesDigest(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()

	a := &models.Artifact{CustomFields: map[string]any{"items": []string{"106.001-T", "106.002-T"}}}
	b := &models.Artifact{CustomFields: map[string]any{"items": []string{"106.002-T", "106.001-T"}}}

	digestA, err := computeManifestDigest(ctx, ws, a, "deadbeef")
	require.NoError(t, err)
	digestB, err := computeManifestDigest(ctx, ws, b, "deadbeef")
	require.NoError(t, err)
	require.NotEqual(t, digestA, digestB, "reordering manifest members must change the digest")
}

// TestComputeManifestDigest_DroppedMemberChangesDigest verifies removing a
// member changes the digest.
func TestComputeManifestDigest_DroppedMemberChangesDigest(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()

	full := &models.Artifact{CustomFields: map[string]any{"items": []string{"106.001-T", "106.002-T", "106.003-T"}}}
	dropped := &models.Artifact{CustomFields: map[string]any{"items": []string{"106.001-T", "106.002-T"}}}

	digestFull, err := computeManifestDigest(ctx, ws, full, "deadbeef")
	require.NoError(t, err)
	digestDropped, err := computeManifestDigest(ctx, ws, dropped, "deadbeef")
	require.NoError(t, err)
	require.NotEqual(t, digestFull, digestDropped, "dropping a manifest member must change the digest")
}

// TestComputeManifestDigest_CoveringFeatureSwapChangesDigest verifies that
// swapping the covering feature (the first dotless-root feature ID in the
// manifest) changes the digest even when the flat item list is otherwise
// identical in count.
func TestComputeManifestDigest_CoveringFeatureSwapChangesDigest(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()

	withFeatureA := &models.Artifact{CustomFields: map[string]any{"items": []string{"200-F", "200.001-T"}}}
	withFeatureB := &models.Artifact{CustomFields: map[string]any{"items": []string{"201-F", "200.001-T"}}}

	digestA, err := computeManifestDigest(ctx, ws, withFeatureA, "deadbeef")
	require.NoError(t, err)
	digestB, err := computeManifestDigest(ctx, ws, withFeatureB, "deadbeef")
	require.NoError(t, err)
	require.NotEqual(t, digestA, digestB, "swapping the covering feature ID must change the digest")
}

// TestComputeManifestDigest_UnchangedManifestSameDigest verifies computing the
// digest twice from the same logical state produces the same value
// (deterministic, not merely "different when different").
func TestComputeManifestDigest_UnchangedManifestSameDigest(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()
	shipment := &models.Artifact{CustomFields: map[string]any{"items": []string{"106.001-T", "106.002-T"}}}

	d1, err := computeManifestDigest(ctx, ws, shipment, "deadbeef")
	require.NoError(t, err)
	d2, err := computeManifestDigest(ctx, ws, shipment, "deadbeef")
	require.NoError(t, err)
	require.Equal(t, d1, d2, "an unchanged manifest must digest identically every time")
}

// TestComputeManifestDigest_ShipmentHeadChangeChangesDigest verifies the
// resolved shipment head is bound into the digest too.
func TestComputeManifestDigest_ShipmentHeadChangeChangesDigest(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()
	shipment := &models.Artifact{CustomFields: map[string]any{"items": []string{"106.001-T"}}}

	d1, err := computeManifestDigest(ctx, ws, shipment, "aaaaaaaa")
	require.NoError(t, err)
	d2, err := computeManifestDigest(ctx, ws, shipment, "bbbbbbbb")
	require.NoError(t, err)
	require.NotEqual(t, d1, d2, "a different resolved shipment head must change the digest")
}

// TestShipmentGate_ManifestBinding_HappyPathAdmitted verifies a full
// ShipShipment flow succeeds and records manifest-bound shipment evidence
// when formal admission is enforced and the manifest is unchanged between
// signing and the self-consistency verification.
func TestShipmentGate_ManifestBinding_HappyPathAdmitted(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	ws := newGateTestWorkspace(t)
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}
	report := `{"reviewers":[{"persona":"Constitution Reviewer","decision":"pass"}]}`
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(report)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	result, err := ShipShipment(ctx, ws, shipmentID, nil)
	require.NoError(t, err, "ship must succeed with an unchanged manifest under formal enforcement")
	require.NotNil(t, result)

	ev := findEvent(eventsFor(t, ws, shipmentID), EventGatePassed)
	require.NotNil(t, ev)
	digest, _ := ev.Delta["manifest_digest"].(string)
	require.NotEmpty(t, digest, "shipment-level evidence must carry a manifest_digest")
}

// TestVerifyShipmentManifestBinding_TamperedDigestRefused verifies that a
// recorded manifest_digest which no longer matches a fresh recomputation
// (simulating the manifest changing between signing and this check) is
// refused with ErrProofInvalid, not silently skipped.
func TestVerifyShipmentManifestBinding_TamperedDigestRefused(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	ws := newGateTestWorkspace(t)
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}
	ctx := context.Background()

	shipment := &models.Artifact{ID: "999-S", CustomFields: map[string]any{"items": []string{"106.001-T"}}}
	delta := map[string]any{"ran": true}
	err := ws.augmentShipmentDeltaWithFormalProof(ctx, shipment, "999-S", "deadbeef", delta)
	require.NoError(t, err)

	// Simulate the manifest changing after signing: an additional member appears.
	shipment.CustomFields["items"] = []string{"106.001-T", "106.002-T"}

	verifyErr := ws.verifyShipmentManifestBinding(ctx, shipment, "999-S", "deadbeef", delta)
	require.Error(t, verifyErr, "a changed manifest must be refused, not silently skipped")
	require.True(t, stderrors.Is(verifyErr, bkerrors.ErrProofInvalid), "err = %v, want ErrProofInvalid", verifyErr)
}

// TestVerifyShipmentManifestBinding_MissingProofUnverifiable verifies that a
// delta with no proof at all is refused as unverifiable rather than skipped.
func TestVerifyShipmentManifestBinding_MissingProofUnverifiable(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	ws := newGateTestWorkspace(t)
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}
	ctx := context.Background()

	shipment := &models.Artifact{ID: "999-S", CustomFields: map[string]any{"items": []string{"106.001-T"}}}
	delta := map[string]any{"ran": true}

	verifyErr := ws.verifyShipmentManifestBinding(ctx, shipment, "999-S", "deadbeef", delta)
	require.Error(t, verifyErr)
	require.True(t, stderrors.Is(verifyErr, bkerrors.ErrProofUnverifiable), "err = %v, want ErrProofUnverifiable", verifyErr)
}
