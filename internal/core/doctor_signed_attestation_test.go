package core

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/events"
)

type staticPinProvider struct{ pins *ExternalPinSet }

func (p staticPinProvider) ExternalPinSet(context.Context) (*ExternalPinSet, error) {
	return p.pins, nil
}

// reconcileEventTime is the timestamp marshalReconciledShippedEvent stamps.
var reconcileEventTime = time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)

func expectedFromDelta(shipmentID string, d ShipmentReconciledShippedDelta) AttestationExpectedEvidence {
	return AttestationExpectedEvidence{
		ShipmentID: shipmentID, MergeSHA: d.Evidence.MergeSHA,
		ManifestDigest: d.Evidence.ManifestDigest, ClosureContentHash: d.Evidence.ClosureContentHash,
	}
}

// signedAttestationFor signs the delta evidence and returns the claim with the
// durable envelope attached.
func signedAttestationFor(t *testing.T, s attestationTestSigner, shipmentID string, d ShipmentReconciledShippedDelta) *ShipmentReconciledShippedAttestation {
	t.Helper()
	env := s.signEnvelope(t, s.id, attestationTestStatement(expectedFromDelta(shipmentID, d), reconcileEventTime))
	claim := attestationTestClaim(t, env, s.id)
	claim.Envelope = string(env)
	return claim
}

// seedReconcileEventShipment archives a shipped shipment with no shipped event
// and writes one reconcile event carrying the attestation built by attFor.
func seedReconcileEventShipment(t *testing.T, ws *Workspace, attFor func(id string, d ShipmentReconciledShippedDelta) *ShipmentReconciledShippedAttestation) string {
	t.Helper()
	shipment := seedArchivedShippedNoEvent(t, ws, "signed reconcile", nil)
	delta := validReconciledShippedDelta()
	if attFor != nil {
		delta.Attestation = attFor(shipment.ID, delta)
	}
	require.NoError(t, os.MkdirAll(WorkspaceLogsRoot(ws.RootPath), 0o755))
	logPath := events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), shipment.ID)
	require.NoError(t, os.WriteFile(logPath, marshalReconciledShippedEvent(t, delta, shipment.ID), 0o644))
	return shipment.ID
}

func doctorShippedAudit(t *testing.T, ws *Workspace, pins *ExternalPinSet) *DoctorReport {
	t.Helper()
	opts := &DoctorOptions{CheckShippedEventCompleteness: true}
	if pins != nil {
		opts.AttestationPins = staticPinProvider{pins: pins}
	}
	report, err := Doctor(context.Background(), ws, opts)
	require.NoError(t, err)
	return report
}

func TestDoctorSignedAttestationReverify(t *testing.T) {
	signer := newAttestationTestSigner(t, "ci-signer")

	t.Run("persisted signed envelope re-verifies offline against the external root", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ws.Config.TrustAnchors = []config.TrustAnchor{signer.anchor}
		seedReconcileEventShipment(t, ws, func(id string, d ShipmentReconciledShippedDelta) *ShipmentReconciledShippedAttestation {
			return signedAttestationFor(t, signer, id, d)
		})
		assert.Empty(t, findingsOfType(doctorShippedAudit(t, ws, signer.pins), FindingMissingShippedEvent))
	})

	t.Run("unsigned metadata only event is rejected fail closed", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ws.Config.TrustAnchors = []config.TrustAnchor{signer.anchor}
		seedReconcileEventShipment(t, ws, func(string, ShipmentReconciledShippedDelta) *ShipmentReconciledShippedAttestation {
			return reconcileTestAttestation()
		})
		assert.Len(t, findingsOfType(doctorShippedAudit(t, ws, signer.pins), FindingMissingShippedEvent), 1)
	})

	t.Run("signed event without an external pin set fails closed", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ws.Config.TrustAnchors = []config.TrustAnchor{signer.anchor}
		seedReconcileEventShipment(t, ws, func(id string, d ShipmentReconciledShippedDelta) *ShipmentReconciledShippedAttestation {
			return signedAttestationFor(t, signer, id, d)
		})
		assert.Len(t, findingsOfType(doctorShippedAudit(t, ws, nil), FindingMissingShippedEvent), 1)
		assert.Len(t, findingsOfType(doctorShippedAudit(t, ws, NewExternalPinSet()), FindingMissingShippedEvent), 1)
	})

	t.Run("reverifier verifies a persisted envelope and rejects a missing one", func(t *testing.T) {
		d := validReconciledShippedDelta()
		rv := &AttestationReverifier{Anchors: []config.TrustAnchor{signer.anchor}, Pins: signer.pins}
		in := AttestationReverifyInput{ShipmentID: "048-S", At: reconcileEventTime, Attestation: signedAttestationFor(t, signer, "048-S", d), Evidence: d.Evidence}
		got, err := rv.Reverify(context.Background(), in)
		require.NoError(t, err)
		assert.Equal(t, signer.id, got.KeyID)

		in.Attestation = reconcileTestAttestation()
		_, err = rv.Reverify(context.Background(), in)
		require.ErrorIs(t, err, ErrAttestationEnvelopeMissing)
	})
}
