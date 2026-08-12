package core

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
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

// TestComputeManifestDigest_CoveringFeatureLookupErrorFailsClosed verifies
// that a covering-feature resolution failure (a genuine DB error, not a
// legitimate "no covering feature" outcome) refuses digest computation
// entirely rather than silently signing an empty covering_feature. The
// presentation-oriented DeriveCoveringFeature is deliberately best-effort
// (a transient DB blip must never break rendering a shipment view), but
// using that same best-effort behavior for a SECURITY BINDING would let an
// indeterminate resolution silently degrade to "no covering feature" in the
// signed proof (106-F F1 review finding).
func TestComputeManifestDigest_CoveringFeatureLookupErrorFailsClosed(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()
	shipment := &models.Artifact{CustomFields: map[string]any{"items": []string{"200-F"}}}

	require.NoError(t, ws.DB.Close()) // force every DB lookup to fail with a genuine (non-ErrNotFound) error

	_, err := computeManifestDigest(ctx, ws, shipment, "deadbeef")
	require.Error(t, err, "a covering-feature lookup failure must fail manifest digest computation closed, not silently sign an empty covering_feature")
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

// TestGateShipmentCompletion_ManifestChangedSinceSnapshot_Refused verifies the
// F3 TOCTOU guard: gateShipmentCompletion re-checks the shipment's CURRENT
// manifest membership against originalManifestItems (the snapshot ShipShipment
// captured before deriving releaseScope and before this function ran)
// immediately before signing the manifest-binding proof. If a concurrent,
// unlocked membership mutation (e.g. backlogit_add_to_shipment) landed after
// that snapshot but before this reload, the mismatch must refuse the whole
// completion rather than sign a proof attesting to members whose gate
// evidence was never validated (106-F F1 review finding F3).
func TestGateShipmentCompletion_ManifestChangedSinceSnapshot_Refused(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", validFormalTestKey)
	t.Setenv("BACKLOGIT_FORMAL_GATE_REQUIRED", "true")
	ws := newGateTestWorkspace(t)
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}
	runner := &taskAwareRunner{
		taskRes:     gate.GateResult{ExitCode: 0, Stdout: []byte(validFormalTestReport)},
		shipmentRes: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)},
	}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	// staleOriginalManifest simulates the snapshot ShipShipment would have
	// captured BEFORE a concurrent AddItemToShipment call changed the
	// shipment's actual stored membership to what newGatedShipment set up
	// (just taskID) -- i.e. the reverse of the real attack (a member was
	// ADDED after the snapshot), but the important thing this proves is that
	// ANY mismatch between the snapshot and the fresh reload refuses,
	// regardless of which direction the manifest drifted.
	staleOriginalManifest := []string{"106.999-T-never-validated"}
	releaseScope := []string{taskID}

	_, gerr := gateShipmentCompletion(ctx, ws, shipmentID, releaseScope, staleOriginalManifest)
	require.Error(t, gerr, "must refuse when the shipment's current manifest differs from the pre-call snapshot")
	require.True(t, stderrors.Is(gerr, bkerrors.ErrFormalGateRequired), "err = %v, want ErrFormalGateRequired", gerr)
	assert.Contains(t, gerr.Error(), "manifest membership changed")

	sh, gErr := GetShipment(ctx, ws, shipmentID)
	require.NoError(t, gErr)
	assert.Equal(t, models.StatusActive, sh.Status, "shipment must not ship on a manifest-drift refusal")
}

// TestGateShipmentCompletion_ManifestUnchangedSincesnapshot_Proceeds is the
// positive counterpart: when originalManifestItems exactly matches the
// current manifest, the TOCTOU guard does not interfere with a normal
// completion.
func TestGateShipmentCompletion_ManifestUnchangedSinceSnapshot_Proceeds(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", validFormalTestKey)
	t.Setenv("BACKLOGIT_FORMAL_GATE_REQUIRED", "true")
	ws := newGateTestWorkspace(t)
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}
	runner := &taskAwareRunner{
		taskRes:     gate.GateResult{ExitCode: 0, Stdout: []byte(validFormalTestReport)},
		shipmentRes: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)},
	}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	releaseScope := []string{taskID}
	unchangedOriginal := []string{taskID} // matches newGatedShipment's actual stored items

	_, gerr := gateShipmentCompletion(ctx, ws, shipmentID, releaseScope, unchangedOriginal)
	require.NoError(t, gerr, "an unchanged manifest must not be refused by the TOCTOU guard")
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
	unlock, err := ws.augmentShipmentDeltaWithFormalProof(ctx, shipment, "999-S", "deadbeef", "refs/heads/main", delta)
	require.NoError(t, err)
	defer unlock()

	// Simulate the manifest changing after signing: an additional member appears.
	shipment.CustomFields["items"] = []string{"106.001-T", "106.002-T"}

	verifyErr := ws.verifyShipmentManifestBinding(ctx, shipment, "999-S", "deadbeef", "refs/heads/main", delta)
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

	verifyErr := ws.verifyShipmentManifestBinding(ctx, shipment, "999-S", "deadbeef", "refs/heads/main", delta)
	require.Error(t, verifyErr)
	require.True(t, stderrors.Is(verifyErr, bkerrors.ErrProofUnverifiable), "err = %v, want ErrProofUnverifiable", verifyErr)
}
