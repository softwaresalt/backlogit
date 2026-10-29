package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/events"
)

// persistedSignedReconcileEvent prepares a real reconcile event over the 048-S
// fixture carrying a signed envelope and returns the persisted event pieces a
// later offline auditor would read back from the log.
func persistedSignedReconcileEvent(t *testing.T, signer attestationTestSigner) (events.Event, ShipmentReconciledShippedDelta) {
	t.Helper()
	fixture := setupShipmentReconcileEvidenceFixture(t)
	fixture.ws.Config.Reconcile = &config.ReconcileConfig{}
	at := time.Now().UTC()

	var expected AttestationExpectedEvidence
	probe := fixture.request("048-S", fixture.merge048)
	probe.AttestationVerifier = func(e AttestationExpectedEvidence) error { expected = e; return errCaptureExpected }
	_, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, probe, fixedShipmentReconcileEvidenceInput())
	require.ErrorIs(t, err, errCaptureExpected)

	envelope := signer.signEnvelope(t, signer.id, attestationTestStatement(expected, at))
	req := fixture.request("048-S", fixture.merge048)
	req.Attestation = attestationTestClaim(t, envelope, signer.id)
	req.Attestation.Envelope = string(envelope)
	req.AttestationVerifier = attestationIntegrationVerifier(t, signer, envelope, at)
	prepared, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, req, fixedShipmentReconcileEvidenceInput())
	require.NoError(t, err)

	var wrapper events.Event
	require.NoError(t, json.Unmarshal(prepared.EventBytes, &wrapper))
	delta, err := decodeShipmentReconciledShippedDelta(wrapper.Delta)
	require.NoError(t, err)
	require.NotNil(t, delta.Attestation)
	return wrapper, delta
}

func TestAttestationReVerifyGuard_OfflineReverify(t *testing.T) {
	signer := newAttestationTestSigner(t, "ci-signer")
	wrapper, delta := persistedSignedReconcileEvent(t, signer)

	rv := &AttestationReverifier{Anchors: []config.TrustAnchor{signer.anchor}, Pins: signer.pins}
	got, err := rv.Reverify(context.Background(), AttestationReverifyInput{
		ShipmentID: "048-S", At: wrapper.Timestamp, Attestation: delta.Attestation, Evidence: delta.Evidence,
	})
	require.NoError(t, err)
	assert.Equal(t, signer.id, got.KeyID)
	assert.Equal(t, delta.Attestation.StatementDigest, got.StatementDigest)
}

func TestAttestationReVerifyGuard_TamperSwapFailClosed(t *testing.T) {
	signer := newAttestationTestSigner(t, "ci-signer")
	wrapper, delta := persistedSignedReconcileEvent(t, signer)
	rv := &AttestationReverifier{Anchors: []config.TrustAnchor{signer.anchor}, Pins: signer.pins}
	in := AttestationReverifyInput{ShipmentID: "048-S", At: wrapper.Timestamp, Attestation: delta.Attestation, Evidence: delta.Evidence}

	tampered := *delta.Attestation
	tampered.Envelope = strings.Replace(tampered.Envelope, `"sig":"`, `"sig":"AAAA`, 1)
	tamperedIn := in
	tamperedIn.Attestation = &tampered
	_, err := rv.Reverify(context.Background(), tamperedIn)
	require.Error(t, err, "a byte-tampered persisted envelope must fail closed")

	swappedIn := in
	swappedIn.ShipmentID = "049-S"
	_, err = rv.Reverify(context.Background(), swappedIn)
	require.Error(t, err, "an envelope swapped onto another shipment must fail closed")
	assert.ErrorIs(t, err, ErrAttestationWrongShipment)
}
